package device

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// seedIPSW creates a fake .ipsw file with the given size under the
// canonical iTunes updates path. Returns the dir so callers can create
// more siblings.
func seedIPSW(t *testing.T, home, name string, size int) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "iTunes", "iPhone Software Updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestIPSW_Missing: no dir → empty list. Mac without iTunes/iPhone
// sync history is the most common case.
func TestIPSW_Missing(t *testing.T) {
	home := t.TempDir()
	if items := ScanIPSW(home); len(items) != 0 {
		t.Fatalf("missing dir should yield 0 items, got %d", len(items))
	}
}

// TestIPSW_DetectsMultiple: several .ipsw files become several items,
// each with its own size and Risk = Safe.
func TestIPSW_DetectsMultiple(t *testing.T) {
	home := t.TempDir()
	seedIPSW(t, home, "iPhone15,2_17.4_21E219_Restore.ipsw", 1024)
	seedIPSW(t, home, "iPad13,4_17.4_21E219_Restore.ipsw", 2048)

	items := ScanIPSW(home)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, it := range items {
		if it.Risk != item.RiskSafe {
			t.Errorf("ipsw should be RiskSafe, got %s", it.Risk)
		}
		if it.Category != item.CategoryDevice {
			t.Errorf("ipsw should be CategoryDevice, got %s", it.Category)
		}
		if it.Tool != "ios-update" {
			t.Errorf("ipsw tool = %s, want ios-update", it.Tool)
		}
	}
}

// TestIPSW_IgnoresNonIPSW: unrelated files in the dir are ignored.
// Defends against an over-eager glob that would pick up logs or .DS_Store.
func TestIPSW_IgnoresNonIPSW(t *testing.T) {
	home := t.TempDir()
	dir := seedIPSW(t, home, "iPhone15,2_17.4_21E219_Restore.ipsw", 100)
	if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := ScanIPSW(home)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Name != "iPhone15,2_17.4_21E219_Restore.ipsw" {
		t.Errorf("unexpected item name: %s", items[0].Name)
	}
}

// TestParseIPSWName_Standard: typical Apple naming maps to (model, version).
func TestParseIPSWName_Standard(t *testing.T) {
	cases := map[string]struct{ device, version string }{
		"iPhone15,2_17.4_21E219_Restore.ipsw": {"iPhone15,2", "17.4"},
		"iPad13,4_17.4_21E219_Restore.ipsw":   {"iPad13,4", "17.4"},
		"AppleTV5,3_17.4_21E219_Restore.ipsw": {"AppleTV5,3", "17.4"},
	}
	for name, want := range cases {
		gotDev, gotVer := parseIPSWName(name)
		if gotDev != want.device || gotVer != want.version {
			t.Errorf("parseIPSWName(%q) = (%q, %q), want (%q, %q)",
				name, gotDev, gotVer, want.device, want.version)
		}
	}
}

// TestParseIPSWName_Unrecognised: unparseable filenames fall back to the
// raw name and an empty version. Must never panic.
func TestParseIPSWName_Unrecognised(t *testing.T) {
	dev, ver := parseIPSWName("something-weird.ipsw")
	if dev != "something-weird.ipsw" || ver != "" {
		t.Errorf("unexpected fallback: device=%q version=%q", dev, ver)
	}
}
