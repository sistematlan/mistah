package cmd

import (
	"fmt"
	"sort"

	"github.com/sistematlan/chipawa/internal/disk"
	"github.com/sistematlan/chipawa/internal/item"
	"github.com/sistematlan/chipawa/internal/orphans"
	"github.com/spf13/cobra"
)

var orphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "Datos hu\u00e9rfanos: apps desinstaladas, media inflada",
	Long: "Detecta directorios grandes que sobreviven cuando una app se desinstala\n" +
		"o que crecen sin l\u00edmite (p.ej. media de WhatsApp). Estos \u00edtems no son\n" +
		"caches: pueden contener datos del usuario y requieren confirmaci\u00f3n.",
	RunE: func(cmd *cobra.Command, args []string) error {
		list, err := orphans.Scan()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("Nada detectado. Disco limpio en este frente.")
			return nil
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Bytes > list[j].Bytes })

		fmt.Printf("%-28s %-10s %-10s %s\n", "\u00cdtem", "Tool", "Tamaño", "Detalle")
		fmt.Printf("%-28s %-10s %-10s %s\n", "----", "----", "------", "-------")
		for _, o := range list {
			fmt.Printf("%-28s %-10s %-10s %s\n",
				truncate(o.Name, 28), o.Tool, disk.FormatBytes(o.Bytes), o.Detail)
		}
		fmt.Printf("\nTotal: %s — requiere confirmaci\u00f3n antes de borrar\n",
			disk.FormatBytes(item.TotalBytes(list)))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(orphansCmd)
}
