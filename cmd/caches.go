package cmd

import (
	"fmt"

	"github.com/sistematlan/chipawa/internal/caches"
	"github.com/sistematlan/chipawa/internal/disk"
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

		var total int64
		fmt.Printf("%-30s %s\n", "Herramienta", "Tamaño")
		fmt.Printf("%-30s %s\n", "-----------", "------")
		for _, c := range list {
			fmt.Printf("%-30s %s\n", c.Name, disk.FormatBytes(c.Bytes))
			total += c.Bytes
		}
		fmt.Printf("\nTotal recuperable: %s\n", disk.FormatBytes(total))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(cachesCmd)
}
