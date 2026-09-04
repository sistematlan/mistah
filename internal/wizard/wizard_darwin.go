//go:build darwin

package wizard

import "path/filepath"

// devPresenceMarkers returns paths whose existence signals "this is a
// developer's Mac" for DetectDevPresence.
func devPresenceMarkers(home string) []string {
	return []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, "Library", "Caches", "Homebrew"),
		filepath.Join(home, "Library", "Caches", "JetBrains"),
		filepath.Join(home, "Library", "Developer", "Xcode"),
		"/Applications/Docker.app",
		filepath.Join(home, ".cargo"),
		filepath.Join(home, "go", "pkg"),
	}
}
