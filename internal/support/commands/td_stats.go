package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tdStatsPath string
	tdStatsJSON bool
	tdStatsMin  bool
)

// TDStatsResult holds the aggregated tech debt statistics
type TDStatsResult struct {
	Severity map[string]TDStatsSeverity `json:"severity"`
	Summary  TDStatsTotals              `json:"summary"`
	Markdown string                     `json:"markdown"`
}

// TDStatsTotals holds aggregate counts across all severities
type TDStatsTotals struct {
	Open     int `json:"open"`
	Deferred int `json:"deferred"`
	Resolved int `json:"resolved"`
	Total    int `json:"total"`
}

// TDStatsSeverity holds counts for a single severity level
type TDStatsSeverity struct {
	Open     int `json:"open"`
	Deferred int `json:"deferred"`
	Resolved int `json:"resolved"`
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

Columns are detected by header name (looks for a column containing checkbox
markers and a column named "Severity").

Output includes both structured severity counts and a pre-rendered markdown
table in the "markdown" field.`,
		RunE: runTDStats,
	}

	cmd.Flags().StringVar(&tdStatsPath, "path", "", "Path to tech debt README (required)")
	cmd.Flags().BoolVar(&tdStatsJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tdStatsMin, "min", false, "Minimal output format")
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

	formatter := output.New(tdStatsJSON, tdStatsMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprint(w, data.(*TDStatsResult).Markdown)
	})
}

func parseTDStats(content string) (*TDStatsResult, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	stats := make(map[string]*TDStatsSeverity)

	checkboxCol := -1
	severityCol := -1
	inTable := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "|") {
			inTable = false
			checkboxCol = -1
			severityCol = -1
			continue
		}

		cells := splitTableRow(line)

		// Detect header row
		if !inTable {
			for i, cell := range cells {
				lower := strings.ToLower(strings.TrimSpace(cell))
				if lower == "severity" {
					severityCol = i
				}
			}
			// Checkbox column won't have a recognizable header name,
			// so we detect it from data rows. Mark that we're in a table.
			inTable = true
			continue
		}

		// Skip separator rows (|---|---|...)
		if isSeparatorRow(cells) {
			continue
		}

		// Auto-detect checkbox column from first data row if not found yet
		if checkboxCol == -1 {
			for i, cell := range cells {
				trimmed := strings.TrimSpace(cell)
				if trimmed == "[ ]" || trimmed == "[x]" || trimmed == "[X]" || trimmed == "[/]" {
					checkboxCol = i
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

		switch checkbox {
		case "[ ]":
			stats[severity].Open++
		case "[/]":
			stats[severity].Deferred++
		case "[x]", "[X]":
			stats[severity].Resolved++
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
	}
	totals.Total = totals.Open + totals.Deferred + totals.Resolved
	result.Summary = totals

	return result, nil
}

func formatTDStatsMarkdown(result *TDStatsResult) string {
	var sb strings.Builder
	sb.WriteString("## Stats\n\n")
	sb.WriteString("| Severity | Open | Deferred | Resolved |\n")
	sb.WriteString("|----------|------|----------|----------|\n")

	for _, sev := range severityOrder {
		s, ok := result.Severity[sev]
		if !ok {
			s = TDStatsSeverity{}
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d |\n", sev, s.Open, s.Deferred, s.Resolved))
	}

	// Include any non-standard severities at the end
	for sev, s := range result.Severity {
		found := false
		for _, std := range severityOrder {
			if sev == std {
				found = true
				break
			}
		}
		if !found {
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d |\n", sev, s.Open, s.Deferred, s.Resolved))
		}
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
