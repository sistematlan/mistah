//go:build linux

package device

import "github.com/sistematlan/mistah/internal/item"

// mobileSyncBackupRoot has no meaningful answer on Linux: there is no
// official iTunes/Apple Devices client. The community alternative
// (libimobiledevice's `idevicebackup2`) lets a user choose an
// arbitrary output directory per invocation rather than writing to one
// standard location the way iTunes does — so there's no single path
// this detector could point at with any confidence. Returning "" makes
// ScanIOSBackups's ReadDir call fail harmlessly (same as "directory
// doesn't exist" on the other platforms), which is the correct
// behaviour: nothing to report, not an error.
func mobileSyncBackupRoot(home string) string {
	return ""
}

// scanIPSW returns nil on Linux for the same reason: .ipsw firmware
// caching is an iTunes-specific behaviour with no standard location
// (or even a standard client) on this platform.
func scanIPSW(home string) []item.Item {
	return nil
}
