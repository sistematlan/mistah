//go:build windows

// system_windows.go implements the Windows detectors for
// internal/system: the Recycle Bin, %TEMP% scratch space, and the
// Explorer thumbnail cache. These are the closest per-OS equivalents to
// the macOS detectors in system_darwin.go (Trash, Mail Downloads,
// QuickLook thumbnails) but use entirely different APIs — there is no
// shared implementation, only a shared item.Item shape.
//
// Not ported (deliberately, see BACKLOG.md):
//   - Windows Update cache (C:\Windows\SoftwareDistribution\Download)
//     and Prefetch (C:\Windows\Prefetch) require administrator rights
//     to clean safely and can affect apps mid-install; out of scope for
//     a per-user CLI that never elevates.
//   - Time Machine snapshots (macOS-only, no Windows equivalent — VSS
//     shadow copies require vssadmin with admin rights).
//   - iMessage attachments (Apple-only, no Windows equivalent).
package system

import (
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/sistematlan/mistah/internal/item"
)

// ScanHome runs the Windows detectors against an arbitrary home
// directory. Used by tests with a TempDir; production code uses Scan()
// which resolves the real home.
//
// recycleBin() deliberately ignores home — the Recycle Bin is a
// per-volume shell object, not a subdirectory of the user's profile —
// but every other detector here follows the standard home-scoped
// convention so tests stay hermetic.
func ScanHome(home string) []item.Item {
	var items []item.Item
	if it, ok := recycleBin(); ok {
		items = append(items, it)
	}
	items = append(items, scanTempFiles(home)...)
	if it, ok := thumbnailCache(home); ok {
		items = append(items, it)
	}
	return items
}

// recycleBin reports the size and item count of the Recycle Bin across
// all volumes, via the Shell32 SHQueryRecycleBinW API — the same call
// Explorer itself uses to show "Recycle Bin (12 items, 340 MB)".
//
// Passing "" as the root path queries every drive at once rather than
// requiring the caller to enumerate volumes and sum per-drive results.
//
// RiskAskBefore, matching the macOS Trash policy: the user explicitly
// deleted these files once already, but final deletion from the
// Recycle Bin is irreversible and deserves one more confirmation.
func recycleBin() (item.Item, bool) {
	size, count, err := shQueryRecycleBin("")
	if err != nil || size <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:       "Recycle Bin",
		NameKey:    "system.trash.name",
		Tool:       "trash",
		Path:       "", // no filesystem path: removed via SHEmptyRecycleBin, not RemoveAll
		Bytes:      size,
		Category:   item.CategorySystem,
		Risk:       item.RiskAskBefore,
		Detail:     "papelera de reciclaje; los archivos se borran de forma definitiva",
		DetailKey:  "system.trash.detail",
		DetailArgs: []any{int(count), 0},
	}, true
}

// shQueryRecycleBinInfo mirrors the Win32 SHQUERYRBINFO struct.
// cbSize must be set to the struct size before the call, per the Win32
// contract for "versioned struct, tell me how big your buffer is".
type shQueryRecycleBinInfo struct {
	cbSize      uint32
	i64Size     int64
	i64NumItems int64
}

var (
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procSHQueryRecycleBinW = shell32.NewProc("SHQueryRecycleBinW")
	procSHEmptyRecycleBinW = shell32.NewProc("SHEmptyRecycleBinW")
)

// shQueryRecycleBin calls SHQueryRecycleBinW for the given root path
// ("" queries every volume). Returns total bytes and item count.
func shQueryRecycleBin(rootPath string) (bytes int64, count int64, err error) {
	var pRoot *uint16
	if rootPath != "" {
		pRoot, err = windows.UTF16PtrFromString(rootPath)
		if err != nil {
			return 0, 0, err
		}
	}
	info := shQueryRecycleBinInfo{cbSize: uint32(unsafe.Sizeof(shQueryRecycleBinInfo{}))}
	r1, _, _ := procSHQueryRecycleBinW.Call(
		uintptr(unsafe.Pointer(pRoot)),
		uintptr(unsafe.Pointer(&info)),
	)
	// SHQueryRecycleBinW returns an HRESULT; S_OK is 0.
	if r1 != 0 {
		return 0, 0, windows.Errno(r1)
	}
	return info.i64Size, info.i64NumItems, nil
}

// emptyRecycleBin calls SHEmptyRecycleBinW for the given root path.
// Flags suppress the confirmation dialog and progress UI and skip the
// "are you sure" sound — mistah already asked the user via its own
// prompter, a second native confirmation would be redundant.
func emptyRecycleBin(rootPath string) error {
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
	r1, _, _ := procSHEmptyRecycleBinW.Call(
		0, // hwnd: no owner window
		uintptr(unsafe.Pointer(pRoot)),
		uintptr(shercNoConfirmation|shercNoProgressUI|shercNoSound),
	)
	if r1 != 0 {
		return windows.Errno(r1)
	}
	return nil
}

// scanTempFiles reports %TEMP% contents older than tempMaxAgeDays.
// Windows and countless installers write scratch files here and rarely
// clean up after themselves; anything untouched for a while is safe
// bulk-delete material, same spirit as macOS's /tmp cleanup that
// launchd handles automatically (Windows has no equivalent daemon).
//
// We report one aggregated item rather than one per file — %TEMP% can
// hold thousands of entries and a per-file list would drown the UI,
// mirroring the iMessage attachments aggregation pattern on macOS.
func scanTempFiles(home string) []item.Item {
	dir := os.TempDir()
	bytes, count, ok := summarizeOldTempFiles(dir, tempMaxAgeDays)
	if !ok || bytes <= 0 || count == 0 {
		return nil
	}
	return []item.Item{{
		Name:       "Archivos temporales",
		Tool:       "temp",
		Path:       dir,
		Bytes:      bytes,
		Category:   item.CategorySystem,
		Risk:       item.RiskSafe,
		Detail:     "archivos temporales antiguos en %TEMP%; Windows y los instaladores los regeneran",
		DetailKey:  "system.temp.detail",
		DetailArgs: []any{count, tempMaxAgeDays},
	}}
}

// tempMaxAgeDays is the cutoff for %TEMP% files. Same 7-day window
// Windows' own Disk Cleanup / Storage Sense tools use by default for
// "temporary files" — conservative enough that an install running
// right now is never touched.
var tempMaxAgeDays = 7

// summarizeOldTempFiles walks dir and totals size/count of files older
// than maxDays. Locked files (still open by another process) fail
// os.Remove later at cleanup time, not here — this function only
// measures.
func summarizeOldTempFiles(dir string, maxDays int) (bytes int64, count int, ok bool) {
	if _, err := os.Stat(dir); err != nil {
		return 0, 0, false
	}
	cutoff := time.Now().Add(-time.Duration(maxDays) * 24 * time.Hour)
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable node (often a locked file), keep walking
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		bytes += info.Size()
		count++
		return nil
	})
	if walkErr != nil {
		return 0, 0, false
	}
	return bytes, count, true
}

// thumbnailCache reports Windows Explorer's thumbnail cache database,
// the closest equivalent to macOS's QuickLook thumbnailcache: a
// per-user cache of small preview images Explorer regenerates on
// demand when browsing folders in icon/thumbnail view.
func thumbnailCache(home string) (item.Item, bool) {
	path := filepath.Join(home, "AppData", "Local", "Microsoft", "Windows", "Explorer")
	entries, err := os.ReadDir(path)
	if err != nil {
		return item.Item{}, false
	}
	var bytes int64
	for _, e := range entries {
		name := e.Name()
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, "thumbcache_") && !strings.HasPrefix(lower, "iconcache_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		bytes += info.Size()
	}
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "Miniaturas de Explorer",
		Tool:      "thumbcache",
		Path:      path,
		Bytes:     bytes,
		Category:  item.CategorySystem,
		Risk:      item.RiskSafe,
		Detail:    "caché de miniaturas de Explorer; Windows la regenera al navegar carpetas",
		DetailKey: "system.thumbcache.detail",
	}, true
}
