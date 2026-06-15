package appcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// withLowThreshold drops minCacheBytes for the duration of a test so we
// don't have to write 10 MB files into the TempDir to exercise the
// detector. Restored on cleanup.
func withLowThreshold(t *testing.T, n int64) {
	t.Helper()
	prev := minCacheBytes
	minCacheBytes = n
	t.Cleanup(func() { minCacheBytes = prev })
}

// seedCache writes a small file inside an entry's relPath under home.
// The file's bytes determine whether disk.DirSize sees the dir.
func seedCache(t *testing.T, home, relPath string, payload []byte) {
	t.Helper()
	full := filepath.Join(home, relPath)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "data"), payload, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestScanHome_NothingInstalled: a pristine home produces no items.
// Most non-Mac users (CI, fresh accounts) end up here.
func TestScanHome_NothingInstalled(t *testing.T) {
	withLowThreshold(t, 1) // even 1-byte caches would qualify; we just don't seed any
	home := t.TempDir()
	if items := ScanHome(home); len(items) != 0 {
		t.Fatalf("empty home should yield 0 items, got %d", len(items))
	}
}

// TestScanHome_DetectsKnownApp: with a Spotify cache present, the
// detector reports an item with the right metadata. This covers the
// happy path that 99% of users hit.
func TestScanHome_DetectsKnownApp(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	seedCache(t, home, "Library/Caches/com.spotify.client", []byte("audio cache"))

	items := ScanHome(home)
	var spotify *item.Item
	for i := range items {
		if items[i].Tool == "com.spotify.client" && items[i].Name == "Spotify" {
			spotify = &items[i]
			break
		}
	}
	if spotify == nil {
		t.Fatalf("Spotify entry not found in items=%+v", items)
	}
	if spotify.Risk != item.RiskSafe {
		t.Errorf("expected RiskSafe, got %s", spotify.Risk)
	}
	if spotify.Category != item.CategorySystem {
		t.Errorf("expected CategorySystem, got %s", spotify.Category)
	}
	if spotify.Bytes <= 0 {
		t.Errorf("expected non-zero bytes, got %d", spotify.Bytes)
	}
	if len(spotify.DetailArgs) != 1 || spotify.DetailArgs[0] != "Spotify" {
		t.Errorf("DetailArgs should carry app name, got %v", spotify.DetailArgs)
	}
}

// TestScanHome_BelowThresholdIgnored: caches under minCacheBytes don't
// appear. Threshold prevents the wizard from showing 200 KB rows.
func TestScanHome_BelowThresholdIgnored(t *testing.T) {
	// Use the production threshold (10 MB) but seed only a few bytes.
	home := t.TempDir()
	seedCache(t, home, "Library/Caches/com.spotify.client", []byte("tiny"))

	items := ScanHome(home)
	for _, it := range items {
		if it.Tool == "com.spotify.client" {
			t.Fatalf("sub-threshold cache should not be reported, got %+v", it)
		}
	}
}

// TestScanHome_MultipleEntriesSameApp: when two entries point at the
// same app (Slack: Caches AND Service Worker), both surface as separate
// items with sublabels. The user can decline one and accept the other.
func TestScanHome_MultipleEntriesSameApp(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	seedCache(t, home, "Library/Caches/com.tinyspeck.slackmacgap", []byte("a"))
	seedCache(t, home, "Library/Application Support/Slack/Service Worker", []byte("b"))

	items := ScanHome(home)
	var primary, secondary *item.Item
	for i := range items {
		if items[i].Tool != "com.tinyspeck.slackmacgap" {
			continue
		}
		switch items[i].Name {
		case "Slack":
			primary = &items[i]
		case "Slack (Service Worker)":
			secondary = &items[i]
		}
	}
	if primary == nil {
		t.Error("missing primary Slack entry")
	}
	if secondary == nil {
		t.Error("missing Slack (Service Worker) entry")
	}
}

// TestScanHome_AllItemsAreSafeAndSystem: invariant — every item this
// detector returns is RiskSafe and CategorySystem. If an entry ever
// gets misconfigured (typo in a future PR), this test fails loudly.
func TestScanHome_AllItemsAreSafeAndSystem(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	// Seed every entry's path so all of them are exercised.
	for _, e := range entries {
		seedCache(t, home, e.relPath, []byte("x"))
	}

	items := ScanHome(home)
	if len(items) != len(entries) {
		t.Fatalf("expected %d items, got %d", len(entries), len(items))
	}
	for _, it := range items {
		if it.Risk != item.RiskSafe {
			t.Errorf("%s: Risk=%s, want RiskSafe", it.Name, it.Risk)
		}
		if it.Category != item.CategorySystem {
			t.Errorf("%s: Category=%s, want CategorySystem", it.Name, it.Category)
		}
		if it.Path == "" {
			t.Errorf("%s: empty Path", it.Name)
		}
	}
}

// TestFormatName_NoSubLabel: bare app name when no sublabel is set.
func TestFormatName_NoSubLabel(t *testing.T) {
	got := formatName(entry{displayName: "Spotify"})
	if got != "Spotify" {
		t.Errorf("formatName = %q, want %q", got, "Spotify")
	}
}

// TestFormatName_WithSubLabel: sublabel wrapped in parens.
func TestFormatName_WithSubLabel(t *testing.T) {
	got := formatName(entry{displayName: "Slack", subLabel: "Service Worker"})
	want := "Slack (Service Worker)"
	if got != want {
		t.Errorf("formatName = %q, want %q", got, want)
	}
}
