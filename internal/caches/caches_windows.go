//go:build windows

package caches

import "path/filepath"

// platformPathCaches returns the simple path-based dev caches for
// Windows. All RiskSafe — these regenerate on demand.
//
// Path notes:
//   - Windows dev tools overwhelmingly cache under
//     %LOCALAPPDATA% (= <home>\AppData\Local), not %APPDATA%\Roaming,
//     because Local is excluded from domain roaming profiles — the
//     same "this is disposable" signal that ~/Library/Caches carries
//     on macOS. We express every path relative to home using that
//     convention so the same filepath.Join(home, rel) callsite in
//     caches.go works unchanged.
//   - Xcode entries have no Windows equivalent (Xcode is Apple-only)
//     and are intentionally omitted, not stubbed.
//   - NuGet has no macOS equivalent in the original table (.NET/NuGet
//     tooling is far more common on Windows) so it's added here as a
//     Windows-only addition rather than forcing a cross-platform entry
//     that would be empty on every Mac.
func platformPathCaches(home string) []pathCache {
	return []pathCache{
		{"npm", "npm cache", "npm", "AppData/Local/npm-cache/_cacache", "downloaded packages"},
		{"npm-npx", "npm npx cache", "npm", "AppData/Local/npm-cache/_npx", "one-shot npx executions"},
		{"npm-logs", "npm logs", "npm", "AppData/Local/npm-cache/_logs", "old install logs"},
		{"pnpm", "pnpm store", "pnpm", "AppData/Local/pnpm/store", "global content-addressable store"},
		{"yarn", "yarn cache", "yarn", "AppData/Local/Yarn/Cache", "downloaded packages"},
		{"yarn-berry", "yarn berry cache", "yarn", "AppData/Local/Yarn/Berry/cache", "downloaded packages (Yarn 2+)"},
		{"go", "Go build cache", "go", "AppData/Local/go-build", "compilation cache"},
		{"pip", "pip cache", "pip", "AppData/Local/pip/Cache", "wheels & http cache"},
		{"uv", "uv cache", "uv", "AppData/Local/uv/cache", "Python package cache"},
		{"composer", "Composer cache", "composer", "AppData/Local/Composer", "PHP packages"},
		{"node-gyp", "node-gyp cache", "node-gyp", "AppData/Local/node-gyp/Cache", "native build headers"},
		{"nuget-packages", "NuGet packages", "nuget", ".nuget/packages", "restored NuGet packages; re-downloaded on next restore"},
		{"nuget-http", "NuGet http cache", "nuget", "AppData/Local/NuGet/v3-cache", "NuGet HTTP response cache"},
	}
}

// jetBrainsRoot returns the directory JetBrains IDEs use for per-version
// cache folders on Windows. JetBrains splits config (Roaming) from
// cache/logs/plugins (Local) since the 2020.1 directory layout change;
// the version-tagged product folders that matter for size (indexes,
// caches, plugin downloads) live under Local.
func jetBrainsRoot(home string) string {
	return filepath.Join(home, "AppData", "Local", "JetBrains")
}
