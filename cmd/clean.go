package cmd

import (
	"fmt"
	"sort"

	"github.com/sistematlan/mistah/internal/cleaner"
	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/inventory"
	"github.com/sistematlan/mistah/internal/item"
	"github.com/spf13/cobra"
)

var (
	cleanDryRun           bool
	cleanYes              bool
	cleanIncludeOrphans   bool
	cleanIncludeDownloads bool
	cleanIncludeSystem    bool
	cleanIncludeDevice    bool
	cleanAll              bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Limpieza interactiva con confirmación por ítem",
	Long: `Recorre los caches detectados y, opcionalmente, otros tipos de datos
recuperables. Por cada ítem pregunta s/N/v(er)/q(uit). Use --dry-run para
previsualizar.

Por defecto solo limpia cachés de desarrollo (comportamiento estable para
scripts). Las flags --include-* suman categorías; --all las activa todas.

Ejemplos:
  mistah clean --dry-run                # ver qué se borraría (solo caches dev)
  mistah clean                          # interactivo, solo caches dev
  mistah clean --include-system         # también papelera, cachés de apps, snapshots, logs
  mistah clean --include-device         # también backups de iPhone, firmware iOS
  mistah clean --include-orphans        # también WhatsApp media, Docker leftover
  mistah clean --include-downloads      # también DMGs ya instalados, ZIPs extraídos, dumps
  mistah clean --all                    # todas las categorías (equivale al wizard Deep)
  mistah clean --yes                    # sin confirmaciones (CI)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Build the candidate list from a single inventory scan.
		items, err := collectCleanCandidates()
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
		// Wire the global flag into the cleaner package so prompts use the
		// right phrasing without passing the bool through every layer.
		cleaner.SimpleMode = SimpleMode()
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

// collectCleanCandidates builds the clean list from one inventory scan,
// picking buckets per the --include-* / --all flags.
//
// Default (no flags): only dev caches. This is the stable contract that
// scripts may rely on — adding flags is additive, never subtractive, and
// the no-flag behaviour must never start deleting more than it used to.
//
// The cleaner enforces per-item confirmation on every RiskAskBefore item
// regardless of which bucket it came from, so including System/Device
// here doesn't mean those get auto-deleted: the user still confirms each
// backup, each Trash, etc. (unless --yes).
//
// Note: downloads come pre-filtered by inventory.Scan() via
// downloads.AsItems(), which drops the "large-other" subcategory — those
// need manual review via `mistah downloads`, not blanket cleaning.
func collectCleanCandidates() ([]item.Item, error) {
	inv, err := inventory.Scan()
	if err != nil {
		return nil, err
	}

	// Dev caches are always included — that's the baseline behaviour.
	all := append([]item.Item{}, inv.Caches...)

	if cleanAll || cleanIncludeSystem {
		all = append(all, inv.System...)
		// DevAdvanced (stale Xcode simulators) rides with system-level
		// cleanup: it's reclaimable dev cruft a thorough sweep wants.
		all = append(all, inv.DevAdvanced...)
	}
	if cleanAll || cleanIncludeDevice {
		all = append(all, inv.Device...)
	}
	if cleanAll || cleanIncludeOrphans {
		all = append(all, inv.Orphans...)
	}
	if cleanAll || cleanIncludeDownloads {
		all = append(all, inv.Downloads...)
	}

	return all, nil
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Reporta sin borrar")
	cleanCmd.Flags().BoolVar(&cleanYes, "yes", false, "Confirmar todo automáticamente")
	cleanCmd.Flags().BoolVar(&cleanIncludeSystem, "include-system", false,
		"Incluye datos de sistema (papelera, cachés de apps, snapshots de Time Machine, logs)")
	cleanCmd.Flags().BoolVar(&cleanIncludeDevice, "include-device", false,
		"Incluye datos de dispositivos (backups de iPhone/iPad, firmware iOS)")
	cleanCmd.Flags().BoolVar(&cleanIncludeOrphans, "include-orphans", false,
		"Incluye datos huérfanos (Docker leftover, WhatsApp media, etc.)")
	cleanCmd.Flags().BoolVar(&cleanIncludeDownloads, "include-downloads", false,
		"Incluye candidatos de ~/Downloads (instaladores ya usados, ZIPs extraídos, dumps viejos)")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false,
		"Incluye todas las categorías (equivale al nivel Profundo del wizard)")
	rootCmd.AddCommand(cleanCmd)
}
