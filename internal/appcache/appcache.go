// Package appcache detects reclaimable cache directories of common
// consumer apps that ship outside the dev toolchain — Spotify, Slack,
// Discord, Zoom, Teams, Notion, Figma, Arc, Telegram, Linear and so on
// — plus desktop browser caches (Chrome, Firefox, Edge, etc.).
//
// These are not orphans (the apps are still installed) and not dev
// caches (the user is not a developer of the app). They are first-class
// caches that the apps regenerate on demand, just like a browser cache.
// All items here are RiskSafe and CategorySystem so the wizard's Light
// level can include them in a single up-front confirmation.
//
// Why a separate package and not a row in caches/?
//
//   - caches/ today is the dev-tools list (npm, brew, pip, JetBrains,
//     Xcode, Go, Composer…). Mixing Spotify there would erode that
//     contract.
//   - The wizard filters Light by "RiskSafe + Path != ”" over inv.Caches.
//     We need these caches to live in CategorySystem so a future wizard
//     bucket "what every Mac has" can pull them without dragging dev
//     caches along on a no-dev machine.
//
// The catalog is data, not code: adding an app means adding one entry
// to the platform-specific entries table (appcache_darwin.go /
// appcache_windows.go). No detector logic is per-app or per-OS — only
// the path convention differs (~/Library/Caches/<bundle-id> vs
// %LOCALAPPDATA%\<Vendor>\<App>\Cache).
package appcache

import (
	"os"
	"path/filepath"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// minCacheBytes filters out tiny app caches that would clutter the wizard
// list without giving the user meaningful space back. 10 MB matches the
// "noise threshold" used elsewhere in the codebase for general-audience
// detectors.
//
// Mutable so tests can lower it without seeding 10 MB of fake cache data.
// Production code never writes to it.
var minCacheBytes int64 = 10 * 1024 * 1024

// entry describes one cache location for one app. An app may declare
// several entries (Slack keeps cache in two places); each becomes its
// own item if it has measurable bytes.
//
// Fields:
//
//	bundleID    — a stable per-app identifier used as the i18n key
//	              suffix and as the Tool field on the resulting Item.
//	              On macOS this is the real bundle identifier; on
//	              Windows there's no equivalent concept, so we reuse a
//	              vendor-style dotted string for consistency (e.g.
//	              "com.spotify.client" on both platforms) so i18n keys
//	              and Tool comparisons work identically cross-platform.
//	displayName — what the user sees in the menu. Localised via i18n
//	              when "appcache.<bundleID>.name" exists; this is the
//	              fallback for missing translations.
//	subLabel    — distinguishes multiple entries from the same app.
//	              Empty when the app has only one path. When non-empty,
//	              appended to the display name as " (sublabel)" so the
//	              user can tell two Slack rows apart.
//	relPath     — path relative to the user's home directory. Joined
//	              with home via filepath.Join.
type entry struct {
	bundleID    string
	displayName string
	subLabel    string
	relPath     string
}

// entries is the canonical list for the running OS, provided by
// appcache_darwin.go or appcache_windows.go via platformEntries().
var entries = platformEntries()

// browserEntry describes one cache directory for a desktop browser.
// Kept as its own table (see browsers table in the platform files) for
// two reasons:
//
//  1. They share a category — "browser cache" reads better in the wizard
//     than 5 generic app rows mixed in with Spotify and Slack.
//  2. They have stricter path discipline than apps: a typo in a Spotify
//     entry costs 2 GB of regenerable cache; a typo in a Chrome entry
//     could nuke bookmarks. The path convention for "just the cache,
//     never the profile" differs per OS but the discipline is the same.
type browserEntry struct {
	tool        string // stable identifier for the cleaner & i18n
	displayName string // user-visible name
	relPath     string // relative to home; must be a pure cache leaf, never profile data
}

// browsers is the canonical browser cache list for the running OS.
var browsers = platformBrowsers()

// Scan inspects the caches of every app in the catalog and returns the
// items found. Apps that aren't installed produce no entries — we don't
// detect installations, just measure caches.
func Scan() ([]item.Item, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return ScanHome(home), nil
}

// ScanHome is Scan with an explicit home directory, for tests using
// t.TempDir(). Production code uses Scan().
//
// Includes both the consumer-app caches and the browser caches.
// Callers who need only one bucket can use ScanApps or ScanBrowsers
// directly.
func ScanHome(home string) []item.Item {
	items := ScanApps(home)
	items = append(items, ScanBrowsers(home)...)
	return items
}

// ScanApps reports only the consumer-app cache items (Spotify, Slack,
// Discord, Notion, etc.). Split out from ScanHome so the wizard can
// group apps and browsers as separate UI rows without having to
// re-filter a merged slice.
func ScanApps(home string) []item.Item {
	var items []item.Item
	for _, e := range entries {
		path := filepath.Join(home, e.relPath)
		bytes, _ := disk.DirSize(path)
		if bytes < minCacheBytes {
			continue
		}
		items = append(items, item.Item{
			Name:       formatName(e),
			Tool:       e.bundleID,
			Path:       path,
			Bytes:      bytes,
			Category:   item.CategorySystem,
			Risk:       item.RiskSafe,
			Detail:     "caché de " + e.displayName + "; la app la regenera al usarse",
			DetailKey:  "appcache.detail",
			DetailArgs: []any{e.displayName},
		})
	}
	return items
}

// ScanBrowsers reports browser cache items below a home directory.
// Same threshold and risk policy as ScanApps: items under minCacheBytes
// are dropped, all results are RiskSafe and CategorySystem.
func ScanBrowsers(home string) []item.Item {
	var items []item.Item
	for _, b := range browsers {
		path := filepath.Join(home, b.relPath)
		bytes, _ := disk.DirSize(path)
		if bytes < minCacheBytes {
			continue
		}
		items = append(items, item.Item{
			Name:       b.displayName,
			Tool:       b.tool,
			Path:       path,
			Bytes:      bytes,
			Category:   item.CategorySystem,
			Risk:       item.RiskSafe,
			Detail:     "caché de " + b.displayName + "; el navegador la regenera al navegar",
			DetailKey:  "appcache.browser.detail",
			DetailArgs: []any{b.displayName},
		})
	}
	return items
}

// formatName composes the display string for an Item. With no sublabel
// it's just the app name; with one we wrap the sublabel in parens so
// "Slack" and "Slack (Service Worker)" are visibly different rows.
func formatName(e entry) string {
	if e.subLabel == "" {
		return e.displayName
	}
	return e.displayName + " (" + e.subLabel + ")"
}
