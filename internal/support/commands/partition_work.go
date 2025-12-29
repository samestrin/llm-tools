package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	partitionStories string
	partitionTasks   string
	partitionJSON    bool
	partitionMinimal bool
	partitionVerbose bool
)

// PartitionGroup represents a group of items
type PartitionGroup struct {
	ID    int      `json:"id,omitempty"`
	I     *int     `json:"i,omitempty"`
	Items []string `json:"items,omitempty"`
	It    []string `json:"it,omitempty"`
}

// PartitionResult represents the partition result
type PartitionResult struct {
	Groups        []PartitionGroup `json:"groups,omitempty"`
	G             []PartitionGroup `json:"g,omitempty"`
	TotalGroups   int              `json:"total_groups,omitempty"`
	TG            *int             `json:"tg,omitempty"`
	ItemsPerGroup []int            `json:"items_per_group,omitempty"`
	IPG           []int            `json:"ipg,omitempty"`
	Message       string           `json:"message,omitempty"`
	M             string           `json:"m,omitempty"`
}

// newPartitionWorkCmd creates the partition-work command
func newPartitionWorkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partition-work",
		Short: "Partition work items into parallel groups",
		Long: `Partition work items (stories/tasks) into parallel groups using graph coloring.

Items that share file dependencies cannot run in parallel and are placed in different groups.
Items with no conflicts can run together in the same group.`,
		RunE: runPartitionWork,
	}

	cmd.Flags().StringVar(&partitionStories, "stories", "", "Directory containing story markdown files")
	cmd.Flags().StringVar(&partitionTasks, "tasks", "", "Directory containing task markdown files")
	cmd.Flags().BoolVar(&partitionJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&partitionMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.Flags().BoolVar(&partitionVerbose, "verbose", false, "Show conflict details")

	return cmd
}

func runPartitionWork(cmd *cobra.Command, args []string) error {
	if partitionStories == "" && partitionTasks == "" {
		return fmt.Errorf("must specify either --stories or --tasks")
	}
	if partitionStories != "" && partitionTasks != "" {
		return fmt.Errorf("cannot specify both --stories and --tasks")
	}

	dirPath := partitionStories
	if dirPath == "" {
		dirPath = partitionTasks
	}

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("directory not found: %s", absPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absPath)
	}

	// Find markdown files
	var mdFiles []string
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdFiles = append(mdFiles, entry.Name())
		}
	}
	sort.Strings(mdFiles)

	if len(mdFiles) == 0 {
		zeroGroups := 0
		var result PartitionResult
		if partitionMinimal {
			result = PartitionResult{
				G:   []PartitionGroup{},
				TG:  &zeroGroups,
				IPG: []int{},
				M:   "No items found to partition",
			}
		} else {
			result = PartitionResult{
				Groups:        []PartitionGroup{},
				TotalGroups:   0,
				ItemsPerGroup: []int{},
				Message:       "No items found to partition",
			}
		}
		formatter := output.New(partitionJSON, partitionMinimal, cmd.OutOrStdout())
		return formatter.Print(result, func(w io.Writer, data interface{}) {
			fmt.Fprintln(w, "No items found to partition")
		})
	}

	// Extract dependencies for each file
	items := make(map[string][]string)
	for _, mdFile := range mdFiles {
		content, err := os.ReadFile(filepath.Join(absPath, mdFile))
		if err != nil {
			if partitionVerbose {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Could not read %s: %v\n", mdFile, err)
			}
			continue
		}
		deps := extractFileDeps(string(content))
		items[mdFile] = deps
	}

	if len(items) == 0 {
		return fmt.Errorf("no valid items found")
	}

	// Build conflict graph
	conflicts := buildConflictGraph(items)

	if partitionVerbose {
		fmt.Fprintln(cmd.OutOrStdout(), "=== Conflict Graph ===")
		for item, neighbors := range conflicts {
			if len(neighbors) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s conflicts with: %s\n", item, strings.Join(neighbors, ", "))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: no conflicts\n", item)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Apply greedy coloring
	colors := greedyColoring(conflicts)

	// Group by color
	groups := make(map[int][]string)
	for item, color := range colors {
		groups[color] = append(groups[color], item)
	}

	// Sort items within each group
	for color := range groups {
		sort.Strings(groups[color])
	}

	// Build result
	var sortedColors []int
	for c := range groups {
		sortedColors = append(sortedColors, c)
	}
	sort.Ints(sortedColors)

	itemsPerGroup := []int{}
	totalGroups := len(groups)

	var result PartitionResult
	if partitionMinimal {
		groupsList := []PartitionGroup{}
		for _, color := range sortedColors {
			colorVal := color
			groupsList = append(groupsList, PartitionGroup{
				I:  &colorVal,
				It: groups[color],
			})
			itemsPerGroup = append(itemsPerGroup, len(groups[color]))
		}
		result = PartitionResult{
			G:   groupsList,
			TG:  &totalGroups,
			IPG: itemsPerGroup,
		}
	} else {
		groupsList := []PartitionGroup{}
		for _, color := range sortedColors {
			groupsList = append(groupsList, PartitionGroup{
				ID:    color,
				Items: groups[color],
			})
			itemsPerGroup = append(itemsPerGroup, len(groups[color]))
		}
		result = PartitionResult{
			Groups:        groupsList,
			TotalGroups:   totalGroups,
			ItemsPerGroup: itemsPerGroup,
		}
	}

	formatter := output.New(partitionJSON, partitionMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if len(groups) == 1 {
			fmt.Fprintln(w, "All items are independent - can run in parallel")
		} else if len(groups) == len(items) {
			fmt.Fprintln(w, "Maximum conflicts detected - items must run sequentially")
		}

		fmt.Fprintln(w)

		for _, color := range sortedColors {
			fmt.Fprintf(w, "Group %d: %s\n", color, strings.Join(groups[color], ", "))
		}
	})
}

func extractFileDeps(content string) []string {
	deps := make(map[string]bool)

	// Pattern for backtick file references
	backtickPattern := regexp.MustCompile("`([^`]+\\.[a-zA-Z]{1,10})`")
	matches := backtickPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if isValidFilePath(match[1]) {
			deps[match[1]] = true
		}
	}

	result := make([]string, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	sort.Strings(result)
	return result
}

func buildConflictGraph(items map[string][]string) map[string][]string {
	conflicts := make(map[string][]string)

	// Initialize all items
	for item := range items {
		conflicts[item] = []string{}
	}

	// Find conflicts (shared dependencies)
	itemNames := make([]string, 0, len(items))
	for name := range items {
		itemNames = append(itemNames, name)
	}
	sort.Strings(itemNames)

	for i := 0; i < len(itemNames); i++ {
		for j := i + 1; j < len(itemNames); j++ {
			item1, item2 := itemNames[i], itemNames[j]
			deps1, deps2 := items[item1], items[item2]

			// Check for shared dependencies
			if hasOverlap(deps1, deps2) {
				conflicts[item1] = append(conflicts[item1], item2)
				conflicts[item2] = append(conflicts[item2], item1)
			}
		}
	}

	return conflicts
}

func hasOverlap(a, b []string) bool {
	set := make(map[string]bool)
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		if set[s] {
			return true
		}
	}
	return false
}

func greedyColoring(graph map[string][]string) map[string]int {
	colors := make(map[string]int)

	// Sort nodes for deterministic results
	nodes := make([]string, 0, len(graph))
	for node := range graph {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	for _, node := range nodes {
		// Find colors used by neighbors
		usedColors := make(map[int]bool)
		for _, neighbor := range graph[node] {
			if color, ok := colors[neighbor]; ok {
				usedColors[color] = true
			}
		}

		// Find smallest available color
		color := 0
		for usedColors[color] {
			color++
		}
		colors[node] = color
	}

	return colors
}

func init() {
	RootCmd.AddCommand(newPartitionWorkCmd())
}
