//go:build darwin

package cleaner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sistematlan/mistah/internal/item"
)

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

// resolveTrash maps a Tool="trash" item to TrashContentsRemover. Split
// out from DefaultResolver so cleaner.go's dispatch stays a single
// readable list regardless of which platform file provides the
// concrete remover.
func resolveTrash(it item.Item) (Remover, bool) {
	if it.Tool == "trash" && it.Path != "" {
		return TrashContentsRemover{}, true
	}
	return nil, false
}

// resolveDevAdvanced maps Time Machine snapshots and stale Xcode
// simulators to their removers. Both Tool values only ever appear in
// items produced by the darwin build of internal/system and
// internal/dev, so this function is unreachable dead code on other
// platforms — which is exactly why it lives here instead of in the
// shared cleaner.go.
func resolveDevAdvanced(it item.Item) (Remover, bool) {
	if it.Tool == "tm-snapshots" {
		return TMSnapshotsRemover{}, true
	}
	if it.Tool == "xcode-simulator" && it.Path != "" {
		return XcodeSimulatorRemover{}, true
	}
	return nil, false
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

// Preview implements Previewer identically to PathRemover's: list
// immediate children so the user can sanity-check what they're about
// to lose.
func (TrashContentsRemover) Preview(it item.Item) string {
	return PathRemover{}.previewPath(it.Path)
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

// XcodeSimulatorRemover deletes a single CoreSimulator device by
// asking xcrun to do it. The detector reports one Item per stale
// device with Path pointing at
// ~/Library/Developer/CoreSimulator/Devices/<UDID>/, and we extract
// the UDID from filepath.Base(Path).
//
// Why not just os.RemoveAll(Path)? `xcrun simctl delete <UDID>` also
// updates Xcode's device index. Removing the directory by hand leaves
// a phantom device in `simctl list` until the user runs `simctl
// erase` or restarts. The shell call costs ~200ms but keeps Xcode
// consistent.
//
// If `xcrun` fails (no Xcode installed, simctl in a bad state), we
// fall back to a plain os.RemoveAll so the user still reclaims the
// bytes. The phantom-index cost is acceptable when the alternative is
// "cleaner did nothing".
type XcodeSimulatorRemover struct{}

func (XcodeSimulatorRemover) Describe(it item.Item) string {
	return fmt.Sprintf("xcrun simctl delete %s", filepath.Base(it.Path))
}

func (XcodeSimulatorRemover) Remove(it item.Item) error {
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
	if isOffLimits(abs) {
		return fmt.Errorf("%w: %s", ErrOffLimits, abs)
	}

	udid := filepath.Base(abs)
	out, xerr := xcrunCommand("simctl", "delete", udid)
	if xerr == nil {
		return nil
	}
	// xcrun failed; fall back to os.RemoveAll so the user still
	// gets the bytes back. Wrap both errors so the caller sees the
	// original xcrun reason in the log.
	if rmErr := os.RemoveAll(abs); rmErr != nil {
		return fmt.Errorf("xcrun delete failed (%v: %s) and rm fallback failed: %w",
			xerr, strings.TrimSpace(string(out)), rmErr)
	}
	return nil
}

// xcrunCommand is the test seam for XcodeSimulatorRemover. Same shape
// as tmutilCommand: a swappable var so tests don't need to mess with
// PATH or build fake binaries.
var xcrunCommand = func(args ...string) ([]byte, error) {
	return exec.Command("xcrun", args...).CombinedOutput()
}
