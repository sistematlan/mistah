// Package device detects reclaimable data tied to devices the user has
// synced to this Mac: iOS device backups, .ipsw firmware caches, and
// similar artifacts that live in ~/Library and ~/Library/iTunes.
//
// Items here are CategoryDevice. Risk varies: .ipsw files are RiskSafe
// because Apple re-serves them on demand; full device backups under
// MobileSync are RiskAskBefore because they may be the only copy of an
// iPhone the user no longer owns or can access.
//
// This package only handles the fast, low-risk detectors. iOS device
// backups (which require parsing Info.plist) live in their own file.
package device

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sistematlan/mistah/internal/item"
)

// Scan returns every device-related item the package detects.
func Scan() ([]item.Item, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	items := scanIPSW(home)
	items = append(items, ScanIOSBackups(home)...)
	return items, nil
}

// ScanIPSW exposes the .ipsw detector for tests and for direct use by
// the cmd layer when only this slice is wanted. The home parameter
// makes the function deterministic in tests.
func ScanIPSW(home string) []item.Item {
	return scanIPSW(home)
}

// scanIPSW lists .ipsw firmware archives stored under
// ~/Library/iTunes/iPhone Software Updates/. macOS keeps them there
// after a device update so a redo doesn't re-download; Apple re-serves
// them anytime, so the user can safely free the space.
//
// Each .ipsw becomes its own item: users often have one per
// device/version, and listing them separately lets the wizard show a
// realistic per-device line ("iPhone 15 17.4 — 6.2 GB").
func scanIPSW(home string) []item.Item {
	root := filepath.Join(home, "Library", "iTunes", "iPhone Software Updates")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil // missing dir is normal, not an error
	}

	var items []item.Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".ipsw") {
			continue
		}
		full := filepath.Join(root, name)
		info, err := e.Info()
		if err != nil || info.Size() <= 0 {
			continue
		}
		device, version := parseIPSWName(name)
		items = append(items, item.Item{
			Name:       name,
			Tool:       "ios-update",
			Path:       full,
			Bytes:      info.Size(),
			Category:   item.CategoryDevice,
			Risk:       item.RiskSafe,
			Detail:     "actualización de iOS; Apple la vuelve a ofrecer al actualizar",
			DetailKey:  "device.ipsw.detail",
			DetailArgs: []any{device, version},
		})
	}
	return items
}

// parseIPSWName extracts the device family and version from a typical
// .ipsw filename. Apple's format is reasonably stable:
//
//	iPhone15,2_17.4_21E219_Restore.ipsw
//	iPad13,4_17.4_21E219_Restore.ipsw
//	AppleTV5,3_17.4_21E219_Restore.ipsw
//
// We split on "_" and read fields by position. The function never errors;
// unrecognised names fall back to the raw filename and an empty version,
// which the i18n template can choose to display as "actualización de iOS".
//
// Returning two strings instead of a richer struct keeps the detector
// honest: anything more elaborate (model name lookup) belongs in a
// dedicated mapping table, not in filename parsing.
func parseIPSWName(name string) (device, version string) {
	stem := strings.TrimSuffix(strings.TrimSuffix(name, ".ipsw"), ".IPSW")
	parts := strings.Split(stem, "_")
	if len(parts) < 2 {
		return name, ""
	}
	return parts[0], parts[1]
}
