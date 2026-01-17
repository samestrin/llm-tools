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
	validateRisksDesign  string
	validateRisksStories string
	validateRisksTasks   string
	validateRisksAC      string
	validateRisksJSON    bool
	validateRisksMinimal bool

	// Risk ID pattern matches R-1, R-2, R1, R2, Risk 1, Risk 2, etc.
	riskIDPattern = regexp.MustCompile(`(?i)\b(?:R-?\d+|Risk\s*\d+)\b`)
	// Pattern to extract just the number from a risk ID
	riskNumberPattern = regexp.MustCompile(`(\d+)`)
	// Risk section heading patterns
	riskSectionPattern = regexp.MustCompile(`(?i)^#+\s*(Risk\s*Analysis|Potential\s*Risks)\s*$`)
	// Table row pattern for risk extraction - supports R-1, R1, Risk 1 formats
	riskTableRowPattern = regexp.MustCompile(`(?i)\|\s*(R-?\d+|Risk\s*\d+)[:\s]*([^|]+)`)
	// List format pattern for risks - supports R-1, R1, Risk 1 formats
	riskListPattern = regexp.MustCompile(`(?i)^[\s]*[-*+]\s+\*{0,2}(R-?\d+|Risk\s*\d+)\*{0,2}[:\s]*(.*)`)
)

// RiskDetail holds information about a single risk
type RiskDetail struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Covered     bool     `json:"covered"`
	CoveredBy   []string `json:"covered_by,omitempty"`
}

// ValidateRisksResult holds the risk validation results
type ValidateRisksResult struct {
	RisksIdentified    int          `json:"risks_identified"`
	RisksAddressed     int          `json:"risks_addressed"`
	RisksUnaddressed   []string     `json:"risks_unaddressed"`
	CoveragePercentage float64      `json:"coverage_percentage"`
	RiskDetails        []RiskDetail `json:"risk_details"`
	DesignFile         string       `json:"design_file"`
	ReadErrors         []string     `json:"read_errors,omitempty"`
}

// normalizeRiskID converts various risk ID formats to standard R-N format
// Supports: R-1, R1, Risk 1, risk-1, RISK 1, etc.
func normalizeRiskID(id string) string {
	match := riskNumberPattern.FindString(id)
	if match == "" {
		return strings.ToUpper(id)
	}
	return "R-" + match
}

// newValidateRisksCmd creates the validate-risks command
func newValidateRisksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-risks",
		Short: "Validate risk coverage from sprint design",
		Long: `Cross-reference sprint-design.md risks with user stories, tasks, or acceptance criteria.

Parses the Risk Analysis section from sprint-design.md and checks if each risk
is addressed in the work items (user stories, tasks, or acceptance criteria).

Output includes:
  - Total risks identified
  - Risks addressed vs unaddressed
  - Coverage percentage
  - Per-risk coverage details

Examples:
  llm-support validate-risks --design ./sprint-design.md --stories ./user-stories/
  llm-support validate-risks --design ./sprint-design.md --tasks ./tasks/
  llm-support validate-risks --design ./sprint-design.md --stories ./user-stories/ --acceptance-criteria ./acceptance-criteria/`,
		RunE: runValidateRisks,
	}

	cmd.Flags().StringVar(&validateRisksDesign, "design", "", "Path to sprint-design.md file (required)")
	cmd.Flags().StringVar(&validateRisksStories, "stories", "", "Path to user stories directory")
	cmd.Flags().StringVar(&validateRisksTasks, "tasks", "", "Path to tasks directory")
	cmd.Flags().StringVar(&validateRisksAC, "acceptance-criteria", "", "Path to acceptance criteria directory")
	cmd.Flags().BoolVar(&validateRisksJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&validateRisksMinimal, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("design")

	return cmd
}

func runValidateRisks(cmd *cobra.Command, args []string) error {
	// Validate design file exists
	if _, err := os.Stat(validateRisksDesign); os.IsNotExist(err) {
		return fmt.Errorf("sprint design file not found: %s", validateRisksDesign)
	}

	// Validate at least one work items directory is provided
	if validateRisksStories == "" && validateRisksTasks == "" {
		return fmt.Errorf("at least one of --stories or --tasks is required")
	}

	// Validate stories directory if provided
	if validateRisksStories != "" {
		info, err := os.Stat(validateRisksStories)
		if os.IsNotExist(err) {
			return fmt.Errorf("stories directory not found: %s", validateRisksStories)
		}
		if !info.IsDir() {
			return fmt.Errorf("stories path is not a directory: %s", validateRisksStories)
		}
	}

	// Validate tasks directory if provided
	if validateRisksTasks != "" {
		info, err := os.Stat(validateRisksTasks)
		if os.IsNotExist(err) {
			return fmt.Errorf("tasks directory not found: %s", validateRisksTasks)
		}
		if !info.IsDir() {
			return fmt.Errorf("tasks path is not a directory: %s", validateRisksTasks)
		}
	}

	// Validate AC directory if provided
	if validateRisksAC != "" {
		info, err := os.Stat(validateRisksAC)
		if os.IsNotExist(err) {
			return fmt.Errorf("acceptance-criteria directory not found: %s", validateRisksAC)
		}
		if !info.IsDir() {
			return fmt.Errorf("acceptance-criteria path is not a directory: %s", validateRisksAC)
		}
	}

	// Parse risks from design file
	risks, err := parseRisksFromDesign(validateRisksDesign)
	if err != nil {
		return fmt.Errorf("failed to parse risks: %w", err)
	}

	// Find coverage from work items
	coverage, readErrors := findRiskCoverage(risks, validateRisksStories, validateRisksTasks, validateRisksAC)

	// Build result
	result := buildRiskResult(risks, coverage, readErrors, validateRisksDesign)

	formatter := output.New(validateRisksJSON, validateRisksMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(ValidateRisksResult)
		fmt.Fprintf(w, "RISK_VALIDATION:\n")
		fmt.Fprintf(w, "  Risks Identified: %d\n", r.RisksIdentified)
		fmt.Fprintf(w, "  Risks Addressed: %d\n", r.RisksAddressed)
		fmt.Fprintf(w, "  Risks Unaddressed: %d\n", len(r.RisksUnaddressed))
		fmt.Fprintf(w, "  Coverage: %.1f%%\n", r.CoveragePercentage)
		if len(r.RisksUnaddressed) > 0 {
			fmt.Fprintf(w, "  Unaddressed Risks:\n")
			for _, risk := range r.RisksUnaddressed {
				fmt.Fprintf(w, "    - %s\n", risk)
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

// parseRisksFromDesign extracts risks from the Risk Analysis section
func parseRisksFromDesign(filePath string) ([]RiskDetail, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	var risks []RiskDetail
	inRiskSection := false
	nextSectionLevel := 0

	for i, line := range lines {
		// Check for risk section heading
		if riskSectionPattern.MatchString(line) {
			inRiskSection = true
			// Determine heading level
			nextSectionLevel = strings.Count(strings.TrimSpace(line), "#")
			continue
		}

		// Check if we've left the risk section (hit another heading of same or higher level)
		if inRiskSection && strings.HasPrefix(strings.TrimSpace(line), "#") {
			headingLevel := 0
			for _, c := range strings.TrimSpace(line) {
				if c == '#' {
					headingLevel++
				} else {
					break
				}
			}
			if headingLevel <= nextSectionLevel {
				inRiskSection = false
				continue
			}
		}

		if !inRiskSection {
			continue
		}

		// Try to extract risk from table row
		if matches := riskTableRowPattern.FindStringSubmatch(line); matches != nil {
			risks = append(risks, RiskDetail{
				ID:          normalizeRiskID(matches[1]),
				Description: strings.TrimSpace(matches[2]),
			})
			continue
		}

		// Try to extract risk from list format
		if matches := riskListPattern.FindStringSubmatch(line); matches != nil {
			risks = append(risks, RiskDetail{
				ID:          normalizeRiskID(matches[1]),
				Description: strings.TrimSpace(matches[2]),
			})
			continue
		}

		// Look for inline risk IDs
		inlineMatches := riskIDPattern.FindAllString(line, -1)
		for _, match := range inlineMatches {
			// Only add if we haven't seen this risk ID yet - normalize to R-N format
			upper := normalizeRiskID(match)
			found := false
			for _, r := range risks {
				if r.ID == upper {
					found = true
					break
				}
			}
			if !found {
				// Try to get description from rest of line
				desc := ""
				if idx := strings.Index(line, match); idx >= 0 {
					desc = strings.TrimSpace(line[idx+len(match):])
					desc = strings.TrimPrefix(desc, ":")
					desc = strings.TrimPrefix(desc, "-")
					desc = strings.TrimSpace(desc)
				}
				risks = append(risks, RiskDetail{
					ID:          upper,
					Description: desc,
				})
			}
		}
		_ = i // unused, just for context
	}

	return risks, nil
}

// findRiskCoverage searches work items for risk references
func findRiskCoverage(risks []RiskDetail, storiesDir, tasksDir, acDir string) (map[string][]string, []string) {
	coverage := make(map[string][]string)
	var readErrors []string

	// Build set of risk IDs to search for
	riskIDs := make(map[string]bool)
	for _, r := range risks {
		riskIDs[r.ID] = true
	}

	// Search stories directory
	if storiesDir != "" {
		searchDirForRisks(storiesDir, riskIDs, coverage, &readErrors)
	}

	// Search tasks directory
	if tasksDir != "" {
		searchDirForRisks(tasksDir, riskIDs, coverage, &readErrors)
	}

	// Search acceptance criteria directory
	if acDir != "" {
		searchDirForRisks(acDir, riskIDs, coverage, &readErrors)
	}

	return coverage, readErrors
}

// searchDirForRisks scans markdown files for risk references
func searchDirForRisks(dir string, riskIDs map[string]bool, coverage map[string][]string, readErrors *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		*readErrors = append(*readErrors, fmt.Sprintf("%s: %v", dir, err))
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		filePath := filepath.Join(dir, name)
		content, err := os.ReadFile(filePath)
		if err != nil {
			*readErrors = append(*readErrors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		// Find all risk ID references in this file
		matches := riskIDPattern.FindAllString(string(content), -1)
		for _, match := range matches {
			upper := normalizeRiskID(match)
			if riskIDs[upper] {
				// Add file to coverage if not already there
				found := false
				for _, f := range coverage[upper] {
					if f == name {
						found = true
						break
					}
				}
				if !found {
					coverage[upper] = append(coverage[upper], name)
				}
			}
		}
	}
}

// buildRiskResult constructs the final result
func buildRiskResult(risks []RiskDetail, coverage map[string][]string, readErrors []string, designFile string) ValidateRisksResult {
	var addressed int
	var unaddressed []string
	details := make([]RiskDetail, len(risks))

	for i, risk := range risks {
		details[i] = RiskDetail{
			ID:          risk.ID,
			Description: risk.Description,
		}

		if files, ok := coverage[risk.ID]; ok && len(files) > 0 {
			details[i].Covered = true
			details[i].CoveredBy = files
			addressed++
		} else {
			details[i].Covered = false
			unaddressed = append(unaddressed, risk.ID)
		}
	}

	// Sort unaddressed for consistent output
	sort.Strings(unaddressed)

	// Calculate percentage
	var percentage float64
	if len(risks) > 0 {
		percentage = float64(addressed) / float64(len(risks)) * 100.0
	}

	return ValidateRisksResult{
		RisksIdentified:    len(risks),
		RisksAddressed:     addressed,
		RisksUnaddressed:   unaddressed,
		CoveragePercentage: percentage,
		RiskDetails:        details,
		DesignFile:         designFile,
		ReadErrors:         readErrors,
	}
}

func init() {
	RootCmd.AddCommand(newValidateRisksCmd())
}
