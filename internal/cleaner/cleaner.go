// Package cleaner removes items previously detected by scanners.
//
// Design:
//   - A Remover knows how to delete one Item (path-based, docker-prune, etc.).
//   - A Plan groups items + their resolved Remover.
//   - Mode controls confirmation flow: DryRun, Interactive, Yes.
//   - Prompter abstracts the user interaction so we can test it.
//
// Safety rules enforced here:
//   - Never delete a path outside a known-safe prefix (home dir, or the
//     OS temp directory).
//   - DryRun never touches the filesystem.
//   - Docker volumes are NEVER pruned (only `system prune -f`, no --volumes).
//   - Empty/unknown paths are skipped, not errored, to avoid panics on malformed Items.
//
// Platform split: the concrete removers for the Trash/Recycle Bin
// (TrashContentsRemover vs RecycleBinRemover), Time Machine snapshots,
// and Xcode simulators live in cleaner_darwin.go / cleaner_windows.go —
// their mechanics (tmutil, xcrun, SHEmptyRecycleBinW) have zero overlap
// across OSes. Everything else in this file (Plan, PathRemover,
// DockerPruneRemover, OldFilesRemover, the SafeRoots/OffLimits guard
// rails) is identical on every platform.
package cleaner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// Mode controls how the cleaner asks for confirmation.
type Mode int

const (
	// DryRun reports what would be removed but never touches disk.
	DryRun Mode = iota
	// Interactive asks the user before each item.
	Interactive
	// Yes assumes confirmation for every item (CI / scripting).
	Yes
)

// Decision is what the user (or auto mode) chose for an item.
type Decision int

const (
	DecisionYes Decision = iota
	DecisionNo
	DecisionView
	DecisionQuit
)

// Prompter renders an item and reads the user's decision.
// It is an interface so tests can inject a deterministic answer stream.
type Prompter interface {
	Ask(it item.Item) Decision
	Show(msg string)
}

// Remover deletes the resource an Item points to.
// Implementations must be idempotent and safe to call after a missing target.
type Remover interface {
	// Describe returns a one-line human description used in dry-run output.
	Describe(it item.Item) string
	// Remove performs the actual deletion. It must not call os.Exit.
	Remove(it item.Item) error
}

// Previewer is an optional capability a Remover can implement to
// support the "v" (view) prompt: a multi-line preview of what would be
// removed, shown before the user commits to Yes/No.
//
// Kept as a separate optional interface (rather than a type switch in
// viewItem over concrete Remover types) so platform-specific removers
// — TrashContentsRemover on macOS, RecycleBinRemover on Windows — can
// each provide their own preview without this file needing to know
// their concrete type. That, in turn, is what lets this file compile
// unchanged on every OS: it never names a type that only exists in
// cleaner_darwin.go or cleaner_windows.go.
type Previewer interface {
	Preview(it item.Item) string
}

// Resolver picks a Remover for an item. The default resolver maps:
//   - Docker reclaimable → DockerPruneRemover
//   - Trash/Recycle Bin → the platform's trash remover (see resolveTrash)
//   - Crash reports / temp files → OldFilesRemover (age-filtered)
//   - Time Machine snapshots, Xcode simulators → platform-only removers
//     (see resolveDevAdvanced; these Tool values never appear in a
//     Windows-built binary's inventory, so falling through harmlessly
//     is fine there)
//   - iMessage attachments → OldFilesRemover (recursive, age-filtered)
//   - everything else with a non-empty Path → PathRemover
type Resolver func(it item.Item) (Remover, error)

// DefaultResolver is the standard mapping used by `mistah clean`.
func DefaultResolver(it item.Item) (Remover, error) {
	if it.Tool == "docker" && it.Path == "" {
		return DockerPruneRemover{}, nil
	}
	if r, ok := resolveTrash(it); ok {
		return r, nil
	}
	if it.Tool == "crash-reports" && it.Path != "" {
		// 30-day cutoff matches scanCrashReports' detection window.
		// Defined here, not in the Item, so the cleaner stays the
		// single source of truth for what gets deleted.
		return OldFilesRemover{
			MaxAgeDays: 30,
			Extensions: []string{".crash", ".diag", ".ips", ".spin", ".hang"},
		}, nil
	}
	if it.Tool == "temp" && it.Path != "" {
		// Mirrors the crash-reports cutoff shape but for Windows'
		// %TEMP% detector (system_windows.go's scanTempFiles): age-only
		// filter, no extension restriction (temp junk has arbitrary
		// extensions), recursive since installers nest subfolders.
		return OldFilesRemover{
			MaxAgeDays: 7,
			Recursive:  true,
		}, nil
	}
	if r, ok := resolveDevAdvanced(it); ok {
		return r, nil
	}
	if it.Tool == "ios-messages" && it.Path != "" {
		// 180-day cutoff matches scanMessagesAttachments' detection
		// window. Recursive (the Attachments tree is hashed) and
		// match-all (attachments have arbitrary extensions). Only the
		// attachment files are removed; chat.db is never touched.
		return OldFilesRemover{
			MaxAgeDays: 180,
			Recursive:  true,
		}, nil
	}
	if it.Path == "" {
		return nil, fmt.Errorf("item %q has no Path and no specialized remover", it.Name)
	}
	return PathRemover{}, nil
}

// Result reports what happened to one item after Run.
type Result struct {
	Item    item.Item
	Skipped bool   // user said no, or DryRun
	Error   error  // non-nil if removal failed
	Reason  string // why it was skipped
}

// Bytes returns the size that was (or would be) freed by this result.
func (r Result) Bytes() int64 {
	if r.Skipped || r.Error != nil {
		return 0
	}
	return r.Item.Bytes
}

// Plan groups the items the user chose to clean.
// A Plan is created from a list of detected items + a Mode + a Prompter.
type Plan struct {
	Items    []item.Item
	Mode     Mode
	Prompter Prompter
	Resolver Resolver
	Out      io.Writer // where dry-run lines and progress go
}

// New builds a plan with sensible defaults. Pass a nil Prompter for non-interactive use.
func New(items []item.Item, mode Mode, p Prompter, out io.Writer) *Plan {
	if out == nil {
		out = os.Stdout
	}
	return &Plan{
		Items:    items,
		Mode:     mode,
		Prompter: p,
		Resolver: DefaultResolver,
		Out:      out,
	}
}

// Run iterates over the plan respecting Mode and Prompter.
// It returns one Result per item, in the same order.
func (p *Plan) Run() []Result {
	results := make([]Result, 0, len(p.Items))
	for _, it := range p.Items {
		// Resolve remover before asking — if the item is malformed we skip with reason.
		remover, err := p.Resolver(it)
		if err != nil {
			results = append(results, Result{Item: it, Skipped: true, Reason: err.Error()})
			continue
		}

		decision := p.decide(it, remover)
		if decision == DecisionQuit {
			results = append(results, Result{Item: it, Skipped: true, Reason: "user quit"})
			break
		}
		if decision == DecisionNo {
			results = append(results, Result{Item: it, Skipped: true, Reason: "user declined"})
			continue
		}

		if p.Mode == DryRun {
			fmt.Fprintf(p.Out, "[dry-run] would remove: %s\n", remover.Describe(it))
			results = append(results, Result{Item: it, Skipped: true, Reason: "dry-run"})
			continue
		}

		fmt.Fprintf(p.Out, "removing %s ... ", it.Name)
		if err := remover.Remove(it); err != nil {
			fmt.Fprintf(p.Out, "FAILED: %v\n", err)
			results = append(results, Result{Item: it, Error: err})
			continue
		}
		fmt.Fprintln(p.Out, "ok")
		results = append(results, Result{Item: it})
	}
	return results
}

// decide returns the user decision for an item according to Mode.
//
// In Yes mode we always return DecisionYes.
// In DryRun mode we always return DecisionYes too — the caller will skip
// the actual deletion but still report what *would* happen.
// In Interactive mode we go through the Prompter and handle the "view" loop.
func (p *Plan) decide(it item.Item, remover Remover) Decision {
	if p.Mode == Yes || p.Mode == DryRun {
		return DecisionYes
	}
	if p.Prompter == nil {
		return DecisionNo // safest fallback for misuse
	}
	for {
		d := p.Prompter.Ask(it)
		if d != DecisionView {
			return d
		}
		// Show extra context (path listing) and ask again.
		p.Prompter.Show(viewItem(it, remover))
	}
}

// viewItem builds the multi-line preview shown when the user types "v".
// Delegates to the remover's own Preview method when it implements
// Previewer; removers with no meaningful preview (or that this build
// doesn't even define, on another OS) get a generic fallback message.
func viewItem(it item.Item, remover Remover) string {
	if p, ok := remover.(Previewer); ok {
		return p.Preview(it)
	}
	return fmt.Sprintf("(no preview available for %T)", remover)
}

// ErrUnsafePath is returned when a PathRemover is asked to delete outside SafeRoots.
var ErrUnsafePath = errors.New("path is outside safe roots; refusing to delete")

// ErrOffLimits is returned when a PathRemover is asked to touch a path
// inside an OffLimits prefix. Distinct from ErrUnsafePath so callers can
// tell "we don't reach there" apart from "we refuse to touch user data".
var ErrOffLimits = errors.New("path is off-limits; mistah refuses to delete user data here")

// OffLimits lists path prefixes that mistah will NEVER delete from,
// regardless of what any detector reported. This is a second defensive
// barrier on top of SafeRoots:
//
//	SafeRoots answers "is this path in a place we're allowed to touch?".
//	OffLimits answers "even if we're allowed, is this user data we must
//	                   not touch?".
//
// Both must pass before PathRemover proceeds. A misbehaving detector that
// reports ~/Documents/foo.txt is caught here, even though ~/Documents is
// inside the user's home (i.e. inside SafeRoots).
//
// Resolved against the user's home directory at process start. Tests can
// rebuild the slice with DefaultOffLimits(tempHome) when they need to
// exercise the check against a fixture.
var OffLimits = DefaultOffLimits(homeOrEmpty())

// homeOrEmpty returns the user's home dir or "" if it can't be resolved.
// Wrapped to keep the OffLimits init expression readable.
func homeOrEmpty() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// PathRemover deletes a directory or file with rm -rf semantics, but only if
// the path is rooted in SafeRoots. Use this for cache directories.
type PathRemover struct{}

func (PathRemover) Describe(it item.Item) string {
	return fmt.Sprintf("%s (%s)", it.Path, it.Name)
}

func (PathRemover) Remove(it item.Item) error {
	if it.Path == "" {
		return errors.New("empty path")
	}
	abs, err := filepath.Abs(it.Path)
	if err != nil {
		return err
	}
	if !isUnderSafeRoot(abs) {
		return fmt.Errorf("%w: %s", ErrUnsafePath, abs)
	}
	// Defense in depth: even paths inside SafeRoots may collide with user
	// data (e.g. ~/Documents lives under the home dir, which is a SafeRoot).
	// Reject those before any detector mistake reaches the filesystem.
	if isOffLimits(abs) {
		return fmt.Errorf("%w: %s", ErrOffLimits, abs)
	}
	return os.RemoveAll(abs)
}

// Preview implements Previewer: up to N child entries with sizes for
// the "view" UI.
func (PathRemover) Preview(it item.Item) string {
	return PathRemover{}.previewPath(it.Path)
}

// previewPath returns up to N child entries with sizes for the "view" UI.
func (PathRemover) previewPath(path string) string {
	const maxEntries = 15
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Sprintf("(cannot read %s: %v)", path, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Contents of %s:\n", path)
	limit := len(entries)
	if limit > maxEntries {
		limit = maxEntries
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintf(&b, "  %s\n", entries[i].Name())
	}
	if len(entries) > maxEntries {
		fmt.Fprintf(&b, "  ... and %d more\n", len(entries)-maxEntries)
	}
	return b.String()
}

// isUnderSafeRoot returns true iff abs starts with one of the SafeRoots
// at a path-component boundary (so /home/foo doesn't match /homer).
func isUnderSafeRoot(abs string) bool {
	for _, root := range SafeRoots {
		if abs == root {
			return true
		}
		if strings.HasPrefix(abs, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// isOffLimits returns true iff abs is, or lives under, any OffLimits prefix.
//
// Boundary discipline matters here: ~/Documents-old must NOT match
// ~/Documents. We require either equality or a separator right after the
// prefix, never a bare prefix string match. Same algorithm as
// isUnderSafeRoot — kept separate to avoid coupling the two policies.
func isOffLimits(abs string) bool {
	for _, root := range OffLimits {
		if abs == root {
			return true
		}
		if strings.HasPrefix(abs, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// DockerPruneRemover invokes `docker system prune -f` to free reclaimable space.
// It does NOT pass --volumes; user data on volumes stays untouched.
// Docker's CLI is identical on every OS mistah supports, so this remover
// needs no platform split.
type DockerPruneRemover struct{}

func (DockerPruneRemover) Describe(it item.Item) string {
	return "docker system prune -f (images, build cache, stopped containers; volumes preserved)"
}

func (DockerPruneRemover) Remove(it item.Item) error {
	cmd := exec.Command("docker", "system", "prune", "-f")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker prune failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Preview implements Previewer by showing `docker system df`, the same
// breakdown the wizard's confirmation banner would want to show before
// pruning.
func (DockerPruneRemover) Preview(it item.Item) string {
	out, _ := exec.Command("docker", "system", "df").Output()
	return string(out)
}

// OldFilesRemover deletes files inside a directory that are older than
// MaxAgeDays and (optionally) match one of Extensions. The directory
// itself always stays in place; only matching files are removed.
//
// Two configurable behaviours:
//
//	Extensions  When non-empty, only files whose name ends in one of
//	            these (case-insensitive) are eligible. When EMPTY, every
//	            file matches — used for opaque blobs like iMessage
//	            attachments or Windows %TEMP% junk where the extension
//	            is irrelevant or unpredictable.
//	Recursive   When false, only immediate children are considered
//	            (crash reports live flat). When true, the whole subtree
//	            is walked — iMessage attachments and %TEMP% both live in
//	            nested subdirectory trees.
//
// The defaults (Extensions set, Recursive false) preserve the original
// crash-reports behaviour, so existing callers don't change.
//
// MaxAgeDays must be > 0; a value of 0 would mean "delete everything
// created today", which is dangerous and almost never intended.
//
// Per-file errors are tolerated: a permission-denied on one file doesn't
// stop the rest. The first error seen is returned at the end. When
// Recursive is true, directories left empty after their files are
// removed are NOT pruned — we only delete files, never directories, so
// the tree structure (and the root) survives.
type OldFilesRemover struct {
	MaxAgeDays int
	Extensions []string // case-insensitive w/ leading dot; empty = match all
	Recursive  bool
}

func (r OldFilesRemover) Describe(it item.Item) string {
	return fmt.Sprintf("borrar archivos >%d días en %s", r.MaxAgeDays, it.Path)
}

func (r OldFilesRemover) Remove(it item.Item) error {
	if it.Path == "" {
		return errors.New("empty path")
	}
	if r.MaxAgeDays <= 0 {
		return errors.New("OldFilesRemover requires MaxAgeDays > 0")
	}
	abs, err := filepath.Abs(it.Path)
	if err != nil {
		return err
	}
	if !isUnderSafeRoot(abs) {
		return fmt.Errorf("%w: %s", ErrUnsafePath, abs)
	}
	if isOffLimits(abs) {
		return fmt.Errorf("%w: %s", ErrOffLimits, abs)
	}

	cutoff := time.Now().Add(-time.Duration(r.MaxAgeDays) * 24 * time.Hour)

	if r.Recursive {
		return r.removeTree(abs, cutoff)
	}
	return r.removeFlat(abs, cutoff)
}

// removeFlat handles immediate children only. This is the crash-reports
// path and keeps the original, well-tested behaviour intact.
func (r OldFilesRemover) removeFlat(dir string, cutoff time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var firstErr error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !r.matchesExt(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue // newer than cutoff: keep
		}
		full := filepath.Join(dir, e.Name())
		if rmErr := os.Remove(full); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
	}
	return firstErr
}

// removeTree walks the whole subtree, deleting eligible files. Only
// files are removed; directories (including the root and now-empty
// intermediates) are left in place. This is the iMessage/temp-files
// path: both have a nested directory layout their owner (Messages.app,
// or arbitrary installers writing into %TEMP%) expects to keep existing.
//
// filepath.WalkDir tolerates per-entry errors: when WalkDir hands us an
// error for a node we record it and skip that node rather than aborting
// the walk, so one unreadable subdirectory doesn't strand the rest.
func (r OldFilesRemover) removeTree(root string, cutoff time.Time) error {
	var firstErr error
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return nil // skip this node, keep walking
		}
		if d.IsDir() {
			return nil
		}
		if !r.matchesExt(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if rmErr := os.Remove(path); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
		return nil
	})
	if walkErr != nil && firstErr == nil {
		firstErr = walkErr
	}
	return firstErr
}

// matchesExt reports whether name is eligible. An empty Extensions list
// means "match everything" — the iMessage/temp-files case, where files
// have arbitrary or no extensions. Otherwise it's case-insensitive suffix
// matching. Centralised so the regression test can call it directly.
func (r OldFilesRemover) matchesExt(name string) bool {
	if len(r.Extensions) == 0 {
		return true
	}
	lower := strings.ToLower(name)
	for _, ext := range r.Extensions {
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// Summary aggregates Run results for the final report.
type Summary struct {
	Removed      int
	Skipped      int
	Failed       int
	BytesFreed   int64
	BytesPlanned int64
}

// Summarize folds a slice of results into a Summary.
func Summarize(results []Result) Summary {
	var s Summary
	for _, r := range results {
		s.BytesPlanned += r.Item.Bytes
		switch {
		case r.Error != nil:
			s.Failed++
		case r.Skipped:
			s.Skipped++
		default:
			s.Removed++
			s.BytesFreed += r.Item.Bytes
		}
	}
	return s
}
