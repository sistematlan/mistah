//go:build linux

// Package dev detects reclaimable artefacts of dev tooling that are
// more involved than a simple cache directory.
//
// On Linux there is currently nothing in this category: Xcode
// Simulators (the only detector in this package) are Apple-only and
// have no Linux equivalent. ScanXcodeSimulators is still exported so
// internal/inventory can call it unconditionally without a
// runtime.GOOS branch — it always returns nil here, mirroring the
// Windows build's dev_windows.go.
package dev

import "github.com/sistematlan/mistah/internal/item"

// ScanXcodeSimulators always returns nil on Linux.
func ScanXcodeSimulators(home string) []item.Item {
	return nil
}
