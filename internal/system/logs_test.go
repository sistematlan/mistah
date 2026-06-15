package system

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// withLowLogThreshold lowers minLogBytes for the test so we don't have
// to seed 10 MB. Restored on cleanup.
func withLowLogThreshold(t *testing.T, n int64) {
	t.Helper()
	prev := minLogBytes
	minLogBytes = n
	t.Cleanup(func() { minLogBytes = prev })
}

// TestLogs_Empty: missing ~/Library/Logs is fine. Many fresh accounts
// don't have it on day one.
func TestLogs_Empty(t *testing.T) {
	home := t.TempDir()
	if items := scanLogs(home); len(items) != 0 {
		t.Fatalf("missing logs dir should yield 0 items, got %d", len(items))
	}
}

// TestLogs_DetectsAppSubdirs: each subdir of ~/Library/Logs above the
// noise threshold becomes its own item with Tool=logs and CategorySystem.
func TestLogs_DetectsAppSubdirs(t *testing.T) {
	withLowLogThreshold(t, 1)
	home := t.TempDir()
	seedFile(t, filepath.Join(home, "Library", "Logs", "Slack"), "session.log", 100, time.Time{})
	seedFile(t, filepath.Join(home, "Library", "Logs", "Chrome"), "stderr.log", 200, time.Time{})

	items := scanLogs(home)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, it := range items {
		if it.Tool != "logs" {
			t.Errorf("Tool = %s, want logs", it.Tool)
		}
		if it.Risk != item.RiskSafe {
			t.Errorf("Risk = %s, want RiskSafe", it.Risk)
		}
		if it.Category != item.CategorySystem {
			t.Errorf("Category = %s, want CategorySystem", it.Category)
		}
	}
}

// TestLogs_SkipsDiagnosticReports: the DiagnosticReports subdir has its
// own age-aware detector. scanLogs MUST NOT report it as a regular log
// dir, otherwise we'd offer to wipe recent crash reports without the
// 30-day filter.
func TestLogs_SkipsDiagnosticReports(t *testing.T) {
	withLowLogThreshold(t, 1)
	home := t.TempDir()
	seedFile(t,
		filepath.Join(home, "Library", "Logs", "DiagnosticReports"),
		"recent.crash", 1024, time.Now())

	for _, it := range scanLogs(home) {
		if filepath.Base(it.Path) == "DiagnosticReports" {
			t.Fatalf("scanLogs leaked DiagnosticReports as a regular log dir: %+v", it)
		}
	}
}

// TestLogs_BelowThresholdIgnored: subdirs under minLogBytes don't show.
func TestLogs_BelowThresholdIgnored(t *testing.T) {
	// Production threshold (10 MB) — we seed only a few bytes.
	home := t.TempDir()
	seedFile(t, filepath.Join(home, "Library", "Logs", "Slack"), "tiny.log", 100, time.Time{})

	if items := scanLogs(home); len(items) != 0 {
		t.Fatalf("sub-threshold logs should not surface, got %d items", len(items))
	}
}

// TestCrashReports_OnlyOldFilesCount: a recent .crash and an old .crash
// are seeded; the detector must report only the old one's bytes/count.
// This is the headline guarantee of the >30-day rule.
func TestCrashReports_OnlyOldFilesCount(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Logs", "DiagnosticReports")
	now := time.Now()
	seedFile(t, dir, "recent.crash", 100, now.Add(-1*24*time.Hour))           // 1 day → keep
	seedFile(t, dir, "old.crash", 200, now.Add(-60*24*time.Hour))             // 60 days → reclaim
	seedFile(t, dir, "ancient.ips", 400, now.Add(-200*24*time.Hour))          // 200 days → reclaim
	seedFile(t, dir, "skip.txt", 1000, now.Add(-200*24*time.Hour))            // wrong ext → ignore

	items := scanCrashReports(home)
	if len(items) != 1 {
		t.Fatalf("expected 1 crash-reports item, got %d", len(items))
	}
	it := items[0]
	if it.Bytes != 600 {
		t.Errorf("bytes = %d, want 600 (200 + 400; recent and skip excluded)", it.Bytes)
	}
	if it.Tool != "crash-reports" {
		t.Errorf("Tool = %s, want crash-reports", it.Tool)
	}
	if it.Risk != item.RiskSafe {
		t.Errorf("Risk = %s, want RiskSafe", it.Risk)
	}
	if len(it.DetailArgs) != 2 {
		t.Fatalf("DetailArgs should carry count + days, got %v", it.DetailArgs)
	}
	if count, ok := it.DetailArgs[0].(int); !ok || count != 2 {
		t.Errorf("count arg = %v, want 2", it.DetailArgs[0])
	}
}

// TestCrashReports_NoOldFiles_NoItem: a directory with only recent
// reports produces no item. The wizard shouldn't offer a 0 GB row.
func TestCrashReports_NoOldFiles_NoItem(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Logs", "DiagnosticReports")
	seedFile(t, dir, "recent.crash", 100, time.Now().Add(-2*24*time.Hour))

	if items := scanCrashReports(home); len(items) != 0 {
		t.Fatalf("only-recent crash reports should yield 0 items, got %+v", items)
	}
}

// TestCrashReports_MissingDir: no DiagnosticReports dir → no item.
func TestCrashReports_MissingDir(t *testing.T) {
	home := t.TempDir()
	if items := scanCrashReports(home); len(items) != 0 {
		t.Fatalf("missing dir should yield 0 items, got %+v", items)
	}
}

// TestIsCrashReportExt: allowlist accepts known Apple extensions and
// rejects everything else. Regression guard: if someone adds .log here
// by mistake, real log files get nuked alongside crash reports.
func TestIsCrashReportExt(t *testing.T) {
	cases := map[string]bool{
		"abc.crash":       true,
		"abc.diag":        true,
		"abc.ips":         true,
		"abc.spin":        true,
		"abc.hang":        true,
		"abc.CRASH":       true, // case-insensitive
		"abc.log":         false,
		"abc.txt":         false,
		"crashreport.bak": false,
		"crash":           false, // no extension
	}
	for name, want := range cases {
		if got := isCrashReportExt(name); got != want {
			t.Errorf("isCrashReportExt(%q) = %v, want %v", name, got, want)
		}
	}
}
