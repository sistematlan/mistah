// Package apps lists installed applications with their on-disk size and
// last-used date, so the wizard/cmd layer can flag apps nobody has
// opened in months.
//
// "Installed application" means something different per OS:
//
//	macOS:   a .app bundle under /Applications or ~/Applications.
//	Windows: an entry in the Registry's Uninstall key (the same
//	         registry Programs & Features reads), which points at an
//	         install directory rather than a single bundle folder.
//
// List() is the only OS-agnostic entry point; the concrete enumeration
// and last-used heuristics live in apps_darwin.go / apps_windows.go.
package apps

import (
	"fmt"
	"time"
)

type App struct {
	Name         string
	Path         string
	Bytes        int64
	LastUsed     time.Time
	NeverUsed    bool
	DaysSinceUse int
}

func (a App) LastUsedLabel() string {
	if a.NeverUsed {
		return "nunca"
	}
	if a.DaysSinceUse == 0 {
		return "hoy"
	}
	if a.DaysSinceUse < 30 {
		return fmt.Sprintf("hace %d días", a.DaysSinceUse)
	}
	return a.LastUsed.Format("2006-01-02")
}
