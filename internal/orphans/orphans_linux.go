//go:build linux

package orphans

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// scanHome runs the Linux orphan detectors against home.
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

// dockerLeftover finds Docker's data root left behind after the
// Docker package itself was removed. Unlike Docker Desktop on
// macOS/Windows (a GUI app with a single well-known install location),
// native Linux Docker is a system service — "uninstalled" means the
// docker/docker-ce/docker.io package is gone, and the daemon's data
// root (images, containers, volumes) at /var/lib/docker survives
// because most package managers don't purge it by default (avoiding
// accidental data loss is the correct default for a system daemon,
// but it does mean multi-GB leftovers on removal).
//
// Heuristic: if /var/lib/docker exists but the docker CLI is nowhere
// on $PATH, treat it as orphaned. Reading /var/lib/docker itself
// typically requires root (dockerd runs as root and the directory is
// 0700), so this detector reports what it can when running unprivileged
// — DirSize degrades to 0 rather than erroring when access is denied,
// same graceful-skip behaviour every other detector in this codebase
// uses for permission failures.
func dockerLeftover(home string) (item.Item, bool) {
	const dataRoot = "/var/lib/docker"
	if _, err := os.Stat(dataRoot); err != nil {
		return item.Item{}, false
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return item.Item{}, false // docker CLI present; not orphaned
	}
	bytes, _ := disk.DirSize(dataRoot)
	if bytes <= 0 {
		return item.Item{}, false
	}
	return item.Item{
		Name:      "Docker leftover",
		NameKey:   "orphans.docker-leftover.name",
		Tool:      "docker",
		Path:      dataRoot,
		Bytes:     bytes,
		Category:  item.CategoryOrphan,
		Risk:      item.RiskAskBefore,
		Detail:    "Docker está desinstalado pero sus datos permanecen en /var/lib/docker",
		DetailKey: "orphans.docker-leftover.detail",
	}, true
}

// whatsappMedia finds media cached by WhatsApp's unofficial Linux
// desktop clients (WhatsApp for Linux, WA Web wrappers built on
// Electron). There's no single official WhatsApp Linux app the way
// there's an official macOS/Windows one, so this points at the most
// common community client's config path; absent that app, DirSize
// simply returns 0 and no item is reported — same "app not installed,
// nothing to detect" behaviour as every other consumer-app entry.
func whatsappMedia(home string) (item.Item, bool) {
	mediaPath := filepath.Join(home, ".config", "WhatsApp", "Media")
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
