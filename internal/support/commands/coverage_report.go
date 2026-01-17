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
	coverageReportRequirements string
	coverageReportStories      string
	coverageReportJSON         bool
	coverageReportMinimal      bool

	// Regex patterns for requirement IDs
	// Matches: REQ-#, R-#, REQUIREMENT-# (case insensitive)
	requirementIDPattern = regexp.MustCompile(`(?i)\b(REQ-\d+|R-\d+|REQUIREMENT-\d+)\b`)
)

// CoverageReportResult holds the coverage analysis results
type CoverageReportResult struct {
	TotalRequirements     int                 `json:"total_requirements"`
	CoveredCount          int                 `json:"covered_count"`
	UncoveredRequirements []string            `json:"uncovered_requirements"`
	CoveragePercentage    float64             `json:"coverage_percentage"`
	CoverageByStory       map[string][]string `json:"coverage_by_story"`
	RequirementsFile      string              `json:"requirements_file"`
	StoriesDirectory      string              `json:"stories_directory"`
	ReadErrors            []string            `json:"read_errors,omitempty"`
}

// newCoverageReportCmd creates the coverage-report command
func newCoverageReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage-report",
		Short: "Calculate requirement coverage from user stories",
		Long: `Analyze requirement coverage by parsing requirements file and user stories.

Parses the requirements file to extract requirement IDs (REQ-#, R-#, REQUIREMENT-#),
then scans user story markdown files to find which requirements are referenced.

Output includes:
  - Total requirement count
  - Covered/uncovered requirement lists
  - Coverage percentage
  - Per-story coverage mapping

Examples:
  llm-support coverage-report --requirements ./original-requirements.md --stories ./user-stories/
  llm-support coverage-report --requirements ./reqs.md --stories ./stories/ --json`,
		RunE: runCoverageReport,
	}

	cmd.Flags().StringVar(&coverageReportRequirements, "requirements", "", "Path to requirements markdown file (required)")
	cmd.Flags().StringVar(&coverageReportStories, "stories", "", "Path to user stories directory (required)")
	cmd.Flags().BoolVar(&coverageReportJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&coverageReportMinimal, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("requirements")
	cmd.MarkFlagRequired("stories")

	return cmd
}

func runCoverageReport(cmd *cobra.Command, args []string) error {
	// Validate requirements file exists
	if _, err := os.Stat(coverageReportRequirements); os.IsNotExist(err) {
		return fmt.Errorf("requirements file not found: %s", coverageReportRequirements)
	}

	// Validate stories directory exists
	info, err := os.Stat(coverageReportStories)
	if os.IsNotExist(err) {
		return fmt.Errorf("stories directory not found: %s", coverageReportStories)
	}
	if !info.IsDir() {
		return fmt.Errorf("stories path is not a directory: %s", coverageReportStories)
	}

	// Parse requirements from file
	requirements, err := extractRequirementsFromFile(coverageReportRequirements)
	if err != nil {
		return fmt.Errorf("failed to parse requirements: %w", err)
	}

	// Scan user stories for coverage
	coverageByStory, readErrors, err := scanStoriesForCoverage(coverageReportStories, requirements)
	if err != nil {
		return fmt.Errorf("failed to scan stories: %w", err)
	}

	// Calculate coverage
	result := calculateCoverage(requirements, coverageByStory, readErrors, coverageReportRequirements, coverageReportStories)

	formatter := output.New(coverageReportJSON, coverageReportMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(CoverageReportResult)
		fmt.Fprintf(w, "COVERAGE_REPORT:\n")
		fmt.Fprintf(w, "  Total Requirements: %d\n", r.TotalRequirements)
		fmt.Fprintf(w, "  Covered: %d\n", r.CoveredCount)
		fmt.Fprintf(w, "  Uncovered: %d\n", len(r.UncoveredRequirements))
		fmt.Fprintf(w, "  Coverage: %.1f%%\n", r.CoveragePercentage)
		if len(r.UncoveredRequirements) > 0 {
			fmt.Fprintf(w, "  Uncovered Requirements:\n")
			for _, req := range r.UncoveredRequirements {
				fmt.Fprintf(w, "    - %s\n", req)
			}
		}
		if len(r.ReadErrors) > 0 {
			fmt.Fprintf(w, "  Read Errors: %d\n", len(r.ReadErrors))
			for _, e := range r.ReadErrors {
				fmt.Fprintf(w, "    - %s\n", e)
			}
		}
	})
}

// extractRequirementsFromFile parses a markdown file and extracts requirement IDs
func extractRequirementsFromFile(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	matches := requirementIDPattern.FindAllString(string(content), -1)

	// Deduplicate and normalize to uppercase
	seen := make(map[string]bool)
	var requirements []string
	for _, match := range matches {
		upper := strings.ToUpper(match)
		if !seen[upper] {
			seen[upper] = true
			requirements = append(requirements, upper)
		}
	}

	// Sort for consistent output
	sort.Strings(requirements)
	return requirements, nil
}

// scanStoriesForCoverage scans markdown files in a directory for requirement references
func scanStoriesForCoverage(storiesDir string, requirements []string) (map[string][]string, []string, error) {
	coverageByStory := make(map[string][]string)
	var readErrors []string

	// Create a set of requirements for quick lookup
	reqSet := make(map[string]bool)
	for _, req := range requirements {
		reqSet[strings.ToUpper(req)] = true
	}

	entries, err := os.ReadDir(storiesDir)
	if err != nil {
		return nil, nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process markdown files
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		filePath := filepath.Join(storiesDir, name)
		content, err := os.ReadFile(filePath)
		if err != nil {
			readErrors = append(readErrors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		// Find all requirement references in this story
		matches := requirementIDPattern.FindAllString(string(content), -1)

		// Deduplicate and filter to known requirements
		seen := make(map[string]bool)
		var storyReqs []string
		for _, match := range matches {
			upper := strings.ToUpper(match)
			if reqSet[upper] && !seen[upper] {
				seen[upper] = true
				storyReqs = append(storyReqs, upper)
			}
		}

		if len(storyReqs) > 0 {
			sort.Strings(storyReqs)
			coverageByStory[name] = storyReqs
		}
	}

	return coverageByStory, readErrors, nil
}

// calculateCoverage computes coverage metrics from requirements and story coverage
func calculateCoverage(requirements []string, coverageByStory map[string][]string, readErrors []string, reqsFile, storiesDir string) CoverageReportResult {
	// Build set of covered requirements
	covered := make(map[string]bool)
	for _, reqs := range coverageByStory {
		for _, req := range reqs {
			covered[req] = true
		}
	}

	// Find uncovered requirements
	uncovered := []string{}
	for _, req := range requirements {
		if !covered[req] {
			uncovered = append(uncovered, req)
		}
	}

	// Calculate percentage
	var percentage float64
	if len(requirements) > 0 {
		percentage = float64(len(covered)) / float64(len(requirements)) * 100.0
	}

	return CoverageReportResult{
		TotalRequirements:     len(requirements),
		CoveredCount:          len(covered),
		UncoveredRequirements: uncovered,
		CoveragePercentage:    percentage,
		CoverageByStory:       coverageByStory,
		RequirementsFile:      reqsFile,
		StoriesDirectory:      storiesDir,
		ReadErrors:            readErrors,
	}
}

func init() {
	RootCmd.AddCommand(newCoverageReportCmd())
}
