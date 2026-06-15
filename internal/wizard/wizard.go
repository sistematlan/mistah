// Package wizard runs the no-args interactive flow: scan, summarize,
// pick a cleaning level, confirm, execute, report.
//
// Audience contract:
//   - The wizard is for people who want mistah to "just work".
//   - It NEVER asks per-item — that's what `mistah clean` is for.
//   - It picks one of three preset levels and shows total impact upfront.
//   - It always offers a final confirm before deleting anything.
//
// Levels:
//
//   Light    — only RiskSafe caches, NO Docker prune. Reversible feel.
//   Standard — Light + Docker prune + JetBrains old versions + Xcode artifacts.
//   Deep     — Standard + orphans + downloads (smart subset).
//
// The level filtering happens here, not in the cleaner, so the cleaner stays
// a pure executor and other entry points (clean cmd) keep their existing
// granular behaviour.
package wizard

import (
	"github.com/sistematlan/mistah/internal/caches"
	"github.com/sistematlan/mistah/internal/downloads"
	"github.com/sistematlan/mistah/internal/item"
	"github.com/sistematlan/mistah/internal/orphans"
)

// Level enumerates the wizard presets.
type Level int

const (
	LevelLight Level = iota
	LevelStandard
	LevelDeep
)

// String returns a stable identifier used for i18n keys and logs.
func (l Level) String() string {
	switch l {
	case LevelLight:
		return "light"
	case LevelStandard:
		return "standard"
	case LevelDeep:
		return "deep"
	default:
		return "unknown"
	}
}

// Inventory is the snapshot the wizard scans before showing the menu.
// Splitting caches/orphans/downloads keeps level-filtering trivial later.
type Inventory struct {
	Caches    []item.Item
	Orphans   []item.Item
	Downloads []item.Item
}

// TotalBytes sums every item the wizard knows about.
func (inv Inventory) TotalBytes() int64 {
	return item.TotalBytes(inv.Caches) +
		item.TotalBytes(inv.Orphans) +
		item.TotalBytes(inv.Downloads)
}

// Scan collects every detector output. It is a thin orchestrator that
// the wizard calls once per session.
func Scan() (Inventory, error) {
	cs, err := caches.Scan()
	if err != nil {
		return Inventory{}, err
	}
	os, err := orphans.Scan()
	if err != nil {
		return Inventory{}, err
	}
	ds, err := downloads.Scan()
	if err != nil {
		return Inventory{}, err
	}
	return Inventory{
		Caches:    cs,
		Orphans:   os,
		Downloads: downloads.AsItems(ds),
	}, nil
}

// PlanFor returns the items that the given level would attempt to remove.
// The same Inventory snapshot is used so totals match what the user saw on screen.
//
// Level rules:
//
//   Light: Caches WITHOUT the Docker entry (no Path) and WITHOUT JetBrains
//          old versions (those touch user settings, RiskAskBefore).
//          Effectively: RiskSafe + Path != "".
//
//   Standard: Light + Docker + JetBrains old versions.
//             Anything with item.CategoryCache, regardless of Risk.
//
//   Deep: Standard + orphans + downloads (smart, large-other excluded already).
func PlanFor(level Level, inv Inventory) []item.Item {
	safe, review := PlanForSplit(level, inv)
	if len(review) == 0 {
		return safe
	}
	out := make([]item.Item, 0, len(safe)+len(review))
	out = append(out, safe...)
	out = append(out, review...)
	return out
}

// PlanForSplit returns the same items as PlanFor, but split in two buckets:
//
//   safe   — items the wizard may delete after the single up-front confirm.
//            Caches (always), orphans (always), and downloads classified as
//            RiskSafe (e.g. installers whose app is already installed).
//
//   review — downloads items classified as RiskAskBefore (project folders
//            with node_modules, archives with sibling extracted folder,
//            DB dumps, old videos, old archives). These contain or might
//            contain user data, so the wizard asks per-item before removing
//            them, even at Deep level.
//
// The split is empty for Light/Standard — those levels never touch downloads.
//
// Rationale: a single "yes" wiping out a forgotten project folder, a video
// the user wanted to keep, or an old DB dump that turned out to be the only
// backup is a support nightmare and a trust killer. Caches and orphans are
// reproducible; user files in ~/Downloads are not.
func PlanForSplit(level Level, inv Inventory) (safe []item.Item, review []item.Item) {
	switch level {
	case LevelLight:
		return filterLight(inv.Caches), nil
	case LevelStandard:
		return inv.Caches, nil
	case LevelDeep:
		safeDownloads, reviewDownloads := splitDownloadsByRisk(inv.Downloads)
		safe = make([]item.Item, 0, len(inv.Caches)+len(inv.Orphans)+len(safeDownloads))
		safe = append(safe, inv.Caches...)
		safe = append(safe, inv.Orphans...)
		safe = append(safe, safeDownloads...)
		return safe, reviewDownloads
	default:
		return nil, nil
	}
}

// splitDownloadsByRisk partitions downloads into auto-deletable and
// must-confirm buckets based on the Risk field set by the downloads scanner.
//
// Anything not RiskSafe falls into review. We deliberately do NOT include
// RiskDangerous handling here because the downloads scanner never emits
// that level — if it ever does, treating it as "review" is the safer
// fallback (the cleaner's per-item prompt will then enforce the type-name
// confirmation on its own).
func splitDownloadsByRisk(items []item.Item) (safe, review []item.Item) {
	for _, it := range items {
		if it.Risk == item.RiskSafe {
			safe = append(safe, it)
		} else {
			review = append(review, it)
		}
	}
	return safe, review
}

// filterLight keeps only items that are RiskSafe AND have a path.
//
// Why exclude no-path items? The only no-path cache is Docker reclaimable;
// it triggers `docker system prune` which can be surprising to non-devs.
// Light should feel boringly safe — that's its job.
//
// Why also drop JetBrains old versions? They're RiskAskBefore (settings
// folder), so even though they're "caches" they don't fit the Light promise.
func filterLight(items []item.Item) []item.Item {
	out := make([]item.Item, 0, len(items))
	for _, it := range items {
		if it.Risk != item.RiskSafe {
			continue
		}
		if it.Path == "" {
			continue
		}
		out = append(out, it)
	}
	return out
}

// LevelTotals is a convenience for the menu: total bytes per level so the
// user sees "Light (2.3 GB) / Standard (5.6 GB) / Deep (7.1 GB)" upfront.
type LevelTotals struct {
	Light    int64
	Standard int64
	Deep     int64
}

// ComputeTotals returns the byte totals each level would clean.
func ComputeTotals(inv Inventory) LevelTotals {
	return LevelTotals{
		Light:    item.TotalBytes(filterLight(inv.Caches)),
		Standard: item.TotalBytes(inv.Caches),
		Deep:     item.TotalBytes(inv.Caches) + item.TotalBytes(inv.Orphans) + item.TotalBytes(inv.Downloads),
	}
}
