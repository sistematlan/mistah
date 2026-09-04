//go:build windows

package appcache

// platformEntries is the canonical consumer-app cache list for Windows.
//
// Path conventions on Windows:
//
//   - %LOCALAPPDATA%\<Vendor>\<App>\Cache          Electron/Chromium apps
//     (Slack, Discord, Teams). Local (not Roaming) because caches are
//     machine-specific and excluded from domain roaming profiles — the
//     same signal ~/Library/Caches carries on macOS.
//   - %APPDATA%\<Vendor>\<App>\...                 A few legacy Win32
//     apps still write cache under Roaming; called out per-entry below.
//   - There is no Windows "bundle identifier" concept, so bundleID here
//     is a synthetic dotted string reusing the macOS bundle ID where one
//     naturally exists (keeps Tool/i18n keys stable cross-platform) or a
//     new one following the same convention otherwise.
//
// AppData/Local is expressed relative to home so the same
// filepath.Join(home, relPath) call in appcache.go works unchanged —
// %LOCALAPPDATA% is always "<home>\AppData\Local" for the current user.
func platformEntries() []entry {
	return []entry{
		// Music & media.
		{
			bundleID:    "com.spotify.client",
			displayName: "Spotify",
			relPath:     "AppData/Local/Spotify/Storage",
		},
		{
			bundleID:    "com.spotify.client",
			displayName: "Spotify",
			subLabel:    "Data",
			relPath:     "AppData/Local/Spotify/Data",
		},

		// Communication apps — Electron-based, tend to grow without bound.
		{
			bundleID:    "com.tinyspeck.slackmacgap",
			displayName: "Slack",
			relPath:     "AppData/Roaming/Slack/Cache",
		},
		{
			bundleID:    "com.tinyspeck.slackmacgap",
			displayName: "Slack",
			subLabel:    "Service Worker",
			relPath:     "AppData/Roaming/Slack/Service Worker/CacheStorage",
		},
		{
			bundleID:    "com.hnc.Discord",
			displayName: "Discord",
			relPath:     "AppData/Roaming/discord/Cache",
		},
		{
			bundleID:    "com.hnc.Discord",
			displayName: "Discord",
			subLabel:    "Code Cache",
			relPath:     "AppData/Roaming/discord/Code Cache",
		},
		{
			bundleID:    "ru.keepcoder.Telegram",
			displayName: "Telegram",
			relPath:     "AppData/Roaming/Telegram Desktop/tdata/user_data/cache",
		},

		// Video conferencing.
		{
			bundleID:    "us.zoom.xos",
			displayName: "Zoom",
			relPath:     "AppData/Roaming/Zoom/data",
		},
		{
			bundleID:    "com.microsoft.teams2",
			displayName: "Microsoft Teams",
			relPath:     "AppData/Local/Packages/MSTeams_8wekyb3d8bbwe/LocalCache",
		},
		{
			bundleID:    "com.microsoft.teams",
			displayName: "Microsoft Teams (legacy)",
			relPath:     "AppData/Roaming/Microsoft/Teams/Cache",
		},

		// Productivity & design.
		{
			bundleID:    "notion.id",
			displayName: "Notion",
			relPath:     "AppData/Roaming/Notion/Cache",
		},
		{
			bundleID:    "com.figma.Desktop",
			displayName: "Figma",
			relPath:     "AppData/Roaming/Figma/Cache",
		},
	}
}

// platformBrowsers is the canonical browser cache location list for
// Windows.
//
// Path notes:
//
//   - Chrome/Brave/Edge (all Chromium-based) keep their disk cache under
//     the default profile's "Cache" folder inside User Data — we point
//     at that leaf specifically (never at the User Data root, which
//     holds bookmarks/cookies/login data) to preserve the same path
//     discipline the macOS table enforces.
//   - Firefox uses a randomly-named profile folder; we point at the
//     "cache2" directory pattern is not enumerable without reading
//     profiles.ini, so Firefox on Windows is intentionally omitted from
//     this fixed table rather than guessing a profile name. (Tracked
//     as a known gap — see BACKLOG.md.)
func platformBrowsers() []browserEntry {
	return []browserEntry{
		{tool: "browser-chrome", displayName: "Google Chrome", relPath: "AppData/Local/Google/Chrome/User Data/Default/Cache"},
		{tool: "browser-chrome-code-cache", displayName: "Google Chrome (Code Cache)", relPath: "AppData/Local/Google/Chrome/User Data/Default/Code Cache"},
		{tool: "browser-edge", displayName: "Microsoft Edge", relPath: "AppData/Local/Microsoft/Edge/User Data/Default/Cache"},
		{tool: "browser-edge-code-cache", displayName: "Microsoft Edge (Code Cache)", relPath: "AppData/Local/Microsoft/Edge/User Data/Default/Code Cache"},
		{tool: "browser-brave", displayName: "Brave", relPath: "AppData/Local/BraveSoftware/Brave-Browser/User Data/Default/Cache"},
	}
}
