package system

import (
	"context"
	"errors"
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// withMockSnapshotCommand swaps snapshotCommand for a function that
// returns canned output. Restored on cleanup. The args parameter is
// ignored — production tmutil takes "listlocalsnapshots /" but tests
// only care about the body of the response.
func withMockSnapshotCommand(t *testing.T, output []byte, err error) {
	t.Helper()
	prev := snapshotCommand
	snapshotCommand = func(ctx context.Context, args ...string) ([]byte, error) {
		return output, err
	}
	t.Cleanup(func() { snapshotCommand = prev })
}

// TestParseSnapshotList_RealOutput: sample matches the format `man tmutil`
// documents. The header line must be skipped; only "com.apple.TimeMachine.*"
// names are kept.
func TestParseSnapshotList_RealOutput(t *testing.T) {
	out := `Snapshots for volume group containing disk /:
com.apple.TimeMachine.2024-06-15-090000.local
com.apple.TimeMachine.2024-06-14-150000.local
com.apple.TimeMachine.2024-06-13-210000.local
`
	got := parseSnapshotList(out)
	if len(got) != 3 {
		t.Fatalf("expected 3 snapshots, got %d: %v", len(got), got)
	}
	if got[0] != "com.apple.TimeMachine.2024-06-15-090000.local" {
		t.Errorf("first snapshot mismatch: %s", got[0])
	}
}

// TestParseSnapshotList_Empty: a system without snapshots returns just
// the header. Must yield zero entries, not panic.
func TestParseSnapshotList_Empty(t *testing.T) {
	out := "Snapshots for volume group containing disk /:\n"
	if got := parseSnapshotList(out); len(got) != 0 {
		t.Fatalf("expected 0 snapshots, got %d: %v", len(got), got)
	}
}

// TestParseSnapshotList_HandlesBlankAndJunk: defensive — extra blank
// lines, indented entries, or garbage from a future tmutil release
// shouldn't break us.
func TestParseSnapshotList_HandlesBlankAndJunk(t *testing.T) {
	out := `
random preamble line

com.apple.TimeMachine.2024-06-15-090000.local

  com.apple.TimeMachine.2024-06-14-150000.local
`
	got := parseSnapshotList(out)
	if len(got) != 2 {
		t.Fatalf("expected 2 snapshots, got %d: %v", len(got), got)
	}
}

// TestSnapshotDate_Strips: well-formed name → just the date. This is
// what `tmutil deletelocalsnapshots` accepts as argument.
func TestSnapshotDate_Strips(t *testing.T) {
	got := snapshotDate("com.apple.TimeMachine.2024-06-15-090000.local")
	if got != "2024-06-15-090000" {
		t.Errorf("snapshotDate = %q, want 2024-06-15-090000", got)
	}
}

// TestSnapshotDate_Unrecognised: malformed input → return as-is so
// the remover can decide what to do (and tmutil errors loudly).
func TestSnapshotDate_Unrecognised(t *testing.T) {
	if got := snapshotDate("garbage"); got != "garbage" {
		t.Errorf("snapshotDate(%q) should pass through, got %q", "garbage", got)
	}
}

// TestScanSnapshots_NoneFound: tmutil returns the header only → no
// item. The wizard shouldn't show "0 snapshots" as a row.
func TestScanSnapshots_NoneFound(t *testing.T) {
	withMockSnapshotCommand(t, []byte("Snapshots for volume group containing disk /:\n"), nil)
	if items := scanSnapshots(); len(items) != 0 {
		t.Fatalf("no snapshots → 0 items, got %d", len(items))
	}
}

// TestScanSnapshots_Detected: with real-looking output, we get one
// item carrying the count in DetailArgs and a Tool the cleaner knows.
func TestScanSnapshots_Detected(t *testing.T) {
	out := []byte(`Snapshots for volume group containing disk /:
com.apple.TimeMachine.2024-06-15-090000.local
com.apple.TimeMachine.2024-06-14-150000.local
`)
	withMockSnapshotCommand(t, out, nil)

	items := scanSnapshots()
	if len(items) != 1 {
		t.Fatalf("expected 1 aggregated item, got %d", len(items))
	}
	it := items[0]
	if it.Tool != "tm-snapshots" {
		t.Errorf("Tool = %s, want tm-snapshots", it.Tool)
	}
	if it.Risk != item.RiskSafe {
		t.Errorf("Risk = %s, want RiskSafe", it.Risk)
	}
	if it.Category != item.CategorySystem {
		t.Errorf("Category = %s, want CategorySystem", it.Category)
	}
	if it.Bytes != 0 {
		t.Errorf("Bytes = %d, want 0 (snapshot size is opaque on APFS)", it.Bytes)
	}
	if len(it.DetailArgs) != 1 {
		t.Fatalf("DetailArgs should have count, got %v", it.DetailArgs)
	}
	if count, ok := it.DetailArgs[0].(int); !ok || count != 2 {
		t.Errorf("count arg = %v, want 2", it.DetailArgs[0])
	}
}

// TestScanSnapshots_TmutilFails: a tmutil that errors out (unlikely
// but possible on stripped systems / SIP issues) should produce no
// item rather than crashing the whole scan.
func TestScanSnapshots_TmutilFails(t *testing.T) {
	withMockSnapshotCommand(t, nil, errors.New("command not found"))
	if items := scanSnapshots(); len(items) != 0 {
		t.Fatalf("tmutil error should yield 0 items, got %d", len(items))
	}
}
