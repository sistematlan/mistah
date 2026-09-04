//go:build windows

package wizard

import "path/filepath"

// devPresenceMarkers returns paths whose existence signals "this is a
// developer's machine" for DetectDevPresence, using Windows dev-tool
// cache conventions (see internal/caches/caches_windows.go for the
// same %LOCALAPPDATA% reasoning).
func devPresenceMarkers(home string) []string {
	return []string{
		filepath.Join(home, "AppData", "Local", "npm-cache"),
		filepath.Join(home, "AppData", "Local", "JetBrains"),
		filepath.Join(home, "AppData", "Local", "go-build"),
		filepath.Join(home, ".cargo"),
		filepath.Join(home, "go", "pkg"),
		filepath.Join(home, "AppData", "Local", "Docker"),
		`C:\Program Files\Docker\Docker\Docker Desktop.exe`,
	}
}
