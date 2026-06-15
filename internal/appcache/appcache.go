// Package appcache detects reclaimable cache directories of common
// consumer apps that ship outside the dev toolchain — Spotify, Slack,
// Discord, Zoom, Teams, Notion, Figma, Arc, Telegram, Linear and so on.
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
//   - The wizard filters Light by "RiskSafe + Path != ''" over inv.Caches.
//     We need these caches to live in CategorySystem so a future wizard
//     bucket "what every Mac has" can pull them without dragging dev
//     caches along on a no-dev machine.
//
// The catalog is data, not code: adding an app means adding one entry
// to the entries slice. No detector logic is per-app.
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
//	bundleID    — the macOS bundle identifier, used as the i18n key
//	              suffix and as the Tool field on the resulting Item.
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

// entries is the canonical list. Adding an app is a one-line change:
// pick the correct bundle ID by checking ~/Library/Caches/<id> on a
// real Mac (or running `mdls -name kMDItemCFBundleIdentifier /Applications/<App>.app`).
//
// Order is roughly "biggest typical cache first" so the menu reads
// naturally when sorted by size, but that's cosmetic — Scan() sorts
// nothing here, callers decide order.
//
// Path conventions on macOS:
//
//   - ~/Library/Caches/<bundle-id>            HTTP and asset caches; sandboxed apps.
//   - ~/Library/Application Support/<App>/Cache  Some Electron apps put cache here.
//   - ~/Library/Containers/<bundle-id>/Data/Library/Caches/  Sandboxed apps (newer macOS).
//
// We list the canonical macOS paths. Apps that store cache OUTSIDE
// these locations (e.g. Spotify's PersistentCache under Application
// Support) get an explicit entry pointing there.
var entries = []entry{
	// Music & media — typically the largest of consumer caches.
	{
		bundleID:    "com.spotify.client",
		displayName: "Spotify",
		relPath:     "Library/Caches/com.spotify.client",
	},
	{
		bundleID:    "com.spotify.client",
		displayName: "Spotify",
		subLabel:    "PersistentCache",
		relPath:     "Library/Application Support/Spotify/PersistentCache",
	},

	// Communication apps — Electron-based, tend to grow without bound.
	{
		bundleID:    "com.tinyspeck.slackmacgap",
		displayName: "Slack",
		relPath:     "Library/Caches/com.tinyspeck.slackmacgap",
	},
	{
		bundleID:    "com.tinyspeck.slackmacgap",
		displayName: "Slack",
		subLabel:    "Service Worker",
		relPath:     "Library/Application Support/Slack/Service Worker",
	},
	{
		bundleID:    "com.hnc.Discord",
		displayName: "Discord",
		relPath:     "Library/Caches/com.hnc.Discord",
	},
	{
		bundleID:    "com.hnc.Discord",
		displayName: "Discord",
		subLabel:    "Cache",
		relPath:     "Library/Application Support/discord/Cache",
	},
	{
		bundleID:    "ru.keepcoder.Telegram",
		displayName: "Telegram",
		relPath:     "Library/Caches/ru.keepcoder.Telegram",
	},
	{
		bundleID:    "ru.keepcoder.Telegram",
		displayName: "Telegram",
		subLabel:    "Group Containers media",
		relPath:     "Library/Group Containers/6N38VWS5BX.ru.keepcoder.Telegram/account-1/postbox/media",
	},

	// Video conferencing.
	{
		bundleID:    "us.zoom.xos",
		displayName: "Zoom",
		relPath:     "Library/Caches/us.zoom.xos",
	},
	{
		bundleID:    "com.microsoft.teams2",
		displayName: "Microsoft Teams",
		relPath:     "Library/Caches/com.microsoft.teams2",
	},
	{
		bundleID:    "com.microsoft.teams",
		displayName: "Microsoft Teams (legacy)",
		relPath:     "Library/Caches/com.microsoft.teams",
	},

	// Productivity & design.
	{
		bundleID:    "notion.id",
		displayName: "Notion",
		relPath:     "Library/Caches/notion.id",
	},
	{
		bundleID:    "com.figma.Desktop",
		displayName: "Figma",
		relPath:     "Library/Caches/com.figma.Desktop",
	},
	{
		bundleID:    "com.linear",
		displayName: "Linear",
		relPath:     "Library/Caches/com.linear",
	},

	// Browsers handled here too when they're "secondary" browsers; the
	// primary browser detector lives in PR 4 with its own table. Arc is
	// included here because users treat it as an app, not a browser.
	{
		bundleID:    "company.thebrowser.Browser",
		displayName: "Arc",
		relPath:     "Library/Caches/company.thebrowser.Browser",
	},
}

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
// Includes both the consumer-app caches in this file and the browser
// caches from browsers.go. Callers who need only one bucket can use
// ScanApps or ScanBrowsers directly.
func ScanHome(home string) []item.Item {
	items := ScanApps(home)
	items = append(items, ScanBrowsers(home)...)
	return items
}

// ScanApps reports only the consumer-app cache items (Spotify, Slack,
// Discord, Notion, etc.). Split out from ScanHome so the wizard can
// group apps and browsers as separate UI rows in PR 9 without having
// to re-filter a merged slice.
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

// formatName composes the display string for an Item. With no sublabel
// it's just the app name; with one we wrap the sublabel in parens so
// "Slack" and "Slack (Service Worker)" are visibly different rows.
func formatName(e entry) string {
	if e.subLabel == "" {
		return e.displayName
	}
	return e.displayName + " (" + e.subLabel + ")"
}
