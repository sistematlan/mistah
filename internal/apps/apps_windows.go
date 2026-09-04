//go:build windows

package apps

import (
	"os"
	"time"

	"golang.org/x/sys/windows/registry"

	"github.com/sistematlan/mistah/internal/disk"
)

// uninstallKeyRoots enumerates the registry locations Programs &
// Features itself reads. Windows keeps three parallel views:
//
//   - HKLM\...\Uninstall              64-bit machine-wide installs
//   - HKLM\...\WOW6432Node\Uninstall  32-bit machine-wide installs
//     (on 64-bit Windows, 32-bit installers register here instead)
//   - HKCU\...\Uninstall              per-user installs (no admin
//     rights needed to install, e.g. many Electron apps' default mode)
//
// Missing any one of these under-reports apps; a user with a mix of
// 32-bit legacy software and modern per-user Electron apps needs all
// three merged.
var uninstallKeyRoots = []struct {
	root registry.Key
	path string
}{
	{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
}

// List enumerates installed applications via the registry Uninstall
// keys — the same source Programs & Features / "Add or remove
// programs" uses. Each entry maps to an on-disk InstallLocation when
// the app declares one; apps that don't (some MSI packages point at a
// single file, not a directory) are still listed with Bytes=0 rather
// than skipped, so the user still sees them and can investigate.
//
// "Last used" has no Windows Registry equivalent (there is no per-app
// "last opened" field the way macOS Spotlight tracks kMDItemLastUsedDate).
// We fall back to the install directory's most recent mtime among its
// top-level files as a rough proxy — not perfectly accurate, but far
// better than reporting every app as "never used", which would make the
// wizard's staleness signal useless on Windows entirely.
func List() ([]App, error) {
	var apps []App
	seen := map[string]bool{}

	for _, root := range uninstallKeyRoots {
		names, err := readUninstallEntries(root.root, root.path)
		if err != nil {
			continue // key missing on this Windows edition is normal
		}
		for _, entry := range names {
			if entry.DisplayName == "" || seen[entry.DisplayName] {
				continue
			}
			// SystemComponent=1 marks runtime/driver packages (VC++
			// Redistributables, .NET runtimes, driver bundles) that
			// Programs & Features itself hides from the visible list.
			// Surfacing those would flood the wizard with entries a
			// user can't meaningfully "clean up" without breaking
			// something else.
			if entry.SystemComponent {
				continue
			}
			seen[entry.DisplayName] = true

			var bytes int64
			path := entry.InstallLocation
			if path != "" {
				bytes, _ = disk.DirSize(path)
			}

			lastUsed, neverUsed := installDirActivity(path)
			days := -1
			if !neverUsed {
				days = int(time.Since(lastUsed).Hours() / 24)
			}

			apps = append(apps, App{
				Name:         entry.DisplayName,
				Path:         path,
				Bytes:        bytes,
				LastUsed:     lastUsed,
				NeverUsed:    neverUsed,
				DaysSinceUse: days,
			})
		}
	}

	return apps, nil
}

// uninstallEntry is the subset of an Uninstall registry subkey's
// values that List() needs.
type uninstallEntry struct {
	DisplayName     string
	InstallLocation string
	SystemComponent bool
}

// readUninstallEntries opens root\path and reads every immediate
// subkey (one per installed product) into an uninstallEntry.
func readUninstallEntries(root registry.Key, path string) ([]uninstallEntry, error) {
	k, err := registry.OpenKey(root, path, registry.ENUMERATE_SUB_KEYS|registry.READ)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	subNames, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var out []uninstallEntry
	for _, sub := range subNames {
		sk, err := registry.OpenKey(root, path+`\`+sub, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		displayName, _, _ := sk.GetStringValue("DisplayName")
		installLocation, _, _ := sk.GetStringValue("InstallLocation")
		systemComponent, _, _ := sk.GetIntegerValue("SystemComponent")
		sk.Close()

		if displayName == "" {
			continue // entries without a DisplayName are noise (patches, hotfixes)
		}
		out = append(out, uninstallEntry{
			DisplayName:     displayName,
			InstallLocation: installLocation,
			SystemComponent: systemComponent == 1,
		})
	}
	return out, nil
}

// installDirActivity approximates "last used" by the newest mtime among
// the install directory's immediate files. Returns neverUsed=true when
// the path is empty, unreadable, or has no files to inspect — the
// caller then shows "nunca" rather than a misleading date.
func installDirActivity(path string) (time.Time, bool) {
	if path == "" {
		return time.Time{}, true
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return time.Time{}, true
	}
	var newest time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if newest.IsZero() {
		return time.Time{}, true
	}
	return newest, false
}
