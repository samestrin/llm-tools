package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePlanValid(t *testing.T) {
	// Create valid plan structure
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	createValidPlanStructure(t, planDir)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", planDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error for valid plan: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "STATUS: VALID") {
		t.Errorf("expected VALID status, got: %s", output)
	}
}

func TestValidatePlanValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	createValidPlanStructure(t, planDir)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", planDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result PlanValidationResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid=true in JSON output")
	}
	if len(result.RequiredFiles) == 0 {
		t.Error("expected required files in JSON output")
	}
}

func TestValidatePlanMissingPlanMD(t *testing.T) {
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	// Create structure without plan.md
	os.MkdirAll(filepath.Join(planDir, "user-stories"), 0755)
	os.MkdirAll(filepath.Join(planDir, "acceptance-criteria"), 0755)
	os.WriteFile(filepath.Join(planDir, "user-stories", "01-story.md"), []byte("# Story"), 0644)
	os.WriteFile(filepath.Join(planDir, "acceptance-criteria", "01-01-criteria.md"), []byte("# AC"), 0644)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", planDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing plan.md")
	}

	output := buf.String()
	if !strings.Contains(output, "missing required file: plan.md") {
		t.Errorf("expected 'missing required file: plan.md' in output, got: %s", output)
	}
}

func TestValidatePlanMissingUserStories(t *testing.T) {
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	// Create structure without user-stories
	os.MkdirAll(filepath.Join(planDir, "acceptance-criteria"), 0755)
	os.WriteFile(filepath.Join(planDir, "plan.md"), []byte("# Plan"), 0644)
	os.WriteFile(filepath.Join(planDir, "acceptance-criteria", "01-01-criteria.md"), []byte("# AC"), 0644)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", planDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing user-stories/")
	}

	output := buf.String()
	if !strings.Contains(output, "missing required directory: user-stories/") {
		t.Errorf("expected 'missing required directory: user-stories/' in output, got: %s", output)
	}
}

func TestValidatePlanNotExists(t *testing.T) {
	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", "/nonexistent/plan"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !strings.Contains(err.Error(), "plan directory not found") {
		t.Errorf("expected 'plan directory not found' error, got: %v", err)
	}
}

func TestValidatePlanNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not-a-dir.txt")
	os.WriteFile(filePath, []byte("content"), 0644)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--path", filePath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for file instead of directory")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' error, got: %v", err)
	}
}

func TestValidatePlanWithFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	createValidPlanStructure(t, planDir)

	// Add frontmatter to plan.md
	planMD := `---
title: Test Plan
type: feature
status: active
---

# Test Plan

Some content here.
`
	os.WriteFile(filepath.Join(planDir, "plan.md"), []byte(planMD), 0644)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", planDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result PlanValidationResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Metadata["title"] != "Test Plan" {
		t.Errorf("expected metadata title 'Test Plan', got: %s", result.Metadata["title"])
	}
	if result.Metadata["type"] != "feature" {
		t.Errorf("expected metadata type 'feature', got: %s", result.Metadata["type"])
	}
}

func TestValidatePlanEmptyDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	// Create structure with empty directories
	os.MkdirAll(filepath.Join(planDir, "user-stories"), 0755)
	os.MkdirAll(filepath.Join(planDir, "acceptance-criteria"), 0755)
	os.WriteFile(filepath.Join(planDir, "plan.md"), []byte("# Plan"), 0644)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", planDir, "--json"})

	// Should succeed but with warnings
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result PlanValidationResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warnings for empty directories")
	}

	hasEmptyWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "is empty") {
			hasEmptyWarning = true
			break
		}
	}
	if !hasEmptyWarning {
		t.Error("expected 'is empty' warning")
	}
}

func TestValidatePlanWithOptionalFiles(t *testing.T) {
	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, "test-plan")

	createValidPlanStructure(t, planDir)

	// Add optional files
	os.WriteFile(filepath.Join(planDir, "original-request.md"), []byte("# Request"), 0644)
	os.WriteFile(filepath.Join(planDir, "sprint-design.md"), []byte("# Design"), 0644)
	os.MkdirAll(filepath.Join(planDir, "documentation"), 0755)

	cmd := newValidatePlanCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", planDir, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result PlanValidationResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check optional files are detected
	foundRequest := false
	foundDesign := false
	foundDocs := false

	for _, f := range result.OptionalFiles {
		if f.Path == "original-request.md" && f.Exists {
			foundRequest = true
		}
		if f.Path == "sprint-design.md" && f.Exists {
			foundDesign = true
		}
		if f.Path == "documentation/" && f.Exists {
			foundDocs = true
		}
	}

	if !foundRequest {
		t.Error("expected original-request.md to be detected")
	}
	if !foundDesign {
		t.Error("expected sprint-design.md to be detected")
	}
	if !foundDocs {
		t.Error("expected documentation/ to be detected")
	}
}

func TestExtractPlanMetadata(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name: "yaml frontmatter",
			content: `---
title: My Plan
type: feature
status: active
---

# Plan content`,
			expected: map[string]string{
				"title":  "My Plan",
				"type":   "feature",
				"status": "active",
			},
		},
		{
			name: "markdown bold keys",
			content: `# Plan

**Type:** Feature
**Priority:** High
**Status:** Active`,
			expected: map[string]string{
				"Type":     "Feature",
				"Priority": "High",
				"Status":   "Active",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPlanMetadata(tt.content)
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("expected %s=%s, got %s=%s", key, expectedValue, key, result[key])
				}
			}
		})
	}
}

// Helper function
func createValidPlanStructure(t *testing.T, planDir string) {
	t.Helper()

	os.MkdirAll(filepath.Join(planDir, "user-stories"), 0755)
	os.MkdirAll(filepath.Join(planDir, "acceptance-criteria"), 0755)

	os.WriteFile(filepath.Join(planDir, "plan.md"), []byte("# Test Plan\n\nPlan content here."), 0644)
	os.WriteFile(filepath.Join(planDir, "user-stories", "01-story.md"), []byte("# User Story 01"), 0644)
	os.WriteFile(filepath.Join(planDir, "acceptance-criteria", "01-01-criteria.md"), []byte("# AC 01-01"), 0644)
}
