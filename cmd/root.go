package cmd

import (
	"fmt"
	"os"

	"github.com/sistematlan/mistah/internal/i18n"
	"github.com/sistematlan/mistah/internal/wizard"
	"github.com/spf13/cobra"
)

// Global flags shared by every command.
var (
	// flagLang overrides locale autodetection. Empty = autodetect.
	flagLang string
	// flagAdvanced opts the user into technical phrasing and verbose paths.
	// Default behaviour is the simple (human-friendly) mode.
	flagAdvanced bool
)

// buildInfo holds the version/commit/date GoReleaser injects into
// main.go's package-level vars. Execute receives them once at process
// start and stashes them here so versionCmd (and --version) can read
// them without cmd importing package main directly — Go doesn't allow
// that import direction, since main already imports cmd.
var buildInfo struct {
	version, commit, date string
}

var rootCmd = &cobra.Command{
	Use:   "mistah",
	Short: "Limpia tu equipo como desarrollador",
	Long:  "mistah — analiza disco, apps, caches y proyectos. Libera espacio con confirmación.",
	// Version wires cobra's built-in --version flag to our injected build
	// info. Cobra prints "mistah version {{.Version}}" verbatim when this
	// is non-empty and --version is passed, so we format the full string
	// ourselves here rather than relying on cobra's default template.
	Version: "",
	// PersistentPreRunE applies the locale before any subcommand executes.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		applyLangFlag()
		return nil
	},
	// RunE only fires when no subcommand is given. We use it to launch the
	// wizard so a bare `mistah` becomes the friendly entry point. Power
	// users still have every subcommand available.
	RunE: func(cmd *cobra.Command, args []string) error {
		return wizard.Run(os.Stdin, cmd.OutOrStdout())
	},
	// Disable cobra's automatic suggestions for misspelled subcommands when
	// no args are passed; otherwise `mistah` would print "did you mean…?"
	// because we now have RunE defined.
	SilenceUsage: true,
}

// versionCmd prints build provenance: the tagged version, the exact
// commit it was built from, and the build timestamp. This exists
// because bug reports are close to useless without knowing which build
// produced them — "it's broken" with no version is unactionable, and
// until this command existed there was no way for a user to answer
// "which version are you running?" at all.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Muestra la versión, commit y fecha de compilación",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "mistah %s (commit %s, %s)\n",
			buildInfo.version, buildInfo.commit, buildInfo.date)
	},
}

// applyLangFlag wires --lang into the i18n package. Accepts "es" / "en" /
// raw locale strings ("es_MX.UTF-8"); anything else triggers autodetect.
func applyLangFlag() {
	switch flagLang {
	case "":
		// no override; let i18n auto-detect from $LANG/$LC_ALL.
	case "es":
		i18n.Set(i18n.LangES)
	case "en":
		i18n.Set(i18n.LangEN)
	default:
		// Treat as locale string. Reuse the package's detection logic by
		// temporarily setting LANG.
		_ = os.Setenv("LANG", flagLang)
		i18n.Set("") // force re-detect
	}
}

// SimpleMode reports whether output should use non-technical phrasing.
// Subcommands query this via cmd.SimpleMode() when rendering.
func SimpleMode() bool { return !flagAdvanced }

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// SetBuildInfo stashes the version/commit/date main.go received from
// GoReleaser's ldflags, and wires cobra's built-in --version flag to
// the same formatted string `mistah version` prints — so `--version`
// and the `version` subcommand never drift apart.
//
// Must run before rootCmd.Execute() so cobra's own --version handling
// (which reads Command.Version at parse time) sees the real value
// instead of the empty string Version was initialized with above.
func SetBuildInfo(version, commit, date string) {
	buildInfo.version = version
	buildInfo.commit = commit
	buildInfo.date = date
	rootCmd.Version = fmt.Sprintf("%s (commit %s, %s)", version, commit, date)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagLang, "lang", "",
		"Idioma (es | en). Por defecto autodetecta desde $LANG.")
	rootCmd.PersistentFlags().BoolVar(&flagAdvanced, "advanced", false,
		"Muestra detalles técnicos y rutas completas (modo desarrollador).")
	rootCmd.AddCommand(versionCmd)
}
