package inventory

import (
	"testing"

	"github.com/sistematlan/mistah/internal/item"
)

// fixture builds an Inventory with a known item in every bucket so the
// aggregation helpers can be checked against hand-computed totals.
func fixture() Inventory {
	return Inventory{
		Caches:      []item.Item{{Name: "npm", Bytes: 10}},
		Orphans:     []item.Item{{Name: "wpp", Bytes: 20}},
		Downloads:   []item.Item{{Name: "old.sql", Bytes: 40}},
		System:      []item.Item{{Name: "Trash", Bytes: 80}, {Name: "Spotify", Bytes: 80}},
		Device:      []item.Item{{Name: "ios-backup", Bytes: 160}},
		DevAdvanced: []item.Item{{Name: "old-sim", Bytes: 320}},
	}
}

// TestTotalBytes_SumsEveryBucket: the total must include all six buckets.
// A regression where a new bucket is added to the struct but forgotten in
// TotalBytes would show up as a too-small total here.
func TestTotalBytes_SumsEveryBucket(t *testing.T) {
	inv := fixture()
	// 10 + 20 + 40 + (80+80) + 160 + 320 = 710
	if got, want := inv.TotalBytes(), int64(710); got != want {
		t.Errorf("TotalBytes() = %d, want %d", got, want)
	}
}

// TestAll_FlattensEveryBucket: All() must return one slice containing
// every item from every bucket, in bucket order.
func TestAll_FlattensEveryBucket(t *testing.T) {
	inv := fixture()
	all := inv.All()

	// 1 + 1 + 1 + 2 + 1 + 1 = 7 items total.
	if len(all) != 7 {
		t.Fatalf("All() returned %d items, want 7", len(all))
	}

	// Order must follow the bucket order documented on All().
	wantOrder := []string{"npm", "wpp", "old.sql", "Trash", "Spotify", "ios-backup", "old-sim"}
	for i, want := range wantOrder {
		if all[i].Name != want {
			t.Errorf("All()[%d] = %q, want %q", i, all[i].Name, want)
		}
	}

	// Conservation: All()'s byte sum equals TotalBytes().
	if item.TotalBytes(all) != inv.TotalBytes() {
		t.Errorf("All() byte sum (%d) != TotalBytes() (%d)",
			item.TotalBytes(all), inv.TotalBytes())
	}
}

// TestAll_EmptyInventory: an empty inventory yields an empty (non-nil)
// slice, never a panic.
func TestAll_EmptyInventory(t *testing.T) {
	var inv Inventory
	all := inv.All()
	if len(all) != 0 {
		t.Errorf("empty inventory should flatten to 0 items, got %d", len(all))
	}
	if inv.TotalBytes() != 0 {
		t.Errorf("empty inventory TotalBytes = %d, want 0", inv.TotalBytes())
	}
}

// TestScan_SmokeTest: Scan() runs against the real home directory. We
// can't assert specific items (they depend on the machine), but it must
// not error or panic, and the returned All()/TotalBytes() must be
// internally consistent.
//
// This is the integration check that all six detectors compose without
// blowing up — the orchestration contract.
func TestScan_SmokeTest(t *testing.T) {
	inv, err := Scan()
	if err != nil {
		t.Fatalf("Scan() errored: %v", err)
	}
	// Consistency: the flattened sum must equal TotalBytes regardless of
	// what's actually on this machine.
	if item.TotalBytes(inv.All()) != inv.TotalBytes() {
		t.Errorf("Scan() inventory inconsistent: All() sum != TotalBytes()")
	}
	// Bytes are never negative.
	if inv.TotalBytes() < 0 {
		t.Errorf("TotalBytes() negative: %d", inv.TotalBytes())
	}
}
