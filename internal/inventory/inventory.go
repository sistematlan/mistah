// Package inventory is the single source of truth for "what mistah found
// on this Mac". It orchestrates every detector once and groups the result
// into buckets by source.
//
// Why a dedicated package? Both the wizard (interactive flow) and the cmd
// layer (`mistah clean`, `mistah scan`) need the same scan. Putting the
// orchestration in `wizard` would force `clean` to import `wizard`, which
// is backwards — the granular command shouldn't depend on the guided one.
// A neutral package that both consume keeps the dependency graph clean:
//
//	inventory  ←  wizard
//	inventory  ←  cmd
//
// The wizard re-exports Inventory and Scan for backwards compatibility,
// so existing callers and tests keep working unchanged.
package inventory

import (
	"os"

	"github.com/sistematlan/mistah/internal/appcache"
	"github.com/sistematlan/mistah/internal/caches"
	"github.com/sistematlan/mistah/internal/device"
	"github.com/sistematlan/mistah/internal/dev"
	"github.com/sistematlan/mistah/internal/downloads"
	"github.com/sistematlan/mistah/internal/item"
	"github.com/sistematlan/mistah/internal/orphans"
	"github.com/sistematlan/mistah/internal/system"
)

// Inventory is the snapshot of everything mistah detected. One field per
// source so consumers can apply different policies per bucket without
// re-classifying items.
//
//	Caches      dev tool caches (npm, brew, Docker, JetBrains, Xcode…)
//	Orphans     leftover data from uninstalled apps
//	Downloads   smart candidates from ~/Downloads
//	System      OS + consumer-app reclaimable data (Trash, Mail, QuickLook,
//	            logs, crash reports, TM snapshots, app & browser caches)
//	Device      synced-device data (iOS backups, .ipsw firmware)
//	DevAdvanced heavier dev artefacts that need shelling out (stale Xcode
//	            simulators)
type Inventory struct {
	Caches      []item.Item
	Orphans     []item.Item
	Downloads   []item.Item
	System      []item.Item
	Device      []item.Item
	DevAdvanced []item.Item
}

// TotalBytes sums every item the inventory knows about.
func (inv Inventory) TotalBytes() int64 {
	return item.TotalBytes(inv.Caches) +
		item.TotalBytes(inv.Orphans) +
		item.TotalBytes(inv.Downloads) +
		item.TotalBytes(inv.System) +
		item.TotalBytes(inv.Device) +
		item.TotalBytes(inv.DevAdvanced)
}

// All flattens every bucket into a single slice, preserving bucket order
// (caches, orphans, downloads, system, device, devAdvanced). Used by the
// cmd layer when it wants the whole catalogue without caring about the
// source split.
func (inv Inventory) All() []item.Item {
	out := make([]item.Item, 0, len(inv.Caches)+len(inv.Orphans)+
		len(inv.Downloads)+len(inv.System)+len(inv.Device)+len(inv.DevAdvanced))
	out = append(out, inv.Caches...)
	out = append(out, inv.Orphans...)
	out = append(out, inv.Downloads...)
	out = append(out, inv.System...)
	out = append(out, inv.Device...)
	out = append(out, inv.DevAdvanced...)
	return out
}

// Scan collects every detector output. It is a thin orchestrator called
// once per session.
//
// A failure in any single detector aborts the scan: a half-populated
// inventory would show the user misleading totals. The detectors
// themselves treat missing paths as "no items", so errors here are
// genuinely unexpected I/O failures worth surfacing.
func Scan() (Inventory, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Inventory{}, err
	}

	cs, err := caches.Scan()
	if err != nil {
		return Inventory{}, err
	}
	orph, err := orphans.Scan()
	if err != nil {
		return Inventory{}, err
	}
	ds, err := downloads.Scan()
	if err != nil {
		return Inventory{}, err
	}
	sys, err := system.Scan()
	if err != nil {
		return Inventory{}, err
	}
	dv, err := device.Scan()
	if err != nil {
		return Inventory{}, err
	}

	// System data spans two packages: system/ (OS data) and appcache/
	// (consumer app + browser caches). Both are CategorySystem; merge
	// them into one bucket so consumers treat them uniformly.
	appCaches, err := appcache.Scan()
	if err != nil {
		return Inventory{}, err
	}
	sys = append(sys, appCaches...)

	return Inventory{
		Caches:      cs,
		Orphans:     orph,
		Downloads:   downloads.AsItems(ds),
		System:      sys,
		Device:      dv,
		DevAdvanced: dev.ScanXcodeSimulators(home),
	}, nil
}
