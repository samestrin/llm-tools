package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tdValidatePath string
	tdValidateRoot string
	tdValidateMode string
	tdValidateJSON bool
	tdValidateMin  bool
)

// lineRangeRe matches a pure line-number or line-range suffix (e.g., "72", "289-322").
var lineRangeRe = regexp.MustCompile(`^\d+(-\d+)?$`)

// TDValidateItem holds the validation result for a single TD row.
type TDValidateItem struct {
	Group       string `json:"group"`
	Checkbox    string `json:"checkbox"`
	Severity    string `json:"severity"`
	FileLine    string `json:"file_line"`
	FilePath    string `json:"file_path"`
	Symbol      string `json:"symbol"`
	FileExists  bool   `json:"file_exists"`
	SymbolFound *bool  `json:"symbol_found"` // null=no symbol to check; true=found; false=not found
	Status      string `json:"status"`       // valid|file_missing|symbol_not_found|no_file
	Section     string `json:"section"`
}

// TDValidateSummary holds aggregate counts across all validated rows.
type TDValidateSummary struct {
	Total           int `json:"total"`
	Valid           int `json:"valid"`
	FileMissing     int `json:"file_missing"`
	SymbolNotFound  int `json:"symbol_not_found"`
	NoFile          int `json:"no_file"`
	OpenChecked     int `json:"open_checked"`
	DeferredChecked int `json:"deferred_checked"`
}

// TDValidateResult is the full JSON payload returned by td-validate.
type TDValidateResult struct {
	Items   []TDValidateItem  `json:"items"`
	Summary TDValidateSummary `json:"summary"`
}

func newTDValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-validate",
		Short: "Verify cited files and symbols in open TD items still exist",
		Long: `Reads a technical-debt README and checks whether the file and optional symbol
cited in each open (or deferred) row still exist in the target repository.

Mode:
  open (default) — only [ ] rows not marked "(intent_note: deferred" in Problem
  all            — also includes [/] rows and [ ] rows with intent_note deferred

Resolved [x]/[X] rows are always excluded.

Per-item status values:
  valid            — file exists; symbol found (or no symbol to check)
  file_missing     — file path does not exist under --root
  symbol_not_found — file exists but symbol is absent
  no_file          — FileLine column is empty or unparseable`,
		RunE: runTDValidate,
	}
	cmd.Flags().StringVar(&tdValidatePath, "path", "", "Path to TD README (required)")
	cmd.Flags().StringVar(&tdValidateRoot, "root", ".", "Repo root for resolving relative file paths")
	cmd.Flags().StringVar(&tdValidateMode, "mode", "open", "Rows to check: open or all")
	cmd.Flags().BoolVar(&tdValidateJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tdValidateMin, "min", false, "Minimal output")
	cmd.MarkFlagRequired("path")
	return cmd
}

func runTDValidate(cmd *cobra.Command, _ []string) error {
	if tdValidateMode != "open" && tdValidateMode != "all" {
		return fmt.Errorf("invalid mode %q: must be open or all", tdValidateMode)
	}

	content, err := os.ReadFile(tdValidatePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	root := tdValidateRoot
	if root == "" {
		root = "."
	}

	rows := parseTDValidateRows(string(content))

	items := make([]TDValidateItem, 0)
	summary := TDValidateSummary{}

	for _, row := range rows {
		deferred := isTDDeferred(row)
		if deferred && tdValidateMode == "open" {
			continue
		}

		item := TDValidateItem{
			Group:    row.Group,
			Checkbox: row.Checkbox,
			Severity: row.Severity,
			FileLine: row.FileLine,
			Section:  row.Section,
		}

		summary.Total++
		if deferred {
			summary.DeferredChecked++
		} else {
			summary.OpenChecked++
		}

		filePath, symbol, ok := parseFileLine(row.FileLine)
		if !ok {
			item.Status = "no_file"
			summary.NoFile++
			items = append(items, item)
			continue
		}

		item.FilePath = filePath
		item.Symbol = symbol

		absPath := filepath.Join(root, filePath)
		_, statErr := os.Stat(absPath)
		item.FileExists = statErr == nil

		if !item.FileExists {
			item.Status = "file_missing"
			summary.FileMissing++
			items = append(items, item)
			continue
		}

		if symbol != "" {
			found := symbolExistsInFile(absPath, symbol)
			item.SymbolFound = &found
			if found {
				item.Status = "valid"
				summary.Valid++
			} else {
				item.Status = "symbol_not_found"
				summary.SymbolNotFound++
			}
		} else {
			item.Status = "valid"
			summary.Valid++
		}

		items = append(items, item)
	}

	result := &TDValidateResult{Items: items, Summary: summary}
	formatter := output.New(tdValidateJSON, tdValidateMin, cmd.OutOrStdout())
	return formatter.Print(result, printTDValidateText)
}

// parseTDValidateRows collects all non-resolved rows from dated From-sections.
// It reuses splitTableRow, isSeparatorRow, and tdFromSectionRe from the package.
// Unlike filterTD, it applies no mode/max/EstMinutes constraints.
func parseTDValidateRows(content string) []TDFilterRow {
	var rows []TDFilterRow
	currentSection := ""
	inFromSection := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			inFromSection = tdFromSectionRe.MatchString(trimmed)
			if inFromSection {
				currentSection = trimmed
			} else {
				currentSection = ""
			}
			continue
		}

		if !inFromSection || !strings.HasPrefix(trimmed, "|") {
			continue
		}

		cells := splitTableRow(trimmed)
		if isSeparatorRow(cells) {
			continue
		}

		// Trim each cell
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}

		if len(cells) < 4 {
			continue
		}

		// Skip the header row (Group column header)
		if strings.EqualFold(cells[0], "Group") {
			continue
		}

		checkbox := cells[1]

		// Always skip resolved rows
		if checkbox == "[x]" || checkbox == "[X]" {
			continue
		}

		row := TDFilterRow{
			Group:    cells[0],
			Checkbox: checkbox,
			Section:  currentSection,
		}
		if len(cells) > 2 {
			row.Severity = strings.ToUpper(cells[2])
		}
		if len(cells) > 3 {
			row.FileLine = cells[3]
		}
		if len(cells) > 4 {
			row.Problem = cells[4]
		}

		rows = append(rows, row)
	}
	return rows
}

// isTDDeferred reports whether a row should be treated as deferred.
// A row is deferred if its checkbox is "[/]" or if its Problem contains
// the "(intent_note: deferred" marker used by newer sprint conventions.
func isTDDeferred(row TDFilterRow) bool {
	if row.Checkbox == "[/]" {
		return true
	}
	return strings.Contains(row.Problem, "(intent_note: deferred")
}

// parseFileLine extracts a file path and optional symbol from a TD FileLine cell.
// Returns (filePath, symbol, ok); ok=false means the field is empty or unparseable.
//
// Supported formats:
//
//	scripts/foo.py:72                     → file=scripts/foo.py, symbol=""
//	scripts/foo.py:run_func               → file=scripts/foo.py, symbol=run_func
//	scripts/foo.py:run_func / _persist    → file=scripts/foo.py, symbol=run_func (take first)
//	scripts/foo.py (some note)            → file=scripts/foo.py, symbol=""
//	scripts/foo.py:289-322                → file=scripts/foo.py, symbol="" (line range)
//	scripts/a.py / scripts/b.py           → file=scripts/a.py,   symbol="" (multi-file, take first)
func parseFileLine(raw string) (filePath, symbol string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", false
	}

	// Multi-file notation: take first segment before " / " or " ; "
	for _, sep := range []string{" / ", " ; "} {
		if idx := strings.Index(s, sep); idx >= 0 {
			candidate := strings.TrimSpace(s[:idx])
			// Only split on multi-file if the candidate does not look like "path:symbol / _inner"
			// Detect: if candidate contains no colon, this is definitely a multi-file split.
			// If candidate has a colon, the right side is a symbol with inner slash notation —
			// handled separately below via the suffix check.
			if !strings.Contains(candidate, ":") {
				s = candidate
				break
			}
		}
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}

	// Strip trailing parenthetical annotation: drop from first " (" onward.
	// Guard: only strip if " (" appears after at least one non-space character.
	if pIdx := strings.Index(s, " ("); pIdx > 0 {
		s = strings.TrimSpace(s[:pIdx])
	}

	// Split on the first colon to separate file path from suffix.
	colonIdx := strings.IndexByte(s, ':')
	if colonIdx < 0 {
		return s, "", true
	}

	filePath = s[:colonIdx]
	rest := strings.TrimSpace(s[colonIdx+1:])

	// Empty suffix → no symbol
	if rest == "" {
		return filePath, "", true
	}

	// Line-number or line-range suffix → no symbol
	if lineRangeRe.MatchString(rest) {
		return filePath, "", true
	}

	// Symbol notation: may contain " / " for multi-symbol — take first token.
	sym := rest
	if idx := strings.Index(sym, " / "); idx >= 0 {
		sym = sym[:idx]
	}
	return filePath, strings.TrimSpace(sym), true
}

// symbolExistsInFile reports whether symbol appears as a whole word in absPath.
func symbolExistsInFile(absPath, symbol string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()

	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(symbol) + `\b`)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if re.MatchString(scanner.Text()) {
			return true
		}
	}
	return false
}

func printTDValidateText(w io.Writer, data interface{}) {
	r := data.(*TDValidateResult)
	fmt.Fprintf(w, "td_validate: %d item(s) checked — %d valid, %d file_missing, %d symbol_not_found\n",
		r.Summary.Total, r.Summary.Valid, r.Summary.FileMissing, r.Summary.SymbolNotFound)

	// Group problematic items by file_line for a compact report.
	type entry struct {
		fileLine string
		count    int
	}
	seen := map[string]int{}
	var order []string
	for _, it := range r.Items {
		if it.Status == "file_missing" || it.Status == "symbol_not_found" || it.Status == "no_file" {
			key := it.Status + ":" + it.FileLine
			if seen[key] == 0 {
				order = append(order, key)
			}
			seen[key]++
		}
	}
	for _, key := range order {
		parts := strings.SplitN(key, ":", 2)
		statusLabel := strings.ToUpper(parts[0])
		fileLine := parts[1]
		count := seen[key]
		if count > 1 {
			fmt.Fprintf(w, "%s: %s (%d items)\n", statusLabel, fileLine, count)
		} else {
			fmt.Fprintf(w, "%s: %s\n", statusLabel, fileLine)
		}
	}
}

func init() {
	RootCmd.AddCommand(newTDValidateCmd())
}
