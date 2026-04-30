package cmd

import (
	"fmt"

	"github.com/sistematlan/chipawa/internal/apps"
	"github.com/sistematlan/chipawa/internal/caches"
	"github.com/sistematlan/chipawa/internal/disk"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Escaneo completo: disco, apps y caches",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== chipawa scan ===\n")

		info, err := disk.Usage("/")
		if err != nil {
			return err
		}
		fmt.Printf("Disco\n  Total: %s  Usado: %s  Libre: %s (%.0f%%)\n\n",
			info.TotalStr, info.UsedStr, info.FreeStr, info.UsedPct)

		appList, err := apps.List()
		if err != nil {
			return err
		}
		fmt.Printf("Apps instaladas: %d\n", len(appList))
		unused := 0
		for _, a := range appList {
			if a.DaysSinceUse < 0 || a.DaysSinceUse > 90 {
				unused++
			}
		}
		fmt.Printf("  Sin uso (>90 días o nunca): %d\n\n", unused)

		cacheList, err := caches.Scan()
		if err != nil {
			return err
		}
		var total int64
		for _, c := range cacheList {
			total += c.Bytes
		}
		fmt.Printf("Caches de desarrollo: %s recuperables\n", disk.FormatBytes(total))
		for _, c := range cacheList {
			fmt.Printf("  %-30s %s\n", c.Name, disk.FormatBytes(c.Bytes))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
