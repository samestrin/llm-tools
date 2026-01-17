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
	alignmentCheckRequirements string
	alignmentCheckStories      string
	alignmentCheckTasks        string
	alignmentCheckJSON         bool
	alignmentCheckMinimal      bool

	// Patterns for requirement parsing
	// REQ-N format
	alignmentReqIDPattern = regexp.MustCompile(`(?i)\b(REQ-\d+)\b`)
	// Numbered list format: "1. Description" -> generates REQ-1
	alignmentNumberedPattern = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)`)
	// Markdown header format: "## REQ-1: Description" or "### 1. Description" or "## Requirement 1"
	alignmentHeaderPattern = regexp.MustCompile(`^#{1,6}\s+(?:(REQ-\d+)|(\d+)\.?)\s*[:\-]?\s*(.*)$`)
	// Status patterns
	alignmentStatusComplete = regexp.MustCompile(`(?i)\bStatus:\s*(Complete|Done|Finished)\b`)
	alignmentStatusPartial  = regexp.MustCompile(`(?i)\bStatus:\s*(Partial|In Progress|WIP)\b`)
)

// AlignmentGap represents a requirement gap
type AlignmentGap struct {
	RequirementID string `json:"requirement_id"`
	Description   string `json:"description,omitempty"`
	Status        string `json:"status"` // "unmet" or "partial"
	Reason        string `json:"reason,omitempty"`
}

// RequirementStatus tracks a requirement's alignment status
type RequirementStatus struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Status      string   `json:"status"` // "met", "partial", "unmet"
	TracedBy    []string `json:"traced_by,omitempty"`
}

// AlignmentCheckResult holds the alignment analysis results
type AlignmentCheckResult struct {
	TotalRequirements   int                 `json:"total_requirements"`
	RequirementsMet     int                 `json:"requirements_met"`
	RequirementsPartial int                 `json:"requirements_partial"`
	RequirementsUnmet   int                 `json:"requirements_unmet"`
	AlignmentScore      float64             `json:"alignment_score"`
	Gaps                []AlignmentGap      `json:"gaps,omitempty"`
	ScopeCreep          []string            `json:"scope_creep,omitempty"`
	RequirementDetails  []RequirementStatus `json:"requirement_details,omitempty"`
	RequirementsFile    string              `json:"requirements_file"`
	StoriesDirectory    string              `json:"stories_directory"`
	TasksDirectory      string              `json:"tasks_directory,omitempty"`
	ReadErrors          []string            `json:"read_errors,omitempty"`
	ParseWarnings       []string            `json:"parse_warnings,omitempty"`
}

// newAlignmentCheckCmd creates the alignment-check command
func newAlignmentCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alignment-check",
		Short: "Verify requirements alignment with delivered work",
		Long: `Compare original requirements against completed user stories.

Parses requirements file to extract requirement IDs (REQ-#, numbered lists),
then scans user story files for traceability references and completion status.

Output includes:
  - Requirements met, partial, unmet counts
  - Alignment score percentage
  - Detailed gaps array
  - Scope creep detection (stories without requirement traces)

Examples:
  llm-support alignment-check --requirements ./original-requirements.md --stories ./user-stories/
  llm-support alignment-check --requirements ./reqs.md --stories ./stories/ --tasks ./tasks/ --json`,
		RunE: runAlignmentCheck,
	}

	cmd.Flags().StringVar(&alignmentCheckRequirements, "requirements", "", "Path to requirements markdown file (required)")
	cmd.Flags().StringVar(&alignmentCheckStories, "stories", "", "Path to user stories directory (required)")
	cmd.Flags().StringVar(&alignmentCheckTasks, "tasks", "", "Path to tasks directory (optional, scanned for additional traceability)")
	cmd.Flags().BoolVar(&alignmentCheckJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&alignmentCheckMinimal, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("requirements")
	cmd.MarkFlagRequired("stories")

	return cmd
}

func runAlignmentCheck(cmd *cobra.Command, args []string) error {
	// Validate requirements file exists
	if _, err := os.Stat(alignmentCheckRequirements); os.IsNotExist(err) {
		return fmt.Errorf("requirements file not found: %s", alignmentCheckRequirements)
	}

	// Validate stories directory exists
	info, err := os.Stat(alignmentCheckStories)
	if os.IsNotExist(err) {
		return fmt.Errorf("stories directory not found: %s", alignmentCheckStories)
	}
	if !info.IsDir() {
		return fmt.Errorf("stories path is not a directory: %s", alignmentCheckStories)
	}

	// Validate tasks directory if provided
	if alignmentCheckTasks != "" {
		tasksInfo, err := os.Stat(alignmentCheckTasks)
		if os.IsNotExist(err) {
			return fmt.Errorf("tasks directory not found: %s", alignmentCheckTasks)
		}
		if !tasksInfo.IsDir() {
			return fmt.Errorf("tasks path is not a directory: %s", alignmentCheckTasks)
		}
	}

	// Parse requirements
	requirements, parseWarnings, err := parseAlignmentRequirements(alignmentCheckRequirements)
	if err != nil {
		return fmt.Errorf("failed to parse requirements: %w", err)
	}

	// Scan stories for traceability and status
	storyTraces, scopeCreep, readErrors, err := scanStoriesForTraceability(alignmentCheckStories, requirements)
	if err != nil {
		return fmt.Errorf("failed to scan stories: %w", err)
	}

	// Scan tasks directory if provided
	if alignmentCheckTasks != "" {
		taskTraces, taskScopeCreep, taskReadErrors, err := scanStoriesForTraceability(alignmentCheckTasks, requirements)
		if err != nil {
			return fmt.Errorf("failed to scan tasks: %w", err)
		}
		// Merge task traces into story traces
		storyTraces = append(storyTraces, taskTraces...)
		// Add task scope creep with prefix to distinguish
		for _, tc := range taskScopeCreep {
			scopeCreep = append(scopeCreep, "tasks/"+tc)
		}
		// Merge task read errors with prefix
		for _, re := range taskReadErrors {
			readErrors = append(readErrors, "tasks/"+re)
		}
	}

	// Calculate alignment
	result := calculateAlignment(requirements, storyTraces, scopeCreep, readErrors, parseWarnings, alignmentCheckRequirements, alignmentCheckStories, alignmentCheckTasks)

	formatter := output.New(alignmentCheckJSON, alignmentCheckMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(AlignmentCheckResult)
		fmt.Fprintf(w, "ALIGNMENT_CHECK:\n")
		fmt.Fprintf(w, "  Total Requirements: %d\n", r.TotalRequirements)
		fmt.Fprintf(w, "  Met: %d\n", r.RequirementsMet)
		fmt.Fprintf(w, "  Partial: %d\n", r.RequirementsPartial)
		fmt.Fprintf(w, "  Unmet: %d\n", r.RequirementsUnmet)
		fmt.Fprintf(w, "  Alignment Score: %.1f%%\n", r.AlignmentScore)
		if len(r.Gaps) > 0 {
			fmt.Fprintf(w, "  Gaps:\n")
			for _, gap := range r.Gaps {
				fmt.Fprintf(w, "    - %s: %s\n", gap.RequirementID, gap.Status)
			}
		}
		if len(r.ScopeCreep) > 0 {
			fmt.Fprintf(w, "  Scope Creep: %d items\n", len(r.ScopeCreep))
		}
		if len(r.ReadErrors) > 0 {
			fmt.Fprintf(w, "  Read Errors: %d\n", len(r.ReadErrors))
			for _, e := range r.ReadErrors {
				fmt.Fprintf(w, "    - %s\n", e)
			}
		}
		if len(r.ParseWarnings) > 0 {
			fmt.Fprintf(w, "  Parse Warnings: %d\n", len(r.ParseWarnings))
			for _, pw := range r.ParseWarnings {
				fmt.Fprintf(w, "    - %s\n", pw)
			}
		}
	})
}

// requirement holds parsed requirement info
type requirement struct {
	ID          string
	Description string
}

// parseAlignmentRequirements extracts requirements from markdown file
func parseAlignmentRequirements(filePath string) ([]requirement, []string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, err
	}

	var requirements []requirement
	var parseWarnings []string
	seen := make(map[string]bool)
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		parsed := false

		// Try markdown header format first: "## REQ-1: Description" or "### 1. Description"
		if headerMatch := alignmentHeaderPattern.FindStringSubmatch(line); headerMatch != nil {
			var id, desc string
			if headerMatch[1] != "" {
				// REQ-N format in header
				id = strings.ToUpper(headerMatch[1])
				desc = strings.TrimSpace(headerMatch[3])
			} else if headerMatch[2] != "" {
				// Numbered format in header
				id = fmt.Sprintf("REQ-%s", headerMatch[2])
				desc = strings.TrimSpace(headerMatch[3])
			}
			if id != "" && !seen[id] {
				seen[id] = true
				requirements = append(requirements, requirement{ID: id, Description: desc})
				parsed = true
			}
		}

		// Try inline REQ-N format
		if !parsed {
			matches := alignmentReqIDPattern.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				id := strings.ToUpper(match[1])
				if !seen[id] {
					seen[id] = true
					// Try to extract description
					desc := extractDescription(line, id)
					requirements = append(requirements, requirement{ID: id, Description: desc})
					parsed = true
				}
			}
		}

		// Try numbered list format if nothing else matched
		if !parsed {
			if numMatch := alignmentNumberedPattern.FindStringSubmatch(line); numMatch != nil {
				id := fmt.Sprintf("REQ-%s", numMatch[1])
				if !seen[id] {
					seen[id] = true
					requirements = append(requirements, requirement{ID: id, Description: strings.TrimSpace(numMatch[2])})
					parsed = true
				}
			}
		}

		// Check for potential malformed requirement entries and warn
		if !parsed && looksLikeRequirement(line) {
			parseWarnings = append(parseWarnings, fmt.Sprintf("line %d: possible unparsed requirement: %s", lineNum+1, truncateLine(line, 60)))
		}
	}

	return requirements, parseWarnings, nil
}

// looksLikeRequirement checks if a line might be a requirement we couldn't parse
func looksLikeRequirement(line string) bool {
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return false
	}
	// Check for common requirement-like patterns
	indicators := []string{"requirement", "req:", "req ", "must ", "shall ", "should "}
	for _, ind := range indicators {
		if strings.Contains(line, ind) {
			return true
		}
	}
	return false
}

// truncateLine shortens a line for display in warnings
func truncateLine(line string, maxLen int) string {
	line = strings.TrimSpace(line)
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen-3] + "..."
}

// extractDescription tries to get description from line after requirement ID
func extractDescription(line, id string) string {
	idx := strings.Index(strings.ToUpper(line), id)
	if idx < 0 {
		return ""
	}
	after := line[idx+len(id):]
	after = strings.TrimPrefix(after, ":")
	after = strings.TrimPrefix(after, "-")
	return strings.TrimSpace(after)
}

// storyTrace holds traceability info from a story
type storyTrace struct {
	FileName        string
	RequirementRefs []string
	IsComplete      bool
	IsPartial       bool
}

// scanStoriesForTraceability scans stories for requirement references
func scanStoriesForTraceability(storiesDir string, requirements []requirement) ([]storyTrace, []string, []string, error) {
	entries, err := os.ReadDir(storiesDir)
	if err != nil {
		return nil, nil, nil, err
	}

	// Build set of valid requirement IDs
	validReqs := make(map[string]bool)
	for _, req := range requirements {
		validReqs[req.ID] = true
	}

	var traces []storyTrace
	var scopeCreep []string
	var readErrors []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}

		filePath := filepath.Join(storiesDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			readErrors = append(readErrors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		contentStr := string(content)

		// Find requirement references
		matches := alignmentReqIDPattern.FindAllString(contentStr, -1)
		var refs []string
		refSeen := make(map[string]bool)
		for _, match := range matches {
			upper := strings.ToUpper(match)
			if validReqs[upper] && !refSeen[upper] {
				refSeen[upper] = true
				refs = append(refs, upper)
			}
		}

		// Determine status
		isComplete := alignmentStatusComplete.MatchString(contentStr)
		isPartial := alignmentStatusPartial.MatchString(contentStr)

		if len(refs) > 0 {
			traces = append(traces, storyTrace{
				FileName:        entry.Name(),
				RequirementRefs: refs,
				IsComplete:      isComplete,
				IsPartial:       isPartial,
			})
		} else {
			// Story with no requirement traces is potential scope creep
			scopeCreep = append(scopeCreep, entry.Name())
		}
	}

	return traces, scopeCreep, readErrors, nil
}

// calculateAlignment computes alignment metrics
func calculateAlignment(requirements []requirement, traces []storyTrace, scopeCreep, readErrors, parseWarnings []string, reqsFile, storiesDir, tasksDir string) AlignmentCheckResult {
	// Build map of requirement status
	reqStatus := make(map[string]*RequirementStatus)
	for _, req := range requirements {
		reqStatus[req.ID] = &RequirementStatus{
			ID:          req.ID,
			Description: req.Description,
			Status:      "unmet",
		}
	}

	// Update status based on story traces
	for _, trace := range traces {
		for _, ref := range trace.RequirementRefs {
			rs := reqStatus[ref]
			if rs == nil {
				continue
			}

			rs.TracedBy = append(rs.TracedBy, trace.FileName)

			// Update status based on story completion
			if trace.IsComplete {
				if rs.Status == "unmet" {
					rs.Status = "met"
				}
			} else if trace.IsPartial {
				if rs.Status == "unmet" {
					rs.Status = "partial"
				}
			} else {
				// Has trace but no status - treat as partial
				if rs.Status == "unmet" {
					rs.Status = "partial"
				}
			}
		}
	}

	// Count statuses and build details/gaps
	var met, partial, unmet int
	var gaps []AlignmentGap
	var details []RequirementStatus

	// Sort requirements for consistent output
	sortedReqs := make([]string, 0, len(reqStatus))
	for id := range reqStatus {
		sortedReqs = append(sortedReqs, id)
	}
	sort.Strings(sortedReqs)

	for _, id := range sortedReqs {
		rs := reqStatus[id]
		details = append(details, *rs)

		switch rs.Status {
		case "met":
			met++
		case "partial":
			partial++
			gaps = append(gaps, AlignmentGap{
				RequirementID: rs.ID,
				Description:   rs.Description,
				Status:        "partial",
				Reason:        "Story in progress or incomplete",
			})
		case "unmet":
			unmet++
			gaps = append(gaps, AlignmentGap{
				RequirementID: rs.ID,
				Description:   rs.Description,
				Status:        "unmet",
				Reason:        "No story traces to this requirement",
			})
		}
	}

	// Calculate alignment score: (met + 0.5*partial) / total * 100
	var score float64
	total := len(requirements)
	if total > 0 {
		score = (float64(met) + 0.5*float64(partial)) / float64(total) * 100.0
	}

	return AlignmentCheckResult{
		TotalRequirements:   total,
		RequirementsMet:     met,
		RequirementsPartial: partial,
		RequirementsUnmet:   unmet,
		AlignmentScore:      score,
		Gaps:                gaps,
		ScopeCreep:          scopeCreep,
		RequirementDetails:  details,
		RequirementsFile:    reqsFile,
		StoriesDirectory:    storiesDir,
		TasksDirectory:      tasksDir,
		ReadErrors:          readErrors,
		ParseWarnings:       parseWarnings,
	}
}

func init() {
	RootCmd.AddCommand(newAlignmentCheckCmd())
}
