package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestAlignmentCheckFullAlignment tests full alignment (all requirements met)
func TestAlignmentCheckFullAlignment(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
## REQ-1: User authentication
## REQ-2: Data validation
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	story := filepath.Join(storiesDir, "01-auth.md")
	storyContent := `# Story 1
Status: Complete
Traces to: REQ-1, REQ-2
`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.AlignmentScore != 100.0 {
		t.Errorf("alignment_score = %.1f, want 100.0", result.AlignmentScore)
	}

	if result.RequirementsMet != 2 {
		t.Errorf("requirements_met = %d, want 2", result.RequirementsMet)
	}
}

// TestAlignmentCheckPartialAlignment tests partial alignment
func TestAlignmentCheckPartialAlignment(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
## REQ-1: User authentication
## REQ-2: Data validation
## REQ-3: Error handling
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	story := filepath.Join(storiesDir, "01-auth.md")
	storyContent := `# Story 1
Status: Complete
Traces to: REQ-1
`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	story2 := filepath.Join(storiesDir, "02-data.md")
	story2Content := `# Story 2
Status: Partial
Traces to: REQ-2
`
	if err := os.WriteFile(story2, []byte(story2Content), 0644); err != nil {
		t.Fatalf("failed to create story2: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// REQ-1: met (1.0), REQ-2: partial (0.5), REQ-3: unmet (0)
	// Score = (1 + 0.5 + 0) / 3 * 100 = 50%
	if result.RequirementsMet != 1 {
		t.Errorf("requirements_met = %d, want 1", result.RequirementsMet)
	}

	if result.RequirementsPartial != 1 {
		t.Errorf("requirements_partial = %d, want 1", result.RequirementsPartial)
	}

	if result.RequirementsUnmet != 1 {
		t.Errorf("requirements_unmet = %d, want 1", result.RequirementsUnmet)
	}
}

// TestAlignmentCheckNoAlignment tests zero alignment
func TestAlignmentCheckNoAlignment(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
## REQ-1: User authentication
## REQ-2: Data validation
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Story that doesn't trace to any requirements
	story := filepath.Join(storiesDir, "01-other.md")
	storyContent := `# Story 1
Status: Complete
Something unrelated.
`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.AlignmentScore != 0.0 {
		t.Errorf("alignment_score = %.1f, want 0.0", result.AlignmentScore)
	}

	if result.RequirementsUnmet != 2 {
		t.Errorf("requirements_unmet = %d, want 2", result.RequirementsUnmet)
	}
}

// TestAlignmentCheckGapsArray tests gap details output
func TestAlignmentCheckGapsArray(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: Auth
## REQ-2: Data
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Only cover REQ-1
	story := filepath.Join(storiesDir, "story.md")
	storyContent := `Traces to: REQ-1
Status: Complete
`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.Gaps) != 1 {
		t.Fatalf("gaps count = %d, want 1", len(result.Gaps))
	}

	if result.Gaps[0].RequirementID != "REQ-2" {
		t.Errorf("gap requirement_id = %s, want REQ-2", result.Gaps[0].RequirementID)
	}

	if result.Gaps[0].Status != "unmet" {
		t.Errorf("gap status = %s, want unmet", result.Gaps[0].Status)
	}
}

// TestAlignmentCheckScopeCreep tests scope creep detection
func TestAlignmentCheckScopeCreep(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: Auth`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Story tracing to REQ-1
	story1 := filepath.Join(storiesDir, "01-auth.md")
	story1Content := `# Auth Story
Traces to: REQ-1
Status: Complete
`
	if err := os.WriteFile(story1, []byte(story1Content), 0644); err != nil {
		t.Fatalf("failed to create story1: %v", err)
	}

	// Story with no requirement trace (scope creep)
	story2 := filepath.Join(storiesDir, "02-bonus.md")
	story2Content := `# Bonus Feature
This adds extra functionality not in requirements.
Status: Complete
`
	if err := os.WriteFile(story2, []byte(story2Content), 0644); err != nil {
		t.Fatalf("failed to create story2: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.ScopeCreep) == 0 {
		t.Error("expected scope_creep to contain the bonus story")
	}
}

// TestAlignmentCheckMissingRequirements tests missing requirements file
func TestAlignmentCheckMissingRequirements(t *testing.T) {
	tmpDir := t.TempDir()

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--requirements", "/nonexistent/requirements.md", "--stories", storiesDir})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing requirements file")
	}
}

// TestAlignmentCheckMissingStoriesDir tests missing stories directory
func TestAlignmentCheckMissingStoriesDir(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	if err := os.WriteFile(reqsFile, []byte("## REQ-1: Test"), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", "/nonexistent/stories"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing stories directory")
	}
}

// TestAlignmentCheckEmptyRequirements tests empty requirements
func TestAlignmentCheckEmptyRequirements(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	if err := os.WriteFile(reqsFile, []byte("# No requirements here"), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.TotalRequirements != 0 {
		t.Errorf("total_requirements = %d, want 0", result.TotalRequirements)
	}
}

// TestAlignmentCheckNumberedListFormat tests numbered list requirement format
func TestAlignmentCheckNumberedListFormat(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `# Requirements
1. User authentication system
2. Input validation
3. Error handling
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	// Use the numbered format REQ-1, REQ-2, REQ-3 in stories
	story := filepath.Join(storiesDir, "story.md")
	storyContent := `Traces to: REQ-1, REQ-2, REQ-3
Status: Complete
`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.TotalRequirements != 3 {
		t.Errorf("total_requirements = %d, want 3", result.TotalRequirements)
	}

	if result.AlignmentScore != 100.0 {
		t.Errorf("alignment_score = %.1f, want 100.0", result.AlignmentScore)
	}
}

// TestAlignmentCheckRequirementDetails tests detailed requirement info
func TestAlignmentCheckRequirementDetails(t *testing.T) {
	tmpDir := t.TempDir()

	reqsFile := filepath.Join(tmpDir, "requirements.md")
	reqsContent := `## REQ-1: Authentication
## REQ-2: Validation
`
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0644); err != nil {
		t.Fatalf("failed to create requirements file: %v", err)
	}

	storiesDir := filepath.Join(tmpDir, "user-stories")
	if err := os.MkdirAll(storiesDir, 0755); err != nil {
		t.Fatalf("failed to create stories dir: %v", err)
	}

	story := filepath.Join(storiesDir, "story.md")
	storyContent := `Traces to: REQ-1
Status: Complete
`
	if err := os.WriteFile(story, []byte(storyContent), 0644); err != nil {
		t.Fatalf("failed to create story: %v", err)
	}

	cmd := newAlignmentCheckCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--requirements", reqsFile, "--stories", storiesDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AlignmentCheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.RequirementDetails) != 2 {
		t.Fatalf("requirement_details count = %d, want 2", len(result.RequirementDetails))
	}
}
