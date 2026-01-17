package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCoverageReportBasicCoverage tests basic requirement coverage calculation
func TestCoverageReportBasicCoverage(t *testing.T) {
	// Create temp dir with test files
	tmpDir := t.TempDir()

	// Create requirements file with 5 requirements
	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Original Requirements

## REQ-1: User Authentication
The system must support user authentication.

## REQ-2: Data Validation
All input data must be validated.

## REQ-3: Error Handling
Errors must be handled gracefully.

## REQ-4: Logging
The system must log all operations.

## REQ-5: Performance
Response time must be under 200ms.
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	// Create user stories directory
	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Create user stories that cover some requirements
	story1 := filepath.Join(storiesDir, "01-auth.md")
	story1Content := `# User Story 1: Authentication
Covers requirements: REQ-1, REQ-3
`
	if err := os.WriteFile(story1, []byte(story1Content), 0644); err != nil {
		t.Fatalf("failed to create story1: %v", err)
	}

	story2 := filepath.Join(storiesDir, "02-data.md")
	story2Content := `# User Story 2: Data Processing
This addresses REQ-2.
`
	if err := os.WriteFile(story2, []byte(story2Content), 0644); err != nil {
		t.Fatalf("failed to create story2: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, buf.String())
	}

	// Should have 5 total, 3 covered (REQ-1, REQ-2, REQ-3), 2 uncovered (REQ-4, REQ-5)
	if result.TotalRequirements != 5 {
		t.Errorf("total_requirements = %d, want 5", result.TotalRequirements)
	}

	if result.CoveredCount != 3 {
		t.Errorf("covered_count = %d, want 3", result.CoveredCount)
	}

	if len(result.UncoveredRequirements) != 2 {
		t.Errorf("uncovered_requirements count = %d, want 2", len(result.UncoveredRequirements))
	}

	// Coverage should be 60%
	expectedPct := 60.0
	if result.CoveragePercentage < expectedPct-0.1 || result.CoveragePercentage > expectedPct+0.1 {
		t.Errorf("coverage_percentage = %.1f, want %.1f", result.CoveragePercentage, expectedPct)
	}
}

// TestCoverageReportFullCoverage tests 100% coverage scenario
func TestCoverageReportFullCoverage(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
## REQ-1: First
## REQ-2: Second
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	storyFile := filepath.Join(storiesDir, "story.md")
	storyContent := `# Story
Covers REQ-1 and REQ-2.
`
	if err := os.WriteFile(storyFile, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.CoveragePercentage != 100.0 {
		t.Errorf("coverage_percentage = %.1f, want 100.0", result.CoveragePercentage)
	}

	if len(result.UncoveredRequirements) != 0 {
		t.Errorf("uncovered_requirements should be empty, got %v", result.UncoveredRequirements)
	}
}

// TestCoverageReportZeroCoverage tests 0% coverage scenario
func TestCoverageReportZeroCoverage(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
## REQ-1: First
## REQ-2: Second
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Story that doesn't reference any requirements
	storyFile := filepath.Join(storiesDir, "story.md")
	storyContent := `# Story
This story doesn't reference any requirements.
`
	if err := os.WriteFile(storyFile, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.CoveragePercentage != 0.0 {
		t.Errorf("coverage_percentage = %.1f, want 0.0", result.CoveragePercentage)
	}

	if len(result.UncoveredRequirements) != 2 {
		t.Errorf("uncovered_requirements count = %d, want 2", len(result.UncoveredRequirements))
	}
}

// TestCoverageReportEmptyRequirements tests empty requirements file
func TestCoverageReportEmptyRequirements(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
No requirement IDs in this file.
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.TotalRequirements != 0 {
		t.Errorf("total_requirements = %d, want 0", result.TotalRequirements)
	}

	// 0/0 should result in 0% coverage (not NaN or error)
	if result.CoveragePercentage != 0.0 {
		t.Errorf("coverage_percentage = %.1f, want 0.0", result.CoveragePercentage)
	}
}

// TestCoverageReportMissingRequirementsFile tests missing requirements file error
func TestCoverageReportMissingRequirementsFile(t *testing.T) {
	tmpDir := t.TempDir()

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--requirements", "/nonexistent/requirements.md", "--stories", storiesDir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing requirements file, got nil")
	}
}

// TestCoverageReportMissingStoriesDir tests missing stories directory error
func TestCoverageReportMissingStoriesDir(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: First`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", "/nonexistent/stories"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing stories directory, got nil")
	}
}

// TestCoverageReportCoverageByStory tests the coverage-by-story mapping
func TestCoverageReportCoverageByStory(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: First
## REQ-2: Second
## REQ-3: Third
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	story1 := filepath.Join(storiesDir, "01-first.md")
	story1Content := `# Story 1
Covers REQ-1.
`
	if err := os.WriteFile(story1, []byte(story1Content), 0644); err != nil {
		t.Fatalf("failed to create story1: %v", err)
	}

	story2 := filepath.Join(storiesDir, "02-second.md")
	story2Content := `# Story 2
Addresses REQ-2 and REQ-3.
`
	if err := os.WriteFile(story2, []byte(story2Content), 0644); err != nil {
		t.Fatalf("failed to create story2: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Check coverage_by_story mapping
	if len(result.CoverageByStory) != 2 {
		t.Errorf("coverage_by_story has %d entries, want 2", len(result.CoverageByStory))
	}

	// Verify story1 covers REQ-1
	story1Reqs, ok := result.CoverageByStory["01-first.md"]
	if !ok {
		t.Error("missing 01-first.md in coverage_by_story")
	} else if len(story1Reqs) != 1 || story1Reqs[0] != "REQ-1" {
		t.Errorf("story1 requirements = %v, want [REQ-1]", story1Reqs)
	}

	// Verify story2 covers REQ-2 and REQ-3
	story2Reqs, ok := result.CoverageByStory["02-second.md"]
	if !ok {
		t.Error("missing 02-second.md in coverage_by_story")
	} else if len(story2Reqs) != 2 {
		t.Errorf("story2 requirements count = %d, want 2", len(story2Reqs))
	}
}

// TestCoverageReportDuplicateRequirements tests deduplication of requirement references
func TestCoverageReportDuplicateRequirements(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: First
## REQ-2: Second
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Story that references REQ-1 multiple times
	storyFile := filepath.Join(storiesDir, "story.md")
	storyContent := `# Story
Covers REQ-1 here.
Also REQ-1 again here.
And again REQ-1.
`
	if err := os.WriteFile(storyFile, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// REQ-1 should only be counted once
	if result.CoveredCount != 1 {
		t.Errorf("covered_count = %d, want 1 (deduplication)", result.CoveredCount)
	}
}

// TestCoverageReportMultipleIDFormats tests support for various requirement ID formats
func TestCoverageReportMultipleIDFormats(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: Standard format
## R-2: Short format
## REQUIREMENT-3: Long format
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	storyFile := filepath.Join(storiesDir, "story.md")
	storyContent := `# Story
Addresses REQ-1, R-2, and REQUIREMENT-3.
`
	if err := os.WriteFile(storyFile, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.TotalRequirements != 3 {
		t.Errorf("total_requirements = %d, want 3", result.TotalRequirements)
	}

	if result.CoveragePercentage != 100.0 {
		t.Errorf("coverage_percentage = %.1f, want 100.0", result.CoveragePercentage)
	}
}

// TestCoverageReportEmptyStoriesDir tests empty user stories directory
func TestCoverageReportEmptyStoriesDir(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: First`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}
	// No story files created - empty directory

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// With no stories, coverage should be 0%
	if result.CoveragePercentage != 0.0 {
		t.Errorf("coverage_percentage = %.1f, want 0.0", result.CoveragePercentage)
	}
}

// TestCoverageReportNonMarkdownFiles tests that non-markdown files are ignored
func TestCoverageReportNonMarkdownFiles(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: First`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Create a .txt file with REQ-1 reference (should be ignored)
	txtFile := filepath.Join(storiesDir, "notes.txt")
	txtContent := `REQ-1 is referenced here`
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("failed to create txt file: %v", err)
	}

	// Create a .md file without REQ-1 reference
	mdFile := filepath.Join(storiesDir, "story.md")
	mdContent := `# Story without requirements`
	if err := os.WriteFile(mdFile, []byte(mdContent), 0644); err != nil {
		t.Fatalf("failed to create md file: %v", err)
	}

	cmd := newCoverageReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result CoverageReportResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// The .txt file should be ignored, so REQ-1 should be uncovered
	if result.CoveredCount != 0 {
		t.Errorf("covered_count = %d, want 0 (txt files should be ignored)", result.CoveredCount)
	}
}
