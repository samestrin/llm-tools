package commands

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tdFilterPath       string
	tdFilterMode       string
	tdFilterSeverity   string
	tdFilterConfidence string
	tdFilterGroup      string
	tdFilterFocus      string
	tdFilterMax        int
	tdFilterJSON       bool
	tdFilterMin        bool
)

func newTDFilterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-filter",
		Short: "Select technical-debt rows from a README by filter criteria",
		Long: `Parses a technical-debt README flat table and emits ONLY the unchecked rows
matching the given filters, plus a summary of counts and (with --group) the
group write-scope. This is the deterministic selection step /resolve-td used to
do in-model.

Filter pipeline (matches /resolve-td): focus section -> unchecked [ ] only ->
mode threshold -> group (+ write-scope) -> severity -> confidence -> max.

  mode:        quick-wins (est < 30) | backlog (30 <= est < 2880) | all
  severity:    comma list (low,medium,high,critical); empty = all
  confidence:  comma list (low,medium,high); empty = all. Rows with no
               Confidence column are excluded when this filter is set.
  group:       Group-column value (e.g. solo,1,2,u); empty = all
  focus:       section header substring, case-insensitive
  max:         cap on returned items (default 10), first N in source order

Output is JSON: {items:[...], summary:{...}}.`,
		RunE: runTDFilter,
	}
	cmd.Flags().StringVar(&tdFilterPath, "path", "", "Path to the technical-debt README (required)")
	cmd.Flags().StringVar(&tdFilterMode, "mode", "quick-wins", "quick-wins | backlog | all")
	cmd.Flags().StringVar(&tdFilterSeverity, "severity", "", "Comma-separated severities to keep (empty = all)")
	cmd.Flags().StringVar(&tdFilterConfidence, "confidence", "", "Comma-separated confidences to keep (empty = all)")
	cmd.Flags().StringVar(&tdFilterGroup, "group", "", "Group-column value to keep (empty = all)")
	cmd.Flags().StringVar(&tdFilterFocus, "focus", "", "Section header substring (case-insensitive)")
	cmd.Flags().IntVar(&tdFilterMax, "max", 10, "Max items to return (first N in source order)")
	cmd.Flags().BoolVar(&tdFilterJSON, "json", true, "Output as JSON (default true)")
	cmd.Flags().BoolVar(&tdFilterMin, "min", false, "Minimal output format")
	cmd.MarkFlagRequired("path")
	return cmd
}

func runTDFilter(cmd *cobra.Command, _ []string) error {
	content, err := os.ReadFile(tdFilterPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	result, err := filterTD(string(content), TDFilterOpts{
		Mode:       tdFilterMode,
		Severity:   splitCSV(tdFilterSeverity),
		Confidence: splitCSV(tdFilterConfidence),
		Group:      strings.TrimSpace(tdFilterGroup),
		Focus:      strings.TrimSpace(tdFilterFocus),
		Max:        tdFilterMax,
	})
	if err != nil {
		return err
	}
	formatter := output.New(tdFilterJSON, tdFilterMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(*TDFilterResult)
		fmt.Fprintf(w, "%d/%d item(s) match (mode=%s)\n", len(r.Items), r.Summary.TotalUnchecked, r.Summary.Mode)
	})
}

func init() {
	RootCmd.AddCommand(newTDFilterCmd())
}

// splitCSV splits a comma-separated string, trimming blanks.
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// TDFilterRow is one unchecked technical-debt item selected from the README.
type TDFilterRow struct {
	Group      string  `json:"group"`
	Checkbox   string  `json:"checkbox"`
	Severity   string  `json:"severity"`
	FileLine   string  `json:"file_line"`
	Problem    string  `json:"problem"`
	Fix        string  `json:"fix"`
	Category   string  `json:"category"`
	EstMinutes float64 `json:"est_minutes"`
	Source     string  `json:"source,omitempty"`
	Reviewers  string  `json:"reviewers,omitempty"`
	Confidence string  `json:"confidence,omitempty"`
	Section    string  `json:"section"`
}

// TDFilterOpts mirrors the /resolve-td selection flags.
type TDFilterOpts struct {
	Mode       string   // quick-wins | backlog | all (default quick-wins)
	Severity   []string // empty = all
	Confidence []string // empty = all
	Group      string   // empty = all
	Focus      string   // section substring, case-insensitive
	Max        int      // default 10
}

// TDFilterSummary reports the counts the skill displays (and the group write-scope).
type TDFilterSummary struct {
	TotalUnchecked       int      `json:"total_unchecked"`
	Matched              int      `json:"matched"` // after all filters, before --max
	Max                  int      `json:"max"`
	Mode                 string   `json:"mode"`
	Focus                string   `json:"focus,omitempty"`
	FocusMatchedSections int      `json:"focus_matched_sections,omitempty"`
	ExcludedByGroup      int      `json:"excluded_by_group"`
	ExcludedBySeverity   int      `json:"excluded_by_severity"`
	ExcludedByConfidence int      `json:"excluded_by_confidence"`
	GroupScope           []string `json:"group_scope,omitempty"`
	Malformed            []string `json:"malformed,omitempty"`
}

// TDFilterResult is the full JSON payload returned to the skill.
type TDFilterResult struct {
	Items   []TDFilterRow   `json:"items"`
	Summary TDFilterSummary `json:"summary"`
}

// tdFromSectionRe matches a dated TD section header: "### [date] From Sprint: x"
// or "### [date] From: x". Only tables under these sections hold TD items.
var tdFromSectionRe = regexp.MustCompile(`(?i)^###\s+.*\bFrom\b`)

// filterTD parses a technical-debt README and returns the unchecked rows
// matching opts, plus a summary. The pipeline order matches /resolve-td exactly:
// focus → unchecked-only → mode → group (+scope, pre-max) → severity →
// confidence → max.
func filterTD(content string, opts TDFilterOpts) (*TDFilterResult, error) {
	mode := opts.Mode
	if mode == "" {
		mode = "quick-wins"
	}
	if mode != "quick-wins" && mode != "backlog" && mode != "all" {
		return nil, fmt.Errorf("invalid mode %q: must be quick-wins, backlog, or all", mode)
	}

	// 1. Walk lines; collect unchecked rows from focus-matched From-sections.
	var unchecked []TDFilterRow
	var malformed []string
	focusMatched := map[string]bool{}
	focusLower := strings.ToLower(opts.Focus)
	currentSection := ""
	inFocus := false // not inside any From-section yet

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if tdFromSectionRe.MatchString(trimmed) {
				currentSection = trimmed
				if opts.Focus == "" {
					inFocus = true
				} else {
					inFocus = strings.Contains(strings.ToLower(trimmed), focusLower)
					if inFocus {
						focusMatched[trimmed] = true
					}
				}
			} else {
				currentSection = ""
				inFocus = false
			}
			continue
		}
		if !inFocus || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := splitTableRow(trimmed)
		if isSeparatorRow(cells) {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cells) < 8 || strings.EqualFold(cells[0], "Group") {
			continue // not a TD data row (or the header row)
		}
		if cells[1] != "[ ]" {
			continue // unchecked only
		}
		row := TDFilterRow{
			Group: cells[0], Checkbox: cells[1], Severity: strings.ToUpper(cells[2]),
			FileLine: cells[3], Problem: cells[4], Fix: cells[5], Category: cells[6],
			Section: currentSection,
		}
		if len(cells) > 8 {
			row.Source = cells[8]
		}
		if len(cells) > 9 {
			row.Reviewers = cells[9]
		}
		if len(cells) > 10 {
			row.Confidence = strings.ToUpper(cells[10])
		}
		est, err := strconv.ParseFloat(cells[7], 64)
		if err != nil {
			// Visible, not silent: an unparseable est can't be mode-classified.
			malformed = append(malformed, row.FileLine)
			continue
		}
		row.EstMinutes = est
		unchecked = append(unchecked, row)
	}

	totalUnchecked := len(unchecked)

	// 2. Mode threshold.
	var modeIn []TDFilterRow
	for _, r := range unchecked {
		switch mode {
		case "all":
			modeIn = append(modeIn, r)
		case "backlog":
			if r.EstMinutes >= 30 && r.EstMinutes < 2880 {
				modeIn = append(modeIn, r)
			}
		case "quick-wins":
			if r.EstMinutes < 30 {
				modeIn = append(modeIn, r)
			}
		}
	}

	// 3. Group filter + write-scope (union of file paths over the group-filtered
	// set, computed BEFORE --max so the scope covers the whole group).
	working := modeIn
	excludedByGroup := 0
	var groupScope []string
	if opts.Group != "" {
		kept := working[:0:0]
		for _, r := range working {
			if strings.EqualFold(r.Group, opts.Group) {
				kept = append(kept, r)
			}
		}
		excludedByGroup = len(working) - len(kept)
		working = kept
		seen := map[string]bool{}
		for _, r := range working {
			if p := filePathOf(r.FileLine); p != "" && !seen[p] {
				seen[p] = true
				groupScope = append(groupScope, p)
			}
		}
	}

	// 4. Severity filter.
	excludedBySeverity := 0
	if len(opts.Severity) > 0 {
		want := upperSet(opts.Severity)
		kept := working[:0:0]
		for _, r := range working {
			if want[r.Severity] {
				kept = append(kept, r)
			}
		}
		excludedBySeverity = len(working) - len(kept)
		working = kept
	}

	// 5. Confidence filter — rows with empty Confidence (legacy) are excluded
	// when this filter is active.
	excludedByConfidence := 0
	if len(opts.Confidence) > 0 {
		want := upperSet(opts.Confidence)
		kept := working[:0:0]
		for _, r := range working {
			if r.Confidence != "" && want[r.Confidence] {
				kept = append(kept, r)
			}
		}
		excludedByConfidence = len(working) - len(kept)
		working = kept
	}

	matched := len(working)

	// Data-loss guard: every mode-in row is either matched or excluded by exactly
	// one sequential filter — no silent drops.
	if len(modeIn) != matched+excludedByGroup+excludedBySeverity+excludedByConfidence {
		return nil, fmt.Errorf("FATAL: filter accounting mismatch: mode_in=%d matched=%d excluded(group=%d severity=%d confidence=%d)",
			len(modeIn), matched, excludedByGroup, excludedBySeverity, excludedByConfidence)
	}

	// 6. Max — first N in source order.
	maxItems := opts.Max
	if maxItems <= 0 {
		maxItems = 10
	}
	items := working
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	if items == nil {
		items = []TDFilterRow{}
	}

	return &TDFilterResult{
		Items: items,
		Summary: TDFilterSummary{
			TotalUnchecked:       totalUnchecked,
			Matched:              matched,
			Max:                  maxItems,
			Mode:                 mode,
			Focus:                opts.Focus,
			FocusMatchedSections: len(focusMatched),
			ExcludedByGroup:      excludedByGroup,
			ExcludedBySeverity:   excludedBySeverity,
			ExcludedByConfidence: excludedByConfidence,
			GroupScope:           groupScope,
			Malformed:            malformed,
		},
	}, nil
}

// filePathOf returns the path portion of a FILE:LINE value (everything before
// the first colon).
func filePathOf(fileLine string) string {
	if i := strings.IndexByte(fileLine, ':'); i >= 0 {
		return fileLine[:i]
	}
	return fileLine
}

// upperSet builds a set of uppercased, trimmed tokens.
func upperSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		if t := strings.ToUpper(strings.TrimSpace(x)); t != "" {
			m[t] = true
		}
	}
	return m
}
