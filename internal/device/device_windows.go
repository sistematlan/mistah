//go:build windows

package device

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sistematlan/mistah/internal/item"
)

// mobileSyncBackupRoot returns the directory where iTunes / Apple
// Devices stores full iOS device backups on Windows. Both the legacy
// iTunes (Microsoft Store or classic installer) and the newer "Apple
// Devices" app write backups to this same MobileSync path under
// Roaming AppData.
func mobileSyncBackupRoot(home string) string {
	return filepath.Join(home, "AppData", "Roaming", "Apple Computer", "MobileSync", "Backup")
}

// scanIPSW lists .ipsw firmware archives cached by iTunes on Windows.
// Classic iTunes stores them under "Apple Computer\iTunes\iPhone
// Software Updates"; re-downloading costs bandwidth but Apple always
// re-serves them, so this is safe to reclaim — same policy as macOS.
func scanIPSW(home string) []item.Item {
	root := filepath.Join(home, "AppData", "Roaming", "Apple Computer", "iTunes", "iPhone Software Updates")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil // missing dir is normal (iTunes not installed, or nothing cached)
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
// .ipsw filename. Mirrors the darwin implementation exactly — Apple's
// naming convention doesn't vary by host OS.
func parseIPSWName(name string) (device, version string) {
	stem := strings.TrimSuffix(strings.TrimSuffix(name, ".ipsw"), ".IPSW")
	parts := strings.Split(stem, "_")
	if len(parts) < 2 {
		return name, ""
	}
	return parts[0], parts[1]
}
