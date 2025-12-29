package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestSuggestConsolidationCmd_FoundSuggestions(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "consolidate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with similar entries
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Vitest",
			Status:            "pending",
		},
		{
			ID:                "clr-20250116-def456",
			CanonicalQuestion: "Which test framework?",
			CurrentAnswer:     "Vitest",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	consolidateFile = ""

	cmd := newTestRootCmd()
	consCmd := *suggestConsolidationCmd
	consCmd.ResetFlags()
	consCmd.Flags().StringVarP(&consolidateFile, "file", "f", "", "Tracking file path")
	consCmd.Flags().BoolVar(&consolidateJSON, "json", false, "Output as JSON")
	consCmd.Flags().BoolVar(&consolidateMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&consCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"suggest-consolidation", "-f", trackingPath, "--json"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"suggestions": [{"primary_id": "clr-20250115-abc123", "merge_ids": ["clr-20250116-def456"], "reason": "Both ask about testing framework"}]}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("suggest-consolidation failed: %v", err)
	}

	// Verify JSON output
	var result ConsolidationResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "suggestions_found" {
		t.Errorf("expected status 'suggestions_found', got %s", result.Status)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 suggestion, got %d", result.Total)
	}
}

func TestSuggestConsolidationCmd_TooFewEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "consolidate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with only 1 entry
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "Only one question?",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	consolidateFile = ""

	cmd := newTestRootCmd()
	consCmd := *suggestConsolidationCmd
	consCmd.ResetFlags()
	consCmd.Flags().StringVarP(&consolidateFile, "file", "f", "", "Tracking file path")
	consCmd.Flags().BoolVar(&consolidateJSON, "json", false, "Output as JSON")
	consCmd.Flags().BoolVar(&consolidateMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&consCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"suggest-consolidation", "-f", trackingPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("suggest-consolidation failed: %v", err)
	}

	// Verify JSON output
	var result ConsolidationResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_suggestions" {
		t.Errorf("expected status 'no_suggestions', got %s", result.Status)
	}
}
