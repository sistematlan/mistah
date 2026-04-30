package disk

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

func Usage(path string) (Info, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Info{}, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return Info{
		Total:    total,
		Used:     used,
		Free:     free,
		UsedPct:  float64(used) / float64(total) * 100,
		TotalStr: FormatBytes(int64(total)),
		UsedStr:  FormatBytes(int64(used)),
		FreeStr:  FormatBytes(int64(free)),
	}, nil
}

// DirSize returns the size of a directory in bytes using du.
func DirSize(path string) (int64, error) {
	out, err := exec.Command("du", "-sk", path).Output()
	if err != nil {
		return 0, nil
	}
	parts := strings.Fields(string(out))
	if len(parts) == 0 {
		return 0, nil
	}
	kb, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, nil
	}
	return kb * 1024, nil
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
