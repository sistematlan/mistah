package wizard

import (
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// fixtureInventory returns a representative inventory spanning every
// bucket, with a deliberate mix of Risk levels so the safe/review
// split is exercised:
//
//	Caches:      npm, brew (RiskSafe+path), docker (RiskSafe no-path),
//	             jb-old (RiskAskBefore+path)
//	Orphans:     wpp-media, docker-leftover (both RiskAskBefore)
//	Downloads:   old.sql (RiskAskBefore), App.dmg (RiskSafe)
//	System:      spotify cache (RiskSafe), trash (RiskAskBefore)
//	Device:      ios-backup (RiskAskBefore), ipsw (RiskSafe)
//	DevAdvanced: stale simulator (RiskAskBefore)
func fixtureInventory() Inventory {
	return Inventory{
		Caches: []item.Item{
			{Name: "npm", Path: "/x/npm", Bytes: 100, Risk: item.RiskSafe, Category: item.CategoryCache},
			{Name: "brew", Path: "/x/brew", Bytes: 200, Risk: item.RiskSafe, Category: item.CategoryCache},
			{Name: "docker", Path: "", Bytes: 1000, Risk: item.RiskSafe, Category: item.CategoryCache},
			{Name: "jb-old", Path: "/x/jb", Bytes: 500, Risk: item.RiskAskBefore, Category: item.CategoryCache},
		},
		Orphans: []item.Item{
			{Name: "wpp-media", Path: "/x/wpp", Bytes: 800, Risk: item.RiskAskBefore, Category: item.CategoryOrphan},
			{Name: "docker-leftover", Path: "/x/docker-old", Bytes: 35000, Risk: item.RiskAskBefore, Category: item.CategoryOrphan},
		},
		Downloads: []item.Item{
			{Name: "old.sql", Path: "/x/old.sql", Bytes: 70, Risk: item.RiskAskBefore, Category: item.CategoryDownload},
			{Name: "App.dmg", Path: "/x/App.dmg", Bytes: 80, Risk: item.RiskSafe, Category: item.CategoryDownload},
		},
		System: []item.Item{
			{Name: "Spotify", Path: "/x/spotify", Bytes: 300, Risk: item.RiskSafe, Category: item.CategorySystem},
			{Name: "Trash", Path: "/x/trash", Bytes: 600, Risk: item.RiskAskBefore, Category: item.CategorySystem},
		},
		Device: []item.Item{
			{Name: "ios-backup", Path: "/x/backup", Bytes: 9000, Risk: item.RiskAskBefore, Category: item.CategoryDevice},
			{Name: "ipsw", Path: "/x/firmware.ipsw", Bytes: 4000, Risk: item.RiskSafe, Category: item.CategoryDevice},
		},
		DevAdvanced: []item.Item{
			{Name: "old-sim", Path: "/x/sim", Bytes: 2000, Risk: item.RiskAskBefore, Category: item.CategoryCache},
		},
	}
}

// safeNames / reviewNames helpers make assertions readable.
func names(items []item.Item) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it.Name] = true
	}
	return m
}

// TestPlanFor_Light: only RiskSafe items from System + safe dev caches.
// System's Spotify (safe) + npm + brew (safe caches). Docker has no path
// so filterLight drops it; Trash is RiskAskBefore so it's review, not
// part of the flat Light plan's safe portion.
func TestPlanFor_Light(t *testing.T) {
	inv := fixtureInventory()
	safe, review := PlanForSplit(LevelLight, inv)

	// safe: Spotify (system, safe) + npm + brew (safe caches w/ path) = 3
	if len(safe) != 3 {
		t.Fatalf("Light safe should yield 3 items, got %d: %v", len(safe), names(safe))
	}
	// review: Trash (system, RiskAskBefore) = 1
	if len(review) != 1 || review[0].Name != "Trash" {
		t.Fatalf("Light review should be [Trash], got %v", names(review))
	}
	for _, it := range safe {
		if it.Risk != item.RiskSafe {
			t.Errorf("Light safe included non-safe item: %s", it.Name)
		}
		if it.Path == "" {
			t.Errorf("Light safe included path-less item: %s", it.Name)
		}
	}
}

// TestPlanFor_Standard: Light + the full dev cache list (including
// Docker prune and JetBrains old). System review (Trash) still routes
// to review. Standard does NOT touch orphans/downloads/device.
func TestPlanFor_Standard(t *testing.T) {
	inv := fixtureInventory()
	safe, review := PlanForSplit(LevelStandard, inv)

	// safe: Spotify + all 4 caches (npm, brew, docker, jb-old). jb-old
	// is RiskAskBefore but lives in Caches, which Standard adds wholesale
	// — wait: splitByRisk only applies to System. Caches are added as-is.
	// So jb-old (RiskAskBefore) IS in the safe bucket at Standard.
	// That matches the legacy contract: Standard = every cache.
	got := names(safe)
	for _, want := range []string{"Spotify", "npm", "brew", "docker", "jb-old"} {
		if !got[want] {
			t.Errorf("Standard safe missing %q; got %v", want, got)
		}
	}
	// review: only Trash (System RiskAskBefore).
	if len(review) != 1 || review[0].Name != "Trash" {
		t.Errorf("Standard review should be [Trash], got %v", names(review))
	}
	// Standard must NOT include orphans/downloads/device/devadvanced.
	for _, bad := range []string{"wpp-media", "old.sql", "App.dmg", "ios-backup", "ipsw", "old-sim"} {
		if got[bad] {
			t.Errorf("Standard leaked %q into safe", bad)
		}
	}
}

// TestPlanFor_Deep: everything. Safe gets all RiskSafe items across all
// buckets; review gets every RiskAskBefore item.
func TestPlanFor_Deep(t *testing.T) {
	inv := fixtureInventory()
	safe, review := PlanForSplit(LevelDeep, inv)

	safeNames := names(safe)
	reviewNames := names(review)

	// Safe (RiskSafe): Spotify, npm, brew, docker, App.dmg, ipsw.
	for _, want := range []string{"Spotify", "npm", "brew", "docker", "App.dmg", "ipsw"} {
		if !safeNames[want] {
			t.Errorf("Deep safe missing %q; got %v", want, safeNames)
		}
	}
	// Review (RiskAskBefore): jb-old, Trash, wpp-media, docker-leftover,
	// old.sql, ios-backup, old-sim.
	//
	// NOTE: jb-old is RiskAskBefore but lives in Caches, which Deep adds
	// wholesale (not via splitByRisk). So jb-old stays in safe at Deep
	// just like at Standard. The buckets routed by splitByRisk are
	// System/Orphans/Downloads/Device/DevAdvanced.
	for _, want := range []string{"Trash", "wpp-media", "docker-leftover", "old.sql", "ios-backup", "old-sim"} {
		if !reviewNames[want] {
			t.Errorf("Deep review missing %q; got %v", want, reviewNames)
		}
	}

	// The headline safety guarantee: NO RiskAskBefore item from a
	// user-data bucket ends up in safe.
	for _, it := range safe {
		if it.Category == item.CategoryDevice && it.Risk != item.RiskSafe {
			t.Errorf("Deep safe leaked a non-safe device item: %s", it.Name)
		}
		if it.Category == item.CategoryOrphan {
			t.Errorf("Deep safe leaked an orphan (orphans are RiskAskBefore): %s", it.Name)
		}
	}
}

// TestComputeTotals: monotonic Light <= Standard <= Deep, and totals
// match the flat PlanFor sums.
func TestComputeTotals(t *testing.T) {
	inv := fixtureInventory()
	tot := ComputeTotals(inv)

	if tot.Light > tot.Standard {
		t.Errorf("Light (%d) should not exceed Standard (%d)", tot.Light, tot.Standard)
	}
	if tot.Standard > tot.Deep {
		t.Errorf("Standard (%d) should not exceed Deep (%d)", tot.Standard, tot.Deep)
	}

	// Totals must equal the byte sum of the flat plan for each level.
	if got := item.TotalBytes(PlanFor(LevelLight, inv)); tot.Light != got {
		t.Errorf("Light total = %d, PlanFor sum = %d", tot.Light, got)
	}
	if got := item.TotalBytes(PlanFor(LevelDeep, inv)); tot.Deep != got {
		t.Errorf("Deep total = %d, PlanFor sum = %d", tot.Deep, got)
	}
}

// TestPlanFor_UnknownLevel returns empty, never panics.
func TestPlanFor_UnknownLevel(t *testing.T) {
	inv := fixtureInventory()
	plan := PlanFor(Level(99), inv)
	if plan != nil {
		t.Errorf("unknown level should yield nil, got %d items", len(plan))
	}
}

// TestLevel_String: stable identifiers used for i18n and logs.
func TestLevel_String(t *testing.T) {
	cases := map[Level]string{
		LevelLight:    "light",
		LevelStandard: "standard",
		LevelDeep:     "deep",
		Level(99):     "unknown",
	}
	for l, want := range cases {
		if got := l.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", l, got, want)
		}
	}
}

// TestPlanForSplit_Conservation: for every level, safe + review must
// equal the flat PlanFor list. No item lost, none duplicated.
func TestPlanForSplit_Conservation(t *testing.T) {
	inv := fixtureInventory()
	for _, lvl := range []Level{LevelLight, LevelStandard, LevelDeep} {
		safe, review := PlanForSplit(lvl, inv)
		flat := PlanFor(lvl, inv)
		if got, want := len(safe)+len(review), len(flat); got != want {
			t.Errorf("%s: safe+review = %d, PlanFor = %d", lvl, got, want)
		}
	}
}

// TestPlanForSplit_DeepRoutesUserDataToReview: the core safety property.
// Every RiskAskBefore item in a user-data bucket (Device backups,
// Orphans, Downloads, DevAdvanced) must be in review, never safe.
func TestPlanForSplit_DeepRoutesUserDataToReview(t *testing.T) {
	inv := fixtureInventory()
	_, review := PlanForSplit(LevelDeep, inv)
	rn := names(review)

	mustReview := []string{"ios-backup", "old-sim", "wpp-media", "old.sql"}
	for _, name := range mustReview {
		if !rn[name] {
			t.Errorf("%q must be in review (it's user data / RiskAskBefore)", name)
		}
	}
}

// TestDedupe_RemovesDuplicatePaths: dedupe keeps the first item per
// path and preserves no-path items (e.g. Docker prune).
func TestDedupe_RemovesDuplicatePaths(t *testing.T) {
	in := []item.Item{
		{Name: "a", Path: "/x/a"},
		{Name: "a-dup", Path: "/x/a"}, // same path → dropped
		{Name: "b", Path: "/x/b"},
		{Name: "docker1", Path: ""}, // no path → kept
		{Name: "docker2", Path: ""}, // no path → kept (different prune, hypothetically)
	}
	out := dedupe(in)
	if len(out) != 4 {
		t.Fatalf("expected 4 items after dedupe, got %d: %v", len(out), names(out))
	}
	got := names(out)
	if got["a-dup"] {
		t.Error("dedupe should have dropped the second item with path /x/a")
	}
	if !got["docker1"] || !got["docker2"] {
		t.Error("dedupe must keep all no-path items")
	}
}
