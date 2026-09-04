// Package disk provides disk-usage primitives shared by every detector:
// total/used/free space for a mount point, and recursive directory sizing.
//
// Usage() is platform-specific (implemented in disk_unix.go and
// disk_windows.go via build tags) because there is no portable syscall
// for "free space on this volume" across darwin/linux/windows.
//
// DirSize() is intentionally NOT platform-specific: it used to shell out
// to `du -sk`, which doesn't exist on Windows and added subprocess
// latency on macOS (see BACKLOG.md's "deuda técnica conocida"). Walking
// the tree with filepath.WalkDir and summing file sizes gives the same
// answer, works identically on every OS, and is faster for the
// repeated small-directory calls detectors make (JetBrains version
// listing, per-app caches, etc.).
package disk

import (
	"fmt"
	"os"
	"path/filepath"
)

type Info struct {
	Total    uint64
	Used     uint64
	Free     uint64
	UsedPct  float64
	TotalStr string
	UsedStr  string
	FreeStr  string
}

// DirSize returns the size of a directory (or file) in bytes by walking
// the tree and summing regular file sizes. Symlinks are not followed
// (WalkDir never descends into them), which matches the old `du`
// behaviour of not double-counting or chasing cycles.
//
// Per-entry errors (permission denied, race with a concurrent delete)
// are tolerated: the walk continues and the entry contributes 0 bytes,
// mirroring the old "du swallows errors" contract every caller already
// depends on.
func DirSize(path string) (int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, nil
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries, keep walking
		}
		if d.IsDir() {
			return nil
		}
		fi, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		total += fi.Size()
		return nil
	})
	if err != nil {
		return total, nil
	}
	return total, nil
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
