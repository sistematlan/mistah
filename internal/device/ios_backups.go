// Package device — iOS device backups.
//
// macOS keeps a full backup of every iPhone/iPad ever synced via Finder
// or iTunes under
//
//	~/Library/Application Support/MobileSync/Backup/<UDID>/
//
// Each UDID is a separate directory; users with three iPhones over five
// years will have three of them. They commonly grow to 4–15 GB each
// and are the single biggest reclaimable item on a non-dev Mac.
//
// They're also user data: a backup may be the only copy of a phone
// that was lost or factory-reset before iCloud sync. Risk is
// RiskAskBefore and the wizard NEVER auto-deletes them.

package device

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// ScanIOSBackups returns one item per iOS backup directory found under
// the user's home. Errors reading individual UDIDs are tolerated; we
// skip them rather than fail the whole detector.
func ScanIOSBackups(home string) []item.Item {
	root := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var items []item.Item
	for _, e := range entries {
		if !e.IsDir() {
			continue // Apple only writes directories here, but be safe
		}
		udid := e.Name()
		if !looksLikeUDID(udid) {
			continue
		}

		dir := filepath.Join(root, udid)
		bytes, _ := disk.DirSize(dir)
		if bytes <= 0 {
			continue
		}

		meta := readBackupMeta(dir)
		// Use the directory mtime as a fallback when the plist has no
		// "Last Backup Date". Apple usually populates that field, but
		// older backups and encrypted ones sometimes don't.
		if meta.LastBackup.IsZero() {
			if info, err := e.Info(); err == nil {
				meta.LastBackup = info.ModTime()
			}
		}

		items = append(items, item.Item{
			Name:       displayName(meta, udid),
			Tool:       "ios-backup",
			Path:       dir,
			Bytes:      bytes,
			Category:   item.CategoryDevice,
			Risk:       item.RiskAskBefore,
			Detail:     "backup completo del dispositivo; revisa antes de borrar",
			DetailKey:  "device.ios-backup.detail",
			DetailArgs: []any{deviceLabel(meta, udid), daysSince(meta.LastBackup)},
		})
	}
	return items
}

// looksLikeUDID returns true for directory names that resemble an Apple
// device UDID. Two formats exist in the wild:
//
//	Pre-iPhone X: 40 hex chars, no dashes      (e.g. 5e5a8c…3f9d)
//	Newer:        24 chars with one dash        (e.g. 00008101-001B…)
//
// We accept both shapes loosely; the goal is to filter out a stray
// .DS_Store or a user-created folder, not to validate Apple's format.
func looksLikeUDID(name string) bool {
	if len(name) < 24 {
		return false
	}
	// Reject hidden entries (.DS_Store, .localized) and obvious junk.
	if strings.HasPrefix(name, ".") {
		return false
	}
	// Must be hex+dash only. One non-allowed char and we bail.
	for _, r := range name {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

// backupMeta is the subset of Info.plist we care about. The plist has
// dozens of keys; we extract three.
type backupMeta struct {
	DeviceName  string    // "iPhone de Christian"
	ProductType string    // "iPhone15,2"
	LastBackup  time.Time // parsed from <date>2024-05-30T14:32:11Z</date>
}

// readBackupMeta parses Info.plist inside a backup directory. If the
// file is missing, malformed, or yields no useful keys, returns a zero
// value — the caller falls back to UDID and dir mtime.
//
// We never error: plist parsing failures are normal for encrypted or
// migrated backups, and a corrupt plist must NOT prevent the user from
// seeing the backup in the wizard.
func readBackupMeta(dir string) backupMeta {
	f, err := os.Open(filepath.Join(dir, "Info.plist"))
	if err != nil {
		return backupMeta{}
	}
	defer f.Close()
	return parseInfoPlist(f)
}

// parseInfoPlist scans an Apple XML plist linearly and pulls the keys
// we want. It does NOT build a full plist tree — that would require
// either a third-party library or 200 lines of recursive XML logic
// that we don't need.
//
// Algorithm:
//
//	On every <key>X</key> we remember X.
//	On the next <string>Y</string> or <date>Y</date> we record (X, Y).
//	Any other element resets the pending key.
//
// This is enough for top-level scalar pairs, which is what the keys we
// want all happen to be. Nested <dict> for things like "Applications"
// is skipped naturally because the next sibling isn't <string|date>.
//
// The function is exported only via readBackupMeta. Pure I/O — no
// filesystem state, no time.Now(); deterministic on a given input.
func parseInfoPlist(r io.Reader) backupMeta {
	dec := xml.NewDecoder(r)
	var meta backupMeta
	var pendingKey string

	for {
		tok, err := dec.Token()
		if err != nil {
			return meta
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "key":
				// Read the key's text content.
				var text string
				if err := dec.DecodeElement(&text, &t); err == nil {
					pendingKey = text
				}
			case "string":
				var text string
				if err := dec.DecodeElement(&text, &t); err == nil {
					applyMeta(&meta, pendingKey, text, "")
				}
				pendingKey = ""
			case "date":
				var text string
				if err := dec.DecodeElement(&text, &t); err == nil {
					applyMeta(&meta, pendingKey, "", text)
				}
				pendingKey = ""
			default:
				// Other element types (array, dict, integer, true, false)
				// reset the pending key; we don't track them.
				pendingKey = ""
			}
		}
	}
}

// applyMeta records a (key, value) pair into meta. Centralised so the
// list of keys we care about lives in one place.
//
// dateStr is the ISO-8601 form Apple writes ("2024-05-30T14:32:11Z").
// time.Parse with time.RFC3339 handles it; failures leave the field
// at zero, which the caller's mtime fallback covers.
func applyMeta(meta *backupMeta, key, str, dateStr string) {
	switch key {
	case "Device Name":
		meta.DeviceName = str
	case "Product Type":
		meta.ProductType = str
	case "Last Backup Date":
		if dateStr == "" {
			return
		}
		t, err := time.Parse(time.RFC3339, dateStr)
		if err == nil {
			meta.LastBackup = t
		}
	}
}

// displayName composes the Item.Name. Prefers the human-friendly
// device name; falls back to product type; finally falls back to a
// truncated UDID so the user has something to recognise.
//
//	"iPhone de Christian"          (best case)
//	"iPhone15,2"                    (no DeviceName but ProductType set)
//	"iOS device 5e5a8c…"            (plist unreadable)
func displayName(m backupMeta, udid string) string {
	if m.DeviceName != "" {
		return m.DeviceName
	}
	if m.ProductType != "" {
		return m.ProductType
	}
	if len(udid) > 8 {
		return "iOS device " + udid[:8] + "…"
	}
	return "iOS device " + udid
}

// deviceLabel is the string used in Detail messages: a slightly more
// descriptive form than displayName, including product type when both
// are known. "iPhone de Christian (iPhone15,2)" reads well enough that
// users recognise the device even if they renamed it cryptically.
func deviceLabel(m backupMeta, udid string) string {
	if m.DeviceName != "" && m.ProductType != "" {
		return fmt.Sprintf("%s (%s)", m.DeviceName, m.ProductType)
	}
	return displayName(m, udid)
}

// daysSince returns the number of full days between t and now. Negative
// values clamp to 0 to avoid clock-skew oddities ("backup hecho hace -3
// días"). A zero time returns 0.
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
