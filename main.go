package main

import "github.com/sistematlan/mistah/cmd"

// version, commit, and date are populated by GoReleaser at build time via
// -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
//
// They must live here, in package main, because ldflags target a fully
// qualified package path + variable name — "-X main.version=..." only
// works if `version` is declared in `main`, not in cmd or any internal
// package. A `go run .` / plain `go build` (no ldflags) leaves these at
// their zero-value defaults below, which is exactly what local dev
// builds should show: "dev", not a version number that would lie about
// having no real build provenance.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetBuildInfo(version, commit, date)
	cmd.Execute()
}
