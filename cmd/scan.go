package cmd

import (
	"fmt"
	"sort"

	"github.com/sistematlan/mistah/internal/apps"
	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/inventory"
	"github.com/sistematlan/mistah/internal/item"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Escaneo completo: disco, apps y todo lo recuperable por categoría",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== mistah scan ===")
		fmt.Println()

		// 1. Disk usage.
		info, err := disk.Usage("/")
		if err != nil {
			return err
		}
		fmt.Printf("Disco\n  Total: %s  Usado: %s  Libre: %s (%.0f%%)\n\n",
			info.TotalStr, info.UsedStr, info.FreeStr, info.UsedPct)

		// 2. Apps with last-use heuristic.
		appList, err := apps.List()
		if err != nil {
			return err
		}
		unused := 0
		for _, a := range appList {
			if a.DaysSinceUse < 0 || a.DaysSinceUse > 90 {
				unused++
			}
		}
		fmt.Printf("Apps instaladas: %d  (sin uso >90d: %d)\n\n", len(appList), unused)

		// 3. Full inventory, grouped by source. This is the same scan the
		// wizard runs, so `scan` and the wizard never disagree on totals.
		inv, err := inventory.Scan()
		if err != nil {
			return err
		}

		groups := []struct {
			title string
			items []item.Item
		}{
			{"Sistema (papelera, cachés de apps, snapshots, Mail…)", inv.System},
			{"Dispositivos (backups de iPhone/iPad, firmware iOS)", inv.Device},
			{"Cachés de desarrollo", inv.Caches},
			{"Datos huérfanos", inv.Orphans},
			{"Candidatos en Downloads", inv.Downloads},
			{"Simuladores Xcode obsoletos", inv.DevAdvanced},
		}

		for _, g := range groups {
			printGroup(g.title, g.items)
		}

		// 4. Grand total.
		fmt.Printf("Recuperable estimado: %s\n", disk.FormatBytes(inv.TotalBytes()))
		fmt.Println("Ejecuta `mistah clean --dry-run` para ver qué se borraría.")
		return nil
	},
}

// printGroup renders one category section: a header with the group total
// and the top entries by size. Empty groups are skipped entirely so the
// output stays tight on machines that don't have, say, iOS backups.
func printGroup(title string, items []item.Item) {
	if len(items) == 0 {
		return
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Bytes > items[j].Bytes })
	fmt.Printf("%s: %s recuperables\n", title, disk.FormatBytes(item.TotalBytes(items)))
	for _, it := range topN(items, 8) {
		fmt.Printf("  %-34s %s\n", it.HumanName(), disk.FormatBytes(it.Bytes))
	}
	if len(items) > 8 {
		fmt.Printf("  ... y %d más\n", len(items)-8)
	}
	fmt.Println()
}

// topN returns the first n items, or all if the slice is shorter.
func topN(items []item.Item, n int) []item.Item {
	if len(items) <= n {
		return items
	}
	return items[:n]
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
