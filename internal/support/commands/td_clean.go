package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tdCleanPath  string
	tdCleanToday string
	tdCleanJSON  bool
	tdCleanMin   bool
)

// TDCleanResult reports what td-clean removed and the post-cleanup stats.
type TDCleanResult struct {
	RemovedRows     int `json:"removed_rows"`
	RemovedSections int `json:"removed_sections"`
	Open            int `json:"open"`
	Deferred        int `json:"deferred"`
	Resolved        int `json:"resolved"`
	Total           int `json:"total"`
}

func newTDCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-clean",
		Short: "Strip resolved [x] rows from a tech debt README and regenerate stats",
		Long: `Deterministically cleans a technical-debt README in place:

  1. Removes resolved table rows (a cell that is exactly "[x]" or "[X]").
     Rows with "[ ]" (open) or "[/]" (deferred) are kept. A literal "[x]"
     appearing inside a prose cell is NOT treated as a checkbox.
  2. Removes "### ... From Sprint:" sections left with no data rows.
  3. Regenerates the "## Stats" section.
  4. Updates the "**Last Modified:**" summary line.

If no resolved rows are found the file is left byte-for-byte unchanged.`,
		RunE: runTDClean,
	}

	cmd.Flags().StringVar(&tdCleanPath, "path", "", "Path to tech debt README (required)")
	cmd.Flags().StringVar(&tdCleanToday, "today", "", "Date for the Last Modified line (YYYY-MM-DD); defaults to today")
	cmd.Flags().BoolVar(&tdCleanJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tdCleanMin, "min", false, "Minimal output format")
	cmd.MarkFlagRequired("path")

	return cmd
}

func runTDClean(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(tdCleanPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	today := tdCleanToday
	if today == "" {
		today = time.Now().Format("2006-01-02")
	}

	cleaned, result := cleanTDReadme(string(content), today)

	// Idempotent no-op: nothing removed means nothing to rewrite.
	if result.RemovedRows > 0 || result.RemovedSections > 0 {
		if err := os.WriteFile(tdCleanPath, []byte(cleaned), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	}

	formatter := output.New(tdCleanJSON, tdCleanMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(*TDCleanResult)
		fmt.Fprintf(w, "Removed %d resolved row(s), %d empty section(s) — %d open / %d deferred / %d resolved\n",
			r.RemovedRows, r.RemovedSections, r.Open, r.Deferred, r.Resolved)
	})
}

// cleanTDReadme performs the full cleanup pass and returns the rewritten
// content alongside the result counts. Counts always reflect the post-cleanup
// state of the table, even when nothing was removed.
func cleanTDReadme(content, today string) (string, *TDCleanResult) {
	result := &TDCleanResult{}

	// Preserve trailing-newline structure: strings.Split keeps a trailing ""
	// element when content ends in "\n", and Join restores it.
	lines := strings.Split(content, "\n")

	// Pass 1: drop resolved data rows.
	stripped := make([]string, 0, len(lines))
	for _, line := range lines {
		if isResolvedRow(line) {
			result.RemovedRows++
			continue
		}
		stripped = append(stripped, line)
	}

	// Pass 2: drop "###" sections whose table now has no data rows.
	stripped, result.RemovedSections = removeEmptySections(stripped)

	cleaned := strings.Join(stripped, "\n")

	// Recompute stats from the cleaned content and refresh the rendered
	// "## Stats" section + "**Last Modified:**" line.
	stats, _ := parseTDStats(cleaned)
	result.Open = stats.Summary.Open
	result.Deferred = stats.Summary.Deferred
	result.Resolved = stats.Summary.Resolved
	result.Total = stats.Summary.Total

	finalLines := strings.Split(cleaned, "\n")
	finalLines = replaceStatsSection(finalLines, formatTDStatsMarkdown(stats))
	finalLines = updateLastModified(finalLines, today, stats.Summary)

	return strings.Join(finalLines, "\n"), result
}

// isResolvedRow reports whether a line is a table data row whose checkbox cell
// is exactly "[x]" or "[X]". Matching the whole trimmed cell (not a substring)
// keeps a literal "[x]" inside a prose cell from being mistaken for a checkbox.
func isResolvedRow(line string) bool {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "|") {
		return false
	}
	cells := splitTableRow(line)
	if isSeparatorRow(cells) {
		return false
	}
	for _, c := range cells {
		switch strings.TrimSpace(c) {
		case "[x]", "[X]":
			return true
		}
	}
	return false
}

// hasCheckboxCell reports whether a line is a table data row carrying any
// checkbox marker. After stripping, surviving markers are "[ ]" / "[/]".
func hasCheckboxCell(line string) bool {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "|") {
		return false
	}
	cells := splitTableRow(line)
	if isSeparatorRow(cells) {
		return false
	}
	for _, c := range cells {
		switch strings.TrimSpace(c) {
		case "[ ]", "[/]", "[x]", "[X]":
			return true
		}
	}
	return false
}

// isTableSeparator reports whether a line is a markdown table separator row.
func isTableSeparator(line string) bool {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "|") {
		return false
	}
	return isSeparatorRow(splitTableRow(line))
}

// isHeadingBoundary reports whether a line ends the current "###" section:
// the next "### " section, or any higher-level "## " heading (e.g. "## Stats").
func isHeadingBoundary(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "### ") || strings.HasPrefix(t, "## ")
}

// removeEmptySections drops every "### " section that contains a table
// (proven by a separator row) but no remaining checkbox data rows. Returns the
// surviving lines and the number of sections removed.
func removeEmptySections(lines []string) ([]string, int) {
	out := make([]string, 0, len(lines))
	removed := 0

	i := 0
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(t, "### ") {
			out = append(out, lines[i])
			i++
			continue
		}

		// Find the end of this section (exclusive): next heading or EOF.
		end := i + 1
		for end < len(lines) && !isHeadingBoundary(lines[end]) {
			end++
		}

		hasTable := false
		hasData := false
		for j := i + 1; j < end; j++ {
			if isTableSeparator(lines[j]) {
				hasTable = true
			}
			if hasCheckboxCell(lines[j]) {
				hasData = true
			}
		}

		if hasTable && !hasData {
			removed++
			i = end // skip the whole section
			continue
		}

		out = append(out, lines[i:end]...)
		i = end
	}

	return out, removed
}

// replaceStatsSection swaps the existing "## Stats" heading and its table for a
// freshly rendered block. statsMarkdown is the output of formatTDStatsMarkdown
// ("## Stats\n\n| Severity | ...table...\n"). If no "## Stats" heading exists,
// the block is appended.
func replaceStatsSection(lines []string, statsMarkdown string) []string {
	block := strings.Split(strings.TrimRight(statsMarkdown, "\n"), "\n")

	statsIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Stats" {
			statsIdx = i
			break
		}
	}

	if statsIdx == -1 {
		// No stats section: append after a blank separator.
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		return append(lines, block...)
	}

	// The stats block spans the heading, any blank lines, and the contiguous
	// run of table rows immediately following it. Everything after that run is
	// preserved.
	end := statsIdx + 1
	for end < len(lines) && strings.TrimSpace(lines[end]) == "" {
		end++
	}
	for end < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[end]), "|") {
		end++
	}

	out := make([]string, 0, len(lines)-(end-statsIdx)+len(block))
	out = append(out, lines[:statsIdx]...)
	out = append(out, block...)
	out = append(out, lines[end:]...)
	return out
}

// updateLastModified rewrites the "**Last Modified:**" line with the given date
// and post-cleanup counts. If no such line exists the input is returned as-is.
func updateLastModified(lines []string, today string, totals TDStatsTotals) []string {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "**Last Modified:**") {
			lines[i] = fmt.Sprintf(
				"**Last Modified:** %s | **Open Items:** %d | **Deferred Items:** %d | **Resolved Items:** %d | **Total Items:** %d",
				today, totals.Open, totals.Deferred, totals.Resolved, totals.Total)
			break
		}
	}
	return lines
}

func init() {
	RootCmd.AddCommand(newTDCleanCmd())
}
