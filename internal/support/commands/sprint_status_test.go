package commands

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestSprintStatusCompleted tests COMPLETED status determination
func TestSprintStatusCompleted(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "10",
		"--tests-passed", "true",
		"--coverage", "85.5",
		"--critical-issues", "0",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.Status != "COMPLETED" {
		t.Errorf("status = %s, want COMPLETED", result.Status)
	}

	if result.TasksTotal != 10 || result.TasksCompleted != 10 {
		t.Errorf("tasks = %d/%d, want 10/10", result.TasksCompleted, result.TasksTotal)
	}
}

// TestSprintStatusPartial tests PARTIAL status determination
func TestSprintStatusPartial(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "7",
		"--tests-passed", "true",
		"--coverage", "75.0",
		"--critical-issues", "0",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Status != "PARTIAL" {
		t.Errorf("status = %s, want PARTIAL (70%% completion)", result.Status)
	}
}

// TestSprintStatusFailed tests FAILED status determination
func TestSprintStatusFailed(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		reason string
	}{
		{
			name: "tests failed",
			args: []string{
				"--tasks-total", "10",
				"--tasks-completed", "10",
				"--tests-passed=false",
				"--coverage", "80.0",
				"--critical-issues", "0",
				"--json",
			},
			reason: "test failures",
		},
		{
			name: "critical issues exist",
			args: []string{
				"--tasks-total", "10",
				"--tasks-completed", "10",
				"--tests-passed", "true",
				"--coverage", "80.0",
				"--critical-issues", "2",
				"--json",
			},
			reason: "critical issues",
		},
		{
			name: "very low task completion",
			args: []string{
				"--tasks-total", "10",
				"--tasks-completed", "3",
				"--tests-passed", "true",
				"--coverage", "80.0",
				"--critical-issues", "0",
				"--json",
			},
			reason: "low completion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSprintStatusCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result SprintStatusResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if result.Status != "FAILED" {
				t.Errorf("status = %s, want FAILED (%s)", result.Status, tt.reason)
			}
		})
	}
}

// TestSprintStatusLowCoverage tests status with low coverage
func TestSprintStatusLowCoverage(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "10",
		"--tests-passed", "true",
		"--coverage", "50.0",
		"--critical-issues", "0",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Low coverage (< 60%) should cause FAILED status
	if result.Status != "FAILED" {
		t.Errorf("status = %s, want FAILED (coverage 50%% < 60%%)", result.Status)
	}
}

// TestSprintStatusCustomThresholds tests custom threshold configuration
func TestSprintStatusCustomThresholds(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "9",
		"--tests-passed", "true",
		"--coverage", "70.0",
		"--critical-issues", "0",
		"--completed-threshold", "0.85",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// 90% completion with 85% threshold should be COMPLETED
	if result.Status != "COMPLETED" {
		t.Errorf("status = %s, want COMPLETED (90%% >= 85%% threshold)", result.Status)
	}
}

// TestSprintStatusSummaryBreakdown tests detailed summary output
func TestSprintStatusSummaryBreakdown(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "8",
		"--tests-passed", "true",
		"--coverage", "82.5",
		"--critical-issues", "1",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.TasksTotal != 10 {
		t.Errorf("tasks_total = %d, want 10", result.TasksTotal)
	}

	if result.TasksCompleted != 8 {
		t.Errorf("tasks_completed = %d, want 8", result.TasksCompleted)
	}

	if result.TestsPassed != true {
		t.Error("tests_passed should be true")
	}

	if result.Coverage != 82.5 {
		t.Errorf("coverage = %.1f, want 82.5", result.Coverage)
	}

	if result.CriticalIssues != 1 {
		t.Errorf("critical_issues = %d, want 1", result.CriticalIssues)
	}
}

// TestSprintStatusZeroTasks tests zero tasks edge case
func TestSprintStatusZeroTasks(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "0",
		"--tasks-completed", "0",
		"--tests-passed", "true",
		"--coverage", "80.0",
		"--critical-issues", "0",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// With zero tasks but tests pass and no issues, should be COMPLETED
	if result.Status != "COMPLETED" {
		t.Errorf("status = %s, want COMPLETED (no tasks, tests pass)", result.Status)
	}
}

// TestSprintStatusMissingInputs tests missing required inputs
func TestSprintStatusMissingInputs(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Missing all inputs
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing required inputs")
	}
}

// TestSprintStatusCompletionPercentage tests completion percentage calculation
func TestSprintStatusCompletionPercentage(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "20",
		"--tasks-completed", "15",
		"--tests-passed", "true",
		"--coverage", "80.0",
		"--critical-issues", "0",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// 15/20 = 75%
	if result.CompletionPercentage != 75.0 {
		t.Errorf("completion_percentage = %.1f, want 75.0", result.CompletionPercentage)
	}
}

// TestSprintStatusBoundaryValues tests status boundary thresholds
func TestSprintStatusBoundaryValues(t *testing.T) {
	tests := []struct {
		name           string
		tasksTotal     int
		tasksCompleted int
		expectedStatus string
	}{
		{"100% -> COMPLETED", 10, 10, "COMPLETED"},
		{"95% -> COMPLETED", 20, 19, "COMPLETED"},
		{"90% -> COMPLETED", 10, 9, "COMPLETED"},
		{"89% -> PARTIAL", 100, 89, "PARTIAL"},
		{"50% -> PARTIAL", 10, 5, "PARTIAL"},
		{"49% -> FAILED", 100, 49, "FAILED"},
		{"30% -> FAILED", 10, 3, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newSprintStatusCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{
				"--tasks-total", string(rune('0'+tt.tasksTotal/10)) + string(rune('0'+tt.tasksTotal%10)),
				"--tasks-completed", string(rune('0'+tt.tasksCompleted/10)) + string(rune('0'+tt.tasksCompleted%10)),
				"--tests-passed", "true",
				"--coverage", "80.0",
				"--critical-issues", "0",
				"--json",
			})

			// Use strconv for proper conversion
			cmd.SetArgs([]string{
				"--tasks-total", itoa(tt.tasksTotal),
				"--tasks-completed", itoa(tt.tasksCompleted),
				"--tests-passed", "true",
				"--coverage", "80.0",
				"--critical-issues", "0",
				"--json",
			})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result SprintStatusResult
			if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("status = %s, want %s", result.Status, tt.expectedStatus)
			}
		})
	}
}

// Helper for int to string conversion in tests
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}

// TestSprintStatusReasons tests that status reasons are populated
func TestSprintStatusReasons(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "10",
		"--tests-passed=false",
		"--coverage", "80.0",
		"--critical-issues", "2",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.Reasons) == 0 {
		t.Error("reasons should not be empty for FAILED status")
	}
}

// TestSprintStatusMinimalOutput tests minimal output mode
func TestSprintStatusMinimalOutput(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "10",
		"--tests-passed", "true",
		"--coverage", "85.0",
		"--critical-issues", "0",
		"--min",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected some output in minimal mode")
	}
}

// TestSprintStatusHumanReadableOutput tests human-readable (non-JSON) output
func TestSprintStatusHumanReadableOutput(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "10",
		"--tests-passed", "true",
		"--coverage", "85.0",
		"--critical-issues", "0",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SPRINT_STATUS") || !strings.Contains(output, "COMPLETED") {
		t.Errorf("expected human-readable output with SPRINT_STATUS, got: %s", output)
	}
}

// TestSprintStatusNegativeTasks tests error for negative task counts
func TestSprintStatusNegativeTasks(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "-5",
		"--tasks-completed", "0",
		"--tests-passed", "true",
		"--coverage", "80.0",
		"--critical-issues", "0",
	})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for negative task count")
	}
}

// TestSprintStatusCompletedExceedsTotal tests error when completed > total
func TestSprintStatusCompletedExceedsTotal(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "15",
		"--tests-passed", "true",
		"--coverage", "80.0",
		"--critical-issues", "0",
	})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when completed > total")
	}
}

// TestSprintStatusWithReasons tests that reasons are populated in human-readable output
func TestSprintStatusWithReasons(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "5",
		"--tests-passed", "false",
		"--coverage", "50.0",
		"--critical-issues", "2",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Reasons") {
		t.Errorf("expected reasons in output for FAILED status, got: %s", output)
	}
}

// TestSprintStatusPartialThreshold tests partial threshold customization
func TestSprintStatusPartialThreshold(t *testing.T) {
	cmd := newSprintStatusCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--tasks-total", "10",
		"--tasks-completed", "4",
		"--tests-passed", "true",
		"--coverage", "80.0",
		"--critical-issues", "0",
		"--partial-threshold", "0.3",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result SprintStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// 40% is above 30% partial threshold, so should be PARTIAL not FAILED
	if result.Status != "PARTIAL" {
		t.Errorf("status = %s, want PARTIAL (40%% >= 30%% threshold)", result.Status)
	}
}
