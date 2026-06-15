package system

import (
	"os"
	"path/filepath"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// minLogBytes filters out small log directories that would clutter the
// wizard list. Same noise threshold as appcache uses (10 MB).
//
// Mutable so tests can lower it without seeding 10 MB of synthetic logs.
var minLogBytes int64 = 10 * 1024 * 1024

// crashReportMaxAgeDays is the cutoff for old diagnostic reports.
// Reports newer than this stay on disk so the user can debug something
// recent; anything older is reclaimable noise.
//
// Apple's Console.app and feedback flows generally don't reference
// reports older than a few weeks. 30 days is a conservative ceiling.
var crashReportMaxAgeDays = 30

// scanLogs returns one Item per app log subdirectory under
// ~/Library/Logs that exceeds the noise threshold. Reporting per-app
// rather than as a single bucket lets the user accept Slack's 800 MB
// of logs and decline the .photoslibrary helper they don't recognise.
//
// The root ~/Library/Logs/ itself is never returned as an Item — that
// would invite a recursive RemoveAll which destroys the directory
// macOS expects to find.
//
// DiagnosticReports is excluded from this walk and handled separately
// by scanCrashReports so the >30-day filter can apply.
func scanLogs(home string) []item.Item {
	root := filepath.Join(home, "Library", "Logs")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil // no Library/Logs is fine
	}

	var items []item.Item
	for _, e := range entries {
		// DiagnosticReports has its own detector with age filtering.
		// Skipping by name is brittle but cheaper than passing flags
		// down the loop; the directory name is stable across macOS.
		if e.Name() == "DiagnosticReports" {
			continue
		}
		if !e.IsDir() {
			// Stray top-level files in ~/Library/Logs do exist
			// (some installers drop *.log there). Add them only if
			// they're large enough to matter.
			info, err := e.Info()
			if err != nil || info.Size() < minLogBytes {
				continue
			}
			full := filepath.Join(root, e.Name())
			items = append(items, item.Item{
				Name:      e.Name(),
				Tool:      "logs",
				Path:      full,
				Bytes:     info.Size(),
				Category:  item.CategorySystem,
				Risk:      item.RiskSafe,
				Detail:    "log file in ~/Library/Logs",
				DetailKey: "system.logs.detail",
			})
			continue
		}

		full := filepath.Join(root, e.Name())
		bytes, _ := disk.DirSize(full)
		if bytes < minLogBytes {
			continue
		}
		items = append(items, item.Item{
			Name:       e.Name() + " (logs)",
			Tool:       "logs",
			Path:       full,
			Bytes:      bytes,
			Category:   item.CategorySystem,
			Risk:       item.RiskSafe,
			Detail:     "logs de " + e.Name() + " en ~/Library/Logs; la app los regenera",
			DetailKey:  "system.logs.detail",
			DetailArgs: []any{e.Name()},
		})
	}
	return items
}

// scanCrashReports returns a single Item pointing at
// ~/Library/Logs/DiagnosticReports. The cleaner uses OldFilesRemover
// (registered for Tool="crash-reports") to delete only files older
// than crashReportMaxAgeDays — recent reports might be the user's
// active debugging context.
//
// We report the directory size including the recent files (the
// wizard's bytes count would be misleading otherwise: showing 4 GB
// when only 3.5 GB is actually reclaimable). The Detail text makes
// the >30 days rule explicit so the user understands the ask.
//
// CrashReporter under Application Support is also handled here as a
// secondary path; it stores tiny bookkeeping files but they accumulate.
func scanCrashReports(home string) []item.Item {
	dir := filepath.Join(home, "Library", "Logs", "DiagnosticReports")
	bytes, count, ok := summarizeOldCrashReports(dir, crashReportMaxAgeDays)
	if !ok || bytes <= 0 || count == 0 {
		return nil
	}
	return []item.Item{{
		Name:       "Diagnostic reports",
		Tool:       "crash-reports",
		Path:       dir,
		Bytes:      bytes,
		Category:   item.CategorySystem,
		Risk:       item.RiskSafe,
		Detail:     "reportes de fallos antiguos (>30 días)",
		DetailKey:  "system.crash-reports.detail",
		DetailArgs: []any{count, crashReportMaxAgeDays},
	}}
}

// summarizeOldCrashReports returns the total bytes and file count of
// crash-report files older than maxDays in dir. Only files matching the
// known Apple extensions are considered:
//
//	.crash  classic crash logs
//	.diag   diagnostic reports
//	.ips    newer Apple format (used since Big Sur)
//	.spin   spin reports
//	.hang   hang reports
//
// Anything else in the directory is ignored. We don't recurse — Apple
// keeps reports flat in this folder.
//
// Returns ok=false only when the directory itself can't be read; an
// empty directory returns (0, 0, true).
func summarizeOldCrashReports(dir string, maxDays int) (bytes int64, count int, ok bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, false
	}
	cutoff := time.Now().Add(-time.Duration(maxDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !isCrashReportExt(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		bytes += info.Size()
		count++
	}
	return bytes, count, true
}

// isCrashReportExt returns true for filenames Apple produces in
// DiagnosticReports. Compared against a fixed allowlist so a stray
// .DS_Store or user-dropped file never gets deleted.
func isCrashReportExt(name string) bool {
	exts := []string{".crash", ".diag", ".ips", ".spin", ".hang"}
	for _, ext := range exts {
		if hasSuffixCI(name, ext) {
			return true
		}
	}
	return false
}

// hasSuffixCI is strings.HasSuffix with case-insensitive comparison.
// Crash reports usually have lowercase extensions but some Apple tools
// have shipped uppercase variants over the years.
func hasSuffixCI(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return equalASCIIFold(s[len(s)-len(suffix):], suffix)
}

// equalASCIIFold compares two ASCII strings case-insensitively. We
// avoid strings.EqualFold here to keep the dependency surface minimal
// in this file (it already imports the bare minimum).
func equalASCIIFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
