package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tdStatsPath  string
	tdStatsJSON  bool
	tdStatsMin   bool
	tdStatsWrite bool
	tdStatsToday string
)

// TDStatsResult holds the aggregated tech debt statistics
type TDStatsResult struct {
	Severity map[string]TDStatsSeverity `json:"severity"`
	Summary  TDStatsTotals              `json:"summary"`
	Markdown string                     `json:"markdown"`
	Written  bool                       `json:"written"`
}

// TDStatsTotals holds aggregate counts across all severities.
//
// Fields are explicit rather than a map keyed by checkboxState.Key: this shape
// is the MCP surface and existing consumers read it, so a new state is added
// here as an additional field (safe) rather than by reshaping (not).
type TDStatsTotals struct {
	Open           int `json:"open"`
	Deferred       int `json:"deferred"`
	Resolved       int `json:"resolved"`
	Unreproducible int `json:"unreproducible"`
	Total          int `json:"total"`
}

// TDStatsSeverity holds counts for a single severity level
type TDStatsSeverity struct {
	Open           int `json:"open"`
	Deferred       int `json:"deferred"`
	Resolved       int `json:"resolved"`
	Unreproducible int `json:"unreproducible"`
}

// counts returns the severity's tallies keyed by checkboxState.Key, so
// rendering can walk the state table instead of naming fields.
func (s TDStatsSeverity) counts() map[string]int {
	return map[string]int{
		"open":           s.Open,
		"deferred":       s.Deferred,
		"resolved":       s.Resolved,
		"unreproducible": s.Unreproducible,
	}
}

// addState increments the count for a state key.
func (s *TDStatsSeverity) addState(key string) {
	switch key {
	case "open":
		s.Open++
	case "deferred":
		s.Deferred++
	case "resolved":
		s.Resolved++
	case "unreproducible":
		s.Unreproducible++
	}
}

func newTDStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-stats",
		Short: "Generate tech debt statistics from a README table",
		Long: `Reads a tech debt README with a markdown table containing checkbox and severity
columns, and produces an aggregated stats summary.

Checkbox states:
  [ ]  = Open
  [/]  = Deferred
  [x]  = Resolved
  [-]  = Unreproducible (closed without a fix)

Total counts every state. Unreproducible is reported separately from Resolved:
nothing was changed, so folding them together overstates what the work closed.

Its column appears only in files that actually use [-], so a README on the
other three states renders byte-identically.

Columns are detected per row: a row carrying a checkbox cell is data, and the
severity column is taken from a "Severity" header when there is one, or
auto-detected from the row when there is not. Headerless tables are read; a
table using a non-standard severity needs its header row to name the column.

Output includes both structured severity counts and a pre-rendered markdown
table in the "markdown" field.

With --write, the "## Stats" section and "**Last Modified:**" line are
rewritten in place deterministically. No data rows are added or removed —
use td-clean for that. This is a no-op read when --write is omitted.`,
		RunE: runTDStats,
	}

	cmd.Flags().StringVar(&tdStatsPath, "path", "", "Path to tech debt README (required)")
	cmd.Flags().BoolVar(&tdStatsJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tdStatsMin, "min", false, "Minimal output format")
	cmd.Flags().BoolVar(&tdStatsWrite, "write", false, "Rewrite the ## Stats section and Last Modified line in place (no rows added/removed)")
	cmd.Flags().StringVar(&tdStatsToday, "today", "", "Date for the Last Modified line (YYYY-MM-DD); defaults to today (only used with --write)")
	cmd.MarkFlagRequired("path")

	return cmd
}

// severityOrder defines the display order for severity levels
var severityOrder = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"}

func runTDStats(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(tdStatsPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	result, err := parseTDStats(string(content))
	if err != nil {
		return err
	}

	result.Markdown = formatTDStatsMarkdown(result)

	if tdStatsWrite {
		today := tdStatsToday
		if today == "" {
			today = time.Now().Format("2006-01-02")
		}

		// Reuses td-clean's in-place rewrite helpers, but skips its row-stripping
		// passes entirely: td-stats --write only refreshes the ## Stats section and
		// Last Modified line to match the file's current data rows, it never adds
		// or removes one. Idempotent — if nothing changed, the file is left
		// byte-for-byte unchanged.
		lines := strings.Split(string(content), "\n")
		lines = replaceStatsSection(lines, result.Markdown)
		lines = updateLastModified(lines, today, result.Summary)
		updated := strings.Join(lines, "\n")

		if updated != string(content) {
			if err := os.WriteFile(tdStatsPath, []byte(updated), 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			result.Written = true
		}
	}

	formatter := output.New(tdStatsJSON, tdStatsMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(*TDStatsResult)
		fmt.Fprint(w, r.Markdown)
		if tdStatsWrite {
			if r.Written {
				fmt.Fprintf(w, "\nWritten to %s\n", tdStatsPath)
			} else {
				fmt.Fprintf(w, "\nNo change — %s already up to date\n", tdStatsPath)
			}
		}
	})
}

func parseTDStats(content string) (*TDStatsResult, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	stats := make(map[string]*TDStatsSeverity)

	checkboxCol := -1
	severityCol := -1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			// Leaving a table invalidates the detected column positions: the
			// next table may lay its columns out differently. The rows
			// themselves are re-detected below, so nothing is lost by
			// forgetting.
			checkboxCol = -1
			severityCol = -1
			continue
		}

		cells := splitTableRow(line)

		// Skip separator rows (|---|---|...)
		if isSeparatorRow(cells) {
			continue
		}

		// A row carrying a checkbox cell is data, always. Deciding "header" by
		// position instead — first "|" line after a break — silently ate the
		// first row of every table that has no header, and then, with no
		// severity column detected, the rows after it as well.
		if !rowHasCheckbox(cells) {
			// A header row: take the severity column from it by name. Anything
			// else without a checkbox (the file's own ## Stats block, a prose
			// table) contributes no counts either way.
			for i, cell := range cells {
				if strings.EqualFold(strings.TrimSpace(cell), "severity") {
					severityCol = i
				}
			}
			continue
		}

		// Auto-detect the checkbox column from the data row itself.
		if checkboxCol == -1 {
			for i, cell := range cells {
				if isCheckboxCell(cell) {
					checkboxCol = i
					break
				}
			}
		}

		// Auto-detect the severity column the same way, for tables that carry
		// no header to name it. Symmetric with the checkbox detection above,
		// and what makes a headerless table parse at all.
		if severityCol == -1 {
			for i, cell := range cells {
				if isSeverityValue(cell) {
					severityCol = i
					break
				}
			}
		}

		if checkboxCol == -1 || severityCol == -1 {
			continue
		}
		if checkboxCol >= len(cells) || severityCol >= len(cells) {
			continue
		}

		checkbox := strings.TrimSpace(cells[checkboxCol])
		severity := strings.TrimSpace(strings.ToUpper(cells[severityCol]))

		if severity == "" {
			continue
		}

		if stats[severity] == nil {
			stats[severity] = &TDStatsSeverity{}
		}

		if state, ok := stateOf(checkbox); ok {
			stats[severity].addState(state.Key)
		}
	}

	// Ensure all standard severities exist in result
	for _, sev := range severityOrder {
		if stats[sev] == nil {
			stats[sev] = &TDStatsSeverity{}
		}
	}

	result := &TDStatsResult{
		Severity: make(map[string]TDStatsSeverity),
	}
	var totals TDStatsTotals
	for k, v := range stats {
		result.Severity[k] = *v
		totals.Open += v.Open
		totals.Deferred += v.Deferred
		totals.Resolved += v.Resolved
		totals.Unreproducible += v.Unreproducible
	}
	// Total is every state, not just the three that close a row by fixing it.
	// Omitting a state here under-reports the file's size.
	totals.Total = totals.Open + totals.Deferred + totals.Resolved + totals.Unreproducible
	result.Summary = totals

	return result, nil
}

// renderedStateKeys returns the state keys whose columns this result should
// carry: the always-on ones, plus any opt-in state the file actually uses.
//
// An opt-in state stays invisible until a row uses it, so adding a state to the
// tool never rewrites a README that does not use it.
func renderedStates(result *TDStatsResult) []checkboxState {
	used := make(map[string]bool)
	for _, severity := range result.Severity {
		for key, count := range severity.counts() {
			if count > 0 {
				used[key] = true
			}
		}
	}

	states := make([]checkboxState, 0, len(checkboxStateOrder))
	for _, state := range distinctStates() {
		if !state.Always && !used[state.Key] {
			continue
		}
		states = append(states, state)
	}
	return states
}

func formatTDStatsMarkdown(result *TDStatsResult) string {
	states := renderedStates(result)

	var header, separator strings.Builder
	header.WriteString("| Severity |")
	separator.WriteString("|----------|")
	for _, state := range states {
		header.WriteString(fmt.Sprintf(" %s |", state.Column))
		// The separator must be as wide as its heading or the table stops
		// rendering as a table in some viewers.
		separator.WriteString(strings.Repeat("-", len(state.Column)+2) + "|")
	}

	var sb strings.Builder
	sb.WriteString("## Stats\n\n")
	sb.WriteString(header.String() + "\n")
	sb.WriteString(separator.String() + "\n")

	writeRow := func(severity string, counts map[string]int) {
		sb.WriteString(fmt.Sprintf("| %s |", severity))
		for _, state := range states {
			sb.WriteString(fmt.Sprintf(" %d |", counts[state.Key]))
		}
		sb.WriteString("\n")
	}

	for _, sev := range severityOrder {
		writeRow(sev, result.Severity[sev].counts())
	}

	// Include any non-standard severities at the end, sorted — ranging over the
	// map directly emitted them in Go's randomised order, so --write reordered
	// the rows on most runs and was not idempotent for files that use one.
	// Matches td-matrix, which already sorts its extra severity columns.
	extras := make([]string, 0, len(result.Severity))
	for sev := range result.Severity {
		if !isSeverityValue(sev) {
			extras = append(extras, sev)
		}
	}
	sort.Strings(extras)
	for _, sev := range extras {
		writeRow(sev, result.Severity[sev].counts())
	}

	return sb.String()
}

// splitTableRow splits a markdown table row into cells.
// Leading/trailing empty cells from the outer pipes are removed.
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	// Remove leading and trailing pipes
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}
	return strings.Split(line, "|")
}

func isSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		trimmed := strings.TrimSpace(cell)
		if trimmed == "" {
			continue
		}
		// Separator cells are like "---", ":---:", ":---", "---:"
		cleaned := strings.Trim(trimmed, "-:")
		if cleaned != "" {
			return false
		}
	}
	return true
}

func init() {
	RootCmd.AddCommand(newTDStatsCmd())
}
