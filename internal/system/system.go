// Package system detects reclaimable data managed by macOS itself or by
// applications that ship with the OS: the Trash, Mail downloads, QuickLook
// thumbnails, Time Machine local snapshots, system logs, etc.
//
// Unlike caches/, items here are NOT dev-specific. They exist on every
// Mac the moment a user starts using the system. Detectors mark them
// CategorySystem so the wizard can group them under the "general" bucket
// independently of dev tooling.
//
// Risk policy:
//   - Anything trivially regenerable (thumbnails, mail downloads) is RiskSafe.
//   - The Trash is RiskAskBefore: it's user-staged data, even if the user's
//     own act of trashing implied intent to delete.
package system

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// Scan inspects the system for general-audience reclaimable data and
// returns the items found. Each detector is independent: a failure in one
// must not stop the rest.
func Scan() ([]item.Item, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return ScanHome(home), nil
}

// ScanHome runs the detectors against an arbitrary home directory.
// Used by tests with a TempDir; production code uses Scan() which
// resolves the real home. Splitting the entry points keeps tests
// hermetic without forcing every detector to take an extra parameter.
func ScanHome(home string) []item.Item {
	var items []item.Item
	if it, ok := trash(home); ok {
		items = append(items, it)
	}
	if it, ok := mailDownloads(home); ok {
		items = append(items, it)
	}
	if it, ok := quicklookThumbnails(home); ok {
		items = append(items, it)
	}
	items = append(items, scanLogs(home)...)
	items = append(items, scanCrashReports(home)...)
	items = append(items, scanSnapshots()...)
	return items
}

// trash reports the user's Trash. We measure it as a single Item even
// though the Trash holds many distinct files; the cleaner uses
// TrashContentsRemover (see remover.go) to wipe children while keeping
// the ~/.Trash directory itself alive — Finder gets confused if its
// canonical path disappears.
//
// External volume trashes (/Volumes/*/.Trashes/<uid>) are intentionally
// out of scope here: they require iterating /Volumes, deal with mounted
// network shares that may stall, and tend to belong to other users on
// shared disks. Skip cleanly until we have a story for them.
func trash(home string) (item.Item, bool) {
	trashPath := filepath.Join(home, ".Trash")
	bytes, count, oldest, ok := summarizeTrash(trashPath)
	if !ok || bytes <= 0 {
		return item.Item{}, false
	}
	args := []any{count, daysSince(oldest)}
	return item.Item{
		Name:       "Trash",
		NameKey:    "system.trash.name",
		Tool:       "trash",
		Path:       trashPath,
		Bytes:      bytes,
		Category:   item.CategorySystem,
		Risk:       item.RiskAskBefore,
		Detail:     "papelera del sistema; los archivos se borran de forma definitiva",
		DetailKey:  "system.trash.detail",
		DetailArgs: args,
	}, true
}

// summarizeTrash returns the total size, item count and oldest mtime of
// the Trash. Hidden files (.DS_Store, .localized) are excluded from the
// count but their bytes still count — we want the user to see the same
// "free space recovered" number Finder would.
//
// The function is forgiving on errors: an unreadable child contributes
// 0 bytes instead of failing the whole detector.
func summarizeTrash(path string) (bytes int64, count int, oldest time.Time, ok bool) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, time.Time{}, false
	}
	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(path, name)
		size := pathSize(full, e.IsDir())
		bytes += size
		// Hidden FS bookkeeping files don't count as user "items".
		if name == ".DS_Store" || name == ".localized" {
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
	return bytes, count, oldest, true
}

// mailDownloads reports the cache where Mail.app stores attachments the
// user opened from a message. The originals stay in the IMAP/POP server
// or in the on-disk mail store; this folder is purely a download cache
// that Mail.app re-creates on demand. Safe.
func mailDownloads(home string) (item.Item, bool) {
	path := filepath.Join(home,
		"Library", "Containers", "com.apple.mail",
		"Data", "Library", "Mail Downloads")
	bytes, _ := disk.DirSize(path)
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "Mail Downloads",
		NameKey:   "system.mail-downloads.name",
		Tool:      "mail",
		Path:      path,
		Bytes:     bytes,
		Category:  item.CategorySystem,
		Risk:      item.RiskSafe,
		Detail:    "adjuntos descargados de Mail; se vuelven a bajar al abrirlos",
		DetailKey: "system.mail-downloads.detail",
	}, true
}

// quicklookThumbnails reports the QuickLook cache, which macOS uses to
// show thumbnails when previewing files in Finder/Spotlight.
//
// The user-scoped path is well-known. The system-scoped path under
// /var/folders/<a>/<b>/C/com.apple.QuickLook.thumbnailcache lives at a
// hash that depends on the user; we leave it for a future enhancement
// (we'd need to call getconf DARWIN_USER_CACHE_DIR or read TMPDIR-like
// env vars carefully). The user-scoped one alone usually accounts for
// the bulk of the cache.
func quicklookThumbnails(home string) (item.Item, bool) {
	path := filepath.Join(home,
		"Library", "Caches", "com.apple.QuickLook.thumbnailcache")
	bytes, _ := disk.DirSize(path)
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "QuickLook thumbnails",
		NameKey:   "system.quicklook.name",
		Tool:      "quicklook",
		Path:      path,
		Bytes:     bytes,
		Category:  item.CategorySystem,
		Risk:      item.RiskSafe,
		Detail:    "miniaturas de QuickLook; macOS las regenera al previsualizar",
		DetailKey: "system.quicklook.detail",
	}, true
}

// pathSize returns the size of a file or directory, swallowing errors as 0.
// Centralised so all detectors here use the same fallback policy.
func pathSize(path string, isDir bool) int64 {
	if isDir {
		bytes, _ := disk.DirSize(path)
		return bytes
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// daysSince returns the number of days between t and now, or 0 if t is zero.
// Used in Detail args ("12 items, oldest 47 days old"). Negative values are
// clamped to 0 so a clock skew never produces "-3 days" in the UI.
func daysSince(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	d := int(time.Since(t).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// SortByBytesDesc orders items largest first. Convenience for the cmd
// layer; not used by Scan itself so callers can decide their own order.
func SortByBytesDesc(items []item.Item) {
	sort.Slice(items, func(i, j int) bool { return items[i].Bytes > items[j].Bytes })
}
