//go:build linux

package appcache

// platformEntries is the canonical consumer-app cache list for Linux.
//
// Path conventions on Linux:
//
//   - ~/.cache/<vendor-slug>            Most Electron apps (Slack,
//     Discord) follow the app name in lowercase as their cache dir
//     under $XDG_CACHE_HOME.
//   - ~/.config/<App>/Cache             A few apps interleave cache
//     under their config dir instead (Spotify does this); we point at
//     the specific Cache leaf, never the config root, mirroring the
//     "never touch profile data" discipline from the darwin/windows
//     tables.
//
// bundleID reuses the macOS bundle identifier string purely so Tool
// values and i18n keys stay consistent across all three platform
// tables — Linux has no bundle-id concept of its own.
func platformEntries() []entry {
	return []entry{
		// Music & media.
		{
			bundleID:    "com.spotify.client",
			displayName: "Spotify",
			relPath:     ".cache/spotify",
		},

		// Communication apps — Electron-based, tend to grow without bound.
		{
			bundleID:    "com.tinyspeck.slackmacgap",
			displayName: "Slack",
			relPath:     ".config/Slack/Cache",
		},
		{
			bundleID:    "com.tinyspeck.slackmacgap",
			displayName: "Slack",
			subLabel:    "Service Worker",
			relPath:     ".config/Slack/Service Worker/CacheStorage",
		},
		{
			bundleID:    "com.hnc.Discord",
			displayName: "Discord",
			relPath:     ".config/discord/Cache",
		},
		{
			bundleID:    "ru.keepcoder.Telegram",
			displayName: "Telegram",
			relPath:     ".local/share/TelegramDesktop/tdata/user_data/cache",
		},

		// Video conferencing.
		{
			bundleID:    "us.zoom.xos",
			displayName: "Zoom",
			relPath:     ".cache/zoom",
		},

		// Productivity & design.
		{
			bundleID:    "notion.id",
			displayName: "Notion",
			relPath:     ".config/Notion/Cache",
		},
		{
			bundleID:    "com.figma.Desktop",
			displayName: "Figma",
			relPath:     ".config/Figma/Cache",
		},
	}
}

// platformBrowsers is the canonical browser cache location list for
// Linux.
//
// Path notes:
//
//   - Chrome/Chromium/Brave/Edge (Chromium-based) all keep their disk
//     cache under the default profile's "Cache" leaf inside
//     ~/.config/<vendor>/<product>/Default/, same layout Chromium uses
//     on Windows, just rooted at ~/.config instead of %LOCALAPPDATA%.
//   - Firefox has the same random-profile-name problem here as on
//     Windows (see appcache_windows.go's comment) — omitted from the
//     fixed table for the same reason, tracked as a shared follow-up.
func platformBrowsers() []browserEntry {
	return []browserEntry{
		{tool: "browser-chrome", displayName: "Google Chrome", relPath: ".config/google-chrome/Default/Cache"},
		{tool: "browser-chromium", displayName: "Chromium", relPath: ".config/chromium/Default/Cache"},
		{tool: "browser-brave", displayName: "Brave", relPath: ".config/BraveSoftware/Brave-Browser/Default/Cache"},
		{tool: "browser-edge", displayName: "Microsoft Edge", relPath: ".config/microsoft-edge/Default/Cache"},
	}
}
