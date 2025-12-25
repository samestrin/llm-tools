package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// MockLLMClient is a mock for testing LLM commands.
type MockLLMClient struct {
	Response string
	Error    error
}

func (m *MockLLMClient) Complete(prompt string, timeout time.Duration) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.Response, nil
}

func TestMatchClarificationCmd_FoundMatch(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "match-test")
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
			CurrentAnswer:     "Use Vitest for unit tests",
			Occurrences:       5,
			Status:            "pending",
		},
		{
			ID:                "clr-20250116-def456",
			CanonicalQuestion: "What database should we use?",
			CurrentAnswer:     "PostgreSQL",
			Occurrences:       3,
			Status:            "promoted",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	matchFile = ""
	matchQuestion = ""

	cmd := newTestRootCmd()
	matchCmd := *matchClarificationCmd
	matchCmd.ResetFlags()
	matchCmd.Flags().StringVarP(&matchFile, "file", "f", "", "Tracking file path")
	matchCmd.Flags().StringVarP(&matchQuestion, "question", "q", "", "Question to match")
	cmd.AddCommand(&matchCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	// Question similar to existing "What testing framework should we use?"
	cmd.SetArgs([]string{"match-clarification", "-f", trackingPath, "-q", "Which testing framework?"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"match_id": "clr-20250115-abc123", "confidence": 0.85, "reason": "Similar question about testing framework"}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("match-clarification failed: %v", err)
	}

	// Verify JSON output
	var result MatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "matched" {
		t.Errorf("expected status 'matched', got %s", result.Status)
	}
	if result.MatchID != "clr-20250115-abc123" {
		t.Errorf("expected match_id 'clr-20250115-abc123', got %s", result.MatchID)
	}
}

func TestMatchClarificationCmd_NoMatch(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "match-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Vitest",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	matchFile = ""
	matchQuestion = ""

	cmd := newTestRootCmd()
	matchCmd := *matchClarificationCmd
	matchCmd.ResetFlags()
	matchCmd.Flags().StringVarP(&matchFile, "file", "f", "", "Tracking file path")
	matchCmd.Flags().StringVarP(&matchQuestion, "question", "q", "", "Question to match")
	cmd.AddCommand(&matchCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"match-clarification", "-f", trackingPath, "-q", "What color scheme should we use?"})

	// Set mock LLM client - no match
	SetLLMClient(&MockLLMClient{
		Response: `{"match_id": null, "confidence": 0, "reason": "No similar questions found"}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("match-clarification failed: %v", err)
	}

	// Verify JSON output
	var result MatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_match" {
		t.Errorf("expected status 'no_match', got %s", result.Status)
	}
}

func TestMatchClarificationCmd_EmptyFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "match-test")
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
	matchFile = ""
	matchQuestion = ""

	cmd := newTestRootCmd()
	matchCmd := *matchClarificationCmd
	matchCmd.ResetFlags()
	matchCmd.Flags().StringVarP(&matchFile, "file", "f", "", "Tracking file path")
	matchCmd.Flags().StringVarP(&matchQuestion, "question", "q", "", "Question to match")
	cmd.AddCommand(&matchCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"match-clarification", "-f", trackingPath, "-q", "Any question?"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("match-clarification failed: %v", err)
	}

	// Verify JSON output - should return no_match for empty file
	var result MatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_match" {
		t.Errorf("expected status 'no_match' for empty file, got %s", result.Status)
	}
}

func TestMatchClarificationCmd_RequiredFlags(t *testing.T) {
	cmd := newTestRootCmd()
	matchCmd := *matchClarificationCmd
	matchCmd.ResetFlags()
	matchCmd.Flags().StringVarP(&matchFile, "file", "f", "", "Tracking file path")
	matchCmd.Flags().StringVarP(&matchQuestion, "question", "q", "", "Question to match")
	matchCmd.MarkFlagRequired("file")
	matchCmd.MarkFlagRequired("question")
	cmd.AddCommand(&matchCmd)

	cmd.SetArgs([]string{"match-clarification"}) // No flags

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when required flags missing")
	}
}
