// Package device detects reclaimable data tied to devices the user has
// synced to this computer: iOS device backups and .ipsw firmware
// caches from iTunes / Apple Devices.
//
// Item Risk varies: .ipsw files are RiskSafe because Apple re-serves
// them on demand; full device backups are RiskAskBefore because they
// may be the only copy of an iPhone the user no longer owns or can
// access.
//
// Only the root paths differ per OS (~/Library/... on macOS vs
// %APPDATA%\Apple Computer\... on Windows, where iTunes/Apple Devices
// stores the same data). The plist-parsing and backup-metadata logic
// in ios_backups.go is 100% shared.
package device

import (
	"os"

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
