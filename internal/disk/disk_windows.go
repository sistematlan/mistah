//go:build windows

package disk

import "golang.org/x/sys/windows"

// Usage reports total/used/free space for the volume containing path,
// via the Win32 GetDiskFreeSpaceEx API. path can be any path on the
// target volume (e.g. "C:\\" or "D:\\Users\\name"); Windows resolves it
// to the owning volume internally.
func Usage(path string) (Info, error) {
	utf16Path, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Info{}, err
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(
		utf16Path,
		&freeBytesAvailable,
		&totalBytes,
		&totalFreeBytes,
	); err != nil {
		return Info{}, err
	}

	total := totalBytes
	free := freeBytesAvailable
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
