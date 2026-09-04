//go:build windows

// Package dev detects reclaimable artefacts of dev tooling that are
// more involved than a simple cache directory.
//
// On Windows there is currently nothing in this category: Xcode
// Simulators (the only detector in this package) are Apple-only and
// have no Windows equivalent. ScanXcodeSimulators is still exported
// so internal/inventory can call it unconditionally without a
// runtime.GOOS branch — it always returns nil here.
package dev

import "github.com/sistematlan/mistah/internal/item"

// ScanXcodeSimulators always returns nil on Windows. Kept as a no-op
// rather than removing the call site in internal/inventory so the
// orchestrator doesn't need platform-specific branches of its own.
func ScanXcodeSimulators(home string) []item.Item {
	return nil
}
