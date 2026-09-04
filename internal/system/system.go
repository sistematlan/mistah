// Package system detects reclaimable data managed by the operating system
// itself or by applications that ship with it: the Trash/Recycle Bin,
// Mail/Outlook downloads, thumbnail caches, filesystem snapshots, system
// logs, etc.
//
// Unlike caches/, items here are NOT dev-specific. They exist on every
// machine the moment a user starts using the system. Detectors mark them
// CategorySystem so the wizard can group them under the "general" bucket
// independently of dev tooling.
//
// Risk policy:
//   - Anything trivially regenerable (thumbnails, mail downloads) is RiskSafe.
//   - The Trash/Recycle Bin is RiskAskBefore: it's user-staged data, even if
//     the user's own act of trashing implied intent to delete.
//
// Platform split: Scan/ScanHome here are the only OS-agnostic entry points.
// The actual detectors live in system_darwin.go and system_windows.go
// (selected via Go build tags) because Trash vs Recycle Bin, Mail vs
// Outlook, and QuickLook vs thumbcache have no shared implementation —
// only a shared shape (item.Item) and a shared contract (best-effort,
// never fail the whole scan for one broken detector).
package system

import (
	"os"
	"sort"
	"time"

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
