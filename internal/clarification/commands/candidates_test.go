package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestIdentifyCandidatesCmd_FoundCandidates(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "candidates-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with eligible entries
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Vitest",
			Occurrences:       5,
			Status:            "pending",
		},
		{
			ID:                "clr-20250116-def456",
			CanonicalQuestion: "What database?",
			CurrentAnswer:     "PostgreSQL",
			Occurrences:       3,
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	candidatesFile = ""
	candidatesMinOccurrences = 3

	cmd := newTestRootCmd()
	candCmd := *identifyCandidatesCmd
	candCmd.ResetFlags()
	candCmd.Flags().StringVarP(&candidatesFile, "file", "f", "", "Tracking file path")
	candCmd.Flags().IntVar(&candidatesMinOccurrences, "min-occurrences", 3, "Minimum occurrences")
	cmd.AddCommand(&candCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"identify-candidates", "-f", trackingPath})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"candidates": [{"id": "clr-20250115-abc123", "confidence": 0.9, "reason": "High confidence, stable answer"}]}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("identify-candidates failed: %v", err)
	}

	// Verify JSON output
	var result CandidatesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "candidates_found" {
		t.Errorf("expected status 'candidates_found', got %s", result.Status)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 candidate, got %d", result.Total)
	}
}

func TestIdentifyCandidatesCmd_NoCandidates(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "candidates-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with low occurrence entries
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "Low occurrence question?",
			Occurrences:       1,
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	candidatesFile = ""
	candidatesMinOccurrences = 3

	cmd := newTestRootCmd()
	candCmd := *identifyCandidatesCmd
	candCmd.ResetFlags()
	candCmd.Flags().StringVarP(&candidatesFile, "file", "f", "", "Tracking file path")
	candCmd.Flags().IntVar(&candidatesMinOccurrences, "min-occurrences", 3, "Minimum occurrences")
	cmd.AddCommand(&candCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"identify-candidates", "-f", trackingPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("identify-candidates failed: %v", err)
	}

	// Verify JSON output
	var result CandidatesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_candidates" {
		t.Errorf("expected status 'no_candidates', got %s", result.Status)
	}
}
