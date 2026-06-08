package cmd

import (
	"fmt"
	"sort"

	"github.com/sistematlan/chipawa/internal/caches"
	"github.com/sistematlan/chipawa/internal/disk"
	"github.com/sistematlan/chipawa/internal/item"
	"github.com/spf13/cobra"
)

var cachesCmd = &cobra.Command{
	Use:   "caches",
	Short: "Detecta caches de herramientas de desarrollo",
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := caches.Scan()
		if err != nil {
			return err
		}
		// Largest first so the user sees impact at the top.
		sort.Slice(list, func(i, j int) bool { return list[i].Bytes > list[j].Bytes })

		fmt.Printf("%-28s %-10s %-10s %s\n", "Cache", "Tool", "Tamaño", "Detalle")
		fmt.Printf("%-28s %-10s %-10s %s\n", "-----", "----", "------", "-------")
		for _, c := range list {
			fmt.Printf("%-28s %-10s %-10s %s\n",
				truncate(c.Name, 28), c.Tool, disk.FormatBytes(c.Bytes), c.Detail)
		}
		fmt.Printf("\nTotal recuperable: %s\n", disk.FormatBytes(item.TotalBytes(list)))
		return nil
	},
}

// truncate cuts s to n runes appending an ellipsis if it doesn't fit.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func init() {
	rootCmd.AddCommand(cachesCmd)
}
