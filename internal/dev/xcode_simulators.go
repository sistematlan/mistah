// Package dev detects reclaimable artefacts of dev tooling that are
// more involved than a simple cache directory. The simple ones live
// in internal/caches; this package is for detectors that need to
// shell out, parse JSON, or otherwise be smarter than path + size.
//
// Today it covers:
//   - Xcode iOS Simulators whose runtimes are no longer available.
//
// Items are CategoryCache (they're dev artefacts, not user data) but
// RiskAskBefore: a stale simulator might still hold app data the
// developer wants. Auto-deletion would be wrong even when the runtime
// is gone.
package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/item"
)

// xcrunCommand is the test seam for xcrun invocations. Production
// shells out to the real binary; tests inject canned output.
//
// We accept a context so production calls can time out — `xcrun simctl`
// stalls indefinitely when the simulator subsystem is in a bad state,
// and we'd rather report no items than hang the wizard.
var xcrunCommand = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "xcrun", args...).CombinedOutput()
}

// ScanXcodeSimulators returns one Item per simulator device whose
// runtime is unavailable. Each device gets its own row so the user can
// keep a specific old simulator if needed.
//
// home is used to compute the on-disk size of each device under
// ~/Library/Developer/CoreSimulator/Devices/<UDID>/. Without it the
// detector has no size to show, which makes the wizard less useful.
//
// Failures (xcrun missing, JSON parse error) yield nil rather than
// surfacing — Xcode is optional on a Mac, and a no-op detector is the
// right behaviour when it's not installed.
func ScanXcodeSimulators(home string) []item.Item {
	out, err := runSimctlList()
	if err != nil {
		return nil
	}
	devices := parseUnavailableDevices(out)
	if len(devices) == 0 {
		return nil
	}

	var items []item.Item
	for _, d := range devices {
		dir := filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices", d.UDID)
		bytes, _ := disk.DirSize(dir)
		// Sub-100KB devices are essentially empty placeholders. Skip
		// them so the wizard doesn't show 8 KB rows.
		if bytes < 100*1024 {
			continue
		}

		runtime := prettyRuntime(d.Runtime)
		items = append(items, item.Item{
			Name:       fmt.Sprintf("%s · %s", d.Name, runtime),
			Tool:       "xcode-simulator",
			Path:       dir,
			Bytes:      bytes,
			Category:   item.CategoryCache,
			Risk:       item.RiskAskBefore,
			Detail:     fmt.Sprintf("simulador %s en runtime %s (no disponible)", d.Name, runtime),
			DetailKey:  "dev.xcode-simulator.detail",
			DetailArgs: []any{d.Name, runtime},
		})
	}
	return items
}

// runSimctlList invokes `xcrun simctl list devices --json` and returns
// raw stdout. A 15-second timeout covers slow startups without leaving
// a stuck process behind.
func runSimctlList() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return xcrunCommand(ctx, "simctl", "list", "devices", "--json")
}

// simulatorDevice is a flat shape combining device data with its
// runtime key, so the detector doesn't have to drag map iteration
// into every consumer.
type simulatorDevice struct {
	UDID    string
	Name    string
	Runtime string // raw runtime key, e.g. "com.apple.CoreSimulator.SimRuntime.iOS-15-5"
}

// simctlListOutput is the JSON shape Apple produces. Only fields we
// actually use are typed; `xcrun` adds new keys over time and we don't
// want to break on them.
type simctlListOutput struct {
	Devices map[string][]struct {
		UDID        string `json:"udid"`
		Name        string `json:"name"`
		IsAvailable bool   `json:"isAvailable"`
	} `json:"devices"`
}

// parseUnavailableDevices extracts every device whose runtime is gone.
// Two signals indicate "unavailable":
//
//   - The runtime KEY contains "unavailable". Apple appends
//     " -- unavailable, runtime profile not found" when the .simruntime
//     bundle is missing. Every device under that key is dead.
//   - The device's isAvailable field is false. Apple sometimes flips
//     this without renaming the key (e.g. when iOS dropped support for
//     a chip family). We catch those too.
//
// A device that's available under an available runtime is skipped —
// it's a legitimate part of the user's Xcode setup.
func parseUnavailableDevices(rawJSON []byte) []simulatorDevice {
	var out simctlListOutput
	if err := json.Unmarshal(rawJSON, &out); err != nil {
		return nil
	}

	var devs []simulatorDevice
	for runtimeKey, entries := range out.Devices {
		runtimeUnavailable := strings.Contains(strings.ToLower(runtimeKey), "unavailable")
		for _, e := range entries {
			if !runtimeUnavailable && e.IsAvailable {
				continue
			}
			devs = append(devs, simulatorDevice{
				UDID:    e.UDID,
				Name:    e.Name,
				Runtime: runtimeKey,
			})
		}
	}
	return devs
}

// prettyRuntime turns a CoreSimulator runtime key into something a
// human can read in the wizard. We strip the constant prefix and the
// trailing " -- unavailable, …" suffix:
//
//	in:  com.apple.CoreSimulator.SimRuntime.iOS-15-5 -- unavailable, runtime profile not found
//	out: iOS-15-5
//
// Unrecognised inputs fall through unchanged, so a future Apple format
// change is visible in the UI rather than silently truncated.
func prettyRuntime(key string) string {
	const prefix = "com.apple.CoreSimulator.SimRuntime."
	short := strings.TrimPrefix(key, prefix)
	if i := strings.Index(short, " -- "); i >= 0 {
		short = short[:i]
	}
	return strings.TrimSpace(short)
}
