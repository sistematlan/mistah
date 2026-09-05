//go:build linux

// system_linux.go implements the Linux detectors for internal/system:
// the freedesktop.org Trash spec, /tmp scratch space, and the GNOME/KDE
// thumbnail cache. These mirror system_darwin.go's Trash/QuickLook and
// system_windows.go's Recycle Bin/thumbcache, but every mechanism is
// different — there is no shared implementation, only the shared
// item.Item shape.
//
// Not ported (deliberately, mirrors the Windows exclusions in
// BACKLOG.md):
//   - Time Machine snapshots (macOS-only; Linux has no equivalent
//     concept built into the desktop — btrfs/LVM snapshots exist but
//     are admin-managed, out of scope for a per-user CLI).
//   - iMessage / Mail.app downloads (Apple-only).
//   - Generic crash reports: Linux's nearest equivalent (systemd-coredump,
//     ~/.cache/abrt) varies too much by distro/desktop to hardcode a
//     single path confidently; left as a follow-up.
package system

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// ScanHome runs the Linux detectors against an arbitrary home
// directory. Used by tests with a TempDir; production code uses
// Scan() which resolves the real home.
func ScanHome(home string) []item.Item {
	var items []item.Item
	if it, ok := freedesktopTrash(home); ok {
		items = append(items, it)
	}
	items = append(items, scanTmpFiles()...)
	if it, ok := thumbnailCache(home); ok {
		items = append(items, it)
	}
	return items
}

// freedesktopTrash reports ~/.local/share/Trash/files, the data
// directory every freedesktop.org-compliant file manager (Nautilus,
// Dolphin, Thunar, PCManFM…) uses for "moved to trash" files. A
// sibling ~/.local/share/Trash/info holds one .trashinfo metadata file
// per trashed item (original path + deletion timestamp) — we sum
// files/ for bytes and use info/ only to get an accurate item count,
// since files/ can contain directories that themselves hold many
// files, which would overcount "items the user trashed".
//
// RiskAskBefore, matching the Trash/Recycle Bin policy on the other
// two platforms: this is user-staged data, even though trashing it
// already signaled intent to delete.
func freedesktopTrash(home string) (item.Item, bool) {
	filesDir := filepath.Join(home, ".local", "share", "Trash", "files")
	infoDir := filepath.Join(home, ".local", "share", "Trash", "info")

	bytes, _ := disk.DirSize(filesDir)
	if bytes <= 0 {
		return item.Item{}, false
	}

	count, oldest := summarizeTrashInfo(infoDir)
	return item.Item{
		Name:       "Papelera",
		NameKey:    "system.trash.name",
		Tool:       "trash",
		Path:       filesDir,
		Bytes:      bytes,
		Category:   item.CategorySystem,
		Risk:       item.RiskAskBefore,
		Detail:     "papelera del sistema; los archivos se borran de forma definitiva",
		DetailKey:  "system.trash.detail",
		DetailArgs: []any{count, daysSince(oldest)},
	}, true
}

// summarizeTrashInfo counts .trashinfo files (one per trashed item,
// regardless of whether the item itself is a file or a whole
// directory tree) and finds the oldest by file mtime. Falls back to
// (0, zero-time) if the info dir is unreadable — the caller still
// reports the Trash by its measured bytes even without a count.
func summarizeTrashInfo(infoDir string) (count int, oldest time.Time) {
	entries, err := os.ReadDir(infoDir)
	if err != nil {
		return 0, time.Time{}
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		count++
		info, err := e.Info()
		if err != nil {
			continue
		}
		mod := info.ModTime()
		if oldest.IsZero() || mod.Before(oldest) {
			oldest = mod
		}
	}
	return count, oldest
}

// scanTmpFiles reports /tmp contents older than tmpMaxAgeDays that
// belong to the current user. Unlike macOS (where launchd periodically
// sweeps /tmp automatically) and unlike systemd-tmpfiles on some
// distros, plenty of common Linux setups (older init systems,
// containers, WSL) never clean /tmp on their own, so old scratch files
// accumulate exactly like Windows' %TEMP% does.
//
// We deliberately do NOT walk all of /tmp indiscriminately: it's a
// shared, world-writable directory on a multi-user system, and other
// users' files there are off-limits by definition (not ours to
// report, let alone delete). We only report files owned by the
// current user's UID.
func scanTmpFiles() []item.Item {
	bytes, count, ok := summarizeOldOwnedFiles("/tmp", tmpMaxAgeDays)
	if !ok || bytes <= 0 || count == 0 {
		return nil
	}
	return []item.Item{{
		Name:       "Archivos temporales",
		Tool:       "temp",
		Path:       "/tmp",
		Bytes:      bytes,
		Category:   item.CategorySystem,
		Risk:       item.RiskSafe,
		Detail:     "archivos temporales antiguos en /tmp; el sistema los regenera",
		DetailKey:  "system.temp.detail",
		DetailArgs: []any{count, tmpMaxAgeDays},
	}}
}

// tmpMaxAgeDays mirrors the %TEMP% cutoff used on Windows.
var tmpMaxAgeDays = 7

// summarizeOldOwnedFiles walks dir and totals size/count of files
// older than maxDays AND owned by the current process's UID — the
// ownership check is what makes it safe to point this at a shared
// directory like /tmp instead of a per-user cache path.
func summarizeOldOwnedFiles(dir string, maxDays int) (bytes int64, count int, ok bool) {
	if _, err := os.Stat(dir); err != nil {
		return 0, 0, false
	}
	uid := os.Getuid()
	cutoff := time.Now().Add(-time.Duration(maxDays) * 24 * time.Hour)
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable node (permission denied on another user's files), keep walking
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !ownedByUID(info, uid) {
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

// thumbnailCache reports the GNOME/KDE-standard thumbnail cache under
// ~/.cache/thumbnails — the shared-desktop-spec equivalent of macOS's
// QuickLook thumbnailcache and Windows' thumbcache_*.db. File managers
// following the freedesktop.org thumbnail spec (Nautilus, Dolphin,
// Thunar) all write here regardless of which one generated a given
// thumbnail, so a single directory covers the desktop environment
// mistah is running under.
func thumbnailCache(home string) (item.Item, bool) {
	path := filepath.Join(home, ".cache", "thumbnails")
	bytes, _ := disk.DirSize(path)
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "Miniaturas",
		Tool:      "thumbcache",
		Path:      path,
		Bytes:     bytes,
		Category:  item.CategorySystem,
		Risk:      item.RiskSafe,
		Detail:    "caché de miniaturas; el gestor de archivos la regenera al navegar",
		DetailKey: "system.thumbcache.detail",
	}, true
}

// ownedByUID reports whether info's underlying syscall stat shows uid
// as the owner. Returns false (safe default: skip the file) if the
// platform-specific Sys() type assertion fails for any reason.
func ownedByUID(info os.FileInfo, uid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(stat.Uid) == uid
}
