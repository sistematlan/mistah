//go:build darwin

package caches

import "path/filepath"

// platformPathCaches returns the simple path-based dev caches for macOS.
// All RiskSafe — these regenerate on demand.
func platformPathCaches(home string) []pathCache {
	return []pathCache{
		{"npm", "npm cache", "npm", ".npm/_cacache", "downloaded packages"},
		{"npm-npx", "npm npx cache", "npm", ".npm/_npx", "one-shot npx executions"},
		{"npm-logs", "npm logs", "npm", ".npm/_logs", "old install logs"},
		{"pnpm", "pnpm store", "pnpm", "Library/pnpm/store", "global content-addressable store"},
		{"yarn", "yarn cache", "yarn", ".yarn/cache", "downloaded packages"},
		{"brew", "Homebrew cache", "brew", "Library/Caches/Homebrew", "downloaded bottles & sources"},
		{"jetbrains", "JetBrains cache", "jetbrains", "Library/Caches/JetBrains", "indexes y logs"},
		{"go", "Go build cache", "go", "Library/Caches/go-build", "compilation cache"},
		{"pip", "pip cache", "pip", "Library/Caches/pip", "wheels & http cache"},
		{"uv", "uv cache", "uv", ".cache/uv", "Python package cache"},
		{"composer", "Composer cache", "composer", "Library/Caches/composer", "PHP packages"},
		{"node-gyp", "node-gyp cache", "node-gyp", "Library/Caches/node-gyp", "native build headers"},
		// Chrome and Firefox previously lived here under "browser cache".
		// They moved to internal/appcache/browsers.go (CategorySystem) so
		// they no longer count as "dev tooling" — a non-dev user with
		// Chrome installed shouldn't trip the dev-tools detector. Adding
		// either browser back here would re-introduce that bug.
		{"xcode-derived", "Xcode DerivedData", "xcode", "Library/Developer/Xcode/DerivedData", "build artifacts"},
		{"xcode-archives", "Xcode Archives", "xcode", "Library/Developer/Xcode/Archives", "old release archives"},
		{"xcode-ios-support", "iOS DeviceSupport", "xcode", "Library/Developer/Xcode/iOS DeviceSupport", "symbol files for old iOS versions"},
		{"xcode-simulator", "CoreSimulator caches", "xcode", "Library/Developer/CoreSimulator/Caches", "simulator caches"},
	}
}

// jetBrainsRoot returns the directory JetBrains IDEs use for per-version
// config/cache folders on macOS.
func jetBrainsRoot(home string) string {
	return filepath.Join(home, "Library", "Application Support", "JetBrains")
}
