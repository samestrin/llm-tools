package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/support/multireview"
)

func TestSprintPlanScopeBlock_Empty(t *testing.T) {
	if got := sprintPlanScopeBlock(""); got != "" {
		t.Errorf("sprintPlanScopeBlock(\"\") = %q, want empty string", got)
	}
}

func TestSprintPlanScopeBlock_Content(t *testing.T) {
	plan := "## Sprint 9.0\n- Task: add widget"
	got := sprintPlanScopeBlock(plan)

	for _, want := range []string{
		"SCOPE CONSTRAINT",
		"--- BEGIN SPRINT PLAN ---",
		plan,
		"--- END SPRINT PLAN ---",
		"OUT-OF-SCOPE",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("scope block missing %q\nblock:\n%s", want, got)
		}
	}
}

func TestBuildDefaultTaskMessage_WithSprintPlan(t *testing.T) {
	diff := multireview.PreComputeDiffResult{
		DiffPath:  "/tmp/diff.txt",
		SizeBytes: 100,
		LineCount: 10,
	}
	plan := "## Sprint 9.0\n- Task: add widget"
	msg := buildDefaultTaskMessage("/remote/repo", "repo", "abc", "def", diff, plan)

	if !strings.Contains(msg, "SCOPE CONSTRAINT") {
		t.Error("expected SCOPE CONSTRAINT block when sprint plan provided")
	}
	if !strings.Contains(msg, plan) {
		t.Error("expected sprint plan content in task message")
	}
}

func TestBuildDefaultTaskMessage_WithoutSprintPlan(t *testing.T) {
	diff := multireview.PreComputeDiffResult{
		DiffPath:  "/tmp/diff.txt",
		SizeBytes: 100,
		LineCount: 10,
	}
	msg := buildDefaultTaskMessage("/remote/repo", "repo", "abc", "def", diff, "")

	if strings.Contains(msg, "SCOPE CONSTRAINT") {
		t.Error("unexpected SCOPE CONSTRAINT block when no sprint plan provided")
	}
}

func TestReadSprintPlan_ExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sprint-plan.md")
	content := "## Sprint 9.0\n- Task one\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var warnings bytes.Buffer
	got := readSprintPlan(path, &warnings)
	if got != content {
		t.Errorf("readSprintPlan = %q, want %q", got, content)
	}
	if warnings.Len() != 0 {
		t.Errorf("unexpected warning output: %s", warnings.String())
	}
}

func TestReadSprintPlan_MissingFile(t *testing.T) {
	var warnings bytes.Buffer
	got := readSprintPlan(filepath.Join(t.TempDir(), "nope.md"), &warnings)
	if got != "" {
		t.Errorf("readSprintPlan on missing file = %q, want empty", got)
	}
	if warnings.Len() != 0 {
		t.Errorf("missing file must be silent, got warning: %s", warnings.String())
	}
}

func TestReadSprintPlan_EmptyPath(t *testing.T) {
	var warnings bytes.Buffer
	got := readSprintPlan("", &warnings)
	if got != "" {
		t.Errorf("readSprintPlan(\"\") = %q, want empty", got)
	}
	if warnings.Len() != 0 {
		t.Errorf("empty path must be silent, got warning: %s", warnings.String())
	}
}

func TestReadSprintPlan_UnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; permission bits not enforced")
	}
	path := filepath.Join(t.TempDir(), "locked.md")
	if err := os.WriteFile(path, []byte("secret"), 0000); err != nil {
		t.Fatal(err)
	}

	var warnings bytes.Buffer
	got := readSprintPlan(path, &warnings)
	if got != "" {
		t.Errorf("readSprintPlan on unreadable file = %q, want empty", got)
	}
	if !strings.Contains(warnings.String(), "warning: could not read --sprint-plan") {
		t.Errorf("expected read warning, got: %q", warnings.String())
	}
}
