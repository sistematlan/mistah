// Package processes lists the OS processes that eat the most memory/CPU
// and can terminate them on request.
//
// Scope: read-mostly diagnostics plus one destructive verb (Kill).
// Listing shells out to the platform's native tool (`ps` / `tasklist`)
// because Go's stdlib has no process enumeration; killing goes through
// os.Process, so no new dependency either way.
package processes

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

// Proc is one running process as the OS reports it.
type Proc struct {
	PID  int
	Name string
	// RSSBytes is resident memory.
	RSSBytes int64
	// CPUPercent is the process' CPU share. Always 0 on Windows:
	// tasklist doesn't expose it.
	CPUPercent float64
}

// List returns every process visible to the current user.
func List() ([]Proc, error) { return listOS() }

// Top sorts in place (rss or cpu, descending) and trims to n. n <= 0 = all.
func Top(all []Proc, n int, byCPU bool) []Proc {
	sort.Slice(all, func(i, j int) bool {
		if byCPU {
			return all[i].CPUPercent > all[j].CPUPercent
		}
		return all[i].RSSBytes > all[j].RSSBytes
	})
	if n > 0 && len(all) > n {
		return all[:n]
	}
	return all
}

// Kill terminates pid: polite signal first, hard kill after grace if it
// is still around.
//
// Guards, never relaxed: pid 0/1 and mistah's own process are refused.
// Killing those takes the machine (or the tool mid-run) down with no undo.
func Kill(pid int, grace time.Duration) error {
	if pid <= 1 {
		return fmt.Errorf("PID %d protegido", pid)
	}
	if pid == os.Getpid() {
		return errors.New("no termino mi propio proceso")
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return terminate(p, grace)
}
