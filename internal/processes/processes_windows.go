//go:build windows

package processes

import (
	"encoding/csv"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// listOS shells out to tasklist. Columns are:
// "Image Name","PID","Session Name","Session#","Mem Usage"
// CPU% is not exposed by tasklist, so CPUPercent stays 0 on Windows.
func listOS() ([]Proc, error) {
	out, err := exec.Command("tasklist", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(string(out)))
	r.FieldsPerRecord = -1 // tasklist rows are uniform, but don't die if not
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	var ps []Proc
	for _, rec := range recs {
		if len(rec) < 5 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			continue
		}
		ps = append(ps, Proc{PID: pid, Name: rec[0], RSSBytes: parseKB(rec[4]) * 1024})
	}
	return ps, nil
}

// parseKB extracts the digits from a locale-shaped size like "1,234 K".
func parseKB(s string) int64 {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	n, _ := strconv.ParseInt(digits, 10, 64)
	return n
}

// ponytail: hard kill only — Windows has no SIGTERM, and os.Process.Kill
// is TerminateProcess. Graceful would be `taskkill /pid N` (WM_CLOSE) then
// Kill; add it if apps start losing unsaved state.
func terminate(p *os.Process, _ time.Duration) error {
	return p.Kill()
}
