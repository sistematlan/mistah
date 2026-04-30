package caches

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sistematlan/chipawa/internal/disk"
)

type Cache struct {
	Name string
	Path string
	Bytes int64
}

func Scan() ([]Cache, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	candidates := []Cache{
		{Name: "npm", Path: filepath.Join(home, ".npm")},
		{Name: "Homebrew", Path: filepath.Join(home, "Library/Caches/Homebrew")},
		{Name: "JetBrains", Path: filepath.Join(home, "Library/Caches/JetBrains")},
		{Name: "Go build", Path: filepath.Join(home, "Library/Caches/go-build")},
		{Name: "pip", Path: filepath.Join(home, "Library/Caches/pip")},
		{Name: "Composer", Path: filepath.Join(home, "Library/Caches/composer")},
		{Name: "node-gyp", Path: filepath.Join(home, "Library/Caches/node-gyp")},
		{Name: "yarn", Path: filepath.Join(home, ".yarn/cache")},
		{Name: "Chrome", Path: filepath.Join(home, "Library/Caches/Google/Chrome")},
		{Name: "Firefox", Path: filepath.Join(home, "Library/Caches/Mozilla")},
		{Name: "Xcode DerivedData", Path: filepath.Join(home, "Library/Developer/Xcode/DerivedData")},
	}

	var result []Cache
	for _, c := range candidates {
		if _, err := os.Stat(c.Path); os.IsNotExist(err) {
			continue
		}
		bytes, _ := disk.DirSize(c.Path)
		if bytes > 0 {
			c.Bytes = bytes
			result = append(result, c)
		}
	}

	// Docker — separate via CLI
	if docker := dockerSize(); docker.Bytes > 0 {
		result = append(result, docker)
	}

	return result, nil
}

func dockerSize() Cache {
	out, err := exec.Command("docker", "system", "df", "--format", "{{.Type}}\t{{.Size}}\t{{.Reclaimable}}").Output()
	if err != nil {
		return Cache{}
	}
	var total int64
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		total += parseDockerSize(parts[1])
	}
	return Cache{Name: "Docker (total)", Path: "", Bytes: total}
}

func parseDockerSize(s string) int64 {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0
	}
	multipliers := map[string]int64{
		"B": 1, "KB": 1024, "MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024, "TB": 1024 * 1024 * 1024 * 1024,
	}
	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			num := strings.TrimSuffix(s, suffix)
			var val float64
			if _, err := fmt.Sscanf(strings.TrimSpace(num), "%f", &val); err == nil {
				return int64(val * float64(mult))
			}
		}
	}
	return 0
}
