package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestValidateClarificationsCmd_AllValid(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "validate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with entries
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Vitest",
			LastSeen:          "2025-01-15",
			Occurrences:       5,
			Status:            "pending",
		},
		{
			ID:                "clr-20250116-def456",
			CanonicalQuestion: "What database should we use?",
			CurrentAnswer:     "PostgreSQL",
			LastSeen:          "2025-01-14",
			Occurrences:       3,
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	validateFile = ""
	validateContext = ""

	cmd := newTestRootCmd()
	valCmd := *validateClarificationsCmd
	valCmd.ResetFlags()
	valCmd.Flags().StringVarP(&validateFile, "file", "f", "", "Tracking file path")
	valCmd.Flags().StringVarP(&validateContext, "context", "c", "", "Project context")
	cmd.AddCommand(&valCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"validate-clarifications", "-f", trackingPath, "-c", "Node.js project with Vitest"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"validations": [{"id": "clr-20250115-abc123", "status": "valid"}, {"id": "clr-20250116-def456", "status": "valid"}], "valid_count": 2, "stale_count": 0, "review_count": 0}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate-clarifications failed: %v", err)
	}

	// Verify JSON output
	var result ValidateClarificationsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "validated" {
		t.Errorf("expected status 'validated', got %s", result.Status)
	}
	if result.ValidCount != 2 {
		t.Errorf("expected 2 valid, got %d", result.ValidCount)
	}
	if result.StaleCount != 0 {
		t.Errorf("expected 0 stale, got %d", result.StaleCount)
	}
}

func TestValidateClarificationsCmd_SomeStale(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "validate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with old entries
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Jest",
			LastSeen:          "2024-06-01",
			Occurrences:       1,
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	validateFile = ""
	validateContext = ""

	cmd := newTestRootCmd()
	valCmd := *validateClarificationsCmd
	valCmd.ResetFlags()
	valCmd.Flags().StringVarP(&validateFile, "file", "f", "", "Tracking file path")
	valCmd.Flags().StringVarP(&validateContext, "context", "c", "", "Project context")
	cmd.AddCommand(&valCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"validate-clarifications", "-f", trackingPath, "-c", "Node.js project"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"validations": [{"id": "clr-20250115-abc123", "status": "stale", "reason": "Not seen in over 200 days", "recommendation": "Review and update or remove"}], "valid_count": 0, "stale_count": 1, "review_count": 0}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate-clarifications failed: %v", err)
	}

	// Verify JSON output
	var result ValidateClarificationsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "validated" {
		t.Errorf("expected status 'validated', got %s", result.Status)
	}
	if result.StaleCount != 1 {
		t.Errorf("expected 1 stale, got %d", result.StaleCount)
	}
}

func TestValidateClarificationsCmd_NoEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "validate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create empty tracking file
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	validateFile = ""
	validateContext = ""

	cmd := newTestRootCmd()
	valCmd := *validateClarificationsCmd
	valCmd.ResetFlags()
	valCmd.Flags().StringVarP(&validateFile, "file", "f", "", "Tracking file path")
	valCmd.Flags().StringVarP(&validateContext, "context", "c", "", "Project context")
	cmd.AddCommand(&valCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"validate-clarifications", "-f", trackingPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate-clarifications failed: %v", err)
	}

	// Verify JSON output
	var result ValidateClarificationsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_entries" {
		t.Errorf("expected status 'no_entries', got %s", result.Status)
	}
}

func TestDetectProjectContext(t *testing.T) {
	// This test just verifies the function doesn't panic
	// The actual detection depends on the test environment
	context := detectProjectContext()
	if context == "" {
		t.Error("expected non-empty context")
	}
}
