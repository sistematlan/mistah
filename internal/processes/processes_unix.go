//go:build darwin || linux

// Both unixes ship a `ps` that understands `-eo pid,rss,pcpu,comm`, so one
// file covers them instead of two copies of the same parser.
package processes

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// psLine matches "  PID   RSS %CPU COMMAND" where COMMAND may hold spaces
// (node/python wrappers rewrite argv[0]), hence the greedy last group.
var psLine = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+([\d.]+)\s+(.*)$`)

// ponytail: shells out to `ps`. Reading /proc on Linux would skip the
// subprocess but needs two samples to compute CPU%; switch if `ps` ever
// goes missing (busybox, slim containers).
func listOS() ([]Proc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ps", "-eo", "pid,rss,pcpu,comm").Output()
	if err != nil {
		return nil, err
	}
	return parsePS(string(out)), nil
}

// parsePS turns `ps` stdout into Procs. Unparseable lines (header, junk)
// are skipped, not fatal: a truncated row shouldn't lose the whole list.
func parsePS(out string) []Proc {
	var ps []Proc
	for _, line := range strings.Split(out, "\n") {
		m := psLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pid, _ := strconv.Atoi(m[1])
		rss, _ := strconv.ParseInt(m[2], 10, 64)
		cpu, _ := strconv.ParseFloat(m[3], 64)
		ps = append(ps, Proc{
			PID:        pid,
			Name:       cleanName(m[4]),
			RSSBytes:   rss * 1024, // ps reports kilobytes
			CPUPercent: cpu,
		})
	}
	return ps
}

// cleanName shortens an executable path to its basename but leaves
// multi-word command lines intact — Base() would mangle those.
//
// The "starts with /" test matters on macOS: app bundles put spaces in
// their paths ("…/Comet Helper (Renderer).app/Contents/MacOS/Comet
// Helper (Renderer)"), so a space is not evidence of an argv[0] rewrite.
func cleanName(raw string) string {
	name := strings.TrimSpace(raw)
	if strings.HasPrefix(name, "/") {
		return filepath.Base(name)
	}
	return name
}

// terminate sends SIGTERM, polls until the process is gone, then SIGKILLs.
// Signal 0 is the liveness probe: ESRCH once the pid is dead.
//
// ponytail: a zombie (dead-but-unreaped child) still answers signal 0, so
// this would wait out the full grace and SIGKILL a corpse. Only reachable
// if mistah itself spawned the target — it never does. Reap-then-probe if
// that ever changes.
func terminate(p *os.Process, grace time.Duration) error {
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return err // ESRCH = ya no existe; EPERM = proceso de otro usuario
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		if p.Signal(syscall.Signal(0)) != nil {
			return nil
		}
	}
	return p.Kill()
}
