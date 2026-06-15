// Package cleaner removes items previously detected by scanners.
//
// Design:
//   - A Remover knows how to delete one Item (path-based, docker-prune, etc.).
//   - A Plan groups items + their resolved Remover.
//   - Mode controls confirmation flow: DryRun, Interactive, Yes.
//   - Prompter abstracts the user interaction so we can test it.
//
// Safety rules enforced here:
//   - Never delete a path outside a known-safe prefix (home dir or /var/folders).
//   - DryRun never touches the filesystem.
//   - Docker volumes are NEVER pruned (only `system prune -f`, no --volumes).
//   - Empty/unknown paths are skipped, not errored, to avoid panics on malformed Items.
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

// Resolver picks a Remover for an item. The default resolver maps:
//   - Docker reclaimable → DockerPruneRemover
//   - Trash → TrashContentsRemover (wipes children, keeps ~/.Trash dir alive)
//   - Crash reports → OldFilesRemover (only files older than the cutoff)
//   - Time Machine snapshots → TMSnapshotsRemover (calls tmutil)
//   - everything else with a non-empty Path → PathRemover
type Resolver func(it item.Item) (Remover, error)

// DefaultResolver is the standard mapping used by `mistah clean`.
func DefaultResolver(it item.Item) (Remover, error) {
	if it.Tool == "docker" && it.Path == "" {
		return DockerPruneRemover{}, nil
	}
	if it.Tool == "trash" && it.Path != "" {
		return TrashContentsRemover{}, nil
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
	if it.Tool == "tm-snapshots" {
		return TMSnapshotsRemover{}, nil
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
// For PathRemover we list the immediate children. For Docker we run `docker system df`.
func viewItem(it item.Item, remover Remover) string {
	switch r := remover.(type) {
	case PathRemover:
		return r.previewPath(it.Path)
	case TrashContentsRemover:
		// The trash preview is identical in spirit to PathRemover's:
		// list immediate children so the user can sanity-check what
		// they're about to lose. Reuse PathRemover.previewPath rather
		// than re-implementing the same logic.
		return PathRemover{}.previewPath(it.Path)
	case DockerPruneRemover:
		out, _ := exec.Command("docker", "system", "df").Output()
		return string(out)
	default:
		return fmt.Sprintf("(no preview available for %T)", remover)
	}
}

// SafeRoots are the only filesystem prefixes a PathRemover will touch.
// Anything outside this set is rejected with ErrUnsafePath.
//
// /var/folders and /tmp are included so tests using TempDir work transparently;
// they are also legitimate caches in macOS.
var SafeRoots = func() []string {
	roots := []string{"/var/folders", "/tmp", "/private/var/folders", "/private/tmp"}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, home)
	}
	return roots
}()

// ErrUnsafePath is returned when a PathRemover is asked to delete outside SafeRoots.
var ErrUnsafePath = errors.New("path is outside safe roots; refusing to delete")

// OffLimits lists path prefixes that mistah will NEVER delete from,
// regardless of what any detector reported. This is a second defensive
// barrier on top of SafeRoots:
//
//   SafeRoots answers "is this path in a place we're allowed to touch?".
//   OffLimits answers "even if we're allowed, is this user data we must
//                      not touch?".
//
// Both must pass before PathRemover proceeds. A misbehaving detector that
// reports ~/Documents/foo.txt is caught here, even though ~/Documents is
// inside the user's home (i.e. inside SafeRoots).
//
// Resolved against the user's home directory at process start. Tests can
// rebuild the slice with DefaultOffLimits(tempHome) when they need to
// exercise the check against a fixture.
var OffLimits = DefaultOffLimits(homeOrEmpty())

// DefaultOffLimits returns the standard list of protected prefixes for a
// given home directory. Exported for tests; production code uses the
// pre-built OffLimits variable.
//
// Notes on the chosen prefixes:
//   - ~/Documents, ~/Desktop, ~/Movies, ~/Music: top-level user data folders
//     where no cache or trash should ever live. Blanket protection.
//   - ~/Pictures: blocked at the root. Future detectors that want to clean
//     specific Photos Library cache subpaths must be reviewed individually
//     and may need to bypass this only for explicitly-whitelisted children.
//   - ~/Library/Mobile Documents: iCloud Drive. Touching this can sync a
//     deletion to every other Apple device the user owns. Hard no.
//   - ~/Library/Keychains: passwords, secure notes, certificates.
//   - ~/Library/Application Support/AddressBook and ~/Library/Calendars:
//     contacts and calendars, irreplaceable user data.
func DefaultOffLimits(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Movies"),
		filepath.Join(home, "Pictures"),
		filepath.Join(home, "Music"),
		filepath.Join(home, "Library", "Mobile Documents"),
		filepath.Join(home, "Library", "Keychains"),
		filepath.Join(home, "Library", "Application Support", "AddressBook"),
		filepath.Join(home, "Library", "Calendars"),
	}
}

// ErrOffLimits is returned when a PathRemover is asked to touch a path
// inside an OffLimits prefix. Distinct from ErrUnsafePath so callers can
// tell "we don't reach there" apart from "we refuse to touch user data".
var ErrOffLimits = errors.New("path is off-limits; mistah refuses to delete user data here")

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

// TrashContentsRemover wipes the contents of a Trash directory while
// keeping the directory itself in place. macOS Finder treats ~/.Trash
// as a special location and stops working correctly if the directory
// disappears, so a plain os.RemoveAll on it is wrong even though it
// would technically free the same bytes.
//
// Children are removed individually with os.RemoveAll so subdirectories
// are wiped recursively. Errors on individual children are tolerated:
// macOS sometimes places files with restricted permissions (especially
// from app sandboxes) and a permission-denied on one item should not
// block the rest. The first error seen is returned at the end so the
// cleaner reports the failure, but the bulk of the trash still gets
// emptied.
//
// The path is validated against SafeRoots and OffLimits via the same
// helpers used by PathRemover before any deletion happens.
type TrashContentsRemover struct{}

func (TrashContentsRemover) Describe(it item.Item) string {
	return fmt.Sprintf("vaciar Trash (%s)", it.Path)
}

func (TrashContentsRemover) Remove(it item.Item) error {
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
	// The Trash directory itself is not in OffLimits, but a future
	// detector mistake (e.g. reporting ~/Documents as the trash path)
	// is exactly the case OffLimits exists to catch.
	if isOffLimits(abs) {
		return fmt.Errorf("%w: %s", ErrOffLimits, abs)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	var firstErr error
	for _, e := range entries {
		child := filepath.Join(abs, e.Name())
		if rmErr := os.RemoveAll(child); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
	}
	return firstErr
}

// OldFilesRemover deletes files inside a directory that are both older
// than MaxAgeDays AND match one of the Extensions. The directory itself
// stays in place; only matching files are removed (no recursion).
//
// This is the right shape for crash reports: we want to free old .crash
// files without wiping a recent debugging session, and we don't want a
// stray .DS_Store next to the reports to ever be deleted.
//
// MaxAgeDays must be > 0; a value of 0 would be interpreted as "delete
// everything created today" which is dangerous and almost never what
// the caller meant. Extensions match case-insensitively.
//
// Per-file errors are tolerated: a permission-denied on one file doesn't
// stop the rest. The first error seen is returned at the end.
type OldFilesRemover struct {
	MaxAgeDays int
	Extensions []string // case-insensitive, must include the leading dot
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

	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-time.Duration(r.MaxAgeDays) * 24 * time.Hour)
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
		full := filepath.Join(abs, e.Name())
		if rmErr := os.Remove(full); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
	}
	return firstErr
}

// matchesExt is case-insensitive suffix matching against r.Extensions.
// Centralised so the regression test can call it directly.
func (r OldFilesRemover) matchesExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range r.Extensions {
		if strings.HasSuffix(lower, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// TMSnapshotsRemover deletes Time Machine local snapshots by shelling
// out to `tmutil`. The detector in internal/system reports them with
// Tool="tm-snapshots"; this remover lists them again at delete time
// and runs `tmutil deletelocalsnapshots <date>` for each.
//
// Why list twice? The detector and remover are decoupled by design:
// the detector lives in internal/system, the remover lives here so
// the cleaner stays the single home of all deletion logic. Sharing
// state through Item fields would force every Item to carry an opaque
// blob of detector-specific data, which complicates the contract.
// `tmutil listlocalsnapshots /` is fast (~50 ms) and idempotent — a
// snapshot deleted between the two listings just doesn't appear on
// the second pass, no error.
//
// Test seam: tmutilCommand is a package-level var so tests can mock
// the binary without messing with PATH.
type TMSnapshotsRemover struct{}

func (TMSnapshotsRemover) Describe(_ item.Item) string {
	return "tmutil deletelocalsnapshots <each>"
}

func (TMSnapshotsRemover) Remove(_ item.Item) error {
	names, err := tmutilList()
	if err != nil {
		return fmt.Errorf("tmutil list failed: %w", err)
	}
	var firstErr error
	for _, name := range names {
		date := tmSnapshotDate(name)
		if date == "" {
			continue
		}
		if err := tmutilDelete(date); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// tmutilCommand is the test seam used by tmutilList and tmutilDelete.
// Production points it at the real binary; tests inject a function
// that returns canned output. The single seam covers both list and
// delete invocations, so a mock can route by inspecting args[0].
var tmutilCommand = func(args ...string) ([]byte, error) {
	return exec.Command("tmutil", args...).CombinedOutput()
}

// tmutilList runs `tmutil listlocalsnapshots /` and returns snapshot
// names. Failure here aborts the remover; we don't try to recover
// from a missing tmutil since macOS always ships it.
func tmutilList() ([]string, error) {
	out, err := tmutilCommand("listlocalsnapshots", "/")
	if err != nil {
		return nil, err
	}
	const prefix = "com.apple.TimeMachine."
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			names = append(names, line)
		}
	}
	return names, nil
}

// tmutilDelete asks tmutil to drop a single snapshot. The date string
// is the part between "com.apple.TimeMachine." and ".local"; tmutil
// rejects anything else with a non-zero exit code, which we surface.
func tmutilDelete(date string) error {
	out, err := tmutilCommand("deletelocalsnapshots", date)
	if err != nil {
		return fmt.Errorf("tmutil delete %s failed: %v: %s",
			date, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// tmSnapshotDate extracts the date portion of a snapshot name.
// Mirrors the detector's parsing in internal/system/snapshots.go;
// kept inline here so the remover doesn't depend on the system
// package (would create a cleaner ↔ system import cycle through
// the resolver).
func tmSnapshotDate(name string) string {
	const prefix = "com.apple.TimeMachine."
	const suffix = ".local"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
}

// Summary aggregates Run results for the final report.
type Summary struct {
	Removed     int
	Skipped     int
	Failed      int
	BytesFreed  int64
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
