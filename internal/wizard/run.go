package wizard

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sistematlan/mistah/internal/cleaner"
	"github.com/sistematlan/mistah/internal/disk"
	"github.com/sistematlan/mistah/internal/i18n"
	"github.com/sistematlan/mistah/internal/item"
	"github.com/sistematlan/mistah/internal/spinner"
)

// Run executes the full wizard flow on the given streams.
//
// Sequence:
//  1. Print intro.
//  2. Spin while scanning.
//  3. Render menu with totals per level.
//  4. Read user choice (1-4).
//  5. Show plan summary and final confirmation.
//  6. Hand over to cleaner.Plan in Yes mode (no per-item prompts).
//  7. Print summary.
//
// Errors are returned to the caller (cobra prints them). User cancellation
// is NOT an error — we return nil after a friendly message.
func Run(in io.Reader, out io.Writer) error {
	printIntro(out)

	// Scan with spinner so the user sees something is happening.
	sp := spinner.New(i18n.T("wizard.scanning"))
	sp.Start(out)
	inv, scanErr := Scan()
	sp.Stop()
	if scanErr != nil {
		return scanErr
	}

	totals := ComputeTotals(inv)
	if totals.Deep == 0 {
		fmt.Fprintln(out, i18n.T("ui.nothing"))
		return nil
	}

	// Print the menu and read choice.
	scanner := bufio.NewScanner(in)
	level, ok := readLevelChoice(scanner, out, totals)
	if !ok {
		fmt.Fprintln(out, i18n.T("wizard.cancelled"))
		return nil
	}

	plan := PlanFor(level, inv)
	if len(plan) == 0 {
		fmt.Fprintln(out, i18n.T("ui.nothing"))
		return nil
	}

	// Final confirmation. Show level name, total, and a clear yes/no.
	if !confirmExecution(scanner, out, level, plan) {
		fmt.Fprintln(out, i18n.T("wizard.cancelled"))
		return nil
	}

	// Split into auto-deletable items (caches/orphans/safe downloads) and
	// review items (downloads classified RiskAskBefore: project folders,
	// extracted archives, DB dumps, old videos, old archives).
	//
	// The single up-front confirm authorises the auto-deletable bucket only.
	// For each review item we ask again, individually, so the user can keep
	// a forgotten project, an old recording, or a DB dump they actually need.
	safeItems, reviewItems := PlanForSplit(level, inv)

	allResults := make([]cleaner.Result, 0, len(plan))

	if len(safeItems) > 0 {
		cleaner.SimpleMode = true
		c := cleaner.New(safeItems, cleaner.Yes, nil, out)
		allResults = append(allResults, c.Run()...)
	}

	if len(reviewItems) > 0 {
		// Banner so the user understands the prompts that follow are NOT a
		// regression — they are the price of touching ~/Downloads safely.
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  "+i18n.T("wizard.review.header"))
		fmt.Fprintln(out, "  "+i18n.T("wizard.review.subtitle"))
		fmt.Fprintln(out)

		// Use simple phrasing in prompts (matches the rest of the wizard)
		// but keep the per-item confirmation flow of `mistah clean`.
		cleaner.SimpleMode = true
		prompter := &cleaner.TerminalPrompter{In: in, Out: out}
		c := cleaner.New(reviewItems, cleaner.Interactive, prompter, out)
		allResults = append(allResults, c.Run()...)
	}

	// Final summary (covers both phases).
	s := cleaner.Summarize(allResults)
	fmt.Fprintln(out)
	fmt.Fprintf(out, i18n.T("cleaner.summary"), s.Removed, s.Skipped, s.Failed)
	fmt.Fprintln(out)
	fmt.Fprintf(out, i18n.T("cleaner.freed"),
		disk.FormatBytes(s.BytesFreed), disk.FormatBytes(s.BytesPlanned))
	fmt.Fprintln(out)
	fmt.Fprintln(out)
	fmt.Fprintln(out, i18n.T("wizard.thanks"))
	return nil
}

// printIntro prints the small banner shown before the scan.
func printIntro(out io.Writer) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  mistah")
	fmt.Fprintln(out, "  ───────")
	fmt.Fprintln(out, "  "+i18n.T("wizard.tagline"))
	fmt.Fprintln(out)
}

// readLevelChoice prints the menu and reads a number from the shared scanner.
//
// Returns (level, true) on a valid pick (1, 2, or 3).
// Returns (_, false) when the user picks 4 (cancel) or input is invalid.
//
// The scanner is shared with the caller so that confirmExecution() can read
// the next line after this one without losing buffered bytes (same pattern
// as cleaner's TerminalPrompter).
func readLevelChoice(scanner *bufio.Scanner, out io.Writer, totals LevelTotals) (Level, bool) {
	fmt.Fprintln(out, i18n.T("wizard.menu.header"))
	fmt.Fprintln(out)
	// Discreet nod to developers when we detect a toolchain. The default
	// copy stays general-audience; this line only appears on dev machines.
	if DetectDevPresence() {
		fmt.Fprintln(out, "  "+i18n.T("wizard.menu.dev-detected"))
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "  1) %-12s %-10s — %s\n",
		i18n.T("wizard.level.light.name"),
		disk.FormatBytes(totals.Light),
		i18n.T("wizard.level.light.desc"))
	fmt.Fprintf(out, "  2) %-12s %-10s — %s\n",
		i18n.T("wizard.level.standard.name"),
		disk.FormatBytes(totals.Standard),
		i18n.T("wizard.level.standard.desc"))
	fmt.Fprintf(out, "  3) %-12s %-10s — %s\n",
		i18n.T("wizard.level.deep.name"),
		disk.FormatBytes(totals.Deep),
		i18n.T("wizard.level.deep.desc"))
	fmt.Fprintf(out, "  4) %s\n", i18n.T("wizard.menu.cancel"))
	fmt.Fprintln(out)
	fmt.Fprint(out, i18n.T("wizard.menu.prompt"))

	if !scanner.Scan() {
		return 0, false
	}
	choice, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil {
		return 0, false
	}
	switch choice {
	case 1:
		return LevelLight, true
	case 2:
		return LevelStandard, true
	case 3:
		return LevelDeep, true
	default:
		return 0, false
	}
}

// confirmExecution shows the plan summary and asks one final yes/no.
// Default is NO (empty input cancels) — same safe default the per-item
// cleaner uses, applied at the wizard scale.
func confirmExecution(scanner *bufio.Scanner, out io.Writer, level Level, plan []item.Item) bool {
	total := item.TotalBytes(plan)

	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s: %s  ·  %d %s  ·  %s\n",
		i18n.T("wizard.confirm.level"),
		i18n.T("wizard.level."+level.String()+".name"),
		len(plan),
		i18n.T("wizard.confirm.items"),
		disk.FormatBytes(total))
	fmt.Fprintln(out)
	fmt.Fprint(out, "  "+i18n.T("wizard.confirm.prompt"))

	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	switch answer {
	case "s", "si", "sí", "y", "yes":
		return true
	default:
		return false
	}
}
