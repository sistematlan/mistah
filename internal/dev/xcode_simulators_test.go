package dev

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// realSimctlOutput is a sample of what `xcrun simctl list devices --json`
// produces on a real Mac with both available and unavailable runtimes.
// One iPhone 13 on iOS 17.4 (current, available) and one iPhone 11 on
// iOS 14.0 (deprecated runtime, all devices unavailable).
const realSimctlOutput = `{
  "devices" : {
    "com.apple.CoreSimulator.SimRuntime.iOS-17-4" : [
      {
        "udid" : "11111111-1111-1111-1111-111111111111",
        "name" : "iPhone 13",
        "deviceTypeIdentifier" : "com.apple.CoreSimulator.SimDeviceType.iPhone-13",
        "state" : "Shutdown",
        "isAvailable" : true
      }
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-14-0 -- unavailable, runtime profile not found" : [
      {
        "udid" : "22222222-2222-2222-2222-222222222222",
        "name" : "iPhone 11",
        "deviceTypeIdentifier" : "com.apple.CoreSimulator.SimDeviceType.iPhone-11",
        "state" : "Shutdown",
        "isAvailable" : false
      },
      {
        "udid" : "33333333-3333-3333-3333-333333333333",
        "name" : "iPhone 11 Pro",
        "deviceTypeIdentifier" : "com.apple.CoreSimulator.SimDeviceType.iPhone-11-Pro",
        "state" : "Shutdown",
        "isAvailable" : false
      }
    ]
  }
}`

// withMockXcrun swaps xcrunCommand for a function returning canned
// output. Restored on cleanup.
func withMockXcrun(t *testing.T, output []byte, err error) {
	t.Helper()
	prev := xcrunCommand
	xcrunCommand = func(ctx context.Context, args ...string) ([]byte, error) {
		return output, err
	}
	t.Cleanup(func() { xcrunCommand = prev })
}

// seedSimulator creates the on-disk directory the detector measures.
// 200 KB so it's above the 100 KB skip threshold.
func seedSimulator(t *testing.T, home, udid string) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices", udid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "device.plist"), make([]byte, 200*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestParseUnavailableDevices_OnlyDeadOnes: from the realistic JSON
// above we must extract the two iPhone 11 devices under the unavailable
// runtime and skip the iPhone 13 on iOS 17.4.
func TestParseUnavailableDevices_OnlyDeadOnes(t *testing.T) {
	devs := parseUnavailableDevices([]byte(realSimctlOutput))
	if len(devs) != 2 {
		t.Fatalf("expected 2 unavailable devices, got %d: %+v", len(devs), devs)
	}
	for _, d := range devs {
		if d.Name == "iPhone 13" {
			t.Errorf("iPhone 13 on iOS-17-4 must NOT be reported (it's available)")
		}
	}
}

// TestParseUnavailableDevices_DeviceFlagOnly: a device marked
// isAvailable=false under an *available* runtime is still reported.
// Apple flips that flag for chip-compat reasons sometimes.
func TestParseUnavailableDevices_DeviceFlagOnly(t *testing.T) {
	in := `{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"udid": "AAA", "name": "iPhone 13", "isAvailable": false}
			]
		}
	}`
	devs := parseUnavailableDevices([]byte(in))
	if len(devs) != 1 || devs[0].UDID != "AAA" {
		t.Fatalf("expected isAvailable=false device to be flagged, got %+v", devs)
	}
}

// TestParseUnavailableDevices_BadJSON: a non-JSON input must return
// nil rather than panicking.
func TestParseUnavailableDevices_BadJSON(t *testing.T) {
	devs := parseUnavailableDevices([]byte("not json"))
	if len(devs) != 0 {
		t.Errorf("malformed JSON should yield 0 devices, got %d", len(devs))
	}
}

// TestPrettyRuntime_StripsPrefixAndSuffix: the runtime key as Apple
// writes it must end up as "iOS-15-5" in the UI.
func TestPrettyRuntime_StripsPrefixAndSuffix(t *testing.T) {
	cases := map[string]string{
		"com.apple.CoreSimulator.SimRuntime.iOS-15-5 -- unavailable, runtime profile not found": "iOS-15-5",
		"com.apple.CoreSimulator.SimRuntime.iOS-17-4":                                           "iOS-17-4",
		"weird-format-future-apple-change":                                                      "weird-format-future-apple-change",
	}
	for in, want := range cases {
		if got := prettyRuntime(in); got != want {
			t.Errorf("prettyRuntime(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestScanXcodeSimulators_HappyPath: with mocked xcrun output and seeded
// device directories, we get one Item per stale device with the right
// metadata.
func TestScanXcodeSimulators_HappyPath(t *testing.T) {
	withMockXcrun(t, []byte(realSimctlOutput), nil)
	home := t.TempDir()
	seedSimulator(t, home, "22222222-2222-2222-2222-222222222222")
	seedSimulator(t, home, "33333333-3333-3333-3333-333333333333")
	// iPhone 13 is available; its dir would be on disk too but is
	// not reported. Don't seed it — saves test time.

	items := ScanXcodeSimulators(home)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}
	for _, it := range items {
		if it.Tool != "xcode-simulator" {
			t.Errorf("Tool = %s, want xcode-simulator", it.Tool)
		}
		// CRITICAL: simulators may hold app data the dev wants to
		// recover. NEVER auto-delete; per-item prompt only.
		if it.Risk != item.RiskAskBefore {
			t.Errorf("Risk = %s, want RiskAskBefore", it.Risk)
		}
		if it.Category != item.CategoryCache {
			t.Errorf("Category = %s, want CategoryCache", it.Category)
		}
		if it.Bytes <= 0 {
			t.Errorf("Bytes = %d, want > 0", it.Bytes)
		}
	}
}

// TestScanXcodeSimulators_NoXcrun: when xcrun returns an error (Xcode
// not installed, command missing), the detector silently produces no
// items. mistah is for both dev and non-dev users; missing dev tools
// should be a clean no-op.
func TestScanXcodeSimulators_NoXcrun(t *testing.T) {
	withMockXcrun(t, nil, errors.New("xcrun: command not found"))
	if items := ScanXcodeSimulators(t.TempDir()); len(items) != 0 {
		t.Fatalf("missing xcrun should yield 0 items, got %d", len(items))
	}
}

// TestScanXcodeSimulators_NoStaleSims: an Xcode install with all
// runtimes available produces no items. The wizard then doesn't show
// a stale-simulator row at all.
func TestScanXcodeSimulators_NoStaleSims(t *testing.T) {
	allHealthy := `{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-4": [
				{"udid": "AAA", "name": "iPhone 13", "isAvailable": true}
			]
		}
	}`
	withMockXcrun(t, []byte(allHealthy), nil)
	if items := ScanXcodeSimulators(t.TempDir()); len(items) != 0 {
		t.Fatalf("only-healthy runtimes should yield 0 items, got %d", len(items))
	}
}

// TestScanXcodeSimulators_TinyDeviceSkipped: simulators with under
// 100 KB on disk (essentially placeholders) don't appear. Saves the
// wizard from showing 8 KB rows.
func TestScanXcodeSimulators_TinyDeviceSkipped(t *testing.T) {
	withMockXcrun(t, []byte(realSimctlOutput), nil)
	home := t.TempDir()
	// Don't seed any device dirs; DirSize on a non-existent path
	// returns 0, which trips the < 100 KB filter.
	if items := ScanXcodeSimulators(home); len(items) != 0 {
		t.Fatalf("empty device dirs should be skipped, got %d items", len(items))
	}
}
