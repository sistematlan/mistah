package appcache

import (
	"strings"
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// TestBrowsers_NothingInstalled: pristine home → 0 browser items.
func TestBrowsers_NothingInstalled(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	if items := ScanBrowsers(home); len(items) != 0 {
		t.Fatalf("empty home should yield 0 browser items, got %d", len(items))
	}
}

// TestBrowsers_DetectsChrome: with a Chrome cache present, the detector
// reports an item with the right metadata.
func TestBrowsers_DetectsChrome(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	seedCache(t, home, "Library/Caches/Google/Chrome", []byte("chrome cache contents"))

	items := ScanBrowsers(home)
	var chrome *item.Item
	for i := range items {
		if items[i].Tool == "browser-chrome" {
			chrome = &items[i]
			break
		}
	}
	if chrome == nil {
		t.Fatalf("Chrome entry not found in items=%+v", items)
	}
	if chrome.Risk != item.RiskSafe {
		t.Errorf("expected RiskSafe, got %s", chrome.Risk)
	}
	if chrome.Category != item.CategorySystem {
		t.Errorf("expected CategorySystem, got %s", chrome.Category)
	}
	if chrome.Name != "Google Chrome" {
		t.Errorf("expected Name='Google Chrome', got %q", chrome.Name)
	}
}

// TestBrowsers_DetectsAllConfigured: when every browser cache exists,
// every entry surfaces. Catches typos in browsers[] that would silently
// drop a row.
func TestBrowsers_DetectsAllConfigured(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	for _, b := range browsers {
		seedCache(t, home, b.relPath, []byte("x"))
	}

	items := ScanBrowsers(home)
	if len(items) != len(browsers) {
		t.Fatalf("expected %d items, got %d", len(browsers), len(items))
	}
	seen := map[string]bool{}
	for _, it := range items {
		seen[it.Tool] = true
	}
	for _, b := range browsers {
		if !seen[b.tool] {
			t.Errorf("browser %s (tool=%s) was not detected", b.displayName, b.tool)
		}
	}
}

// TestBrowsers_BelowThresholdIgnored: caches under minCacheBytes don't
// appear, just like the apps detector.
func TestBrowsers_BelowThresholdIgnored(t *testing.T) {
	home := t.TempDir() // production threshold (10 MB)
	seedCache(t, home, "Library/Caches/Google/Chrome", []byte("tiny"))

	items := ScanBrowsers(home)
	for _, it := range items {
		if it.Tool == "browser-chrome" {
			t.Fatalf("sub-threshold cache should not be reported, got %+v", it)
		}
	}
}

// TestBrowsers_PathDisciplineGuard is the headline regression test for
// this file. It enforces two rules at compile-test time:
//
//  1. Every entry's relPath MUST start with "Library/Caches/". Anything
//     under "Application Support" or "Library/Containers" risks touching
//     bookmarks, cookies, or login data.
//
//  2. The relPath MUST NOT contain "Default", "Profiles", "User Data" or
//     similar profile-data markers. Those names indicate the path includes
//     non-cache assets even when followed by "Cache".
//
// If a future PR tries to add a path like
// "Library/Application Support/Google/Chrome/Default/Cache",
// this test fails and forces the change to either refactor (split cache
// vs profile detection) or document the exception explicitly.
func TestBrowsers_PathDisciplineGuard(t *testing.T) {
	mustBeUnder := "Library/Caches/"
	bannedSegments := []string{"Default", "Profiles", "User Data", "Application Support", "Containers"}

	for _, b := range browsers {
		if !strings.HasPrefix(b.relPath, mustBeUnder) {
			t.Errorf("%s: relPath %q must start with %q",
				b.tool, b.relPath, mustBeUnder)
		}
		for _, banned := range bannedSegments {
			if strings.Contains(b.relPath, banned) {
				t.Errorf("%s: relPath %q contains forbidden segment %q",
					b.tool, b.relPath, banned)
			}
		}
	}
}

// TestScanHome_IncludesBothAppsAndBrowsers: ScanHome aggregates apps and
// browsers in a single slice. Sanity check that both detectors fire.
func TestScanHome_IncludesBothAppsAndBrowsers(t *testing.T) {
	withLowThreshold(t, 1)
	home := t.TempDir()
	seedCache(t, home, "Library/Caches/com.spotify.client", []byte("a"))
	seedCache(t, home, "Library/Caches/Google/Chrome", []byte("b"))

	items := ScanHome(home)
	var sawApp, sawBrowser bool
	for _, it := range items {
		if it.Tool == "com.spotify.client" {
			sawApp = true
		}
		if it.Tool == "browser-chrome" {
			sawBrowser = true
		}
	}
	if !sawApp {
		t.Error("ScanHome missed the consumer app entry")
	}
	if !sawBrowser {
		t.Error("ScanHome missed the browser entry")
	}
}
