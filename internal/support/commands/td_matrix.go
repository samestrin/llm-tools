package commands

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tdMatrixPath string
	tdMatrixJSON bool
	tdMatrixMin  bool
)

// TDMatrixResult is a severity × group cross-tab of OPEN technical-debt items.
type TDMatrixResult struct {
	Severities []string                  `json:"severities"` // column order: standard four, then extras
	Groups     []string                  `json:"groups"`     // row order: numeric asc, then non-numeric asc
	Counts     map[string]map[string]int `json:"counts"`     // group -> severity -> count
	RowTotals  map[string]int            `json:"row_totals"` // group -> total open
	ColTotals  map[string]int            `json:"col_totals"` // severity -> total open
	Total      int                       `json:"total"`      // grand total open
	Markdown   string                    `json:"markdown"`
}

func newTDMatrixCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-matrix",
		Short: "Cross-tab of OPEN tech debt items by severity and group",
		Long: `Reads a technical-debt README and tabulates OPEN ("[ ]") items as a
severity × group matrix (rows = group, columns = severity), with row and
column totals. Resolved ("[x]") and deferred ("[/]") items are excluded.

Only rows inside dated "### [date] From..." sections are counted, so unrelated
tables (e.g. the Stats table) are ignored. Read-only — the file is never
modified.`,
		RunE: runTDMatrix,
	}

	cmd.Flags().StringVar(&tdMatrixPath, "path", "", "Path to tech debt README (required)")
	cmd.Flags().BoolVar(&tdMatrixJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tdMatrixMin, "min", false, "Minimal output format")
	cmd.MarkFlagRequired("path")

	return cmd
}

func runTDMatrix(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(tdMatrixPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	result := buildTDMatrix(string(content))

	formatter := output.New(tdMatrixJSON, tdMatrixMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprint(w, data.(*TDMatrixResult).Markdown)
	})
}

// buildTDMatrix walks the README, counting open rows by (group, severity).
func buildTDMatrix(content string) *TDMatrixResult {
	counts := map[string]map[string]int{}
	extraSev := map[string]bool{} // severities outside the standard four
	inFromSection := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			inFromSection = tdFromSectionRe.MatchString(trimmed)
			continue
		}
		if !inFromSection || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitTableRow(trimmed)
		if isSeparatorRow(cells) {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		// Fixed TD column layout: [0]=Group [1]=Checkbox [2]=Severity ...
		if len(cells) < 3 || strings.EqualFold(cells[0], "Group") {
			continue
		}
		if cells[1] != "[ ]" { // open items only
			continue
		}
		group := cells[0]
		severity := strings.ToUpper(cells[2])
		if group == "" || severity == "" {
			continue
		}
		if counts[group] == nil {
			counts[group] = map[string]int{}
		}
		counts[group][severity]++
		if !isStandardSeverity(severity) {
			extraSev[severity] = true
		}
	}

	return assembleTDMatrix(counts, extraSev)
}

// assembleTDMatrix orders rows/columns, computes totals, and renders markdown.
func assembleTDMatrix(counts map[string]map[string]int, extraSev map[string]bool) *TDMatrixResult {
	// Column order: standard four, then any extras sorted alphabetically.
	severities := append([]string{}, severityOrder...)
	extras := make([]string, 0, len(extraSev))
	for s := range extraSev {
		extras = append(extras, s)
	}
	sort.Strings(extras)
	severities = append(severities, extras...)

	// Row order: numeric groups ascending, then non-numeric ascending.
	groups := make([]string, 0, len(counts))
	for g := range counts {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		ni, ei := strconv.Atoi(groups[i])
		nj, ej := strconv.Atoi(groups[j])
		switch {
		case ei == nil && ej == nil:
			return ni < nj
		case ei == nil:
			return true
		case ej == nil:
			return false
		default:
			return groups[i] < groups[j]
		}
	})

	rowTotals := map[string]int{}
	colTotals := map[string]int{}
	for _, s := range severities {
		colTotals[s] = 0
	}
	total := 0
	for g, row := range counts {
		for s, n := range row {
			rowTotals[g] += n
			colTotals[s] += n
			total += n
		}
	}

	result := &TDMatrixResult{
		Severities: severities,
		Groups:     groups,
		Counts:     counts,
		RowTotals:  rowTotals,
		ColTotals:  colTotals,
		Total:      total,
	}
	result.Markdown = formatTDMatrixMarkdown(result)
	return result
}

func formatTDMatrixMarkdown(r *TDMatrixResult) string {
	var sb strings.Builder
	sb.WriteString("## Open TD by Severity × Group\n\n")

	header := append([]string{"Group"}, r.Severities...)
	header = append(header, "Total")
	sb.WriteString(renderRow(header))

	sep := make([]string, len(header))
	for i := range sep {
		sep[i] = "---"
	}
	sb.WriteString(renderRow(sep))

	for _, g := range r.Groups {
		cells := []string{g}
		for _, s := range r.Severities {
			cells = append(cells, strconv.Itoa(r.Counts[g][s]))
		}
		cells = append(cells, strconv.Itoa(r.RowTotals[g]))
		sb.WriteString(renderRow(cells))
	}

	totalCells := []string{"**Total**"}
	for _, s := range r.Severities {
		totalCells = append(totalCells, strconv.Itoa(r.ColTotals[s]))
	}
	totalCells = append(totalCells, strconv.Itoa(r.Total))
	sb.WriteString(renderRow(totalCells))

	return sb.String()
}

func renderRow(cells []string) string {
	return "| " + strings.Join(cells, " | ") + " |\n"
}

func isStandardSeverity(s string) bool {
	for _, std := range severityOrder {
		if s == std {
			return true
		}
	}
	return false
}

func init() {
	RootCmd.AddCommand(newTDMatrixCmd())
}
