package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	highestPath    string
	highestPaths   []string
	highestPattern string
	highestType    string
	highestPrefix  string
	highestJSON    bool
	highestMinimal bool
)

// HighestResult represents the output of the highest command
type HighestResult struct {
	Highest  string `json:"highest"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
	Next     string `json:"next"`
	Count    int    `json:"count"`
}

// defaultPatterns maps directory context to default regex patterns
var defaultPatterns = map[string]string{
	"plans":               `^(\d+)\.(\d+)[-_]`,
	"sprints":             `^(\d+)\.(\d+)[-_]`,
	"user-stories":        `^(\d+)[-_]`,
	"acceptance-criteria": `^(\d+)[-_](\d+)[-_]`,
	"tasks":               `^(?:task[-_])?(\d+)[-_]`,
	"technical-debt":      `(?i)^td[-_](\d+)[-_]`,
}

// newHighestCmd creates the highest command
func newHighestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "highest",
		Short: "Find highest numbered directory or file",
		Long: `Find the highest numbered directory or file matching a pattern.

Useful for determining the next plan number, sprint number, user story, etc.

Default patterns by directory context:
  plans/sprints:        ^(\d+)\.(\d+)[-_]   → extracts "115.0"
  user-stories:         ^(\d+)[-_]          → extracts "01"
  acceptance-criteria:  ^(\d+)[-_](\d+)[-_] → extracts "01-01"
  tasks:                ^(?:task[-_])?(\d+)[-_] → extracts "01"
  technical-debt:       (?i)^td[-_](\d+)[-_]    → extracts "22"

Both plans and sprints support active/pending/completed subdirectories:
  .planning/plans/active/      - plans currently being worked on
  .planning/plans/pending/     - plans awaiting implementation
  .planning/plans/completed/   - finished plans
  .planning/sprints/active/    - sprints currently being executed
  .planning/sprints/pending/   - sprints awaiting execution
  .planning/sprints/completed/ - finished sprints

Use --paths to search multiple directories and find the global highest:
  llm-support highest --paths .planning/plans/active,.planning/plans/completed

Examples:
  llm-support highest --path .planning/plans/active
  llm-support highest --paths .planning/plans/active,.planning/plans/completed --type dir
  llm-support highest --path .planning/sprints/active --type dir
  llm-support highest --path .planning/plans/active/115.0-feature/user-stories
  llm-support highest --path .planning/plans/active/115.0-feature/acceptance-criteria --prefix "01-"`,
		RunE: runHighest,
	}
	cmd.Flags().StringVar(&highestPath, "path", ".", "Directory to search in")
	cmd.Flags().StringSliceVar(&highestPaths, "paths", nil, "Multiple directories to search (comma-separated)")
	cmd.Flags().StringVar(&highestPattern, "pattern", "", "Custom regex pattern (auto-detected if not provided)")
	cmd.Flags().StringVar(&highestType, "type", "both", "Type to search: dir, file, both")
	cmd.Flags().StringVar(&highestPrefix, "prefix", "", "Filter to items starting with this prefix")
	cmd.Flags().BoolVar(&highestJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&highestMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runHighest(cmd *cobra.Command, args []string) error {
	// Collect search paths
	var searchPaths []string
	if len(highestPaths) > 0 {
		for _, p := range highestPaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				return fmt.Errorf("invalid path: %s", p)
			}
			searchPaths = append(searchPaths, absPath)
		}
	} else {
		absPath, err := filepath.Abs(highestPath)
		if err != nil {
			return fmt.Errorf("invalid path: %s", highestPath)
		}
		searchPaths = []string{absPath}
	}

	// Validate all paths exist and are directories
	for _, searchPath := range searchPaths {
		info, err := os.Stat(searchPath)
		if err != nil {
			return fmt.Errorf("path does not exist: %s", searchPath)
		}
		if !info.IsDir() {
			return fmt.Errorf("path must be a directory: %s", searchPath)
		}
	}

	// Determine pattern (use first path for context detection)
	pattern := highestPattern
	if pattern == "" {
		pattern = detectPattern(searchPaths[0], highestPrefix)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %s", pattern)
	}

	// Extract version numbers from all paths
	type versionedEntry struct {
		name     string
		fullPath string
		version  string
		sortKey  float64
	}

	var versioned []versionedEntry

	for _, searchPath := range searchPaths {
		// Read directory entries
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %w", searchPath, err)
		}

		// Filter by type
		var candidates []os.DirEntry
		for _, e := range entries {
			switch highestType {
			case "dir":
				if e.IsDir() {
					candidates = append(candidates, e)
				}
			case "file":
				if !e.IsDir() {
					candidates = append(candidates, e)
				}
			default: // "both"
				candidates = append(candidates, e)
			}
		}

		// Filter by prefix if specified
		if highestPrefix != "" {
			var filtered []os.DirEntry
			for _, e := range candidates {
				if strings.HasPrefix(e.Name(), highestPrefix) {
					filtered = append(filtered, e)
				}
			}
			candidates = filtered
		}

		// Extract versions from this path's candidates
		for _, e := range candidates {
			name := e.Name()
			// If prefix is specified, strip it before matching
			matchName := name
			if highestPrefix != "" {
				matchName = strings.TrimPrefix(name, highestPrefix)
			}

			matches := re.FindStringSubmatch(matchName)
			if matches == nil {
				// Try matching the full name if prefix stripping didn't help
				matches = re.FindStringSubmatch(name)
				if matches == nil {
					continue
				}
			}

			// Build version string and sort key from captured groups
			version, sortKey := buildVersionInfo(matches)
			versioned = append(versioned, versionedEntry{
				name:     name,
				fullPath: filepath.Join(searchPath, name),
				version:  version,
				sortKey:  sortKey,
			})
		}
	}

	if len(versioned) == 0 {
		result := HighestResult{
			Highest:  "",
			Name:     "",
			FullPath: "",
			Next:     "1",
			Count:    0,
		}
		formatter := output.New(highestJSON, highestMinimal, cmd.OutOrStdout())
		return formatter.Print(result, func(w io.Writer, data interface{}) {
			printHighestResult(w, data.(HighestResult))
		})
	}

	// Sort by version (descending)
	sort.Slice(versioned, func(i, j int) bool {
		return versioned[i].sortKey > versioned[j].sortKey
	})

	highest := versioned[0]
	next := calculateNext(highest.version, highestPrefix)

	result := HighestResult{
		Highest:  highest.version,
		Name:     highest.name,
		FullPath: highest.fullPath,
		Next:     next,
		Count:    len(versioned),
	}
	formatter := output.New(highestJSON, highestMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		printHighestResult(w, data.(HighestResult))
	})
}

func printHighestResult(w io.Writer, r HighestResult) {
	fmt.Fprintf(w, "HIGHEST: %s\n", r.Highest)
	fmt.Fprintf(w, "NAME: %s\n", r.Name)
	fmt.Fprintf(w, "FULL_PATH: %s\n", r.FullPath)
	fmt.Fprintf(w, "NEXT: %s\n", r.Next)
	fmt.Fprintf(w, "COUNT: %d\n", r.Count)
}

// detectPattern determines the best pattern based on directory path context
func detectPattern(path string, prefix string) string {
	// Use the final directory name for context detection (more specific)
	dirName := strings.ToLower(filepath.Base(path))

	// If prefix is provided for acceptance-criteria, use simpler pattern
	if prefix != "" && dirName == "acceptance-criteria" {
		return `^(\d+)[-_]`
	}

	// Check directory name first (most specific)
	if pattern, ok := defaultPatterns[dirName]; ok {
		return pattern
	}

	// Check if directory name starts with any known context
	// (handles cases like "tasks" being part of path)
	for context, pattern := range defaultPatterns {
		if strings.HasPrefix(dirName, context) {
			return pattern
		}
	}

	// Fall back to checking the full path (for nested structures)
	pathLower := strings.ToLower(path)
	// Check more specific patterns first
	specificOrder := []string{"acceptance-criteria", "user-stories", "technical-debt", "tasks", "sprints", "plans"}
	for _, context := range specificOrder {
		if strings.Contains(pathLower, context) {
			return defaultPatterns[context]
		}
	}

	// Fallback: any leading number
	return `^(\d+)[-_.]`
}

// buildVersionInfo extracts version string and sortable key from regex matches
func buildVersionInfo(matches []string) (string, float64) {
	if len(matches) < 2 {
		return "", 0
	}

	// Single capture group
	if len(matches) == 2 {
		v, _ := strconv.ParseFloat(matches[1], 64)
		return matches[1], v
	}

	// Two capture groups (e.g., "115.0" or "01-02")
	if len(matches) >= 3 {
		major, _ := strconv.ParseFloat(matches[1], 64)
		minor, _ := strconv.ParseFloat(matches[2], 64)

		// Check the full match (matches[0]) to determine the separator used
		// If it contains a literal dot between the numbers, it's a decimal version
		// Otherwise it's a compound version (using - or _)
		fullMatch := matches[0]
		dotPattern := regexp.MustCompile(`^\d+\.\d+`)
		if dotPattern.MatchString(fullMatch) {
			// Decimal version like 115.0
			version := fmt.Sprintf("%s.%s", matches[1], matches[2])
			sortKey := major + minor/10.0
			return version, sortKey
		} else {
			// Compound version like 01-01 (uses - or _ separator)
			version := fmt.Sprintf("%s-%s", matches[1], matches[2])
			sortKey := major*1000 + minor
			return version, sortKey
		}
	}

	return matches[1], 0
}

// calculateNext determines the next number in sequence
func calculateNext(version string, prefix string) string {
	// Handle decimal versions (e.g., "115.0" -> "116.0")
	if strings.Contains(version, ".") {
		parts := strings.Split(version, ".")
		if len(parts) == 2 {
			major, err := strconv.Atoi(parts[0])
			if err == nil {
				return fmt.Sprintf("%d.0", major+1)
			}
		}
	}

	// Handle compound versions with prefix context (e.g., prefix="01-", version="03" -> "04")
	if prefix != "" && !strings.Contains(version, "-") {
		v, err := strconv.Atoi(version)
		if err == nil {
			return fmt.Sprintf("%02d", v+1)
		}
	}

	// Handle compound versions (e.g., "01-03" -> keep story, increment AC is context-specific)
	// For now, just increment the last number
	if strings.Contains(version, "-") {
		parts := strings.Split(version, "-")
		if len(parts) == 2 {
			// If we have a prefix, this is the second number (AC within story)
			if prefix != "" {
				v, err := strconv.Atoi(parts[1])
				if err == nil {
					return fmt.Sprintf("%02d", v+1)
				}
			}
			// Otherwise increment the compound
			major, _ := strconv.Atoi(parts[0])
			minor, err := strconv.Atoi(parts[1])
			if err == nil {
				return fmt.Sprintf("%02d-%02d", major, minor+1)
			}
		}
	}

	// Simple integer increment
	v, err := strconv.Atoi(version)
	if err == nil {
		// Preserve leading zeros
		width := len(version)
		return fmt.Sprintf("%0*d", width, v+1)
	}

	return version
}

func init() {
	RootCmd.AddCommand(newHighestCmd())
}
