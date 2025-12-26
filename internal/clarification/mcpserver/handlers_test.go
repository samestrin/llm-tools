package mcpserver

import (
	"reflect"
	"testing"
)

func TestBuildMatchArgs(t *testing.T) {
	args := map[string]interface{}{
		"question":     "What testing framework?",
		"entries_file": "tracking.yaml",
		"timeout":      float64(60),
	}

	got := buildMatchArgs(args)
	want := []string{"match-clarification", "--question", "What testing framework?", "--entries-file", "tracking.yaml", "--timeout", "60"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildMatchArgs() = %v, want %v", got, want)
	}
}

func TestBuildInitArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic init",
			args: map[string]interface{}{"output": "tracking.yaml"},
			want: []string{"init-tracking", "--output", "tracking.yaml"},
		},
		{
			name: "with force",
			args: map[string]interface{}{"output": "tracking.yaml", "force": true},
			want: []string{"init-tracking", "--output", "tracking.yaml", "--force"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInitArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildInitArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAddArgs(t *testing.T) {
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"question":      "What is the testing approach?",
		"answer":        "Use Jest for unit tests",
		"sprint_id":     "sprint-1",
		"context_tags":  "testing,frontend",
	}

	got := buildAddArgs(args)

	// Check key elements exist
	expected := []string{"add-clarification", "--tracking-file", "tracking.yaml"}
	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("buildAddArgs()[%d] = %v, want %v", i, got[i], exp)
		}
	}

	// Check question is present
	hasQuestion := false
	for i, arg := range got {
		if arg == "--question" && i+1 < len(got) && got[i+1] == "What is the testing approach?" {
			hasQuestion = true
			break
		}
	}
	if !hasQuestion {
		t.Error("Expected --question flag with correct value")
	}
}

func TestBuildPromoteArgs(t *testing.T) {
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"id":            "clarify-001",
		"target":        "CLAUDE.md",
		"force":         true,
	}

	got := buildPromoteArgs(args)
	want := []string{"promote-clarification", "--tracking-file", "tracking.yaml", "--id", "clarify-001", "--target", "CLAUDE.md", "--force"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildPromoteArgs() = %v, want %v", got, want)
	}
}

func TestBuildListArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic list",
			args: map[string]interface{}{"tracking_file": "tracking.yaml"},
			want: []string{"list-entries", "tracking.yaml"},
		},
		{
			name: "with filters",
			args: map[string]interface{}{
				"tracking_file":   "tracking.yaml",
				"status":          "pending",
				"min_occurrences": float64(3),
				"json_output":     true,
			},
			want: []string{"list-entries", "tracking.yaml", "--status", "pending", "--min-occurrences", "3", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildListArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildListArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDetectConflictsArgs(t *testing.T) {
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"timeout":       float64(45),
	}

	got := buildDetectConflictsArgs(args)
	want := []string{"detect-conflicts", "tracking.yaml", "--timeout", "45"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildDetectConflictsArgs() = %v, want %v", got, want)
	}
}

func TestBuildValidateArgs(t *testing.T) {
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"context":       "React frontend project",
	}

	got := buildValidateArgs(args)
	want := []string{"validate-clarifications", "tracking.yaml", "--context", "React frontend project"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildValidateArgs() = %v, want %v", got, want)
	}
}

func TestBuildClusterArgs(t *testing.T) {
	args := map[string]interface{}{
		"questions_file": "questions.txt",
		"timeout":        float64(30),
	}

	got := buildClusterArgs(args)
	want := []string{"cluster-clarifications", "--questions-file", "questions.txt", "--timeout", "30"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildClusterArgs() = %v, want %v", got, want)
	}
}
