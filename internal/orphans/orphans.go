// Package orphans detects user data that belongs to apps no longer installed,
// or media caches that grew without bound. These items are NOT regular caches:
// they may hold user-visible content, so detectors mark them RiskAskBefore.
//
// Scan is the only OS-agnostic entry point here — the detectors
// themselves (which app, which path, how to tell "uninstalled" from
// "installed") are platform-specific and live in orphans_darwin.go /
// orphans_windows.go.
package orphans

import (
	"os"

	"github.com/sistematlan/mistah/internal/item"
)

// Scan inspects the system for orphaned data and returns the items found.
func Scan() ([]item.Item, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return scanHome(home), nil
}
