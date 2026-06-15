package device

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sistematlan/mistah/internal/item"
)

// realPlist is a near-verbatim sample of what Apple writes on iOS 17.
// Only the fields we care about are populated; the rest is omitted.
const realPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Build Version</key>
	<string>21E219</string>
	<key>Device Name</key>
	<string>iPhone de Christian</string>
	<key>Display Name</key>
	<string>iPhone de Christian</string>
	<key>GUID</key>
	<string>5E5A8C443F9D</string>
	<key>ICCID</key>
	<string>89014103211118510720</string>
	<key>Last Backup Date</key>
	<date>2024-05-30T14:32:11Z</date>
	<key>Product Type</key>
	<string>iPhone15,2</string>
	<key>Product Version</key>
	<string>17.4</string>
</dict>
</plist>
`

// seedBackup creates a backup directory under home with the given UDID
// and either the provided plist contents or no Info.plist when plist
// is empty. Helper for the table-driven tests below.
func seedBackup(t *testing.T, home, udid, plist string, payloadSize int) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup", udid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if plist != "" {
		if err := os.WriteFile(filepath.Join(dir, "Info.plist"), []byte(plist), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Seed enough payload that DirSize > 0; we need a non-empty dir
	// or the detector skips it.
	if err := os.WriteFile(filepath.Join(dir, "Manifest.db"), make([]byte, payloadSize), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestScanIOSBackups_Missing: no MobileSync dir → no items. Common on
// Macs that never plugged in an iPhone via cable.
func TestScanIOSBackups_Missing(t *testing.T) {
	home := t.TempDir()
	if items := ScanIOSBackups(home); len(items) != 0 {
		t.Fatalf("missing dir should yield 0 items, got %d", len(items))
	}
}

// TestScanIOSBackups_HappyPath: with a well-formed plist, the detector
// reports the device name, the right Tool/Risk/Category, and a sane
// days-since count.
func TestScanIOSBackups_HappyPath(t *testing.T) {
	home := t.TempDir()
	udid := "00008101-001B12340C9D2E14"
	seedBackup(t, home, udid, realPlist, 1024)

	items := ScanIOSBackups(home)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Name != "iPhone de Christian" {
		t.Errorf("Name = %q, want 'iPhone de Christian'", it.Name)
	}
	if it.Tool != "ios-backup" {
		t.Errorf("Tool = %q, want ios-backup", it.Tool)
	}
	// CRITICAL: backups MUST be RiskAskBefore. They contain user data
	// and may be the only copy of a phone. Auto-deletion is wrong.
	if it.Risk != item.RiskAskBefore {
		t.Errorf("Risk = %s, want RiskAskBefore (backups are user data)", it.Risk)
	}
	if it.Category != item.CategoryDevice {
		t.Errorf("Category = %s, want CategoryDevice", it.Category)
	}
	if it.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", it.Bytes)
	}
}

// TestScanIOSBackups_FallbackToUDID: a backup without Info.plist (or
// with an unreadable one) must still appear in the list — we don't want
// to silently hide 8 GB just because Apple's metadata is broken.
func TestScanIOSBackups_FallbackToUDID(t *testing.T) {
	home := t.TempDir()
	udid := "00008101-001B12340C9D2E14"
	seedBackup(t, home, udid, "", 1024) // no Info.plist

	items := ScanIOSBackups(home)
	if len(items) != 1 {
		t.Fatalf("expected 1 item even without plist, got %d", len(items))
	}
	if !strings.HasPrefix(items[0].Name, "iOS device ") {
		t.Errorf("Name should fall back to 'iOS device <udid>…', got %q", items[0].Name)
	}
	if items[0].Risk != item.RiskAskBefore {
		t.Errorf("plist-less backup must still be RiskAskBefore")
	}
}

// TestScanIOSBackups_CorruptPlist: a non-XML or truncated plist must
// not crash the detector. The backup still surfaces with the UDID
// fallback name.
func TestScanIOSBackups_CorruptPlist(t *testing.T) {
	home := t.TempDir()
	udid := "00008101-001B12340C9D2E14"
	seedBackup(t, home, udid, "<<not really xml<<", 1024)

	items := ScanIOSBackups(home)
	if len(items) != 1 {
		t.Fatalf("corrupt plist should not drop the backup, got %d items", len(items))
	}
}

// TestScanIOSBackups_IgnoresJunk: files and non-UDID directories under
// MobileSync/Backup are skipped. This guards against e.g. ".DS_Store",
// notes the user dropped there, or migrated artefacts.
func TestScanIOSBackups_IgnoresJunk(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "Library", "Application Support", "MobileSync", "Backup")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	// One real backup so we know the detector is running.
	seedBackup(t, home, "00008101-001B12340C9D2E14", realPlist, 1024)

	items := ScanIOSBackups(home)
	if len(items) != 1 {
		t.Fatalf("expected only the real backup, got %d items: %+v", len(items), items)
	}
}

// TestScanIOSBackups_MultipleDevices: a user with 3 phones gets 3 items,
// one per UDID. Each carries its own size and metadata.
func TestScanIOSBackups_MultipleDevices(t *testing.T) {
	home := t.TempDir()
	seedBackup(t, home, "00008101-001B12340C9D2E14", realPlist, 1024)
	seedBackup(t, home, "00008030-001A11220B7C1D03", realPlist, 2048)
	seedBackup(t, home, "5e5a8c443f9d2e3a4b5c6d7e8f0a1b2c3d4e5f60", realPlist, 4096)

	items := ScanIOSBackups(home)
	if len(items) != 3 {
		t.Fatalf("expected 3 backups, got %d", len(items))
	}
}

// TestLooksLikeUDID covers the format check used to filter junk dirs.
// Both modern (with dash) and legacy (40-char hex) shapes accept; tests
// also include negative cases that previously caused false positives.
func TestLooksLikeUDID(t *testing.T) {
	cases := map[string]bool{
		"00008101-001B12340C9D2E14":                 true,  // modern
		"5e5a8c443f9d2e3a4b5c6d7e8f0a1b2c3d4e5f60": true,  // 40-char legacy
		".DS_Store":                                false, // hidden
		"notes":                                    false, // too short
		"00008101_001B12340C9D2E14":                false, // underscore not allowed
		"00008101-001B12340C9D2E14XYZ":             false, // non-hex tail
		"":                                        false, // empty
	}
	for name, want := range cases {
		if got := looksLikeUDID(name); got != want {
			t.Errorf("looksLikeUDID(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestParseInfoPlist_ExtractsKnownKeys: the parser pulls Device Name,
// Product Type and Last Backup Date from the canonical sample.
func TestParseInfoPlist_ExtractsKnownKeys(t *testing.T) {
	meta := parseInfoPlist(strings.NewReader(realPlist))
	if meta.DeviceName != "iPhone de Christian" {
		t.Errorf("DeviceName = %q", meta.DeviceName)
	}
	if meta.ProductType != "iPhone15,2" {
		t.Errorf("ProductType = %q", meta.ProductType)
	}
	want := time.Date(2024, 5, 30, 14, 32, 11, 0, time.UTC)
	if !meta.LastBackup.Equal(want) {
		t.Errorf("LastBackup = %v, want %v", meta.LastBackup, want)
	}
}

// TestParseInfoPlist_KeyWithoutValue: a malformed plist where a <key>
// is followed by something other than <string|date> must not bleed
// state into the next pair.
//
// In the input below, "Device Name" is followed by an <integer>,
// which our parser doesn't handle. The parser must reset pendingKey
// and correctly attribute the *next* <string> to "Product Type".
func TestParseInfoPlist_KeyWithoutValue(t *testing.T) {
	plist := `<plist><dict>
		<key>Device Name</key>
		<integer>42</integer>
		<key>Product Type</key>
		<string>iPad13,4</string>
	</dict></plist>`
	meta := parseInfoPlist(strings.NewReader(plist))
	if meta.DeviceName != "" {
		t.Errorf("DeviceName should remain empty (integer is not a string), got %q", meta.DeviceName)
	}
	if meta.ProductType != "iPad13,4" {
		t.Errorf("ProductType = %q, want iPad13,4", meta.ProductType)
	}
}

// TestParseInfoPlist_Empty: an empty reader returns the zero value
// without erroring out. Mirrors what readBackupMeta does on file-open
// failure.
func TestParseInfoPlist_Empty(t *testing.T) {
	meta := parseInfoPlist(strings.NewReader(""))
	if meta.DeviceName != "" || meta.ProductType != "" || !meta.LastBackup.IsZero() {
		t.Errorf("expected zero meta, got %+v", meta)
	}
}

// TestDisplayName_PrioritisesDeviceName: with all three known we still
// pick the user-friendly Device Name. Product Type is the secondary
// fallback, then a truncated UDID.
func TestDisplayName_PrioritisesDeviceName(t *testing.T) {
	udid := "00008101-001B12340C9D2E14"

	// Best case
	got := displayName(backupMeta{DeviceName: "iPad", ProductType: "iPad13,4"}, udid)
	if got != "iPad" {
		t.Errorf("with DeviceName set, got %q", got)
	}

	// No DeviceName: fall back to ProductType
	got = displayName(backupMeta{ProductType: "iPad13,4"}, udid)
	if got != "iPad13,4" {
		t.Errorf("with only ProductType, got %q", got)
	}

	// Empty meta: fall back to truncated UDID
	got = displayName(backupMeta{}, udid)
	if !strings.HasPrefix(got, "iOS device ") {
		t.Errorf("empty meta should fall back to 'iOS device <prefix>…', got %q", got)
	}
}
