package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestValidateRisksTableFormat tests parsing risks from table format
func TestValidateRisksTableFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create sprint-design.md with table format risks
	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `# Sprint Design

## Risk Analysis

| Risk | Impact | Mitigation |
|------|--------|------------|
| R-1: Performance degradation | High | Implement caching |
| R-2: Data loss | Critical | Add backup system |
| R-3: API breaking changes | Medium | Version the API |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	// Create empty stories dir
	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.RisksIdentified != 3 {
		t.Errorf("risks_identified = %d, want 3", result.RisksIdentified)
	}
}

// TestValidateRisksListFormat tests parsing risks from list format
func TestValidateRisksListFormat(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `# Sprint Design

## Risk Analysis

- **R-1**: Performance issues under load
- **R-2**: Memory leaks in long-running processes
- **R-3**: Authentication bypass vulnerability
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksIdentified != 3 {
		t.Errorf("risks_identified = %d, want 3", result.RisksIdentified)
	}
}

// TestValidateRisksWithCoverage tests risk coverage detection
func TestValidateRisksWithCoverage(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `# Sprint Design

## Risk Analysis

| Risk | Impact | Mitigation |
|------|--------|------------|
| R-1: Performance issues | High | Caching |
| R-2: Data corruption | Critical | Validation |
| R-3: API breaking | Medium | Versioning |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Story that addresses R-1
	story1 := filepath.Join(storiesDir, "01-perf.md")
	story1Content := `# Story 1: Performance Optimization
Addresses R-1 by implementing caching layer.
`
	if err := os.WriteFile(story1, []byte(story1Content), 0644); err != nil {
		t.Fatalf("failed to create story1: %v", err)
	}

	// Story that addresses R-2
	story2 := filepath.Join(storiesDir, "02-data.md")
	story2Content := `# Story 2: Data Integrity
Mitigates R-2 with input validation.
`
	if err := os.WriteFile(story2, []byte(story2Content), 0644); err != nil {
		t.Fatalf("failed to create story2: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksIdentified != 3 {
		t.Errorf("risks_identified = %d, want 3", result.RisksIdentified)
	}

	if result.RisksAddressed != 2 {
		t.Errorf("risks_addressed = %d, want 2 (R-1 and R-2)", result.RisksAddressed)
	}

	if len(result.RisksUnaddressed) != 1 {
		t.Errorf("risks_unaddressed count = %d, want 1 (R-3)", len(result.RisksUnaddressed))
	}

	// Coverage should be 66.7%
	expectedPct := 66.7
	if result.CoveragePercentage < expectedPct-1 || result.CoveragePercentage > expectedPct+1 {
		t.Errorf("coverage_percentage = %.1f, want ~%.1f", result.CoveragePercentage, expectedPct)
	}
}

// TestValidateRisksEmptyRiskSection tests empty Risk Analysis section
func TestValidateRisksEmptyRiskSection(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `# Sprint Design

## Risk Analysis

No risks identified for this sprint.
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksIdentified != 0 {
		t.Errorf("risks_identified = %d, want 0", result.RisksIdentified)
	}
}

// TestValidateRisksMissingRiskSection tests missing Risk Analysis section
func TestValidateRisksMissingRiskSection(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `# Sprint Design

## Overview
This sprint focuses on new features.

## Tasks
- Task 1
- Task 2
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksIdentified != 0 {
		t.Errorf("risks_identified = %d, want 0 (no risk section)", result.RisksIdentified)
	}
}

// TestValidateRisksMissingDesignFile tests missing design file error
func TestValidateRisksMissingDesignFile(t *testing.T) {
	tmpDir := t.TempDir()

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--design", "/nonexistent/sprint-design.md", "--stories", storiesDir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing design file, got nil")
	}
}

// TestValidateRisksMissingStoriesDir tests missing stories directory error
func TestValidateRisksMissingStoriesDir(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `## Risk Analysis
| Risk | Impact |
| R-1: Test | High |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", "/nonexistent/stories"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing stories directory, got nil")
	}
}

// TestValidateRisksWithTasks tests coverage from tasks directory
func TestValidateRisksWithTasks(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `## Risk Analysis

| Risk | Impact | Mitigation |
|------|--------|------------|
| R-1: Build failure | High | CI improvements |
| R-2: Deploy issues | Medium | Rollback strategy |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	tasksDir := filepath.Join(tmpDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	// Task that addresses R-1
	task1 := filepath.Join(tasksDir, "01-ci.md")
	task1Content := `# Task 1: CI Improvements
Addresses R-1 by adding build validation.
`
	if err := os.WriteFile(task1, []byte(task1Content), 0644); err != nil {
		t.Fatalf("failed to create task1: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--tasks", tasksDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksAddressed != 1 {
		t.Errorf("risks_addressed = %d, want 1", result.RisksAddressed)
	}
}

// TestValidateRisksWithAcceptanceCriteria tests coverage from AC directory
func TestValidateRisksWithAcceptanceCriteria(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `## Risk Analysis

| Risk | Impact | Mitigation |
|------|--------|------------|
| R-1: Security vuln | Critical | Input validation |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	acDir := filepath.Join(tmpDir, "acceptance-criteria")
	if err := os.MkdirAll(acDir, 0755); err != nil {
		t.Fatalf("failed to create AC dir: %v", err)
	}

	// AC that addresses R-1
	ac1 := filepath.Join(acDir, "01-01-security.md")
	ac1Content := `# AC: Security Validation
Addresses R-1 by requiring input sanitization.
`
	if err := os.WriteFile(ac1, []byte(ac1Content), 0644); err != nil {
		t.Fatalf("failed to create AC: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--acceptance-criteria", acDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksAddressed != 1 {
		t.Errorf("risks_addressed = %d, want 1", result.RisksAddressed)
	}

	if result.CoveragePercentage != 100.0 {
		t.Errorf("coverage_percentage = %.1f, want 100.0", result.CoveragePercentage)
	}
}

// TestValidateRisksRiskDetails tests that risk details are populated
func TestValidateRisksRiskDetails(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `## Risk Analysis

| Risk | Impact | Mitigation |
|------|--------|------------|
| R-1: Test risk | High | Test mitigation |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	story1 := filepath.Join(storiesDir, "01-story.md")
	story1Content := `# Story addressing R-1`
	if err := os.WriteFile(story1, []byte(story1Content), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.RiskDetails) != 1 {
		t.Fatalf("risk_details count = %d, want 1", len(result.RiskDetails))
	}

	detail := result.RiskDetails[0]
	if detail.ID != "R-1" {
		t.Errorf("risk ID = %s, want R-1", detail.ID)
	}
	if !detail.Covered {
		t.Error("risk should be marked as covered")
	}
	if len(detail.CoveredBy) == 0 {
		t.Error("covered_by should not be empty")
	}
}

// TestValidateRisksFullCoverage tests 100% coverage scenario
func TestValidateRisksFullCoverage(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `## Risk Analysis
| Risk | Impact |
|------|--------|
| R-1: First | Low |
| R-2: Second | Low |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	story := filepath.Join(storiesDir, "story.md")
	storyContent := `Addresses R-1 and R-2.`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.CoveragePercentage != 100.0 {
		t.Errorf("coverage_percentage = %.1f, want 100.0", result.CoveragePercentage)
	}

	if len(result.RisksUnaddressed) != 0 {
		t.Errorf("risks_unaddressed should be empty, got %v", result.RisksUnaddressed)
	}
}

// TestValidateRisksPotentialRisksSection tests "Potential Risks" heading variant
func TestValidateRisksPotentialRisksSection(t *testing.T) {
	tmpDir := t.TempDir()

	designFile := filepath.Join(tmpDir, "sprint-design.md")
	designContent := `# Sprint Design

## Potential Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| R-1: Test risk | High | Test mitigation |
`
	if err := os.WriteFile(designFile, []byte(designContent), 0644); err != nil {
		t.Fatalf("failed to create design file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newValidateRisksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--design", designFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result ValidateRisksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.RisksIdentified != 1 {
		t.Errorf("risks_identified = %d, want 1", result.RisksIdentified)
	}
}
