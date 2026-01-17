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
	alignmentCheckJSON         bool
	alignmentCheckMinimal      bool

	// Patterns for requirement parsing
	// REQ-N format
	alignmentReqIDPattern = regexp.MustCompile(`(?i)\b(REQ-\d+)\b`)
	// Numbered list format: "1. Description" -> generates REQ-1
	alignmentNumberedPattern = regexp.MustCompile(`^\s*(\d+)\.\s+(.+)`)
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
  llm-support alignment-check --requirements ./reqs.md --stories ./stories/ --json`,
		RunE: runAlignmentCheck,
	}

	cmd.Flags().StringVar(&alignmentCheckRequirements, "requirements", "", "Path to requirements markdown file (required)")
	cmd.Flags().StringVar(&alignmentCheckStories, "stories", "", "Path to user stories directory (required)")
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

	// Parse requirements
	requirements, err := parseAlignmentRequirements(alignmentCheckRequirements)
	if err != nil {
		return fmt.Errorf("failed to parse requirements: %w", err)
	}

	// Scan stories for traceability and status
	storyTraces, scopeCreep, err := scanStoriesForTraceability(alignmentCheckStories, requirements)
	if err != nil {
		return fmt.Errorf("failed to scan stories: %w", err)
	}

	// Calculate alignment
	result := calculateAlignment(requirements, storyTraces, scopeCreep, alignmentCheckRequirements, alignmentCheckStories)

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
	})
}

// requirement holds parsed requirement info
type requirement struct {
	ID          string
	Description string
}

// parseAlignmentRequirements extracts requirements from markdown file
func parseAlignmentRequirements(filePath string) ([]requirement, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var requirements []requirement
	seen := make(map[string]bool)
	lines := strings.Split(string(content), "\n")

	numberedCount := 0

	for _, line := range lines {
		// Try REQ-N format first
		matches := alignmentReqIDPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			id := strings.ToUpper(match[1])
			if !seen[id] {
				seen[id] = true
				// Try to extract description
				desc := extractDescription(line, id)
				requirements = append(requirements, requirement{ID: id, Description: desc})
			}
		}

		// Try numbered list format if no REQ-N found
		if len(matches) == 0 {
			if numMatch := alignmentNumberedPattern.FindStringSubmatch(line); numMatch != nil {
				numberedCount++
				id := fmt.Sprintf("REQ-%s", numMatch[1])
				if !seen[id] {
					seen[id] = true
					requirements = append(requirements, requirement{ID: id, Description: strings.TrimSpace(numMatch[2])})
				}
			}
		}
	}

	return requirements, nil
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
func scanStoriesForTraceability(storiesDir string, requirements []requirement) ([]storyTrace, []string, error) {
	entries, err := os.ReadDir(storiesDir)
	if err != nil {
		return nil, nil, err
	}

	// Build set of valid requirement IDs
	validReqs := make(map[string]bool)
	for _, req := range requirements {
		validReqs[req.ID] = true
	}

	var traces []storyTrace
	var scopeCreep []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}

		filePath := filepath.Join(storiesDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
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

	return traces, scopeCreep, nil
}

// calculateAlignment computes alignment metrics
func calculateAlignment(requirements []requirement, traces []storyTrace, scopeCreep []string, reqsFile, storiesDir string) AlignmentCheckResult {
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
	}
}

func init() {
	RootCmd.AddCommand(newAlignmentCheckCmd())
}
