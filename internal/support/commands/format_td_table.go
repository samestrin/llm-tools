package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

// Flag variables
var (
	formatTDTableFile    string
	formatTDTableContent string
	formatTDTableSection string
	formatTDTableJSON    bool
	formatTDTableMinimal bool
)

// Constants
const (
	formatTDTableMaxInputSize = 10 * 1024 * 1024 // 10MB
	sectionQuickWins          = "quick_wins"
	sectionBacklog            = "backlog"
	sectionTDFiles            = "td_files"
	sectionAll                = "all"
)

// TDItem represents a technical debt item
type TDItem struct {
	ID          string      `json:"ID,omitempty"`
	Severity    string      `json:"SEVERITY,omitempty"`
	Category    string      `json:"CATEGORY,omitempty"`
	FileLine    string      `json:"FILE_LINE,omitempty"`
	Problem     string      `json:"PROBLEM,omitempty"`
	Fix         string      `json:"FIX,omitempty"`
	EstMinutes  interface{} `json:"EST_MINUTES,omitempty"`
	Description string      `json:"DESCRIPTION,omitempty"`
}

// RoutedTDInput represents the output from route_td command
type RoutedTDInput struct {
	QuickWins []map[string]interface{} `json:"quick_wins"`
	Backlog   []map[string]interface{} `json:"backlog"`
	TDFiles   []map[string]interface{} `json:"td_files"`
}

// RawTDInput represents a raw array of TD items
type RawTDInput struct {
	Items []map[string]interface{} `json:"items"`
	Rows  []map[string]interface{} `json:"rows"`
}

// FormatTDTableResult represents the output
type FormatTDTableResult struct {
	Tables  map[string]string `json:"tables"`
	Summary FormatTDSummary   `json:"summary"`
}

// FormatTDSummary provides counts
type FormatTDSummary struct {
	TotalItems        int            `json:"total_items"`
	SectionsFormatted int            `json:"sections_formatted"`
	ItemsPerSection   map[string]int `json:"items_per_section"`
}

func newFormatTDTableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format-td-table",
		Short: "Format technical debt items as markdown tables",
		Long: `Format technical debt items as markdown tables for README.md.

Accepts either:
- Routed output from route-td (with quick_wins, backlog, td_files sections)
- Raw array of TD items (via "items" or "rows" key)

Examples:
  # Format all sections from routed output
  llm-support format-td-table --file=routed.json

  # Format specific section
  llm-support format-td-table --file=routed.json --section=quick_wins

  # Format from stdin
  cat routed.json | llm-support format-td-table --section=backlog`,
		RunE: runFormatTDTable,
	}

	cmd.Flags().StringVar(&formatTDTableFile, "file", "", "Input JSON file path")
	cmd.Flags().StringVar(&formatTDTableContent, "content", "", "Direct JSON content")
	cmd.Flags().StringVar(&formatTDTableSection, "section", sectionAll, "Section to format: quick_wins, backlog, td_files, all")
	cmd.Flags().BoolVar(&formatTDTableJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&formatTDTableMinimal, "min", false, "Minimal output format")

	return cmd
}

func init() {
	RootCmd.AddCommand(newFormatTDTableCmd())
}

func runFormatTDTable(cmd *cobra.Command, args []string) error {
	// Validate section flag
	validSections := map[string]bool{
		sectionQuickWins: true,
		sectionBacklog:   true,
		sectionTDFiles:   true,
		sectionAll:       true,
	}
	if !validSections[formatTDTableSection] {
		return fmt.Errorf("invalid section: %s (valid: quick_wins, backlog, td_files, all)", formatTDTableSection)
	}

	// Get input
	input, err := getFormatTDTableInput(cmd)
	if err != nil {
		return err
	}

	// Parse and determine input type
	sections, err := parseFormatTDInput(input)
	if err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	// Generate tables
	result := generateTables(sections, formatTDTableSection)

	// Output
	formatter := output.New(formatTDTableJSON, formatTDTableMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(FormatTDTableResult)
		printFormatTDTableText(w, r, formatTDTableMinimal)
	})
}

func getFormatTDTableInput(cmd *cobra.Command) (string, error) {
	// Priority 1: --content flag
	if formatTDTableContent != "" {
		if len(formatTDTableContent) > formatTDTableMaxInputSize {
			return "", fmt.Errorf("content exceeds maximum size of %d bytes", formatTDTableMaxInputSize)
		}
		return formatTDTableContent, nil
	}

	// Priority 2: --file flag
	if formatTDTableFile != "" {
		info, err := os.Stat(formatTDTableFile)
		if err != nil {
			return "", fmt.Errorf("cannot access file: %w", err)
		}
		if info.Size() > formatTDTableMaxInputSize {
			return "", fmt.Errorf("file size %d exceeds maximum %d bytes", info.Size(), formatTDTableMaxInputSize)
		}
		data, err := os.ReadFile(formatTDTableFile)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		return string(data), nil
	}

	// Priority 3: stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		limitedReader := io.LimitReader(os.Stdin, formatTDTableMaxInputSize+1)
		data, err := io.ReadAll(limitedReader)
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		if len(data) > formatTDTableMaxInputSize {
			return "", fmt.Errorf("stdin exceeds maximum size of %d bytes", formatTDTableMaxInputSize)
		}
		return string(data), nil
	}

	return "", fmt.Errorf("no input provided: use --file, --content, or pipe to stdin")
}

func parseFormatTDInput(input string) (map[string][]map[string]interface{}, error) {
	sections := make(map[string][]map[string]interface{})

	// Try parsing as routed output first
	var routed RoutedTDInput
	if err := json.Unmarshal([]byte(input), &routed); err == nil {
		if routed.QuickWins != nil || routed.Backlog != nil || routed.TDFiles != nil {
			if routed.QuickWins != nil {
				sections[sectionQuickWins] = routed.QuickWins
			}
			if routed.Backlog != nil {
				sections[sectionBacklog] = routed.Backlog
			}
			if routed.TDFiles != nil {
				sections[sectionTDFiles] = routed.TDFiles
			}
			return sections, nil
		}
	}

	// Try parsing as raw input with items/rows key
	var raw RawTDInput
	if err := json.Unmarshal([]byte(input), &raw); err == nil {
		items := raw.Items
		if items == nil {
			items = raw.Rows
		}
		if items != nil {
			// Put all items in a generic "items" section
			sections["items"] = items
			return sections, nil
		}
	}

	// Try parsing as raw array
	var rawArray []map[string]interface{}
	if err := json.Unmarshal([]byte(input), &rawArray); err == nil {
		sections["items"] = rawArray
		return sections, nil
	}

	return nil, fmt.Errorf("could not parse input as routed output, {items:[...]}, {rows:[...]}, or raw array")
}

func generateTables(sections map[string][]map[string]interface{}, section string) FormatTDTableResult {
	result := FormatTDTableResult{
		Tables: make(map[string]string),
		Summary: FormatTDSummary{
			ItemsPerSection: make(map[string]int),
		},
	}

	// Determine which sections to format
	sectionsToFormat := []string{}
	if section == sectionAll {
		// Format all available sections in order
		for _, s := range []string{sectionQuickWins, sectionBacklog, sectionTDFiles, "items"} {
			if items, ok := sections[s]; ok && len(items) > 0 {
				sectionsToFormat = append(sectionsToFormat, s)
			}
		}
	} else {
		if items, ok := sections[section]; ok && len(items) > 0 {
			sectionsToFormat = append(sectionsToFormat, section)
		} else if items, ok := sections["items"]; ok && len(items) > 0 {
			// If specific section requested but we have raw items, use those
			sectionsToFormat = append(sectionsToFormat, "items")
		}
	}

	// Generate tables
	for _, s := range sectionsToFormat {
		items := sections[s]
		table := formatItemsAsTable(items)
		result.Tables[s] = table
		result.Summary.ItemsPerSection[s] = len(items)
		result.Summary.TotalItems += len(items)
	}
	result.Summary.SectionsFormatted = len(sectionsToFormat)

	return result
}

func formatItemsAsTable(items []map[string]interface{}) string {
	if len(items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Determine columns from first item, with preferred order
	preferredOrder := []string{"SEVERITY", "FILE_LINE", "CATEGORY", "PROBLEM", "FIX", "EST_MINUTES", "ID", "DESCRIPTION"}
	columns := []string{}
	columnSet := make(map[string]bool)

	// First, add columns in preferred order if they exist
	for _, col := range preferredOrder {
		for _, item := range items {
			if _, ok := item[col]; ok {
				if !columnSet[col] {
					columns = append(columns, col)
					columnSet[col] = true
				}
				break
			}
		}
	}

	// Then add any remaining columns not in preferred order
	remainingCols := []string{}
	for _, item := range items {
		for key := range item {
			if !columnSet[key] {
				remainingCols = append(remainingCols, key)
				columnSet[key] = true
			}
		}
	}
	sort.Strings(remainingCols)
	columns = append(columns, remainingCols...)

	if len(columns) == 0 {
		return ""
	}

	// Build header
	sb.WriteString("| ")
	for i, col := range columns {
		sb.WriteString(formatColumnHeader(col))
		if i < len(columns)-1 {
			sb.WriteString(" | ")
		}
	}
	sb.WriteString(" |\n")

	// Build separator
	sb.WriteString("|")
	for range columns {
		sb.WriteString("------|")
	}
	sb.WriteString("\n")

	// Build rows
	for _, item := range items {
		sb.WriteString("| ")
		for i, col := range columns {
			value := formatCellValue(item[col])
			sb.WriteString(value)
			if i < len(columns)-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString(" |\n")
	}

	return sb.String()
}

func formatColumnHeader(col string) string {
	// Convert UPPER_CASE to Title Case
	parts := strings.Split(col, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

func formatCellValue(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		// Escape pipe characters and newlines for markdown
		escaped := strings.ReplaceAll(val, "|", "\\|")
		escaped = strings.ReplaceAll(escaped, "\n", " ")
		return escaped
	case float64:
		// Format as integer if whole number
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%.1f", val)
	case int, int64:
		return fmt.Sprintf("%d", val)
	case json.Number:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func printFormatTDTableText(w io.Writer, result FormatTDTableResult, minimal bool) {
	if minimal {
		// Just print tables without headers
		for _, section := range []string{sectionQuickWins, sectionBacklog, sectionTDFiles, "items"} {
			if table, ok := result.Tables[section]; ok && table != "" {
				fmt.Fprint(w, table)
			}
		}
		return
	}

	// Print with section headers
	sectionNames := map[string]string{
		sectionQuickWins: "Quick Wins (< 30 min)",
		sectionBacklog:   "Backlog (30 min - 48 hrs)",
		sectionTDFiles:   "Sprint-Sized (48+ hrs)",
		"items":          "Technical Debt Items",
	}

	for _, section := range []string{sectionQuickWins, sectionBacklog, sectionTDFiles, "items"} {
		if table, ok := result.Tables[section]; ok && table != "" {
			fmt.Fprintf(w, "### %s\n\n", sectionNames[section])
			fmt.Fprint(w, table)
			fmt.Fprintln(w)
		}
	}

	// Print summary
	fmt.Fprintf(w, "---\nTotal: %d items in %d section(s)\n",
		result.Summary.TotalItems, result.Summary.SectionsFormatted)
}
