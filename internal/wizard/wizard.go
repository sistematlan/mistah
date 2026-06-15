// Package wizard runs the no-args interactive flow: scan, summarize,
// pick a cleaning level, confirm, execute, report.
//
// Audience contract:
//   - The wizard is for people who want mistah to "just work".
//   - It NEVER auto-deletes anything that holds user data — those go
//     through a per-item review phase even at the deepest level.
//   - It picks one of three preset levels and shows total impact upfront.
//   - It always offers a final confirm before deleting anything.
//
// Levels (rebalanced for a general audience in Phase 1):
//
//   Light    — Reproducible caches that ANY Mac accumulates: app caches
//              (Spotify, Slack, browsers…), QuickLook thumbnails, Mail
//              downloads, the Trash, .ipsw firmware, and the safe dev
//              caches (npm, brew, pip). All RiskSafe.
//
//   Standard — Light + the heavier reproducible stuff: Docker prune,
//              JetBrains old versions, Xcode build artifacts, Time
//              Machine local snapshots, logs and crash reports.
//
//   Deep     — Standard + data that MIGHT matter to the user: orphan
//              data, Downloads candidates, iOS device backups, stale
//              Xcode simulators. Everything RiskAskBefore in here goes
//              through the per-item review phase.
//
// The level filtering happens here, not in the cleaner, so the cleaner
// stays a pure executor and other entry points (clean cmd) keep their
// existing granular behaviour.
package wizard

import (
	"os"
	"path/filepath"

	"github.com/sistematlan/mistah/internal/appcache"
	"github.com/sistematlan/mistah/internal/caches"
	"github.com/sistematlan/mistah/internal/device"
	"github.com/sistematlan/mistah/internal/dev"
	"github.com/sistematlan/mistah/internal/downloads"
	"github.com/sistematlan/mistah/internal/item"
	"github.com/sistematlan/mistah/internal/orphans"
	"github.com/sistematlan/mistah/internal/system"
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
// One field per source so level-filtering stays a matter of picking
// buckets, not re-classifying items.
//
//	Caches      dev tool caches (npm, brew, Docker, JetBrains, Xcode…)
//	Orphans     leftover data from uninstalled apps
//	Downloads   smart candidates from ~/Downloads
//	System      OS + consumer-app reclaimable data (Trash, Mail, QuickLook,
//	            logs, crash reports, TM snapshots, app & browser caches)
//	Device      synced-device data (iOS backups, .ipsw firmware)
//	DevAdvanced heavier dev artefacts that need shelling out (stale Xcode
//	            simulators). Kept apart from Caches because they're
//	            RiskAskBefore and only relevant to devs.
type Inventory struct {
	Caches      []item.Item
	Orphans     []item.Item
	Downloads   []item.Item
	System      []item.Item
	Device      []item.Item
	DevAdvanced []item.Item
}

// TotalBytes sums every item the wizard knows about.
func (inv Inventory) TotalBytes() int64 {
	return item.TotalBytes(inv.Caches) +
		item.TotalBytes(inv.Orphans) +
		item.TotalBytes(inv.Downloads) +
		item.TotalBytes(inv.System) +
		item.TotalBytes(inv.Device) +
		item.TotalBytes(inv.DevAdvanced)
}

// Scan collects every detector output. It is a thin orchestrator that
// the wizard calls once per session.
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
	// them into one bucket so the wizard treats them uniformly.
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

// PlanFor returns the items that the given level would attempt to remove.
// The same Inventory snapshot is used so totals match what the user saw
// on screen. See PlanForSplit for the safe/review distinction.
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

// PlanForSplit returns the items for a level, split into two buckets:
//
//	safe   — deletable after the single up-front confirmation. Everything
//	         RiskSafe: reproducible caches, OS data, redundant firmware.
//	review — RiskAskBefore items that might hold user data. The wizard
//	         asks per-item before removing these, even at Deep level.
//
// The split is computed by Risk, not by source, so a future detector
// that emits a RiskAskBefore item in any bucket is automatically routed
// to review. This is the safety net that keeps a single "yes" from
// wiping out an iOS backup or a forgotten project folder.
//
// Level membership:
//
//	Light:    System (app/browser caches, Trash, QuickLook, Mail, .ipsw)
//	          + safe dev caches.
//	Standard: Light + all dev caches (Docker, JetBrains, Xcode artifacts)
//	          + Device firmware that's RiskSafe (already in System/.ipsw).
//	          In practice: Light + the rest of Caches.
//	Deep:     Standard + Orphans + Downloads + Device backups + DevAdvanced.
func PlanForSplit(level Level, inv Inventory) (safe []item.Item, review []item.Item) {
	switch level {
	case LevelLight:
		// System bucket is all RiskSafe by construction, but Trash is
		// RiskAskBefore — route by Risk so Trash lands in review.
		s, r := splitByRisk(inv.System)
		s = append(s, filterLight(inv.Caches)...)
		return s, r

	case LevelStandard:
		// Everything Light had, plus the full dev cache list (Docker,
		// JetBrains, Xcode). System minus its review items already
		// counted; add the non-light dev caches.
		s, r := splitByRisk(inv.System)
		s = append(s, inv.Caches...) // full cache list, supersedes filterLight
		return dedupe(s), r

	case LevelDeep:
		// Standard + the "might be user data" buckets. Each gets split
		// by Risk so RiskAskBefore items go to review.
		var allSafe, allReview []item.Item

		s, r := splitByRisk(inv.System)
		allSafe, allReview = append(allSafe, s...), append(allReview, r...)
		allSafe = append(allSafe, inv.Caches...)

		for _, bucket := range [][]item.Item{inv.Orphans, inv.Downloads, inv.Device, inv.DevAdvanced} {
			bs, br := splitByRisk(bucket)
			allSafe = append(allSafe, bs...)
			allReview = append(allReview, br...)
		}
		return dedupe(allSafe), allReview

	default:
		return nil, nil
	}
}

// splitByRisk partitions items into (safe, review) by their Risk field.
// RiskSafe → safe; everything else → review. The single source of truth
// for the "ask before deleting user data" rule.
func splitByRisk(items []item.Item) (safe, review []item.Item) {
	for _, it := range items {
		if it.Risk == item.RiskSafe {
			safe = append(safe, it)
		} else {
			review = append(review, it)
		}
	}
	return safe, review
}

// dedupe removes items with duplicate paths, keeping the first seen.
// Standard adds filterLight's output implicitly via the full Caches
// list; without dedupe an item could appear twice when buckets overlap.
// Items with an empty Path (e.g. Docker prune) are always kept — they
// have no path to deduplicate on and there's only ever one of each.
func dedupe(items []item.Item) []item.Item {
	seen := make(map[string]bool, len(items))
	out := make([]item.Item, 0, len(items))
	for _, it := range items {
		if it.Path == "" {
			out = append(out, it)
			continue
		}
		if seen[it.Path] {
			continue
		}
		seen[it.Path] = true
		out = append(out, it)
	}
	return out
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

// ComputeTotals returns the byte totals each level would clean. Derived
// from PlanFor so the menu numbers always match what gets deleted —
// no parallel accounting that could drift out of sync.
func ComputeTotals(inv Inventory) LevelTotals {
	return LevelTotals{
		Light:    item.TotalBytes(PlanFor(LevelLight, inv)),
		Standard: item.TotalBytes(PlanFor(LevelStandard, inv)),
		Deep:     item.TotalBytes(PlanFor(LevelDeep, inv)),
	}
}

// DetectDevPresence reports whether the machine looks like a developer's.
// Used by the menu to add a discreet "your dev caches are included too"
// banner — the copy stays general-audience by default and only nods to
// devs when there's evidence of a toolchain.
//
// Heuristic: any of a handful of canonical dev markers existing on disk.
// We check cheap stat()s, not the inventory, so the banner decision is
// independent of whether those caches happened to have reclaimable bytes
// this run (a dev with an already-clean npm cache is still a dev).
func DetectDevPresence() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	markers := []string{
		filepath.Join(home, ".npm"),
		filepath.Join(home, "Library", "Caches", "Homebrew"),
		filepath.Join(home, "Library", "Caches", "JetBrains"),
		filepath.Join(home, "Library", "Developer", "Xcode"),
		"/Applications/Docker.app",
		filepath.Join(home, ".cargo"),
		filepath.Join(home, "go", "pkg"),
	}
	for _, m := range markers {
		if _, err := os.Stat(m); err == nil {
			return true
		}
	}
	return false
}

