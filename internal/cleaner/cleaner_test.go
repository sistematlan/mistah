package cleaner

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// scriptedPrompter returns a fixed sequence of decisions, one per Ask call.
// Tests fail loudly if Ask is called more times than answers were provided.
type scriptedPrompter struct {
	answers []Decision
	calls   int
	shown   []string
}

func (p *scriptedPrompter) Ask(_ item.Item) Decision {
	if p.calls >= len(p.answers) {
		panic("scriptedPrompter: Ask called more times than answers")
	}
	d := p.answers[p.calls]
	p.calls++
	return d
}
func (p *scriptedPrompter) Show(msg string) { p.shown = append(p.shown, msg) }

// makeFile creates a temp directory with one byte of content so DirSize > 0.
func makeFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestDryRun_DoesNotDelete: dry-run never touches disk, even with Yes answers.
func TestDryRun_DoesNotDelete(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "cache")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	makeFile(t, target, "a.txt")

	it := item.Item{Name: "test", Path: target, Bytes: 1, Risk: item.RiskSafe}
	plan := New([]item.Item{it}, DryRun, &scriptedPrompter{answers: []Decision{DecisionYes}}, &bytes.Buffer{})
	results := plan.Run()

	if _, err := os.Stat(target); err != nil {
		t.Fatalf("dry-run deleted target: %v", err)
	}
	if len(results) != 1 || !results[0].Skipped {
		t.Fatalf("expected 1 skipped result, got %+v", results)
	}
}

// TestYesMode_DeletesWithoutPrompt: Yes mode removes every item, no prompter calls.
func TestYesMode_DeletesWithoutPrompt(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		makeFile(t, p, "f")
	}
	items := []item.Item{
		{Name: "a", Path: a, Bytes: 10, Risk: item.RiskSafe},
		{Name: "b", Path: b, Bytes: 20, Risk: item.RiskSafe},
	}
	// nil prompter is fine in Yes mode — it shouldn't be called.
	plan := New(items, Yes, nil, &bytes.Buffer{})
	results := plan.Run()

	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Errorf("a should be gone, got err=%v", err)
	}
	if _, err := os.Stat(b); !os.IsNotExist(err) {
		t.Errorf("b should be gone, got err=%v", err)
	}
	s := Summarize(results)
	if s.Removed != 2 || s.BytesFreed != 30 {
		t.Fatalf("Summary = %+v", s)
	}
}

// TestInteractive_NoSkips: a "no" answer skips that item, others continue.
func TestInteractive_NoSkips(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	for _, p := range []string{a, b} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	items := []item.Item{
		{Name: "a", Path: a, Bytes: 1, Risk: item.RiskSafe},
		{Name: "b", Path: b, Bytes: 1, Risk: item.RiskSafe},
	}
	pr := &scriptedPrompter{answers: []Decision{DecisionNo, DecisionYes}}
	plan := New(items, Interactive, pr, &bytes.Buffer{})
	results := plan.Run()

	if _, err := os.Stat(a); err != nil {
		t.Errorf("a should still exist (declined)")
	}
	if _, err := os.Stat(b); !os.IsNotExist(err) {
		t.Errorf("b should be gone (accepted)")
	}
	if results[0].Skipped != true || results[1].Skipped != false {
		t.Fatalf("results = %+v", results)
	}
}

// TestInteractive_QuitStopsPlan: a quit answer halts processing of remaining items.
func TestInteractive_QuitStopsPlan(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	b := filepath.Join(tmp, "b")
	for _, p := range []string{a, b} {
		_ = os.MkdirAll(p, 0o755)
	}
	items := []item.Item{
		{Name: "a", Path: a, Bytes: 1, Risk: item.RiskSafe},
		{Name: "b", Path: b, Bytes: 1, Risk: item.RiskSafe},
	}
	pr := &scriptedPrompter{answers: []Decision{DecisionQuit}}
	plan := New(items, Interactive, pr, &bytes.Buffer{})
	results := plan.Run()

	if len(results) != 1 {
		t.Fatalf("expected processing to stop after quit, got %d results", len(results))
	}
	if pr.calls != 1 {
		t.Fatalf("prompter called %d times after quit", pr.calls)
	}
}

// TestInteractive_ViewLoopThenYes: view triggers Show, then next Ask is consulted.
func TestInteractive_ViewLoopThenYes(t *testing.T) {
	tmp := t.TempDir()
	a := filepath.Join(tmp, "a")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	makeFile(t, a, "child.txt")

	items := []item.Item{{Name: "a", Path: a, Bytes: 1, Risk: item.RiskSafe}}
	pr := &scriptedPrompter{answers: []Decision{DecisionView, DecisionYes}}
	plan := New(items, Interactive, pr, &bytes.Buffer{})
	plan.Run()

	if _, err := os.Stat(a); !os.IsNotExist(err) {
		t.Errorf("a should be removed after view+yes")
	}
	if len(pr.shown) != 1 {
		t.Errorf("Show called %d times, want 1", len(pr.shown))
	}
}

// TestPathRemover_RejectsUnsafePath: deletion outside SafeRoots must error.
func TestPathRemover_RejectsUnsafePath(t *testing.T) {
	r := PathRemover{}
	err := r.Remove(item.Item{Path: "/etc/hosts"})
	if err == nil {
		t.Fatal("expected error for unsafe path")
	}
}

// TestPathRemover_EmptyPath: empty Path is a hard error, never a panic.
func TestPathRemover_EmptyPath(t *testing.T) {
	if err := (PathRemover{}).Remove(item.Item{}); err == nil {
		t.Fatal("expected error for empty path")
	}
}

// TestDefaultResolver_PicksDocker: docker tool with no Path → DockerPruneRemover.
func TestDefaultResolver_PicksDocker(t *testing.T) {
	r, err := DefaultResolver(item.Item{Tool: "docker", Path: ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(DockerPruneRemover); !ok {
		t.Fatalf("expected DockerPruneRemover, got %T", r)
	}
}

// TestDefaultResolver_PicksPath: any item with a Path → PathRemover.
func TestDefaultResolver_PicksPath(t *testing.T) {
	r, err := DefaultResolver(item.Item{Tool: "npm", Path: "/x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(PathRemover); !ok {
		t.Fatalf("expected PathRemover, got %T", r)
	}
}

// TestDefaultResolver_RejectsMalformed: empty Path + non-docker tool → error.
func TestDefaultResolver_RejectsMalformed(t *testing.T) {
	if _, err := DefaultResolver(item.Item{Tool: "npm", Path: ""}); err == nil {
		t.Fatal("expected resolver to reject empty-path non-docker item")
	}
}

// withFakeHome reroutes OffLimits and SafeRoots to a temporary directory
// so off-limits checks can be exercised against real files without
// touching the actual user home. Returns the fake-home path.
//
// Tests must call this via t.Cleanup; the helper restores the real
// OffLimits and SafeRoots when the test finishes.
//
// Why both? OffLimits paths (e.g. ~/Documents) must live under a
// SafeRoot or the SafeRoot check rejects them first and we'd never
// reach the OffLimits check we're trying to test. Pointing both at
// the same tempdir keeps the tests honest about the order of checks.
func withFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()

	prevOff := OffLimits
	prevSafe := SafeRoots
	OffLimits = DefaultOffLimits(home)
	SafeRoots = append([]string{home}, prevSafe...)
	t.Cleanup(func() {
		OffLimits = prevOff
		SafeRoots = prevSafe
	})

	// Pre-create the off-limits dirs so tests can place real files inside
	// them without doing it themselves.
	for _, p := range OffLimits {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("seed off-limits dir %s: %v", p, err)
		}
	}
	return home
}

// TestPathRemover_RejectsOffLimits_Documents: ~/Documents (and any child)
// must NEVER be deleted, even if a detector mistakenly reports it. This is
// the headline guarantee of the OffLimits barrier.
func TestPathRemover_RejectsOffLimits_Documents(t *testing.T) {
	home := withFakeHome(t)
	target := filepath.Join(home, "Documents", "important.txt")
	if err := os.WriteFile(target, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := PathRemover{}.Remove(item.Item{Path: target})
	if !errors.Is(err, ErrOffLimits) {
		t.Fatalf("expected ErrOffLimits, got %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("file under ~/Documents must still exist: %v", statErr)
	}
}

// TestPathRemover_RejectsOffLimits_PhotosLibrary: ~/Pictures itself is
// off-limits, and so is anything inside (Photos library, screenshots,
// the .photoslibrary bundle). Catches detectors that try to read inside
// Photos and end up listing the library as a candidate.
func TestPathRemover_RejectsOffLimits_PhotosLibrary(t *testing.T) {
	home := withFakeHome(t)
	target := filepath.Join(home, "Pictures", "Photos Library.photoslibrary")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	err := PathRemover{}.Remove(item.Item{Path: target})
	if !errors.Is(err, ErrOffLimits) {
		t.Fatalf("expected ErrOffLimits, got %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatalf("Photos Library must still exist: %v", statErr)
	}
}

// TestPathRemover_AllowsTrash: ~/.Trash is NOT off-limits. The Trash
// detector must be able to clear it. This guards against the easy
// over-correction of blacklisting too much of the home dir.
func TestPathRemover_AllowsTrash(t *testing.T) {
	home := withFakeHome(t)
	trashDir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(trashDir, "old.dmg")
	if err := os.WriteFile(target, []byte("trashed"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (PathRemover{}).Remove(item.Item{Path: target}); err != nil {
		t.Fatalf("Trash file should be removable, got %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("Trash file should be gone, stat err=%v", statErr)
	}
}

// TestPathRemover_AllowsLibraryCaches: ~/Library/Caches is the bread and
// butter of mistah. It MUST stay deletable. Regression test: if anyone
// ever adds ~/Library to OffLimits, every cache detector breaks.
func TestPathRemover_AllowsLibraryCaches(t *testing.T) {
	home := withFakeHome(t)
	cacheDir := filepath.Join(home, "Library", "Caches", "com.spotify.client")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(cacheDir, "data.db")
	if err := os.WriteFile(target, []byte("cache"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (PathRemover{}).Remove(item.Item{Path: cacheDir}); err != nil {
		t.Fatalf("Library cache should be removable, got %v", err)
	}
	if _, statErr := os.Stat(cacheDir); !os.IsNotExist(statErr) {
		t.Fatalf("cache dir should be gone, stat err=%v", statErr)
	}
}

// TestPathRemover_OffLimitsBoundary: ~/Documents-old must NOT be confused
// with ~/Documents. The check must respect path-component boundaries —
// a bare strings.HasPrefix would match both, which is the textbook
// off-by-one in path policy code.
func TestPathRemover_OffLimitsBoundary(t *testing.T) {
	home := withFakeHome(t)
	// Sibling of ~/Documents whose name starts with the same letters.
	// This is a real-world case: people do rename their Documents folder
	// to "Documents-old" when migrating Macs.
	siblingDir := filepath.Join(home, "Documents-old")
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(siblingDir, "leftover.txt")
	if err := os.WriteFile(target, []byte("ok to delete"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (PathRemover{}).Remove(item.Item{Path: target}); err != nil {
		t.Fatalf("~/Documents-old/* must NOT be off-limits, got %v", err)
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("file should be gone, stat err=%v", statErr)
	}
}

// TestTrashContentsRemover_KeepsRoot: TrashContentsRemover must wipe
// children but keep the ~/.Trash directory itself in place. macOS Finder
// and the system relies on that path existing.
func TestTrashContentsRemover_KeepsRoot(t *testing.T) {
	home := withFakeHome(t)
	trashDir := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Mix of file and directory children, so we exercise the recursive
	// branch of os.RemoveAll on subfolders too.
	if err := os.WriteFile(filepath.Join(trashDir, "old.dmg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(trashDir, "ProjectFolder")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := TrashContentsRemover{}
	if err := r.Remove(item.Item{Tool: "trash", Path: trashDir}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := os.Stat(trashDir); err != nil {
		t.Fatalf("~/.Trash must still exist after wipe: %v", err)
	}
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Trash should be empty, has %d entries", len(entries))
	}
}

// TestDefaultResolver_PicksTrash: an item with Tool=trash must resolve
// to TrashContentsRemover so the wizard never accidentally calls
// os.RemoveAll on ~/.Trash itself.
func TestDefaultResolver_PicksTrash(t *testing.T) {
	r, err := DefaultResolver(item.Item{Tool: "trash", Path: "/some/path"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(TrashContentsRemover); !ok {
		t.Fatalf("expected TrashContentsRemover, got %T", r)
	}
}

// TestDefaultResolver_PicksOldFiles: crash-reports tool maps to the
// age-filtered OldFilesRemover with the canonical extensions list.
// Without this mapping, a crash-reports item would fall through to
// PathRemover and wipe the whole DiagnosticReports directory including
// the user's recent debugging session.
func TestDefaultResolver_PicksOldFiles(t *testing.T) {
	r, err := DefaultResolver(item.Item{Tool: "crash-reports", Path: "/some/path"})
	if err != nil {
		t.Fatal(err)
	}
	old, ok := r.(OldFilesRemover)
	if !ok {
		t.Fatalf("expected OldFilesRemover, got %T", r)
	}
	if old.MaxAgeDays != 30 {
		t.Errorf("MaxAgeDays = %d, want 30", old.MaxAgeDays)
	}
	if len(old.Extensions) == 0 {
		t.Errorf("Extensions must not be empty")
	}
}

// TestOldFilesRemover_OnlyOldFiles: the remover wipes files matching
// the extension AND older than the cutoff, while keeping recent files
// and unrelated extensions untouched.
func TestOldFilesRemover_OnlyOldFiles(t *testing.T) {
	withFakeHome(t)
	tmp := t.TempDir()
	old := filepath.Join(tmp, "old.crash")
	recent := filepath.Join(tmp, "recent.crash")
	other := filepath.Join(tmp, "notes.txt")
	for _, p := range []string{old, recent, other} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	if err := os.Chtimes(old, now.AddDate(0, 0, -60), now.AddDate(0, 0, -60)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(recent, now.AddDate(0, 0, -1), now.AddDate(0, 0, -1)); err != nil {
		t.Fatal(err)
	}
	// other.txt keeps the default mtime (now).

	r := OldFilesRemover{MaxAgeDays: 30, Extensions: []string{".crash"}}
	if err := r.Remove(item.Item{Tool: "crash-reports", Path: tmp}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("old.crash should be gone, stat err=%v", err)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("recent.crash should still exist, got %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("notes.txt should still exist, got %v", err)
	}
}

// TestOldFilesRemover_RejectsZeroAge: MaxAgeDays=0 is a misconfiguration
// (would mean "delete everything created today"). Must error rather
// than silently obey, because callers passing 0 almost certainly meant
// to pass a real number.
func TestOldFilesRemover_RejectsZeroAge(t *testing.T) {
	r := OldFilesRemover{MaxAgeDays: 0, Extensions: []string{".crash"}}
	err := r.Remove(item.Item{Path: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when MaxAgeDays <= 0")
	}
}

// TestOldFilesRemover_RespectsOffLimits: even with valid age and
// extensions, OldFilesRemover refuses to touch a path inside OffLimits.
// Defense in depth on top of the detector's own discipline.
func TestOldFilesRemover_RespectsOffLimits(t *testing.T) {
	home := withFakeHome(t)
	docs := filepath.Join(home, "Documents") // pre-seeded by withFakeHome
	if err := os.WriteFile(filepath.Join(docs, "report.crash"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := OldFilesRemover{MaxAgeDays: 1, Extensions: []string{".crash"}}
	err := r.Remove(item.Item{Path: docs})
	if !errors.Is(err, ErrOffLimits) {
		t.Fatalf("expected ErrOffLimits, got %v", err)
	}
}

// withMockTmutil replaces the package-level tmutilCommand with a fake
// that records args and returns canned output. The dispatcher checks
// args[0] so a single mock can serve list and delete in the same test.
//
// Returns a pointer to a slice that accumulates every invocation, so
// the test can assert "delete was called once per snapshot" etc.
func withMockTmutil(t *testing.T, list []byte, listErr, deleteErr error) *[][]string {
	t.Helper()
	var calls [][]string
	prev := tmutilCommand
	tmutilCommand = func(args ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, args...))
		if len(args) == 0 {
			return nil, errors.New("no args")
		}
		switch args[0] {
		case "listlocalsnapshots":
			return list, listErr
		case "deletelocalsnapshots":
			return nil, deleteErr
		}
		return nil, errors.New("unexpected tmutil call")
	}
	t.Cleanup(func() { tmutilCommand = prev })
	return &calls
}

// TestDefaultResolver_PicksTMSnapshots: tm-snapshots tool maps to
// TMSnapshotsRemover. Without this, a snapshots item would fall
// through to PathRemover trying to os.RemoveAll "tmutil" and error
// in confusing ways.
func TestDefaultResolver_PicksTMSnapshots(t *testing.T) {
	r, err := DefaultResolver(item.Item{Tool: "tm-snapshots", Path: "tmutil"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(TMSnapshotsRemover); !ok {
		t.Fatalf("expected TMSnapshotsRemover, got %T", r)
	}
}

// TestTMSnapshotsRemover_DeletesEachListed: the remover lists
// snapshots, then calls deletelocalsnapshots once per name. This
// covers the canonical happy path on a normal Mac.
func TestTMSnapshotsRemover_DeletesEachListed(t *testing.T) {
	out := []byte(`Snapshots for volume group containing disk /:
com.apple.TimeMachine.2024-06-15-090000.local
com.apple.TimeMachine.2024-06-14-150000.local
`)
	calls := withMockTmutil(t, out, nil, nil)

	r := TMSnapshotsRemover{}
	if err := r.Remove(item.Item{Tool: "tm-snapshots"}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// 1 list + 2 deletes = 3 calls.
	if len(*calls) != 3 {
		t.Fatalf("expected 3 tmutil calls, got %d: %v", len(*calls), *calls)
	}
	if (*calls)[0][0] != "listlocalsnapshots" {
		t.Errorf("first call should be listlocalsnapshots, got %v", (*calls)[0])
	}
	for i, c := range (*calls)[1:] {
		if c[0] != "deletelocalsnapshots" {
			t.Errorf("call %d should be deletelocalsnapshots, got %v", i+1, c)
		}
		// The argument must be just the date — no prefix, no .local.
		if c[1] == "" || c[1] == "com.apple.TimeMachine.2024-06-15-090000.local" {
			t.Errorf("delete arg should be a bare date, got %q", c[1])
		}
	}
}

// TestTMSnapshotsRemover_NoSnapshots: empty list → no deletes, no
// error. Calling the remover on a clean system is a no-op.
func TestTMSnapshotsRemover_NoSnapshots(t *testing.T) {
	calls := withMockTmutil(t, []byte("Snapshots for volume group containing disk /:\n"), nil, nil)

	r := TMSnapshotsRemover{}
	if err := r.Remove(item.Item{Tool: "tm-snapshots"}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("expected only the list call, got %d total: %v", len(*calls), *calls)
	}
}

// TestTMSnapshotsRemover_ListFails: if tmutil itself can't list,
// surface the error instead of silently succeeding.
func TestTMSnapshotsRemover_ListFails(t *testing.T) {
	withMockTmutil(t, nil, errors.New("tmutil bork"), nil)

	r := TMSnapshotsRemover{}
	err := r.Remove(item.Item{Tool: "tm-snapshots"})
	if err == nil {
		t.Fatal("expected error when list fails")
	}
}

// TestTMSnapshotsRemover_DeleteFails: a delete failure on one snapshot
// must not abort the rest. The first error is returned at the end.
func TestTMSnapshotsRemover_DeleteFails(t *testing.T) {
	out := []byte(`Snapshots for volume group containing disk /:
com.apple.TimeMachine.2024-06-15-090000.local
com.apple.TimeMachine.2024-06-14-150000.local
`)
	calls := withMockTmutil(t, out, nil, errors.New("locked snapshot"))

	r := TMSnapshotsRemover{}
	err := r.Remove(item.Item{Tool: "tm-snapshots"})
	if err == nil {
		t.Fatal("expected error to surface")
	}
	// Both deletes were attempted regardless of the first failure.
	if len(*calls) != 3 {
		t.Errorf("expected list + 2 delete attempts, got %d: %v", len(*calls), *calls)
	}
}
