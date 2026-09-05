//go:build linux

package wizard

import "path/filepath"

// devPresenceMarkers returns paths whose existence signals "this is a
// developer's machine" for DetectDevPresence, using Linux/XDG dev-tool
// cache conventions (see internal/caches/caches_linux.go for the same
// $XDG_CACHE_HOME reasoning).
func devPresenceMarkers(home string) []string {
	return []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, ".cache", "JetBrains"),
		filepath.Join(home, ".cache", "go-build"),
		filepath.Join(home, ".cargo"),
		filepath.Join(home, "go", "pkg"),
		filepath.Join(home, ".docker"),
		"/var/lib/docker",
	}
}
