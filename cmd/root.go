package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chipawa",
	Short: "Limpia tu Mac como desarrollador",
	Long:  "chipawa — analiza disco, apps, caches y proyectos. Libera espacio con confirmación.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
