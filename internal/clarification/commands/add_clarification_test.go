package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestAddClarificationCmd_NewEntry(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "add-clarification-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial tracking file
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	addFile = ""
	addQuestion = ""
	addAnswer = ""
	addID = ""
	addSprint = ""
	addTags = nil
	addCheckMatch = false

	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addCmd.Flags().StringVar(&addID, "id", "", "Entry ID")
	addCmd.Flags().StringVarP(&addSprint, "sprint", "s", "", "Sprint name")
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Context tags")
	addCmd.Flags().BoolVar(&addCheckMatch, "check-match", false, "Check for similar questions")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "Output as JSON")
	addCmd.Flags().BoolVar(&addMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&addCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"add-clarification", "-f", trackingPath, "-q", "What framework?", "-a", "Use React", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add-clarification failed: %v", err)
	}

	// Verify JSON output
	var result AddClarificationResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "created" {
		t.Errorf("expected status 'created', got %s", result.Status)
	}
	if result.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Verify ID format: clr-YYYYMMDD-xxxxxx
	pattern := `^clr-\d{8}-[a-f0-9]{6}$`
	matched, _ := regexp.MatchString(pattern, result.ID)
	if !matched {
		t.Errorf("ID does not match expected format: %s", result.ID)
	}

	// Verify entry was added to file
	loaded, err := tracking.LoadTrackingFile(trackingPath)
	if err != nil {
		t.Fatalf("failed to load tracking file: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].CanonicalQuestion != "What framework?" {
		t.Errorf("expected question 'What framework?', got %s", loaded.Entries[0].CanonicalQuestion)
	}
}

func TestAddClarificationCmd_UpdateExisting(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "add-clarification-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with existing entry
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What framework?",
			CurrentAnswer:     "Use Vue",
			Occurrences:       1,
			FirstSeen:         "2025-01-15",
			LastSeen:          "2025-01-15",
			Status:            "pending",
			Confidence:        "medium",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	addFile = ""
	addQuestion = ""
	addAnswer = ""
	addID = ""
	addSprint = ""
	addTags = nil
	addCheckMatch = false

	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addCmd.Flags().StringVar(&addID, "id", "", "Entry ID")
	addCmd.Flags().StringVarP(&addSprint, "sprint", "s", "", "Sprint name")
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Context tags")
	addCmd.Flags().BoolVar(&addCheckMatch, "check-match", false, "Check for similar questions")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "Output as JSON")
	addCmd.Flags().BoolVar(&addMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&addCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"add-clarification", "-f", trackingPath, "--id", "clr-20250115-abc123", "-a", "Use React", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add-clarification failed: %v", err)
	}

	// Verify JSON output
	var result AddClarificationResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "updated" {
		t.Errorf("expected status 'updated', got %s", result.Status)
	}

	// Verify entry was updated
	loaded, err := tracking.LoadTrackingFile(trackingPath)
	if err != nil {
		t.Fatalf("failed to load tracking file: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].CurrentAnswer != "Use React" {
		t.Errorf("expected answer 'Use React', got %s", loaded.Entries[0].CurrentAnswer)
	}
	if loaded.Entries[0].Occurrences != 2 {
		t.Errorf("expected occurrences 2, got %d", loaded.Entries[0].Occurrences)
	}
}

func TestAddClarificationCmd_WithSprintAndTags(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "add-clarification-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial tracking file
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	addFile = ""
	addQuestion = ""
	addAnswer = ""
	addID = ""
	addSprint = ""
	addTags = nil
	addCheckMatch = false

	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addCmd.Flags().StringVar(&addID, "id", "", "Entry ID")
	addCmd.Flags().StringVarP(&addSprint, "sprint", "s", "", "Sprint name")
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Context tags")
	addCmd.Flags().BoolVar(&addCheckMatch, "check-match", false, "Check for similar questions")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "Output as JSON")
	addCmd.Flags().BoolVar(&addMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&addCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"add-clarification",
		"-f", trackingPath,
		"-q", "What database?",
		"-a", "PostgreSQL",
		"-s", "sprint-2.0",
		"-t", "backend",
		"-t", "database",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add-clarification failed: %v", err)
	}

	// Verify entry has sprint and tags
	loaded, err := tracking.LoadTrackingFile(trackingPath)
	if err != nil {
		t.Fatalf("failed to load tracking file: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
	}

	entry := loaded.Entries[0]
	if len(entry.SprintsSeen) != 1 || entry.SprintsSeen[0] != "sprint-2.0" {
		t.Errorf("expected sprint 'sprint-2.0', got %v", entry.SprintsSeen)
	}
	if len(entry.ContextTags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(entry.ContextTags))
	}
}

func TestAddClarificationCmd_RequiredFlags(t *testing.T) {
	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addCmd.MarkFlagRequired("file")
	cmd.AddCommand(&addCmd)

	cmd.SetArgs([]string{"add-clarification"}) // No flags

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when required flags missing")
	}
}

func TestAddClarificationCmd_FileNotFound(t *testing.T) {
	// Reset flags
	addFile = ""
	addQuestion = ""
	addAnswer = ""
	addID = ""

	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	cmd.AddCommand(&addCmd)

	cmd.SetArgs([]string{"add-clarification", "-f", "/nonexistent/tracking.yaml", "-q", "Test?", "-a", "Test"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestAddClarificationCmd_CheckMatch(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "add-clarification-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with existing entry
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Vitest",
			Occurrences:       1,
			FirstSeen:         "2025-01-15",
			LastSeen:          "2025-01-15",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	addFile = ""
	addQuestion = ""
	addAnswer = ""
	addID = ""
	addSprint = ""
	addTags = nil
	addCheckMatch = false

	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addCmd.Flags().StringVar(&addID, "id", "", "Entry ID")
	addCmd.Flags().StringVarP(&addSprint, "sprint", "s", "", "Sprint name")
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Context tags")
	addCmd.Flags().BoolVar(&addCheckMatch, "check-match", false, "Check for similar questions")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "Output as JSON")
	addCmd.Flags().BoolVar(&addMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&addCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	// Ask a similar question with --check-match
	cmd.SetArgs([]string{"add-clarification",
		"-f", trackingPath,
		"-q", "Which testing framework?", // Similar to existing
		"-a", "Still Vitest",
		"--check-match",
		"--json",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add-clarification failed: %v", err)
	}

	// Verify the response includes potential matches
	var result AddClarificationResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// With --check-match, should find similar question
	if len(result.PotentialMatches) == 0 {
		t.Error("expected to find potential matches with --check-match")
	}
}

func TestAddClarificationCmd_JSONOutput(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "add-clarification-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create initial tracking file
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	addFile = ""
	addQuestion = ""
	addAnswer = ""

	cmd := newTestRootCmd()
	addCmd := *addClarificationCmd
	addCmd.ResetFlags()
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "Tracking file path")
	addCmd.Flags().StringVarP(&addQuestion, "question", "q", "", "Question text")
	addCmd.Flags().StringVarP(&addAnswer, "answer", "a", "", "Answer text")
	addCmd.Flags().StringVar(&addID, "id", "", "Entry ID")
	addCmd.Flags().StringVarP(&addSprint, "sprint", "s", "", "Sprint name")
	addCmd.Flags().StringSliceVarP(&addTags, "tag", "t", nil, "Context tags")
	addCmd.Flags().BoolVar(&addCheckMatch, "check-match", false, "Check for similar questions")
	addCmd.Flags().BoolVar(&addJSON, "json", false, "Output as JSON")
	addCmd.Flags().BoolVar(&addMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&addCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"add-clarification", "-f", trackingPath, "-q", "Test?", "-a", "Test answer", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("add-clarification failed: %v", err)
	}

	// Verify output is valid JSON with expected fields
	output := stdout.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	requiredFields := []string{"status", "id", "message"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}
