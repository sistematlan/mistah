package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// withLowMessagesAge lowers the cutoff so tests can use recent files
// instead of back-dating everything 6 months. Restored on cleanup.
func withLowMessagesAge(t *testing.T, days int) {
	t.Helper()
	prev := messagesAttachmentMaxAgeDays
	messagesAttachmentMaxAgeDays = days
	t.Cleanup(func() { messagesAttachmentMaxAgeDays = prev })
}

// seedAttachment writes a file deep in the hashed Attachments tree with
// a given size and mtime, mirroring the real ab/cd/<guid>/file layout.
func seedAttachment(t *testing.T, home, hashA, hashB, guid, name string, size int, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Messages", "Attachments", hashA, hashB, guid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	if !mtime.IsZero() {
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// TestMessages_Missing: no Attachments dir → no item. Macs that never
// used iMessage on the desktop are in this state.
func TestMessages_Missing(t *testing.T) {
	home := t.TempDir()
	for _, it := range ScanHome(home) {
		if it.Tool == "ios-messages" {
			t.Fatalf("missing Attachments should produce no item, got %+v", it)
		}
	}
}

// TestMessages_OnlyOldCounted: a recent and an old attachment are
// seeded; only the old one's bytes/count appear. This is the headline
// guarantee — recent conversations stay fully intact.
func TestMessages_OnlyOldCounted(t *testing.T) {
	home := t.TempDir()
	now := time.Now()
	// Old (200 days) → counted.
	seedAttachment(t, home, "ab", "cd", "GUID-OLD", "IMG_0001.heic", 5000, now.AddDate(0, 0, -200))
	// Recent (10 days) → ignored.
	seedAttachment(t, home, "ef", "01", "GUID-NEW", "IMG_9999.heic", 9999, now.AddDate(0, 0, -10))

	var found *item.Item
	for _, it := range ScanHome(home) {
		if it.Tool == "ios-messages" {
			found = &it
		}
	}
	if found == nil {
		t.Fatal("expected an ios-messages item")
	}
	if found.Bytes != 5000 {
		t.Errorf("bytes = %d, want 5000 (only the old attachment)", found.Bytes)
	}
	// CRITICAL: attachments are user data. Must be RiskAskBefore.
	if found.Risk != item.RiskAskBefore {
		t.Errorf("Risk = %s, want RiskAskBefore (attachments are user data)", found.Risk)
	}
	if found.Category != item.CategorySystem {
		t.Errorf("Category = %s, want CategorySystem", found.Category)
	}
	if len(found.DetailArgs) != 2 {
		t.Fatalf("DetailArgs should carry count + months, got %v", found.DetailArgs)
	}
	if count, ok := found.DetailArgs[0].(int); !ok || count != 1 {
		t.Errorf("count arg = %v, want 1", found.DetailArgs[0])
	}
}

// TestMessages_NoOldFiles_NoItem: only-recent attachments → no item.
// The wizard shouldn't show a 0-byte iMessage row.
func TestMessages_NoOldFiles_NoItem(t *testing.T) {
	home := t.TempDir()
	seedAttachment(t, home, "ab", "cd", "GUID-NEW", "IMG_0001.heic", 5000, time.Now().AddDate(0, 0, -5))

	for _, it := range ScanHome(home) {
		if it.Tool == "ios-messages" {
			t.Fatalf("only-recent attachments should yield no item, got %+v", it)
		}
	}
}

// TestMessages_AggregatesAcrossTree: multiple old attachments in
// different hashed branches sum into a single item. Verifies the
// recursive walk and the "one aggregated item" design.
func TestMessages_AggregatesAcrossTree(t *testing.T) {
	home := t.TempDir()
	old := time.Now().AddDate(0, 0, -200)
	seedAttachment(t, home, "ab", "cd", "G1", "a.heic", 1000, old)
	seedAttachment(t, home, "ab", "ef", "G2", "b.mov", 2000, old)
	seedAttachment(t, home, "f0", "12", "G3", "c.pdf", 3000, old)

	var found *item.Item
	for _, it := range ScanHome(home) {
		if it.Tool == "ios-messages" {
			found = &it
		}
	}
	if found == nil {
		t.Fatal("expected an aggregated ios-messages item")
	}
	if found.Bytes != 6000 {
		t.Errorf("bytes = %d, want 6000 (1000+2000+3000)", found.Bytes)
	}
	if count := found.DetailArgs[0].(int); count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// TestSummarizeOldAttachments_MissingRoot: a missing root returns
// ok=false, distinct from an empty-but-present dir.
func TestSummarizeOldAttachments_MissingRoot(t *testing.T) {
	_, _, ok := summarizeOldAttachments(filepath.Join(t.TempDir(), "nope"), 180)
	if ok {
		t.Error("missing root should return ok=false")
	}
}

// TestSummarizeOldAttachments_EmptyPresent: an existing but empty dir
// returns ok=true with zero bytes/count.
func TestSummarizeOldAttachments_EmptyPresent(t *testing.T) {
	dir := t.TempDir()
	bytes, count, ok := summarizeOldAttachments(dir, 180)
	if !ok {
		t.Error("present empty dir should return ok=true")
	}
	if bytes != 0 || count != 0 {
		t.Errorf("empty dir should be (0,0), got (%d,%d)", bytes, count)
	}
}

// TestMessages_LowAgeThresholdHonored: the mutable cutoff is respected,
// so the test seam used by the suite actually changes behaviour.
func TestMessages_LowAgeThresholdHonored(t *testing.T) {
	withLowMessagesAge(t, 7)
	home := t.TempDir()
	// 30 days old — older than the lowered 7-day cutoff → counted.
	seedAttachment(t, home, "ab", "cd", "G1", "a.heic", 1234, time.Now().AddDate(0, 0, -30))

	var found bool
	for _, it := range ScanHome(home) {
		if it.Tool == "ios-messages" {
			found = true
		}
	}
	if !found {
		t.Error("with a 7-day cutoff, a 30-day-old attachment should be reported")
	}
}
