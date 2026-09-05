//go:build linux

package apps

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
)

// desktopDirs lists the standard freedesktop.org locations for .desktop
// entry files — the closest Linux analogue to macOS's .app bundles and
// Windows' registry Uninstall keys. System-wide entries live under
// /usr/share/applications (and /usr/local/share for locally-built
// packages); per-user installs (Flatpak user installs, AppImage
// launchers some tools create) land under ~/.local/share/applications.
var desktopDirs = []string{
	"/usr/share/applications",
	"/usr/local/share/applications",
}

// List enumerates installed applications by parsing .desktop entry
// files, the standard Linux mechanism every major desktop environment
// (GNOME, KDE, XFCE) and application menu reads.
//
// Sizing and "last used" both have weaker signals on Linux than on
// macOS/Windows:
//
//   - Bytes: a .desktop file only points at an Exec= binary path, not
//     an install directory — there's no single directory to
//     disk.DirSize the way /Applications/Foo.app or a registry
//     InstallLocation gives us. We report the size of the referenced
//     executable itself (rarely meaningful, usually tiny) rather than
//     guess at a package's full footprint; a future enhancement could
//     shell out to the system package manager (dpkg -L, rpm -ql) to
//     get accurate installed-file totals, but that's package-manager
//     specific and out of scope for this pass.
//   - Last used: same absence of a system-wide "last opened" registry
//     as Windows. We fall back to the .desktop file's own mtime as a
//     weak proxy (it changes on package upgrade, not on launch) —
//     honest but not very informative; flagged in BACKLOG.md as a
//     known gap rather than silently reporting fake precision.
func List() ([]App, error) {
	var apps []App
	seen := map[string]bool{}

	dirs := append([]string{}, desktopDirs...)
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "applications"))
	}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			full := filepath.Join(dir, e.Name())
			de, ok := parseDesktopEntry(full)
			if !ok || de.Name == "" || de.NoDisplay || de.Hidden {
				continue // skip malformed entries and ones the desktop itself hides
			}
			if seen[de.Name] {
				continue
			}
			seen[de.Name] = true

			execPath := resolveExecPath(de.Exec)
			var bytes int64
			var lastUsed time.Time
			neverUsed := true
			if execPath != "" {
				bytes, _ = disk.DirSize(execPath)
				if info, err := os.Stat(execPath); err == nil {
					lastUsed = info.ModTime()
					neverUsed = false
				}
			}
			days := -1
			if !neverUsed {
				days = int(time.Since(lastUsed).Hours() / 24)
			}

			apps = append(apps, App{
				Name:         de.Name,
				Path:         execPath,
				Bytes:        bytes,
				LastUsed:     lastUsed,
				NeverUsed:    neverUsed,
				DaysSinceUse: days,
			})
		}
	}

	return apps, nil
}

// desktopEntry is the subset of a .desktop file's [Desktop Entry]
// section that List() needs.
type desktopEntry struct {
	Name      string
	Exec      string
	NoDisplay bool
	Hidden    bool
}

// parseDesktopEntry does a minimal line-based parse of a .desktop
// file's [Desktop Entry] group. We intentionally don't pull in a full
// INI parser: the format is simple enough (key=value, # comments,
// [Group Name] headers) that a linear scan covers everything List()
// needs, and stopping at the first non-"[Desktop Entry]" group header
// avoids accidentally reading a localized [Desktop Entry][es] variant
// or an action sub-group.
func parseDesktopEntry(path string) (desktopEntry, bool) {
	f, err := os.Open(path)
	if err != nil {
		return desktopEntry{}, false
	}
	defer f.Close()

	var de desktopEntry
	inMainGroup := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if inMainGroup {
				break // left [Desktop Entry] into the next group; we're done
			}
			inMainGroup = line == "[Desktop Entry]"
			continue
		}
		if !inMainGroup {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "Name":
			de.Name = val
		case "Exec":
			de.Exec = val
		case "NoDisplay":
			de.NoDisplay = val == "true"
		case "Hidden":
			de.Hidden = val == "true"
		}
	}
	return de, de.Name != "" || de.Exec != ""
}

// resolveExecPath extracts a usable filesystem path from a .desktop
// Exec= line. Exec values look like "firefox %u" or "/opt/app/bin
// --flag %f" — field codes (%u, %f, %U, %F, etc.) are stripped, and we
// take only the first whitespace-delimited token (the binary itself).
//
// If the token isn't already an absolute path (most Exec= lines just
// name a binary expected to be on $PATH, e.g. "firefox"), we resolve
// it via $PATH ourselves rather than depending on exec.LookPath's
// error handling — a missing binary just means we report Bytes=0,
// which is the same graceful degradation the Windows apps detector
// uses for entries without an InstallLocation.
func resolveExecPath(exec string) string {
	fields := strings.Fields(exec)
	if len(fields) == 0 {
		return ""
	}
	bin := fields[0]
	if filepath.IsAbs(bin) {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
		return ""
	}
	for _, dir := range strings.Split(os.Getenv("PATH"), ":") {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, bin)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}
