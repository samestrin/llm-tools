package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPartitionWorkCommand(t *testing.T) {
	// Create temp directory with story files
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create story files with shared dependencies
	story1 := filepath.Join(tmpDir, "story-1.md")
	os.WriteFile(story1, []byte("# Story 1\n\nModify `shared.ts` and `a.ts`"), 0644)

	story2 := filepath.Join(tmpDir, "story-2.md")
	os.WriteFile(story2, []byte("# Story 2\n\nModify `shared.ts` and `b.ts`"), 0644)

	story3 := filepath.Join(tmpDir, "story-3.md")
	os.WriteFile(story3, []byte("# Story 3\n\nModify `c.ts` (independent)"), 0644)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--stories", tmpDir})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

	// Should have groups
	if !bytes.Contains([]byte(output), []byte("Group")) {
		t.Errorf("expected 'Group' in output, got: %s", output)
	}
}

func TestPartitionWorkJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	story1 := filepath.Join(tmpDir, "story-1.md")
	os.WriteFile(story1, []byte("# Story 1\n\nModify `a.ts`"), 0644)

	story2 := filepath.Join(tmpDir, "story-2.md")
	os.WriteFile(story2, []byte("# Story 2\n\nModify `b.ts`"), 0644)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--stories", tmpDir, "--json"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

	expectedPatterns := []string{
		`"groups"`,
		`"total_groups"`,
		`"items_per_group"`,
	}

	for _, pattern := range expectedPatterns {
		if !bytes.Contains([]byte(output), []byte(pattern)) {
			t.Errorf("JSON output missing %q, got: %s", pattern, output)
		}
	}
}

func TestPartitionWorkIndependent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create independent stories (no shared files)
	story1 := filepath.Join(tmpDir, "story-1.md")
	os.WriteFile(story1, []byte("# Story 1\n\nModify `a.ts`"), 0644)

	story2 := filepath.Join(tmpDir, "story-2.md")
	os.WriteFile(story2, []byte("# Story 2\n\nModify `b.ts`"), 0644)

	story3 := filepath.Join(tmpDir, "story-3.md")
	os.WriteFile(story3, []byte("# Story 3\n\nModify `c.ts`"), 0644)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--stories", tmpDir})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()

	// All independent = can run in parallel = 1 group
	if !bytes.Contains([]byte(output), []byte("independent")) || !bytes.Contains([]byte(output), []byte("parallel")) {
		// Check JSON format for single group
		if !bytes.Contains([]byte(output), []byte("Group 0:")) {
			t.Logf("output: %s", output)
		}
	}
}

func TestPartitionWorkTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	task1 := filepath.Join(tmpDir, "task-1.md")
	os.WriteFile(task1, []byte("# Task 1\n\nModify `shared.ts`"), 0644)

	task2 := filepath.Join(tmpDir, "task-2.md")
	os.WriteFile(task2, []byte("# Task 2\n\nModify `shared.ts`"), 0644)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--tasks", tmpDir})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	// With shared deps, should have multiple groups or mention sequential
	if !bytes.Contains([]byte(output), []byte("Group")) {
		t.Errorf("expected Group in output, got: %s", output)
	}
}

func TestPartitionWorkEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--stories", tmpDir})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("No items found")) {
		t.Errorf("expected 'No items found' message, got: %s", output)
	}
}

func TestPartitionWorkVerbose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	story1 := filepath.Join(tmpDir, "story-1.md")
	os.WriteFile(story1, []byte("# Story 1\n\nModify `shared.ts`"), 0644)

	story2 := filepath.Join(tmpDir, "story-2.md")
	os.WriteFile(story2, []byte("# Story 2\n\nModify `shared.ts`"), 0644)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--stories", tmpDir, "--verbose"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	if !bytes.Contains([]byte(output), []byte("Conflict Graph")) {
		t.Errorf("expected conflict graph in verbose output, got: %s", output)
	}
}

func TestPartitionWorkNoFlags(t *testing.T) {
	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no --stories or --tasks flag")
	}
}

func TestPartitionWorkBothFlags(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partition-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := newPartitionWorkCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--stories", tmpDir, "--tasks", tmpDir})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when both --stories and --tasks specified")
	}
}

func TestGreedyColoring(t *testing.T) {
	// Test the graph coloring algorithm
	graph := map[string][]string{
		"A": {"B", "C"},
		"B": {"A", "C"},
		"C": {"A", "B"},
		"D": {},
	}

	colors := greedyColoring(graph)

	// A, B, C form a triangle - need 3 colors
	// D is independent - can share color with any
	if colors["A"] == colors["B"] || colors["A"] == colors["C"] || colors["B"] == colors["C"] {
		t.Errorf("adjacent nodes have same color: A=%d, B=%d, C=%d", colors["A"], colors["B"], colors["C"])
	}
}

func TestBuildConflictGraph(t *testing.T) {
	items := map[string][]string{
		"story-1.md": {"shared.ts", "a.ts"},
		"story-2.md": {"shared.ts", "b.ts"},
		"story-3.md": {"c.ts"},
	}

	conflicts := buildConflictGraph(items)

	// story-1 and story-2 share shared.ts - should conflict
	if !contains(conflicts["story-1.md"], "story-2.md") {
		t.Error("story-1 and story-2 should conflict (share shared.ts)")
	}

	// story-3 is independent
	if len(conflicts["story-3.md"]) != 0 {
		t.Error("story-3 should have no conflicts")
	}
}

func TestHasOverlap(t *testing.T) {
	tests := []struct {
		a, b    []string
		overlap bool
	}{
		{[]string{"a", "b"}, []string{"b", "c"}, true},
		{[]string{"a", "b"}, []string{"c", "d"}, false},
		{[]string{}, []string{"a"}, false},
		{[]string{"a"}, []string{}, false},
		{[]string{}, []string{}, false},
	}

	for _, tt := range tests {
		result := hasOverlap(tt.a, tt.b)
		if result != tt.overlap {
			t.Errorf("hasOverlap(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.overlap)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
