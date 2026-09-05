//go:build linux

package cleaner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sistematlan/mistah/internal/item"
)

// SafeRoots are the only filesystem prefixes a PathRemover will touch.
// Anything outside this set is rejected with ErrUnsafePath.
//
// /tmp is included for the same reason it is on macOS: some detectors
// (and every test using TempDir) legitimately point there.
var SafeRoots = func() []string {
	roots := []string{"/tmp", os.TempDir()}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, home)
	}
	return roots
}()

// DefaultOffLimits returns the standard list of protected prefixes for
// a given home directory on Linux. Exported for tests; production
// code uses the pre-built OffLimits variable.
//
// Notes on the chosen prefixes (Linux/XDG equivalents of the
// macOS/Windows lists):
//   - Documents, Desktop, Videos, Pictures, Music: the XDG user-dirs
//     folders every desktop environment creates by default (see
//     xdg-user-dirs(5)). Same blanket top-level protection as the
//     other two platforms.
//   - .gnupg, .ssh: private keys and GPG secrets — the closest analogue
//     to macOS Keychains / Windows Credential Manager. Unlike those
//     two (which are OS-managed stores with no plain directory to
//     protect), Linux really does keep this material in ordinary
//     files under $HOME, so it needs its own explicit entry here.
//   - .mozilla, .thunderbird: mail/profile data for Thunderbird users,
//     the closest Linux desktop has to a native "Mail.app" — protected
//     wholesale rather than trying to carve out just the cache subtree
//     safely.
func DefaultOffLimits(home string) []string {
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Videos"),
		filepath.Join(home, "Pictures"),
		filepath.Join(home, "Music"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".mozilla"),
		filepath.Join(home, ".thunderbird"),
	}
}

// resolveTrash maps a Tool="trash" item to TrashRemover.
func resolveTrash(it item.Item) (Remover, bool) {
	if it.Tool == "trash" && it.Path != "" {
		return TrashRemover{}, true
	}
	return nil, false
}

// resolveDevAdvanced has nothing to resolve on Linux: Time Machine
// snapshots and Xcode simulators are macOS-only concepts. Mirrors the
// Windows no-op in cleaner_windows.go.
func resolveDevAdvanced(it item.Item) (Remover, bool) {
	return nil, false
}

// TrashRemover empties the freedesktop.org Trash by deleting both
// halves of each trashed item: the payload under
// ~/.local/share/Trash/files/<name> AND its metadata sidecar
// ~/.local/share/Trash/info/<name>.trashinfo.
//
// Deleting only files/ (which is what the detector's Item.Path points
// at, for sizing purposes) would leave orphaned .trashinfo records
// behind — file managers that later try to restore-from-trash would
// show phantom entries pointing at nothing. The spec models a trashed
// item as the *pair*, so the remover must operate on the pair even
// though the Item only carries one path.
//
// The path is validated against SafeRoots and OffLimits via the same
// helpers PathRemover uses, exactly like TrashContentsRemover on macOS
// and RecycleBinRemover on Windows.
type TrashRemover struct{}

func (TrashRemover) Describe(it item.Item) string {
	return fmt.Sprintf("vaciar papelera (%s)", it.Path)
}

func (TrashRemover) Remove(it item.Item) error {
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

	entries, err := os.ReadDir(abs)
	if err != nil {
		return err
	}
	// info/ sits alongside files/ under the same Trash root
	// (…/Trash/files and …/Trash/info are siblings), so we derive it
	// from files/'s parent rather than requiring a second Item field.
	infoDir := filepath.Join(filepath.Dir(abs), "info")

	var firstErr error
	for _, e := range entries {
		name := e.Name()
		if rmErr := os.RemoveAll(filepath.Join(abs, name)); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
		// Best-effort: a missing .trashinfo (already cleaned up by
		// another tool, or a files/ entry that predates the spec)
		// shouldn't fail the whole operation.
		_ = os.Remove(filepath.Join(infoDir, name+".trashinfo"))
	}
	return firstErr
}

// Preview implements Previewer identically to the other platforms'
// trash removers: list immediate children so the user can sanity-check
// what they're about to lose.
func (TrashRemover) Preview(it item.Item) string {
	return PathRemover{}.previewPath(it.Path)
}
