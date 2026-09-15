package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/i18n"
	"github.com/sistematlan/mistah/internal/processes"
	"github.com/spf13/cobra"
)

var (
	procLimit int
	procSort  string
	procAll   bool
	procKill  string
	procYes   bool
)

var processesCmd = &cobra.Command{
	Use:   "processes",
	Short: "Procesos que más RAM/CPU consumen; --kill los termina",
	Long: `Lista los procesos del equipo ordenados por memoria (o CPU) para que
puedas ver qué se está comiendo el equipo, y terminarlos con --kill.

No libera disco: libera RAM y CPU de procesos colgados o dev servers
olvidados. Matar un proceso no guarda su trabajo, por eso pide
confirmación salvo con --yes.

Ejemplos:
  mistah processes                     # top 15 por RAM
  mistah processes --sort cpu --limit 5
  mistah processes --all               # sin recorte
  mistah processes --kill 4821         # pregunta antes
  mistah processes --kill 4821,4902 --yes`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if procKill != "" {
			return runKill(procKill)
		}
		all, err := processes.List()
		if err != nil {
			return err
		}
		limit := procLimit
		if procAll {
			limit = 0
		}
		top := processes.Top(all, limit, procSort == "cpu")

		fmt.Printf("%-7s %-36s %-10s %s\n",
			cap1(i18n.T("processes.pid")), cap1(i18n.T("processes.name")),
			cap1(i18n.T("processes.ram")), cap1(i18n.T("processes.cpu")))
		fmt.Printf("%-7s %-36s %-10s %s\n", "---", "----", "----", "----")
		for _, p := range top {
			fmt.Printf("%-7d %-36s %-10s %.1f\n",
				p.PID, truncate(p.Name, 36), disk.FormatBytes(p.RSSBytes), p.CPUPercent)
		}
		return nil
	},
}

// runKill parses the pid list, confirms once for the whole batch, then
// terminates each. One failure doesn't abort the rest: the user asked for
// all of them and the usual cause (EPERM, already gone) is per-process.
func runKill(spec string) error {
	pids, err := parsePIDs(spec)
	if err != nil {
		return err
	}
	if !procYes && !confirmKill(pids) {
		fmt.Println(i18n.T("processes.kill.cancelled"))
		return nil
	}
	failed := 0
	for _, pid := range pids {
		if err := processes.Kill(pid, 3*time.Second); err != nil {
			failed++
			fmt.Printf("PID %d: %v\n", pid, err)
			continue
		}
		fmt.Println(i18n.T("processes.kill.done", pid))
	}
	if failed == len(pids) {
		return fmt.Errorf("no se pudo terminar ningún proceso (%d intentos)", failed)
	}
	return nil
}

// parsePIDs accepts "4821", "4821,4902" or "4821 4902". Rejects garbage
// instead of silently dropping it — a mistyped pid that gets ignored reads
// like success to the user.
func parsePIDs(spec string) ([]int, error) {
	fields := strings.FieldsFunc(spec, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	if len(fields) == 0 {
		return nil, fmt.Errorf("--kill necesita al menos un PID")
	}
	pids := make([]int, 0, len(fields))
	for _, f := range fields {
		pid, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("PID inválido %q", f)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

// confirmKill asks one y/N question for the whole batch. A non-interactive
// stdin (piped, closed) yields no line and therefore "no" — the safe side.
func confirmKill(pids []int) bool {
	names := make([]string, 0, len(pids))
	for _, pid := range pids {
		names = append(names, strconv.Itoa(pid))
	}
	fmt.Print(i18n.T("processes.kill.confirm", strings.Join(names, ", ")))

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false // stdin cerrado o sin línea: por defecto "no"
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "s", "si", "sí", "y", "yes":
		return true
	}
	return false
}

func init() {
	processesCmd.Flags().IntVar(&procLimit, "limit", 15, "Cuántos procesos mostrar")
	processesCmd.Flags().StringVar(&procSort, "sort", "ram", "Orden: ram | cpu")
	processesCmd.Flags().BoolVar(&procAll, "all", false, "Lista todos los procesos, sin recorte")
	processesCmd.Flags().StringVar(&procKill, "kill", "", "PIDs a terminar (ej. 4821,4902)")
	processesCmd.Flags().BoolVar(&procYes, "yes", false, "Termina sin preguntar (con --kill)")
	rootCmd.AddCommand(processesCmd)
}
