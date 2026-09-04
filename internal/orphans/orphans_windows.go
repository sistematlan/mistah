//go:build windows

package orphans

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// scanHome runs the Windows orphan detectors against home.
func scanHome(home string) []item.Item {
	var items []item.Item
	if it, ok := dockerLeftover(home); ok {
		items = append(items, it)
	}
	if it, ok := whatsappMedia(home); ok {
		items = append(items, it)
	}
	return items
}

// dockerLeftover finds Docker Desktop's WSL2 data (.vhdx virtual disks)
// left behind after the app itself was uninstalled. This is a much
// bigger problem on Windows than macOS: the WSL2 backing store grows
// but almost never shrinks automatically, and 20-40 GB .vhdx files
// surviving an uninstall are a common complaint (see BACKLOG.md).
//
// Heuristic: Docker Desktop's own uninstaller removes the Docker
// Desktop.exe under Program Files but historically leaves
// %LOCALAPPDATA%\Docker\wsl\ behind. If that directory exists and the
// installed binary doesn't, treat it as orphaned.
func dockerLeftover(home string) (item.Item, bool) {
	wslDataPath := filepath.Join(home, "AppData", "Local", "Docker", "wsl")
	if _, err := os.Stat(wslDataPath); err != nil {
		return item.Item{}, false
	}
	if dockerDesktopInstalled() {
		return item.Item{}, false // Docker Desktop is installed; not orphan
	}
	bytes, _ := disk.DirSize(wslDataPath)
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "Docker Desktop leftover",
		NameKey:   "orphans.docker-leftover.name",
		Tool:      "docker",
		Path:      wslDataPath,
		Bytes:     bytes,
		Category:  item.CategoryOrphan,
		Risk:      item.RiskAskBefore,
		Detail:    "Docker Desktop está desinstalado pero sus datos de WSL2 permanecen",
		DetailKey: "orphans.docker-leftover.detail",
	}, true
}

// dockerDesktopInstalled checks for Docker Desktop via its usual
// Program Files location. We deliberately avoid a full registry
// uninstall-key scan here (see internal/apps for that pattern) — this
// detector only needs a yes/no signal, not a version number.
func dockerDesktopInstalled() bool {
	candidates := []string{
		`C:\Program Files\Docker\Docker\Docker Desktop.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	// Fall back to checking if the docker CLI resolves at all — covers
	// non-default install locations without hardcoding every possible path.
	if _, err := exec.LookPath("docker"); err == nil {
		return true
	}
	return false
}

// whatsappMedia finds media cached by WhatsApp Desktop (the Win32/Store
// app, not WhatsApp Web). Removing it does NOT delete chats — only
// photos, videos and audio that re-download on demand from the linked
// phone.
func whatsappMedia(home string) (item.Item, bool) {
	mediaPath := filepath.Join(home, "AppData", "Roaming", "WhatsApp", "Media")
	bytes, _ := disk.DirSize(mediaPath)
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "WhatsApp media",
		NameKey:   "orphans.whatsapp-media.name",
		Tool:      "whatsapp",
		Path:      mediaPath,
		Bytes:     bytes,
		Category:  item.CategoryOrphan,
		Risk:      item.RiskAskBefore,
		Detail:    "downloaded photos/videos/audio (chats not affected)",
		DetailKey: "orphans.whatsapp-media.detail",
	}, true
}
