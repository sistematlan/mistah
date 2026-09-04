//go:build darwin

package appcache

// platformEntries is the canonical consumer-app cache list for macOS.
// Adding an app is a one-line change: pick the correct bundle ID by
// checking ~/Library/Caches/<id> on a real Mac (or running `mdls -name
// kMDItemCFBundleIdentifier /Applications/<App>.app`).
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
func platformEntries() []entry {
	return []entry{
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
		// primary browser detector lives in browsers table with its own
		// table. Arc is included here because users treat it as an app,
		// not a browser.
		{
			bundleID:    "company.thebrowser.Browser",
			displayName: "Arc",
			relPath:     "Library/Caches/company.thebrowser.Browser",
		},
	}
}

// platformBrowsers is the canonical browser cache location list for
// macOS.
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
func platformBrowsers() []browserEntry {
	return []browserEntry{
		{tool: "browser-chrome", displayName: "Google Chrome", relPath: "Library/Caches/Google/Chrome"},
		{tool: "browser-safari", displayName: "Safari", relPath: "Library/Caches/com.apple.Safari"},
		{tool: "browser-firefox", displayName: "Firefox", relPath: "Library/Caches/Firefox"},
		{tool: "browser-firefox-legacy", displayName: "Firefox (legacy Mozilla path)", relPath: "Library/Caches/Mozilla"},
		{tool: "browser-brave", displayName: "Brave", relPath: "Library/Caches/BraveSoftware/Brave-Browser"},
		{tool: "browser-edge", displayName: "Microsoft Edge", relPath: "Library/Caches/Microsoft Edge"},
	}
}
