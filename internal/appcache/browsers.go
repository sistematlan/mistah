package appcache

import (
	"path/filepath"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// browserEntry describes one cache directory for a desktop browser.
//
// Browsers get their own table (and their own Tool tag) for two reasons:
//
//  1. They share a category — "browser cache" reads better in the wizard
//     than 5 generic app rows mixed in with Spotify and Slack.
//  2. They have stricter path discipline than apps: a typo in a Spotify
//     entry costs 2 GB of regenerable cache; a typo in a Chrome entry
//     could nuke bookmarks. We keep the table tight to "Library/Caches"
//     leaves only — never anything under "Application Support" where
//     cookies and bookmarks live.
//
// Adding a browser:
//   - Verify the path is under ~/Library/Caches and contains "cache" or
//     a vendor folder (no "Default", "Profiles", "User Data" etc.).
//   - Run the existing tests; they enforce the path discipline above as
//     a regression guard (see TestBrowsers_PathDisciplineGuard).
type browserEntry struct {
	tool        string // stable identifier for the cleaner & i18n
	displayName string // user-visible name
	relPath     string // relative to home, must be a Library/Caches subtree
}

// browsers is the canonical list of browser cache locations.
//
// Path notes:
//
//   - Chrome and Brave keep their HTTP/asset cache under
//     ~/Library/Caches/<vendor>/<product>. The Application Support tree
//     also has a Cache, but it's interleaved with profile data; we leave
//     it alone.
//   - Firefox writes its disk cache under ~/Library/Caches/Firefox in
//     modern versions; older builds used ~/Library/Caches/Mozilla. We
//     list both because users migrate Macs without cleaning the old.
//   - Safari is the simplest: a single sandbox-named cache directory.
//   - Edge follows Chrome's convention but uses the literal product
//     name as the directory.
//
// Arc lives in the main appcache table (com.thebrowser.Browser) because
// users treat it as a chat-style app, not a traditional browser. Listing
// it here too would double-count its cache.
var browsers = []browserEntry{
	{tool: "browser-chrome", displayName: "Google Chrome", relPath: "Library/Caches/Google/Chrome"},
	{tool: "browser-safari", displayName: "Safari", relPath: "Library/Caches/com.apple.Safari"},
	{tool: "browser-firefox", displayName: "Firefox", relPath: "Library/Caches/Firefox"},
	{tool: "browser-firefox-legacy", displayName: "Firefox (legacy Mozilla path)", relPath: "Library/Caches/Mozilla"},
	{tool: "browser-brave", displayName: "Brave", relPath: "Library/Caches/BraveSoftware/Brave-Browser"},
	{tool: "browser-edge", displayName: "Microsoft Edge", relPath: "Library/Caches/Microsoft Edge"},
}

// ScanBrowsers reports browser cache items below a home directory.
// Same threshold and risk policy as the rest of the appcache package:
// items under minCacheBytes are dropped, all results are RiskSafe and
// CategorySystem.
//
// Exposed separately from ScanHome so the cmd layer (or the wizard)
// can list browsers in their own group when that lands in PR 9.
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
