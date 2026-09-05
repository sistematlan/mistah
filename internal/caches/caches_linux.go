//go:build linux

package caches

import "path/filepath"

// platformPathCaches returns the simple path-based dev caches for
// Linux, following the XDG Base Directory spec that most modern CLI
// tools respect: $XDG_CACHE_HOME (default ~/.cache) for disposable
// cache data — the same "safe to delete" signal ~/Library/Caches
// carries on macOS and %LOCALAPPDATA% carries on Windows.
//
// We hardcode ~/.cache rather than reading $XDG_CACHE_HOME because:
//   - The vast majority of installs never set it and get the default.
//   - Reading it correctly would mean falling back to ~/.cache anyway
//     when unset, and Item.Path needs a concrete path regardless.
//
// A future enhancement could honor $XDG_CACHE_HOME when set; tracked
// in BACKLOG.md alongside the Firefox-on-Windows profile gap.
func platformPathCaches(home string) []pathCache {
	return []pathCache{
		{"npm", "npm cache", "npm", ".npm/_cacache", "downloaded packages"},
		{"npm-npx", "npm npx cache", "npm", ".npm/_npx", "one-shot npx executions"},
		{"npm-logs", "npm logs", "npm", ".npm/_logs", "old install logs"},
		{"pnpm", "pnpm store", "pnpm", ".local/share/pnpm/store", "global content-addressable store"},
		{"yarn", "yarn cache", "yarn", ".cache/yarn", "downloaded packages"},
		{"yarn-berry", "yarn berry cache", "yarn", ".yarn/berry/cache", "downloaded packages (Yarn 2+)"},
		{"go", "Go build cache", "go", ".cache/go-build", "compilation cache"},
		{"pip", "pip cache", "pip", ".cache/pip", "wheels & http cache"},
		{"uv", "uv cache", "uv", ".cache/uv", "Python package cache"},
		{"composer", "Composer cache", "composer", ".cache/composer", "PHP packages"},
		{"node-gyp", "node-gyp cache", "node-gyp", ".cache/node-gyp", "native build headers"},
		// No Homebrew/Xcode entries here: Homebrew on Linux (linuxbrew)
		// exists but is rare enough outside dev containers that we skip
		// it until there's a real signal users want it; Xcode has no
		// Linux build at all.
	}
}

// jetBrainsRoot returns the directory JetBrains IDEs use for
// per-version cache folders on Linux: ~/.cache/JetBrains, mirroring
// the "cache/logs/plugins go in the OS cache dir, not config" split
// JetBrains uses on every platform since the 2020.1 layout change.
func jetBrainsRoot(home string) string {
	return filepath.Join(home, ".cache", "JetBrains")
}
