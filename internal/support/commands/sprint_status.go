package commands

import (
	"fmt"
	"io"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	sprintStatusTasksTotal        int
	sprintStatusTasksCompleted    int
	sprintStatusTestsPassed       bool
	sprintStatusCoverage          float64
	sprintStatusCriticalIssues    int
	sprintStatusCompletedThresh   float64
	sprintStatusPartialThresh     float64
	sprintStatusCoverageMinThresh float64
	sprintStatusJSON              bool
	sprintStatusMinimal           bool
)

// Default thresholds
const (
	defaultCompletedThreshold   = 0.90 // 90% completion for COMPLETED
	defaultPartialThreshold     = 0.50 // 50% completion for PARTIAL (below is FAILED)
	defaultCoverageMinThreshold = 60.0 // Minimum 60% coverage to avoid FAILED
)

// SprintStatusResult holds the sprint status determination
type SprintStatusResult struct {
	Status               string   `json:"status"` // COMPLETED, PARTIAL, FAILED
	TasksTotal           int      `json:"tasks_total"`
	TasksCompleted       int      `json:"tasks_completed"`
	CompletionPercentage float64  `json:"completion_percentage"`
	TestsPassed          bool     `json:"tests_passed"`
	Coverage             float64  `json:"coverage"`
	CriticalIssues       int      `json:"critical_issues"`
	Reasons              []string `json:"reasons,omitempty"`
}

// newSprintStatusCmd creates the sprint-status command
func newSprintStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sprint-status",
		Short: "Determine sprint completion status",
		Long: `Analyze sprint completion data and return a definitive status.

Evaluates tasks completed, tests passed, coverage, and critical issues
to determine if sprint is COMPLETED, PARTIAL, or FAILED.

Status determination:
  COMPLETED: All conditions met (tasks >= threshold, tests pass, coverage ok, no critical issues)
  PARTIAL:   Some tasks incomplete but no blocking failures
  FAILED:    Tests fail, critical issues exist, or completion severely below threshold

Default thresholds:
  - Completion >= 90%: COMPLETED eligible
  - Completion >= 50%: PARTIAL
  - Completion < 50%: FAILED
  - Coverage < 60%: FAILED

Examples:
  llm-support sprint-status --tasks-total 10 --tasks-completed 10 --tests-passed true --coverage 80 --critical-issues 0
  llm-support sprint-status --tasks-total 10 --tasks-completed 7 --tests-passed true --coverage 75 --critical-issues 0 --json`,
		RunE: runSprintStatus,
	}

	cmd.Flags().IntVar(&sprintStatusTasksTotal, "tasks-total", 0, "Total number of tasks")
	cmd.Flags().IntVar(&sprintStatusTasksCompleted, "tasks-completed", 0, "Number of completed tasks")
	cmd.Flags().BoolVar(&sprintStatusTestsPassed, "tests-passed", false, "Whether tests passed")
	cmd.Flags().Float64Var(&sprintStatusCoverage, "coverage", 0, "Coverage percentage")
	cmd.Flags().IntVar(&sprintStatusCriticalIssues, "critical-issues", 0, "Number of critical issues")
	cmd.Flags().Float64Var(&sprintStatusCompletedThresh, "completed-threshold", defaultCompletedThreshold, "Completion threshold for COMPLETED status (0.0-1.0)")
	cmd.Flags().Float64Var(&sprintStatusPartialThresh, "partial-threshold", defaultPartialThreshold, "Completion threshold for PARTIAL status (0.0-1.0)")
	cmd.Flags().Float64Var(&sprintStatusCoverageMinThresh, "coverage-threshold", defaultCoverageMinThreshold, "Minimum coverage to avoid FAILED")
	cmd.Flags().BoolVar(&sprintStatusJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&sprintStatusMinimal, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("tests-passed")

	return cmd
}

func runSprintStatus(cmd *cobra.Command, args []string) error {
	// Validate inputs - at minimum we need tests-passed and some way to determine completion
	// tasks-total and tasks-completed should either both be 0 or both be provided
	if sprintStatusTasksTotal < 0 || sprintStatusTasksCompleted < 0 {
		return fmt.Errorf("task counts cannot be negative")
	}
	if sprintStatusTasksCompleted > sprintStatusTasksTotal {
		return fmt.Errorf("tasks-completed (%d) cannot exceed tasks-total (%d)", sprintStatusTasksCompleted, sprintStatusTasksTotal)
	}

	// Calculate completion percentage
	var completionPct float64
	if sprintStatusTasksTotal > 0 {
		completionPct = float64(sprintStatusTasksCompleted) / float64(sprintStatusTasksTotal) * 100.0
	} else {
		// No tasks means 100% complete (vacuously true)
		completionPct = 100.0
	}

	// Determine status and collect reasons
	status, reasons := determineStatus(
		completionPct,
		sprintStatusTestsPassed,
		sprintStatusCoverage,
		sprintStatusCriticalIssues,
		sprintStatusCompletedThresh,
		sprintStatusPartialThresh,
		sprintStatusCoverageMinThresh,
	)

	result := SprintStatusResult{
		Status:               status,
		TasksTotal:           sprintStatusTasksTotal,
		TasksCompleted:       sprintStatusTasksCompleted,
		CompletionPercentage: completionPct,
		TestsPassed:          sprintStatusTestsPassed,
		Coverage:             sprintStatusCoverage,
		CriticalIssues:       sprintStatusCriticalIssues,
		Reasons:              reasons,
	}

	formatter := output.New(sprintStatusJSON, sprintStatusMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(SprintStatusResult)
		fmt.Fprintf(w, "SPRINT_STATUS: %s\n", r.Status)
		fmt.Fprintf(w, "  Tasks: %d/%d (%.1f%%)\n", r.TasksCompleted, r.TasksTotal, r.CompletionPercentage)
		fmt.Fprintf(w, "  Tests Passed: %v\n", r.TestsPassed)
		fmt.Fprintf(w, "  Coverage: %.1f%%\n", r.Coverage)
		fmt.Fprintf(w, "  Critical Issues: %d\n", r.CriticalIssues)
		if len(r.Reasons) > 0 {
			fmt.Fprintf(w, "  Reasons:\n")
			for _, reason := range r.Reasons {
				fmt.Fprintf(w, "    - %s\n", reason)
			}
		}
	})
}

// determineStatus evaluates all factors and returns status with reasons
func determineStatus(completionPct float64, testsPassed bool, coverage float64, criticalIssues int, completedThresh, partialThresh, coverageMin float64) (string, []string) {
	var reasons []string

	// Check for FAILED conditions first (blocking issues)
	if !testsPassed {
		reasons = append(reasons, "Tests failed")
	}

	if criticalIssues > 0 {
		reasons = append(reasons, fmt.Sprintf("Critical issues: %d", criticalIssues))
	}

	if coverage < coverageMin && coverage > 0 {
		reasons = append(reasons, fmt.Sprintf("Coverage %.1f%% below minimum %.1f%%", coverage, coverageMin))
	}

	completionRatio := completionPct / 100.0
	if completionRatio < partialThresh {
		reasons = append(reasons, fmt.Sprintf("Completion %.1f%% below partial threshold %.0f%%", completionPct, partialThresh*100))
	}

	// If any blocking issues, status is FAILED
	if !testsPassed || criticalIssues > 0 || (coverage > 0 && coverage < coverageMin) || completionRatio < partialThresh {
		return "FAILED", reasons
	}

	// Check for COMPLETED conditions
	if completionRatio >= completedThresh {
		return "COMPLETED", nil
	}

	// Otherwise PARTIAL
	reasons = append(reasons, fmt.Sprintf("Completion %.1f%% below completed threshold %.0f%%", completionPct, completedThresh*100))
	return "PARTIAL", reasons
}

func init() {
	RootCmd.AddCommand(newSprintStatusCmd())
}
