//go:build darwin || linux

package disk

import "syscall"

// Usage reports total/used/free space for the filesystem containing path.
// darwin and linux both expose statfs(2) with a compatible enough Statfs_t
// (Bsize/Blocks/Bavail) that a single implementation covers both.
func Usage(path string) (Info, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return Info{}, err
	}

	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
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
