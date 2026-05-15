package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

// Flag variables
var (
	groupTDFile             string
	groupTDContent          string
	groupTDGroupBy          string
	groupTDPathDepth        int
	groupTDMinGroupSize     int
	groupTDCriticalOverride bool
	groupTDRootTheme        string
	groupTDJSON             bool
	groupTDMinimal          bool
	groupTDAssignNumbers    bool
	groupTDOutputFile       string
	groupTDCheckbox         bool
	groupTDSprintLabel      string
	groupTDDateLabel        string
	groupTDFormat           string
	groupTDHeaders          string
	groupTDDelimiter        string
	groupTDRenumber         bool
)

// Constants
const (
	groupTDMaxInputSize = 10 * 1024 * 1024 // 10MB
	groupByPath         = "path"
	groupByCategory     = "category"
	groupByFile         = "file"
	defaultPathDepth    = 2
	defaultMinGroupSize = 3
	defaultRootTheme    = "misc"
	criticalSeverity    = "CRITICAL"
	highSeverity        = "HIGH"
	soloTheme           = "solo"
	ungroupedLabel      = "U"
)

// GroupTDInput represents the input format
type GroupTDInput struct {
	Items []map[string]interface{} `json:"items"`
	Rows  []map[string]interface{} `json:"rows"`
}

// GroupTDResult represents the output
type GroupTDResult struct {
	Groups    []TDGroup                `json:"groups"`
	Ungrouped []map[string]interface{} `json:"ungrouped"`
	Summary   GroupTDSummary           `json:"summary"`
}

// TDGroup represents a group of related TD items
type TDGroup struct {
	Theme        string                   `json:"theme"`
	Number       interface{}              `json:"number,omitempty"`
	PathPattern  string                   `json:"path_pattern,omitempty"`
	Items        []map[string]interface{} `json:"items"`
	Count        int                      `json:"count"`
	TotalMinutes int                      `json:"total_minutes"`
}

// GroupTDSummary provides counts
type GroupTDSummary struct {
	TotalItems     int `json:"total_items"`
	GroupedCount   int `json:"grouped_count"`
	UngroupedCount int `json:"ungrouped_count"`
	GroupCount     int `json:"group_count"`
}

func newGroupTDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group-td",
		Short: "Group technical debt items by path, category, or file",
		Long: `Group technical debt items using a deterministic algorithm.

Grouping strategies (--group-by):
  path     - Group by directory prefix (default)
  category - Group by CATEGORY field
  file     - Group by exact file path (strictest)

The path strategy extracts directory prefixes from FILE_LINE field.
Depth is controlled by --path-depth (default: 2).

Example:
  src/auth/handlers/login.ts:45 with depth=2 → theme "src-auth"
  src/auth/handlers/login.ts:45 with depth=3 → theme "src-auth-handlers"

Items are ungrouped if:
  - Their group has fewer than --min-group-size items (default: 3)
  - They have no FILE_LINE and --group-by=path

CRITICAL severity items always get their own group (--critical-override).

Examples:
  # Group by path prefix with default depth
  llm-support group-td --file=td_items.json

  # Group with deeper path matching
  llm-support group-td --content='[...]' --path-depth=3

  # Group by category field
  llm-support group-td --file=items.json --group-by=category

  # Require larger groups
  llm-support group-td --file=items.json --min-group-size=5`,
		RunE: runGroupTD,
	}

	cmd.Flags().StringVar(&groupTDFile, "file", "", "Input JSON file path")
	cmd.Flags().StringVar(&groupTDContent, "content", "", "Direct JSON content")
	cmd.Flags().StringVar(&groupTDGroupBy, "group-by", groupByPath, "Grouping strategy: path, category, file")
	cmd.Flags().IntVar(&groupTDPathDepth, "path-depth", defaultPathDepth, "Number of path segments for theme (path mode)")
	cmd.Flags().IntVar(&groupTDMinGroupSize, "min-group-size", defaultMinGroupSize, "Minimum items to form a group")
	cmd.Flags().BoolVar(&groupTDCriticalOverride, "critical-override", true, "CRITICAL severity always gets own group")
	cmd.Flags().StringVar(&groupTDRootTheme, "root-theme", defaultRootTheme, "Theme for items without directory structure")
	cmd.Flags().BoolVar(&groupTDJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&groupTDMinimal, "min", false, "Minimal output format")
	cmd.Flags().BoolVar(&groupTDAssignNumbers, "assign-numbers", false, "Assign group numbers to items and groups")
	cmd.Flags().StringVar(&groupTDOutputFile, "output-file", "", "Write grouped items as markdown table to file")
	cmd.Flags().BoolVar(&groupTDCheckbox, "checkbox", false, "Add checkbox column (requires --output-file)")
	cmd.Flags().StringVar(&groupTDSprintLabel, "sprint-label", "", "Sprint name for section header")
	cmd.Flags().StringVar(&groupTDDateLabel, "date-label", "", "Date for section header")
	cmd.Flags().StringVar(&groupTDFormat, "format", "json", "Input format: json or pipe")
	cmd.Flags().StringVar(&groupTDHeaders, "headers", "", "Comma-separated headers for pipe format (required with --format=pipe)")
	cmd.Flags().StringVar(&groupTDDelimiter, "delimiter", "|", "Field delimiter for pipe format")
	cmd.Flags().BoolVar(&groupTDRenumber, "renumber", false, "Renumber active groups in --output-file so each globally-active group number is unique. No input needed. Inactive ([x]-only) sections keep their numbers.")

	return cmd
}

func init() {
	RootCmd.AddCommand(newGroupTDCmd())
}

func runGroupTD(cmd *cobra.Command, args []string) error {
	// --renumber short-circuit: don't parse input, just rewrite groups in
	// the existing --output-file so each active group number is globally
	// unique. No input means no new section to append.
	if groupTDRenumber {
		if groupTDOutputFile == "" {
			return fmt.Errorf("--renumber requires --output-file")
		}
		return renumberActiveGroupsInPlace(groupTDOutputFile)
	}

	// Validate flags
	validGroupBy := map[string]bool{
		groupByPath:     true,
		groupByCategory: true,
		groupByFile:     true,
	}
	if !validGroupBy[groupTDGroupBy] {
		return fmt.Errorf("invalid group-by: %s (valid: path, category, file)", groupTDGroupBy)
	}

	if groupTDPathDepth < 1 {
		return fmt.Errorf("path-depth must be at least 1, got: %d", groupTDPathDepth)
	}

	if groupTDMinGroupSize < 1 {
		return fmt.Errorf("min-group-size must be at least 1, got: %d", groupTDMinGroupSize)
	}

	// Get input
	input, err := getGroupTDInput(cmd)
	if err != nil {
		return err
	}

	// Parse input
	items, err := parseGroupTDInput(input, groupTDFormat, groupTDHeaders, groupTDDelimiter)
	if err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	// Resolve global-numbering offset. When the caller writes a new section
	// to an existing README (--output-file points at a file that already
	// contains sections), start numbering from max(active groups) + 1 so
	// the new section's group numbers don't collide with existing active
	// groups in other sections. Inactive sections (all [x]) don't reserve
	// numbers.
	startNumber := 0
	if groupTDOutputFile != "" && groupTDAssignNumbers {
		existingMax, scanErr := scanExistingActiveGroupNumbers(groupTDOutputFile)
		if scanErr != nil {
			// Non-fatal: warn and fall back to start-at-1 behavior.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not scan existing output file for group numbers: %v\n", scanErr)
		} else if existingMax > 0 {
			startNumber = existingMax + 1
		}
	}

	// Group items
	result := groupItems(items, groupTDGroupBy, groupTDPathDepth, groupTDMinGroupSize, groupTDCriticalOverride, groupTDRootTheme, groupTDAssignNumbers, startNumber)

	// Validate no data loss
	totalOutput := result.Summary.GroupedCount + result.Summary.UngroupedCount
	if totalOutput != len(items) {
		return fmt.Errorf("FATAL: Data loss detected - input: %d, output: %d", len(items), totalOutput)
	}

	// Write output file if requested
	if groupTDOutputFile != "" {
		if err := writeGroupedMarkdown(result, groupTDOutputFile, groupTDCheckbox, groupTDSprintLabel, groupTDDateLabel); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
	}

	// Output
	formatter := output.New(groupTDJSON, groupTDMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(GroupTDResult)
		printGroupTDText(w, r, groupTDMinimal)
	})
}

func getGroupTDInput(cmd *cobra.Command) (string, error) {
	// Priority 1: --content flag
	if groupTDContent != "" {
		if len(groupTDContent) > groupTDMaxInputSize {
			return "", fmt.Errorf("content exceeds maximum size of %d bytes", groupTDMaxInputSize)
		}
		return groupTDContent, nil
	}

	// Priority 2: --file flag
	if groupTDFile != "" {
		info, err := os.Stat(groupTDFile)
		if err != nil {
			return "", fmt.Errorf("cannot access file: %w", err)
		}
		if info.Size() > groupTDMaxInputSize {
			return "", fmt.Errorf("file size %d exceeds maximum %d bytes", info.Size(), groupTDMaxInputSize)
		}
		data, err := os.ReadFile(groupTDFile)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		return string(data), nil
	}

	// Priority 3: stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		limitedReader := io.LimitReader(os.Stdin, groupTDMaxInputSize+1)
		data, err := io.ReadAll(limitedReader)
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		if len(data) > groupTDMaxInputSize {
			return "", fmt.Errorf("stdin exceeds maximum size of %d bytes", groupTDMaxInputSize)
		}
		return string(data), nil
	}

	return "", fmt.Errorf("no input provided: use --file, --content, or pipe to stdin")
}

func parseGroupTDInput(input string, format string, headers string, delimiter string) ([]map[string]interface{}, error) {
	if format == "pipe" {
		return parsePipeInput(input, headers, delimiter)
	}

	// JSON format (default) - try parsing with items/rows wrapper
	var wrapped GroupTDInput
	if err := json.Unmarshal([]byte(input), &wrapped); err == nil {
		if wrapped.Items != nil {
			return wrapped.Items, nil
		}
		if wrapped.Rows != nil {
			return wrapped.Rows, nil
		}
	}

	// Try parsing as raw array
	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(input), &items); err == nil {
		return items, nil
	}

	return nil, fmt.Errorf("could not parse input as {items:[...]}, {rows:[...]}, or raw array")
}

func parsePipeInput(input string, headersStr string, delimiter string) ([]map[string]interface{}, error) {
	if headersStr == "" {
		return nil, fmt.Errorf("--headers required with --format=pipe")
	}

	headers := strings.Split(headersStr, ",")
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	lines := strings.Split(strings.TrimSpace(input), "\n")
	var items []map[string]interface{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := strings.Split(trimmed, delimiter)
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		// Skip lines that don't have enough fields (likely malformed)
		if len(fields) < 2 {
			continue
		}

		row := make(map[string]interface{})
		for j, header := range headers {
			if j < len(fields) {
				row[header] = fields[j]
			} else {
				row[header] = ""
			}
		}
		items = append(items, row)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no data rows found in pipe input")
	}

	return items, nil
}

// groupItems clusters TD items into groups. When assignNumbers is true and
// startNumber > 0, numbered groups begin at startNumber (used to continue
// globally-unique numbering across existing README sections). startNumber=0
// is treated the same as 1 (today's behavior) for backward compatibility
// with callers that don't pass an explicit start.
func groupItems(items []map[string]interface{}, groupBy string, pathDepth, minGroupSize int, criticalOverride bool, rootTheme string, assignNumbers bool, startNumber int) GroupTDResult {
	// Step 1: Extract theme for each item
	itemThemes := make(map[int]string)
	for i, item := range items {
		theme := extractTheme(item, groupBy, pathDepth, rootTheme)
		itemThemes[i] = theme
	}

	// Step 2: Separate critical items if override enabled
	criticalItems := []map[string]interface{}{}
	regularIndices := []int{}

	if criticalOverride {
		for i, item := range items {
			if severity, ok := item["SEVERITY"].(string); ok && strings.ToUpper(severity) == criticalSeverity {
				criticalItems = append(criticalItems, item)
			} else {
				regularIndices = append(regularIndices, i)
			}
		}
	} else {
		for i := range items {
			regularIndices = append(regularIndices, i)
		}
	}

	// Step 3: Count items per theme for regular items
	themeCounts := make(map[string]int)
	for _, i := range regularIndices {
		theme := itemThemes[i]
		themeCounts[theme]++
	}

	// Step 4: Determine which themes qualify as groups
	qualifiedThemes := make(map[string]bool)
	for theme, count := range themeCounts {
		if count >= minGroupSize {
			qualifiedThemes[theme] = true
		}
	}

	// Step 5: Build groups and ungrouped
	groupMap := make(map[string]*TDGroup)
	ungrouped := []map[string]interface{}{}

	for _, i := range regularIndices {
		theme := itemThemes[i]
		item := items[i]

		if qualifiedThemes[theme] {
			if groupMap[theme] == nil {
				groupMap[theme] = &TDGroup{
					Theme:       theme,
					PathPattern: buildPathPattern(theme, groupBy),
					Items:       []map[string]interface{}{},
				}
			}
			groupMap[theme].Items = append(groupMap[theme].Items, item)
			groupMap[theme].TotalMinutes += extractEstMinutesInt(item)
		} else {
			ungrouped = append(ungrouped, item)
		}
	}

	// Step 6: Solo detection - HIGH/CRITICAL ungrouped items run solo
	// Only when assignNumbers is enabled (code-review pipeline)
	if assignNumbers {
		soloItems := []map[string]interface{}{}
		remainingUngrouped := []map[string]interface{}{}
		for _, item := range ungrouped {
			severity, _ := item["SEVERITY"].(string)
			sev := strings.ToUpper(severity)
			if sev == criticalSeverity || sev == highSeverity {
				soloItems = append(soloItems, item)
			} else {
				remainingUngrouped = append(remainingUngrouped, item)
			}
		}
		ungrouped = remainingUngrouped

		if len(soloItems) > 0 {
			soloGroup := &TDGroup{
				Theme:       soloTheme,
				PathPattern: "HIGH/CRITICAL ungrouped items (run solo)",
				Items:       soloItems,
			}
			for _, item := range soloItems {
				soloGroup.TotalMinutes += extractEstMinutesInt(item)
			}
			groupMap[soloTheme] = soloGroup
		}
	}

	// Step 7: Add critical items as separate group(s)
	if len(criticalItems) > 0 {
		criticalGroup := &TDGroup{
			Theme:       "critical",
			PathPattern: "CRITICAL severity items",
			Items:       criticalItems,
		}
		for _, item := range criticalItems {
			criticalGroup.TotalMinutes += extractEstMinutesInt(item)
		}
		groupMap["critical"] = criticalGroup
	}

	// Step 8: Convert map to sorted slice
	groups := []TDGroup{}
	themeOrder := []string{}
	for theme := range groupMap {
		themeOrder = append(themeOrder, theme)
	}
	sort.Strings(themeOrder)

	// Order: solo first, then critical, then alphabetical
	finalOrder := []string{}
	for _, t := range themeOrder {
		if t == soloTheme || t == "critical" {
			continue
		}
		finalOrder = append(finalOrder, t)
	}
	// Prepend critical if present
	if _, ok := groupMap["critical"]; ok {
		finalOrder = append([]string{"critical"}, finalOrder...)
	}
	// Prepend solo if present
	if _, ok := groupMap[soloTheme]; ok {
		finalOrder = append([]string{soloTheme}, finalOrder...)
	}

	for _, theme := range finalOrder {
		g := groupMap[theme]
		g.Count = len(g.Items)
		groups = append(groups, *g)
	}

	// Step 9: Assign group numbers if requested
	if assignNumbers {
		num := 1
		if startNumber > 0 {
			num = startNumber
		}
		for i := range groups {
			if groups[i].Theme == soloTheme {
				groups[i].Number = 0
			} else {
				groups[i].Number = num
				num++
			}
			// Inject group label into each item
			label := fmt.Sprintf("%v", groups[i].Number)
			if groups[i].Theme == soloTheme {
				label = "Solo"
			}
			for j := range groups[i].Items {
				groups[i].Items[j]["GROUP"] = label
			}
		}
		// Label ungrouped items
		for j := range ungrouped {
			ungrouped[j]["GROUP"] = ungroupedLabel
		}
	}

	// Step 10: Build result
	groupedCount := 0
	for _, g := range groups {
		groupedCount += g.Count
	}

	return GroupTDResult{
		Groups:    groups,
		Ungrouped: ungrouped,
		Summary: GroupTDSummary{
			TotalItems:     len(items),
			GroupedCount:   groupedCount,
			UngroupedCount: len(ungrouped),
			GroupCount:     len(groups),
		},
	}
}

func extractTheme(item map[string]interface{}, groupBy string, pathDepth int, rootTheme string) string {
	switch groupBy {
	case groupByCategory:
		if cat, ok := item["CATEGORY"].(string); ok && cat != "" {
			return strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
		}
		return rootTheme

	case groupByFile:
		fileLine := extractFileLine(item)
		if fileLine == "" {
			return rootTheme
		}
		// Extract just the file path (no line number)
		path := strings.Split(fileLine, ":")[0]
		return strings.ReplaceAll(path, "/", "-")

	case groupByPath:
		fallthrough
	default:
		fileLine := extractFileLine(item)
		if fileLine == "" {
			// Fallback to category if no file
			if cat, ok := item["CATEGORY"].(string); ok && cat != "" {
				return strings.ToLower(strings.ReplaceAll(cat, " ", "-"))
			}
			return rootTheme
		}
		return extractPathTheme(fileLine, pathDepth, rootTheme)
	}
}

func extractFileLine(item map[string]interface{}) string {
	// Try FILE:LINE field (default in TD_STREAM format)
	if fl, ok := item["FILE:LINE"].(string); ok && fl != "" {
		return fl
	}
	// Try FILE_LINE field (legacy fallback)
	if fl, ok := item["FILE_LINE"].(string); ok && fl != "" {
		return fl
	}
	// Try FILE field
	if f, ok := item["FILE"].(string); ok && f != "" {
		return f
	}
	// Try PATH field
	if p, ok := item["PATH"].(string); ok && p != "" {
		return p
	}
	return ""
}

func extractPathTheme(fileLine string, depth int, rootTheme string) string {
	// Remove line number if present
	path := strings.Split(fileLine, ":")[0]

	// Normalize backslashes to forward slashes for cross-platform compatibility
	path = strings.ReplaceAll(path, "\\", "/")

	// Get directory part
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return rootTheme
	}

	// Split into segments
	// Use forward slash for consistency (handles any remaining backslashes)
	normalizedDir := strings.ReplaceAll(dir, "\\", "/")
	segments := strings.Split(normalizedDir, "/")

	// Filter out empty segments
	filteredSegments := []string{}
	for _, s := range segments {
		if s != "" && s != "." {
			filteredSegments = append(filteredSegments, s)
		}
	}

	if len(filteredSegments) == 0 {
		return rootTheme
	}

	// Take up to 'depth' segments
	if len(filteredSegments) > depth {
		filteredSegments = filteredSegments[:depth]
	}

	// Join with hyphen
	theme := strings.Join(filteredSegments, "-")
	return strings.ToLower(theme)
}

func buildPathPattern(theme string, groupBy string) string {
	if groupBy == groupByCategory {
		return fmt.Sprintf("CATEGORY=%s", theme)
	}
	if groupBy == groupByFile {
		return strings.ReplaceAll(theme, "-", "/")
	}
	// For path grouping, convert back to glob pattern
	return strings.ReplaceAll(theme, "-", "/") + "/*"
}

func extractEstMinutesInt(item map[string]interface{}) int {
	val, ok := item["EST_MINUTES"]
	if !ok {
		return 0
	}

	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return int(f)
		}
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return int(f)
		}
	}
	return 0
}

func printGroupTDText(w io.Writer, result GroupTDResult, minimal bool) {
	if minimal {
		// Just print group names and counts
		for _, g := range result.Groups {
			fmt.Fprintf(w, "%s: %d items (%d min)\n", g.Theme, g.Count, g.TotalMinutes)
		}
		if len(result.Ungrouped) > 0 {
			fmt.Fprintf(w, "ungrouped: %d items\n", len(result.Ungrouped))
		}
		return
	}

	// Full output
	fmt.Fprintf(w, "GROUPS (%d)\n", result.Summary.GroupCount)
	fmt.Fprintln(w, strings.Repeat("-", 50))

	for _, g := range result.Groups {
		fmt.Fprintf(w, "\n[%s] %d items, %d total minutes\n", g.Theme, g.Count, g.TotalMinutes)
		if g.PathPattern != "" {
			fmt.Fprintf(w, "  Pattern: %s\n", g.PathPattern)
		}
		fmt.Fprintln(w, "  Items:")
		for _, item := range g.Items {
			fileLine := extractFileLine(item)
			problem := ""
			if p, ok := item["PROBLEM"].(string); ok {
				problem = p
				if len(problem) > 50 {
					problem = problem[:50] + "..."
				}
			}
			if fileLine != "" {
				fmt.Fprintf(w, "    - %s: %s\n", fileLine, problem)
			} else {
				fmt.Fprintf(w, "    - %s\n", problem)
			}
		}
	}

	if len(result.Ungrouped) > 0 {
		fmt.Fprintf(w, "\nUNGROUPED (%d)\n", len(result.Ungrouped))
		fmt.Fprintln(w, strings.Repeat("-", 50))
		for _, item := range result.Ungrouped {
			fileLine := extractFileLine(item)
			problem := ""
			if p, ok := item["PROBLEM"].(string); ok {
				problem = p
				if len(problem) > 50 {
					problem = problem[:50] + "..."
				}
			}
			if fileLine != "" {
				fmt.Fprintf(w, "  - %s: %s\n", fileLine, problem)
			} else {
				fmt.Fprintf(w, "  - %s\n", problem)
			}
		}
	}

	fmt.Fprintf(w, "\nSUMMARY\n")
	fmt.Fprintln(w, strings.Repeat("-", 50))
	fmt.Fprintf(w, "Total items:   %d\n", result.Summary.TotalItems)
	fmt.Fprintf(w, "Grouped:       %d\n", result.Summary.GroupedCount)
	fmt.Fprintf(w, "Ungrouped:     %d\n", result.Summary.UngroupedCount)
	fmt.Fprintf(w, "Group count:   %d\n", result.Summary.GroupCount)
}

func writeGroupedMarkdown(result GroupTDResult, outputFile string, checkbox bool, sprintLabel, dateLabel string) error {
	// Read existing file if it exists
	existingContent := ""
	if data, err := os.ReadFile(outputFile); err == nil {
		existingContent = string(data)
	}

	// If file doesn't exist, create with header
	if existingContent == "" {
		existingContent = "# Technical Debt Backlog\n\nItems from code review. Use `/resolve-td --group=N` to fix by group.\nUse `/promote-tech-debt` to graduate items to formal TD sprint plans.\n"
	}

	// Build section header
	sectionHeader := "### "
	if dateLabel != "" {
		sectionHeader += "[" + dateLabel + "] "
	}
	if sprintLabel != "" {
		sectionHeader += "From Sprint: " + sprintLabel
	}
	if dateLabel == "" && sprintLabel == "" {
		sectionHeader += "Items"
	}

	// Feature-flag: emit trailing optional columns (SOURCE, REVIEWERS,
	// CONFIDENCE) only when at least one input row carries a non-empty
	// value. Preserves the original 7-column output for legacy callers and
	// the 8-column-with-Source output for the unified-TD-capture pipeline.
	hasField := func(key string) bool {
		for _, g := range result.Groups {
			for _, item := range g.Items {
				if s, _ := item[key].(string); strings.TrimSpace(s) != "" {
					return true
				}
			}
		}
		for _, item := range result.Ungrouped {
			if s, _ := item[key].(string); strings.TrimSpace(s) != "" {
				return true
			}
		}
		return false
	}
	hasSource := hasField("SOURCE")
	hasReviewers := hasField("REVIEWERS")
	hasConfidence := hasField("CONFIDENCE")

	// Build markdown table
	var buf strings.Builder
	buf.WriteString("\n" + sectionHeader + "\n\n")

	// Header is assembled dynamically so adding feature-flagged trailing
	// columns doesn't combinatorially explode the switch (3 flags = 8 cases).
	//
	// Cell rendering: data rows preserve the legacy `| %s |` spacing — empty
	// data cells render as `|  |` (two spaces around empty) so resolve-td and
	// other downstream column-position parsers see the same shape they've
	// always seen. The header row's empty checkbox cell renders as `| |`
	// (single space) to match the prior hardcoded header.
	writeHeaderRow := func(b *strings.Builder, cells []string) {
		b.WriteString("|")
		for _, c := range cells {
			if c == "" {
				b.WriteString(" |")
			} else {
				b.WriteString(" " + c + " |")
			}
		}
		b.WriteString("\n")
	}
	writeDataRow := func(b *strings.Builder, cells []string) {
		// Always `| <cell> |` — matches fmt.Sprintf("%s") behavior for empty.
		b.WriteString("|")
		for _, c := range cells {
			b.WriteString(" " + c + " |")
		}
		b.WriteString("\n")
	}
	headers := []string{"Group"}
	dashes := []string{"-------"}
	if checkbox {
		headers = append(headers, "")
		dashes = append(dashes, "---")
	}
	headers = append(headers, "Severity", "File", "Problem", "Fix", "Category", "Est Minutes")
	dashes = append(dashes, "----------", "------", "---------", "-----", "----------", "-------------")
	if hasSource {
		headers = append(headers, "Source")
		dashes = append(dashes, "--------")
	}
	if hasReviewers {
		headers = append(headers, "Reviewers")
		dashes = append(dashes, "---------")
	}
	if hasConfidence {
		headers = append(headers, "Confidence")
		dashes = append(dashes, "----------")
	}
	writeHeaderRow(&buf, headers)
	buf.WriteString("|" + strings.Join(dashes, "|") + "|\n")

	// Collect all rows: groups first, then ungrouped
	type rowData struct {
		group      string
		sortKey    int // 0=solo, 1..N=groups, 9999=ungrouped
		severity   string
		fileLine   string
		problem    string
		fix        string
		category   string
		estMin     int
		source     string
		reviewers  string
		confidence string
	}

	mkRow := func(groupLabel string, sortKey int, item map[string]interface{}) rowData {
		severity, _ := item["SEVERITY"].(string)
		problem, _ := item["PROBLEM"].(string)
		fix, _ := item["FIX"].(string)
		category, _ := item["CATEGORY"].(string)
		source, _ := item["SOURCE"].(string)
		reviewers, _ := item["REVIEWERS"].(string)
		confidence, _ := item["CONFIDENCE"].(string)
		// REVIEWERS is stored comma-joined (e.g. "bruce,greta"); render
		// with a space after each comma for readability in the table cell.
		reviewers = strings.ReplaceAll(reviewers, ",", ", ")
		return rowData{
			group:      groupLabel,
			sortKey:    sortKey,
			severity:   severity,
			fileLine:   extractFileLine(item),
			problem:    problem,
			fix:        fix,
			category:   category,
			estMin:     extractEstMinutesInt(item),
			source:     source,
			reviewers:  reviewers,
			confidence: confidence,
		}
	}

	var rows []rowData

	for _, g := range result.Groups {
		groupLabel := fmt.Sprintf("%v", g.Number)
		if g.Theme == soloTheme {
			groupLabel = "Solo"
		}
		sortKey := 9998
		if g.Theme == soloTheme {
			sortKey = 0
		} else if num, ok := g.Number.(int); ok {
			sortKey = num
		}
		for _, item := range g.Items {
			rows = append(rows, mkRow(groupLabel, sortKey, item))
		}
	}

	// Ungrouped items
	for _, item := range result.Ungrouped {
		rows = append(rows, mkRow(ungroupedLabel, 9999, item))
	}

	// Sort by sortKey (solo=0, groups=1..N, ungrouped=9999)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].sortKey < rows[j].sortKey
	})

	// Write rows — cells assembled to match the dynamic header.
	for _, r := range rows {
		cells := []string{r.group}
		if checkbox {
			cells = append(cells, "[ ]")
		}
		cells = append(cells, r.severity, r.fileLine, r.problem, r.fix, r.category, fmt.Sprintf("%d", r.estMin))
		if hasSource {
			cells = append(cells, r.source)
		}
		if hasReviewers {
			cells = append(cells, r.reviewers)
		}
		if hasConfidence {
			cells = append(cells, r.confidence)
		}
		writeDataRow(&buf, cells)
	}

	// Verify row count against buffer before writing
	bufContent := buf.String()
	bufLines := strings.Split(bufContent, "\n")
	tableRowCount := 0
	for _, line := range bufLines {
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") &&
			!strings.HasPrefix(line, "| Group") {
			tableRowCount++
		}
	}
	expectedRows := len(rows)
	if tableRowCount != expectedRows {
		return fmt.Errorf("row count mismatch: expected %d, written %d", expectedRows, tableRowCount)
	}

	// Insert new section after header block (newest first)
	// Header ends after the intro paragraph. Find first "### " section marker
	// to insert before it; if none exists, append to end.
	insertPos := len(existingContent)
	if idx := strings.Index(existingContent, "\n### "); idx >= 0 {
		insertPos = idx
	}

	fullContent := existingContent[:insertPos] + bufContent + existingContent[insertPos:]

	if err := os.WriteFile(outputFile, []byte(fullContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// scanExistingActiveGroupNumbers reads a README.md-style file, parses all
// markdown table rows across all sections, and returns the highest "active"
// group number found.
//
// A group is **active** if at least one of its rows is an unchecked checkbox
// (`[ ]`). Groups consisting entirely of `[x]` rows are inactive — completed
// work that won't be re-processed by /resolve-td, so their numbers are free
// for reuse by new sections.
//
// Solo, U, and other non-integer Group values are ignored. A missing file is
// not an error (returns 0, nil) — callers use the result + 1 as the starting
// number, so missing-file naturally produces "start at 1" for a fresh README.
//
// Caller uses scanExistingActiveGroupNumbers + 1 as the starting number for
// a new section's groups, achieving global uniqueness across all active
// sections in the file.
func scanExistingActiveGroupNumbers(path string) (int, error) {
	if path == "" {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	// activeGroups[n] = true if group n has at least one [ ] row.
	activeGroups := make(map[int]bool)

	for _, line := range strings.Split(string(data), "\n") {
		groupNum, isUnchecked, ok := parseTableRowForGroupState(line)
		if !ok {
			continue
		}
		if isUnchecked {
			activeGroups[groupNum] = true
		} else if _, seen := activeGroups[groupNum]; !seen {
			// First time we've seen this group AND the row is checked.
			// Record as inactive (false) so the max-scan below sees it.
			// We don't overwrite an existing `true` — once active, stays active.
			activeGroups[groupNum] = false
		}
	}

	max := 0
	for num, active := range activeGroups {
		if active && num > max {
			max = num
		}
	}
	return max, nil
}

// parseTableRowForGroupState parses one line of a markdown table row in the
// README format. Returns:
//
//	(groupNumber, isUnchecked, ok=true)  for a real numbered-group data row
//	(0, false, ok=false)                  for anything else (header, separator,
//	                                       Solo/U row, non-table line)
//
// Expected row shape:
//
//	| <group> | <checkbox> | <severity> | <file> | <problem> | ... |
//
// where <group> is an integer (we skip "Solo", "U", "Group"), and <checkbox>
// is "[ ]" or "[x]" / "[X]".
func parseTableRowForGroupState(line string) (int, bool, bool) {
	if !strings.HasPrefix(line, "|") {
		return 0, false, false
	}
	// Skip separator lines like "|---|---|..."
	if strings.Contains(line, "|---") || strings.Contains(line, "|------") {
		return 0, false, false
	}
	// Split on `|` and strip empty leading/trailing splits.
	cells := strings.Split(line, "|")
	if len(cells) < 4 { // need at least: "", group, checkbox, severity, ...
		return 0, false, false
	}
	// cells[0] is empty (leading "|"), cells[1] is group, cells[2] is checkbox
	group := strings.TrimSpace(cells[1])
	checkbox := strings.TrimSpace(cells[2])

	// Skip header row ("Group" / "-------")
	if group == "Group" || group == "" || strings.HasPrefix(group, "-") {
		return 0, false, false
	}

	// Try to parse as integer. Solo / U / anything non-integer is ignored.
	num, err := strconv.Atoi(group)
	if err != nil {
		return 0, false, false
	}

	isUnchecked := checkbox == "[ ]"
	return num, isUnchecked, true
}

// renumberActiveGroupsInPlace reads a README.md-style file, parses all
// sections, and rewrites the file so that every ACTIVE group (a group with
// at least one [ ] row anywhere in the file) has a globally-unique number.
//
// Algorithm:
//
//  1. First pass: identify every (section, oldGroupNum) tuple that's active.
//     A group is active if at least one of its rows in that section is `[ ]`.
//     Solo and U rows are not numbered groups and are left alone.
//  2. Second pass: assign new numbers 1, 2, 3, ... to active groups in
//     section-order, then within each section in first-appearance order.
//  3. Third pass: rewrite the file, replacing each active group's number in
//     its rows. Inactive groups (all [x]) keep their old numbers.
//
// Idempotent: re-running renumber on an already-renumbered file produces no
// change to active-group numbers (they're already in section-order, starting
// at 1 for the first active section).
func renumberActiveGroupsInPlace(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(string(data), "\n")

	// Parse: walk lines, find section boundaries, for each section determine
	// which (oldNum) groups are active within that section. Track the order
	// in which active groups first appear per section so we can renumber
	// deterministically.

	type sectionInfo struct {
		startLine     int          // index of `### ...` header
		endLine       int          // exclusive end (next section header or EOF)
		activeOldNums []int        // ordered, deduplicated old numbers of active groups
		activeOldSet  map[int]bool // membership check
	}
	var sections []*sectionInfo
	var cur *sectionInfo

	for i, line := range lines {
		if strings.HasPrefix(line, "### ") {
			if cur != nil {
				cur.endLine = i
				sections = append(sections, cur)
			}
			cur = &sectionInfo{startLine: i, activeOldSet: map[int]bool{}}
			continue
		}
		if cur == nil {
			continue
		}
		num, isUnchecked, ok := parseTableRowForGroupState(line)
		if !ok {
			continue
		}
		if isUnchecked {
			if !cur.activeOldSet[num] {
				cur.activeOldSet[num] = true
				cur.activeOldNums = append(cur.activeOldNums, num)
			}
		}
	}
	if cur != nil {
		cur.endLine = len(lines)
		sections = append(sections, cur)
	}

	// Build the per-section old→new mapping.
	// New numbers start at 1 and increment for every active group encountered,
	// in section-order then first-appearance order within section.
	nextNum := 1
	renumberMaps := make([]map[int]int, len(sections))
	for si, s := range sections {
		m := map[int]int{}
		for _, oldNum := range s.activeOldNums {
			m[oldNum] = nextNum
			nextNum++
		}
		renumberMaps[si] = m
	}

	// Rewrite lines.
	out := make([]string, len(lines))
	copy(out, lines)
	for si, s := range sections {
		m := renumberMaps[si]
		if len(m) == 0 {
			continue // no active groups in this section
		}
		for i := s.startLine; i < s.endLine; i++ {
			oldNum, isUnchecked, ok := parseTableRowForGroupState(out[i])
			if !ok {
				continue
			}
			// Only renumber rows that belong to a group we're remapping.
			// Inactive groups (all [x]) won't be in `m`. But we also need to
			// renumber [x] rows of groups that ARE active (so all rows in an
			// active group share the same new number).
			newNum, mapped := m[oldNum]
			if !mapped {
				// Group is inactive — leave the row alone.
				_ = isUnchecked // shut up unused-warning even though not used here
				continue
			}
			out[i] = replaceGroupNumberInRow(out[i], newNum)
		}
	}

	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// replaceGroupNumberInRow rewrites the first cell of a markdown table data
// row to contain the given integer. Preserves the rest of the row verbatim.
// Returns the line unchanged if it doesn't look like a numbered-group data
// row.
func replaceGroupNumberInRow(line string, newNum int) string {
	if _, _, ok := parseTableRowForGroupState(line); !ok {
		return line
	}
	// Find the first "|" and the second "|"; replace the cell between them.
	first := strings.Index(line, "|")
	if first < 0 {
		return line
	}
	second := strings.Index(line[first+1:], "|")
	if second < 0 {
		return line
	}
	second += first + 1
	// Reconstruct: keep prefix up to first "|" + " <newNum> " + rest from second "|"
	return line[:first] + "| " + strconv.Itoa(newNum) + " " + line[second:]
}
