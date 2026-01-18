package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
)

// Constants
const (
	groupTDMaxInputSize    = 10 * 1024 * 1024 // 10MB
	groupByPath            = "path"
	groupByCategory        = "category"
	groupByFile            = "file"
	defaultPathDepth       = 2
	defaultMinGroupSize    = 3
	defaultRootTheme       = "misc"
	criticalSeverity       = "CRITICAL"
)

// GroupTDInput represents the input format
type GroupTDInput struct {
	Items []map[string]interface{} `json:"items"`
	Rows  []map[string]interface{} `json:"rows"`
}

// GroupTDResult represents the output
type GroupTDResult struct {
	Groups    []TDGroup       `json:"groups"`
	Ungrouped []map[string]interface{} `json:"ungrouped"`
	Summary   GroupTDSummary  `json:"summary"`
}

// TDGroup represents a group of related TD items
type TDGroup struct {
	Theme        string                   `json:"theme"`
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

	return cmd
}

func init() {
	RootCmd.AddCommand(newGroupTDCmd())
}

func runGroupTD(cmd *cobra.Command, args []string) error {
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
	items, err := parseGroupTDInput(input)
	if err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	// Group items
	result := groupItems(items, groupTDGroupBy, groupTDPathDepth, groupTDMinGroupSize, groupTDCriticalOverride, groupTDRootTheme)

	// Validate no data loss
	totalOutput := result.Summary.GroupedCount + result.Summary.UngroupedCount
	if totalOutput != len(items) {
		return fmt.Errorf("FATAL: Data loss detected - input: %d, output: %d", len(items), totalOutput)
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

func parseGroupTDInput(input string) ([]map[string]interface{}, error) {
	// Try parsing with items/rows wrapper
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

func groupItems(items []map[string]interface{}, groupBy string, pathDepth, minGroupSize int, criticalOverride bool, rootTheme string) GroupTDResult {
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

	// Step 6: Add critical items as separate group(s)
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

	// Step 7: Convert map to sorted slice
	groups := []TDGroup{}
	themeOrder := []string{}
	for theme := range groupMap {
		themeOrder = append(themeOrder, theme)
	}
	sort.Strings(themeOrder)

	// Put critical first if present
	finalOrder := []string{}
	for _, t := range themeOrder {
		if t == "critical" {
			finalOrder = append([]string{"critical"}, finalOrder...)
		} else {
			finalOrder = append(finalOrder, t)
		}
	}

	for _, theme := range finalOrder {
		g := groupMap[theme]
		g.Count = len(g.Items)
		groups = append(groups, *g)
	}

	// Step 8: Build result
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
	// Try FILE_LINE field
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
