//go:build windows

package cleaner

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/sistematlan/mistah/internal/item"
)

// SafeRoots are the only filesystem prefixes a PathRemover will touch.
// Anything outside this set is rejected with ErrUnsafePath.
//
// os.TempDir() on Windows resolves to %TMP%/%TEMP% for the current
// user (falling back to the Windows temp directory), which is where
// installers and the OS itself stage disposable scratch files — the
// same role /tmp and /var/folders play on macOS.
var SafeRoots = func() []string {
	roots := []string{os.TempDir()}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, home)
	}
	return roots
}()

// DefaultOffLimits returns the standard list of protected prefixes for a
// given home directory on Windows. Exported for tests; production code
// uses the pre-built OffLimits variable.
//
// Notes on the chosen prefixes (Windows equivalents of the macOS list
// in cleaner_darwin.go):
//   - Documents, Desktop, Videos, Pictures, Music: the standard Windows
//     user shell folders. "Videos" replaces macOS's "Movies" — that's
//     the only naming difference; the protection intent is identical.
//   - AppData\Roaming\Microsoft\Credentials and AppData\Roaming\Microsoft\Crypto:
//     Windows Credential Manager and DPAPI key material — the closest
//     analogue to macOS's Keychains directory. Deleting these can lock
//     the user out of saved passwords or break app-level encryption.
//   - AppData\Roaming\Microsoft\Windows\Libraries: knownfolder/library
//     definitions; corrupting this breaks Explorer's folder shortcuts,
//     similar in spirit to protecting ~/Library/Mobile Documents.
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
		filepath.Join(home, "AppData", "Roaming", "Microsoft", "Credentials"),
		filepath.Join(home, "AppData", "Roaming", "Microsoft", "Crypto"),
		filepath.Join(home, "AppData", "Roaming", "Microsoft", "Windows", "Libraries"),
	}
}

// resolveTrash maps a Tool="trash" item to RecycleBinRemover.
func resolveTrash(it item.Item) (Remover, bool) {
	if it.Tool == "trash" {
		return RecycleBinRemover{}, true
	}
	return nil, false
}

// resolveDevAdvanced has nothing to resolve on Windows: Time Machine
// snapshots and Xcode simulators are macOS-only concepts and no
// Windows detector ever produces those Tool values. Kept as a
// same-signature no-op so DefaultResolver's dispatch list in
// cleaner.go doesn't need a build-tagged branch of its own.
func resolveDevAdvanced(it item.Item) (Remover, bool) {
	return nil, false
}

// RecycleBinRemover empties the Windows Recycle Bin via the Shell32
// SHEmptyRecycleBinW API — the same call Explorer's own "Empty Recycle
// Bin" context menu item makes.
//
// Unlike TrashContentsRemover on macOS (which deletes files directly
// with os.RemoveAll because ~/.Trash is a plain folder), the Recycle
// Bin is a virtual shell namespace, not an iterable directory — there
// is no "list of files inside $Recycle.Bin" API contract mistah can
// rely on across Windows versions. SHEmptyRecycleBinW is the
// documented, stable way to clear it.
//
// The detector (system.recycleBin) deliberately sets Item.Path to "",
// so Remove ignores SafeRoots/OffLimits path checks entirely — there's
// no filesystem path to validate, only a shell operation to invoke.
type RecycleBinRemover struct{}

func (RecycleBinRemover) Describe(it item.Item) string {
	return "SHEmptyRecycleBinW (vaciar papelera de reciclaje en todas las unidades)"
}

func (RecycleBinRemover) Remove(it item.Item) error {
	return emptyRecycleBinShell("")
}

// Preview implements Previewer with a best-effort byte/item count via
// the same SHQueryRecycleBinW call the detector uses. Re-querying
// rather than caching the detector's numbers keeps this remover
// self-contained, mirroring how DockerPruneRemover re-runs `docker
// system df` for its own preview instead of trusting stale Item.Bytes.
func (RecycleBinRemover) Preview(it item.Item) string {
	size, count, err := shQueryRecycleBinInfo("")
	if err != nil {
		return fmt.Sprintf("(no se pudo consultar la papelera: %v)", err)
	}
	return fmt.Sprintf("Papelera de reciclaje: %d elementos, %d bytes", count, size)
}

// shell32 and its procedures are declared here (not shared with
// internal/system) to keep cleaner_windows.go self-contained — the
// same independence tmutilCommand/xcrunCommand have from
// internal/system on macOS. Loading the same system DLL twice via
// LazyDLL is harmless; Windows reference-counts the module handle.
var (
	shell32Cleaner          = windows.NewLazySystemDLL("shell32.dll")
	procSHEmptyRecycleBinW2 = shell32Cleaner.NewProc("SHEmptyRecycleBinW")
	procSHQueryRecycleBinW2 = shell32Cleaner.NewProc("SHQueryRecycleBinW")
)

// shQueryRecycleBinInfoStruct mirrors the Win32 SHQUERYRBINFO struct.
type shQueryRecycleBinInfoStruct struct {
	cbSize      uint32
	i64Size     int64
	i64NumItems int64
}

func shQueryRecycleBinInfo(rootPath string) (bytes int64, count int64, err error) {
	var pRoot *uint16
	if rootPath != "" {
		pRoot, err = windows.UTF16PtrFromString(rootPath)
		if err != nil {
			return 0, 0, err
		}
	}
	info := shQueryRecycleBinInfoStruct{cbSize: uint32(unsafe.Sizeof(shQueryRecycleBinInfoStruct{}))}
	r1, _, _ := procSHQueryRecycleBinW2.Call(
		uintptr(unsafe.Pointer(pRoot)),
		uintptr(unsafe.Pointer(&info)),
	)
	if r1 != 0 {
		return 0, 0, windows.Errno(r1)
	}
	return info.i64Size, info.i64NumItems, nil
}

// emptyRecycleBinShell calls SHEmptyRecycleBinW for rootPath ("" = every
// volume). Flags suppress the confirmation dialog, progress UI and
// sound — mistah's own Prompter already asked the user, so a second
// native "are you sure?" would be redundant and would block
// non-interactive (Yes/DryRun-then-Yes) runs on a UI thread that never
// exists in a CLI process.
func emptyRecycleBinShell(rootPath string) error {
	const (
		shercNoConfirmation = 0x00000001
		shercNoProgressUI   = 0x00000002
		shercNoSound        = 0x00000004
	)
	var pRoot *uint16
	if rootPath != "" {
		p, err := windows.UTF16PtrFromString(rootPath)
		if err != nil {
			return err
		}
		pRoot = p
	}
	r1, _, _ := procSHEmptyRecycleBinW2.Call(
		0, // hwnd: no owner window
		uintptr(unsafe.Pointer(pRoot)),
		uintptr(shercNoConfirmation|shercNoProgressUI|shercNoSound),
	)
	if r1 != 0 {
		return fmt.Errorf("SHEmptyRecycleBinW failed: %w", windows.Errno(r1))
	}
	return nil
}
