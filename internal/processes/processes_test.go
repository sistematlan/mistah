//go:build darwin || linux

package processes

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestParsePS(t *testing.T) {
	out := "  PID    RSS  %CPU COMM\n" +
		"    1  25696   0.0 /sbin/launchd\n" +
		"  499   6528   1.5 npm exec @perplexity-ai/mcp-server@0.9.0\n" +
		"  501   6944  12.3 /Applications/Cursor.app/Contents/MacOS/Cursor\n" +
		// Real macOS shape: app bundles put spaces in the executable path,
		// so "has a space" can't be the argv[0] heuristic.
		" 5102  28240   0.0 /Applications/Comet.app/Contents/Frameworks/Comet Framework.framework/Versions/152/Helpers/Comet Helper.app/Contents/MacOS/Comet Helper\n" +
		"garbage line without numbers\n"

	got := parsePS(out)
	if len(got) != 4 {
		t.Fatalf("want 4 procs, got %d: %+v", len(got), got)
	}
	if got[0].PID != 1 || got[0].Name != "launchd" || got[0].RSSBytes != 25696*1024 {
		t.Errorf("path row mis-parsed: %+v", got[0])
	}
	// argv[0] with spaces must survive intact — Base() would mangle it.
	if got[1].Name != "npm exec @perplexity-ai/mcp-server@0.9.0" {
		t.Errorf("multi-word name mangled: %q", got[1].Name)
	}
	if got[2].Name != "Cursor" || got[2].CPUPercent != 12.3 {
		t.Errorf("basename/cpu wrong: %+v", got[2])
	}
	if got[3].Name != "Comet Helper" {
		t.Errorf("app path with spaces not shortened: %q", got[3].Name)
	}
}

func TestTop(t *testing.T) {
	all := []Proc{
		{PID: 1, RSSBytes: 100, CPUPercent: 1},
		{PID: 2, RSSBytes: 900, CPUPercent: 90},
		{PID: 3, RSSBytes: 500, CPUPercent: 5},
	}
	if top := Top(append([]Proc(nil), all...), 2, false); len(top) != 2 || top[0].PID != 2 || top[1].PID != 3 {
		t.Errorf("rss sort wrong: %+v", top)
	}
	if top := Top(append([]Proc(nil), all...), 1, true); len(top) != 1 || top[0].PID != 2 {
		t.Errorf("cpu sort wrong: %+v", top)
	}
	if top := Top(append([]Proc(nil), all...), 0, false); len(top) != 3 {
		t.Errorf("limit 0 should keep all, got %d", len(top))
	}
}

func TestKillGuards(t *testing.T) {
	for _, pid := range []int{0, 1, -3, os.Getpid()} {
		if err := Kill(pid, 0); err == nil {
			t.Errorf("Kill(%d) must be refused", pid)
		}
	}
}

// TestKillRealProcess is the end-to-end check: a live child must actually
// die from Kill's SIGTERM path, not just from the guard tests above.
func TestKillRealProcess(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("no sleep binary: %v", err)
	}
	// Reap concurrently. Without a Wait the child lingers as a zombie and
	// `kill -0` keeps succeeding, so Kill would burn its whole grace period
	// before SIGKILL — the test would pass via the wrong path. In production
	// the target isn't ours, and its own parent reaps it.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	start := time.Now()
	if err := Kill(cmd.Process.Pid, 5*time.Second); err != nil {
		t.Fatalf("Kill(%d): %v", cmd.Process.Pid, err)
	}
	if err := <-done; err == nil {
		t.Fatal("child exited cleanly; Kill did not terminate it")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("SIGTERM path took %v; expected the poll loop to notice quickly", elapsed)
	}
}
