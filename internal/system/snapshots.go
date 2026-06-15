package system

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// snapshotCommand is the function used to invoke `tmutil`. Tests
// override it to return synthetic output without running a real command.
//
// The function takes a context so the production version respects
// cancellation; tests just ignore it.
var snapshotCommand = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "tmutil", args...).Output()
}

// scanSnapshots reports Time Machine local snapshots that macOS keeps
// on disk to back the local rollback feature. They get freed
// automatically when APFS detects pressure, but on disks that never
// fill up they accumulate and can hold double-digit GBs.
//
// We report them as a single Item carrying the count, NOT individual
// bytes. APFS measures snapshot size in terms of "blocks unique to
// this snapshot", which depends on what other snapshots are still
// alive — a number that's both expensive to compute and confusing
// when shown out of context. The wizard's UX is "X snapshots, macOS
// will recreate them on demand". Bytes stay 0; the cleaner respects
// that by reporting "freed (variable)" rather than a fake total.
//
// `tmutil listlocalsnapshots /` is the canonical way to list them.
// The output format is documented in `man tmutil`:
//
//	Snapshots for volume group containing disk /:
//	com.apple.TimeMachine.2024-06-15-090000.local
//	com.apple.TimeMachine.2024-06-14-150000.local
//
// We accept a leading header line and any blank lines, then collect
// every line that starts with "com.apple.TimeMachine.".
func scanSnapshots() []item.Item {
	names, err := listSnapshots()
	if err != nil || len(names) == 0 {
		// `tmutil` not available or no snapshots: nothing to report.
		// We deliberately don't surface the error — this detector is
		// best-effort, like the rest of the package.
		return nil
	}
	return []item.Item{{
		Name:       "Time Machine snapshots",
		Tool:       "tm-snapshots",
		Path:       "tmutil",
		Bytes:      0, // size is opaque on APFS; see doc above
		Category:   item.CategorySystem,
		Risk:       item.RiskSafe,
		Detail:     "snapshots locales de Time Machine; macOS los recreará si los necesita",
		DetailKey:  "system.snapshots.detail",
		DetailArgs: []any{len(names)},
	}}
}

// listSnapshots returns the snapshot names produced by `tmutil
// listlocalsnapshots /`. Empty slice when no snapshots exist; error
// when the command itself fails (tmutil missing, permissions, etc.).
//
// Names look like "com.apple.TimeMachine.2024-06-15-090000.local".
// The date portion is what `tmutil deletelocalsnapshots` accepts as
// argument, so we keep the raw string and let the remover slice it.
func listSnapshots() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := snapshotCommand(ctx, "listlocalsnapshots", "/")
	if err != nil {
		return nil, err
	}
	return parseSnapshotList(string(out)), nil
}

// parseSnapshotList extracts snapshot names from `tmutil` output.
// Lines that don't start with the expected prefix are skipped silently;
// this handles both the header line and any future format additions.
//
// Pure function so tests can exercise weird outputs without invoking
// any binary.
func parseSnapshotList(out string) []string {
	const prefix = "com.apple.TimeMachine."
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		names = append(names, line)
	}
	return names
}

// snapshotDate extracts the date portion `tmutil deletelocalsnapshots`
// expects from a full snapshot name.
//
//	com.apple.TimeMachine.2024-06-15-090000.local
//	                      ^^^^^^^^^^^^^^^^^^^^^^^
//	                      this is what we return
//
// Returns the input unchanged if the prefix or the .local suffix are
// missing; the remover then passes whatever it got to tmutil and lets
// tmutil reject it. Better than parsing too cleverly and silently
// dropping a snapshot we should have deleted.
func snapshotDate(name string) string {
	const prefix = "com.apple.TimeMachine."
	const suffix = ".local"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return name
	}
	return strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
}
