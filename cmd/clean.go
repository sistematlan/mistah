package cmd

import (
	"fmt"
	"sort"

	"github.com/sistematlan/chipawa/internal/caches"
	"github.com/sistematlan/chipawa/internal/cleaner"
	"github.com/sistematlan/chipawa/internal/disk"
	"github.com/sistematlan/chipawa/internal/item"
	"github.com/sistematlan/chipawa/internal/orphans"
	"github.com/spf13/cobra"
)

var (
	cleanDryRun         bool
	cleanYes            bool
	cleanIncludeOrphans bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Limpieza interactiva con confirmación por ítem",
	Long: `Recorre los caches detectados y, opcionalmente, los datos huérfanos.
Por cada ítem pregunta s/N/v(er)/q(uit). Use --dry-run para previsualizar.

Ejemplos:
  chipawa clean --dry-run                # ver qué se borraría
  chipawa clean                          # interactivo, solo caches
  chipawa clean --include-orphans        # también WhatsApp media, Docker leftover
  chipawa clean --yes                    # sin confirmaciones (CI)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Build the candidate list.
		items, err := collectCleanCandidates(cleanIncludeOrphans)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Println("Nada que limpiar. Disco en orden.")
			return nil
		}
		// Largest first so the impact is visible up top.
		sort.Slice(items, func(i, j int) bool { return items[i].Bytes > items[j].Bytes })

		// 2. Pick the execution mode.
		mode := cleaner.Interactive
		switch {
		case cleanDryRun:
			mode = cleaner.DryRun
		case cleanYes:
			mode = cleaner.Yes
		}

		// 3. Print a header so the user sees the plan total.
		fmt.Printf("Plan: %d ítems, %s recuperables\n",
			len(items), disk.FormatBytes(item.TotalBytes(items)))
		if mode == cleaner.DryRun {
			fmt.Println("Modo: dry-run (no se borrará nada)")
		}

		// 4. Run.
		var prompter cleaner.Prompter
		if mode == cleaner.Interactive {
			prompter = cleaner.NewTerminalPrompter()
		}
		plan := cleaner.New(items, mode, prompter, cmd.OutOrStdout())
		results := plan.Run()

		// 5. Summary.
		s := cleaner.Summarize(results)
		fmt.Println()
		fmt.Printf("Resumen: %d eliminados, %d omitidos, %d fallidos\n",
			s.Removed, s.Skipped, s.Failed)
		fmt.Printf("Espacio liberado: %s (de %s planificados)\n",
			disk.FormatBytes(s.BytesFreed), disk.FormatBytes(s.BytesPlanned))
		return nil
	},
}

// collectCleanCandidates merges caches and (optionally) orphans into one list.
// Items with no path AND no specialized remover are filtered out by the
// resolver later, so we don't need to skip them here.
func collectCleanCandidates(includeOrphans bool) ([]item.Item, error) {
	cs, err := caches.Scan()
	if err != nil {
		return nil, err
	}
	all := cs

	if includeOrphans {
		os, err := orphans.Scan()
		if err != nil {
			return nil, err
		}
		all = append(all, os...)
	}
	return all, nil
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Reporta sin borrar")
	cleanCmd.Flags().BoolVar(&cleanYes, "yes", false, "Confirmar todo autom\u00e1ticamente")
	cleanCmd.Flags().BoolVar(&cleanIncludeOrphans, "include-orphans", false,
		"Incluye datos huérfanos (Docker leftover, WhatsApp media, etc.)")
	rootCmd.AddCommand(cleanCmd)
}
