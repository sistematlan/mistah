package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// seedFile writes a file with given size and modification time.
// Returns the full path. Helper because every test here builds the
// same shape: a directory with N files of K bytes each.
func seedFile(t *testing.T, dir, name string, size int, mtime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	data := make([]byte, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// TestTrash_Empty: an empty Trash returns no item. The wizard must not
// show "0 bytes" lines; absent is better than zero.
func TestTrash_Empty(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".Trash"), 0o755); err != nil {
		t.Fatal(err)
	}
	if items := ScanHome(home); len(items) != 0 {
		t.Fatalf("empty Trash should yield 0 items, got %d", len(items))
	}
}

// TestTrash_Missing: home without ~/.Trash also returns no item.
// macOS lazily creates it, so absence is normal on a fresh user.
func TestTrash_Missing(t *testing.T) {
	home := t.TempDir()
	if items := ScanHome(home); len(items) != 0 {
		t.Fatalf("missing Trash should yield 0 items, got %d", len(items))
	}
}

// TestTrash_CountsAndAge: with three files of varying ages, the Trash
// item must reflect total bytes, item count and the oldest age.
// This is the headline UX the wizard shows.
func TestTrash_CountsAndAge(t *testing.T) {
	home := t.TempDir()
	trashDir := filepath.Join(home, ".Trash")
	now := time.Now()
	seedFile(t, trashDir, "recent.txt", 100, now.Add(-1*24*time.Hour))
	seedFile(t, trashDir, "older.bin", 500, now.Add(-10*24*time.Hour))
	seedFile(t, trashDir, "ancient.dmg", 2000, now.Add(-90*24*time.Hour))
	// Hidden FS bookkeeping should NOT count as an "item" but ITS
	// bytes should still be reflected (matches Finder's number).
	seedFile(t, trashDir, ".DS_Store", 50, now)

	items := ScanHome(home)
	var trashItem *item.Item
	for i := range items {
		if items[i].Tool == "trash" {
			trashItem = &items[i]
		}
	}
	if trashItem == nil {
		t.Fatal("expected trash item, got none")
	}
	if want := int64(100 + 500 + 2000 + 50); trashItem.Bytes != want {
		t.Errorf("Trash bytes = %d, want %d", trashItem.Bytes, want)
	}
	if trashItem.Risk != item.RiskAskBefore {
		t.Errorf("Trash should be RiskAskBefore, got %s", trashItem.Risk)
	}
	if trashItem.Category != item.CategorySystem {
		t.Errorf("Trash should be CategorySystem, got %s", trashItem.Category)
	}
	if len(trashItem.DetailArgs) != 2 {
		t.Fatalf("DetailArgs should have count + days, got %v", trashItem.DetailArgs)
	}
	count, ok := trashItem.DetailArgs[0].(int)
	if !ok || count != 3 {
		t.Errorf("user-visible count should be 3 (excluding .DS_Store), got %v", trashItem.DetailArgs[0])
	}
	days, ok := trashItem.DetailArgs[1].(int)
	if !ok || days < 89 || days > 91 {
		t.Errorf("oldest should be ~90 days, got %v", trashItem.DetailArgs[1])
	}
}

// TestMailDownloads_DetectsAndSizes: a non-empty Mail Downloads dir
// reports bytes, RiskSafe, CategorySystem.
//
// Sizes are checked as a lower bound rather than equality because
// disk.DirSize uses `du -sk`, which rounds to filesystem block size
// (4 KB on APFS). A 1-KB file occupies one block; we just want to
// assert "non-zero and at least the file's logical size".
func TestMailDownloads_DetectsAndSizes(t *testing.T) {
	home := t.TempDir()
	mdir := filepath.Join(home, "Library", "Containers", "com.apple.mail",
		"Data", "Library", "Mail Downloads")
	seedFile(t, mdir, "report.pdf", 1024, time.Time{})

	items := ScanHome(home)
	var found *item.Item
	for i := range items {
		if items[i].Tool == "mail" {
			found = &items[i]
		}
	}
	if found == nil {
		t.Fatal("expected mail downloads item")
	}
	if found.Bytes < 1024 {
		t.Errorf("bytes = %d, want >= 1024", found.Bytes)
	}
	if found.Risk != item.RiskSafe {
		t.Errorf("Mail Downloads should be RiskSafe, got %s", found.Risk)
	}
	if found.Category != item.CategorySystem {
		t.Errorf("Mail Downloads should be CategorySystem, got %s", found.Category)
	}
}

// TestMailDownloads_Missing: no item when the dir doesn't exist.
// Most Macs that never opened Mail.app are in this state.
func TestMailDownloads_Missing(t *testing.T) {
	home := t.TempDir()
	for _, it := range ScanHome(home) {
		if it.Tool == "mail" {
			t.Fatalf("missing Mail dir should produce no item, got %+v", it)
		}
	}
}

// TestQuickLook_DetectsAndSizes: thumbnail cache reports as RiskSafe.
// Same disk-block-rounding caveat as Mail Downloads applies.
func TestQuickLook_DetectsAndSizes(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Library", "Caches", "com.apple.QuickLook.thumbnailcache")
	seedFile(t, dir, "thumb.data", 512, time.Time{})
	seedFile(t, dir, "index.sqlite", 256, time.Time{})

	items := ScanHome(home)
	var found *item.Item
	for i := range items {
		if items[i].Tool == "quicklook" {
			found = &items[i]
		}
	}
	if found == nil {
		t.Fatal("expected quicklook item")
	}
	if found.Bytes <= 0 {
		t.Errorf("bytes should be > 0, got %d", found.Bytes)
	}
	if found.Risk != item.RiskSafe {
		t.Errorf("QuickLook should be RiskSafe, got %s", found.Risk)
	}
}

// TestDaysSince_NeverNegative: clock skew or a future mtime must not
// produce a negative day count in the UI.
func TestDaysSince_NeverNegative(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	if d := daysSince(future); d != 0 {
		t.Errorf("future time should clamp to 0 days, got %d", d)
	}
	if d := daysSince(time.Time{}); d != 0 {
		t.Errorf("zero time should yield 0 days, got %d", d)
	}
}
