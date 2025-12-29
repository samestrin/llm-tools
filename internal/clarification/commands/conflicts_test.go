package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestDetectConflictsCmd_ConflictsFound(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "conflicts-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with conflicting entries
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
			CurrentAnswer:     "Use Jest",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	conflictsFile = ""

	cmd := newTestRootCmd()
	confCmd := *detectConflictsCmd
	confCmd.ResetFlags()
	confCmd.Flags().StringVarP(&conflictsFile, "file", "f", "", "Tracking file path")
	confCmd.Flags().BoolVar(&conflictsJSON, "json", false, "Output as JSON")
	confCmd.Flags().BoolVar(&conflictsMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&confCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"detect-conflicts", "-f", trackingPath, "--json"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"conflicts": [{"entry_ids": ["clr-20250115-abc123", "clr-20250116-def456"], "reason": "Both ask about testing framework but recommend different tools", "severity": "high", "suggestion": "Consolidate to one framework recommendation"}], "conflict_count": 1}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("detect-conflicts failed: %v", err)
	}

	// Verify JSON output
	var result ConflictsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "conflicts_found" {
		t.Errorf("expected status 'conflicts_found', got %s", result.Status)
	}
	if result.ConflictCount != 1 {
		t.Errorf("expected 1 conflict, got %d", result.ConflictCount)
	}
	if len(result.Conflicts) != 1 {
		t.Errorf("expected 1 conflict in array, got %d", len(result.Conflicts))
	}
}

func TestDetectConflictsCmd_NoConflicts(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "conflicts-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with non-conflicting entries
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
			CanonicalQuestion: "What database should we use?",
			CurrentAnswer:     "PostgreSQL",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	conflictsFile = ""

	cmd := newTestRootCmd()
	confCmd := *detectConflictsCmd
	confCmd.ResetFlags()
	confCmd.Flags().StringVarP(&conflictsFile, "file", "f", "", "Tracking file path")
	confCmd.Flags().BoolVar(&conflictsJSON, "json", false, "Output as JSON")
	confCmd.Flags().BoolVar(&conflictsMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&confCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"detect-conflicts", "-f", trackingPath, "--json"})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"conflicts": [], "conflict_count": 0}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("detect-conflicts failed: %v", err)
	}

	// Verify JSON output
	var result ConflictsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_conflicts" {
		t.Errorf("expected status 'no_conflicts', got %s", result.Status)
	}
	if result.ConflictCount != 0 {
		t.Errorf("expected 0 conflicts, got %d", result.ConflictCount)
	}
}

func TestDetectConflictsCmd_TooFewEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "conflicts-test")
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
	conflictsFile = ""

	cmd := newTestRootCmd()
	confCmd := *detectConflictsCmd
	confCmd.ResetFlags()
	confCmd.Flags().StringVarP(&conflictsFile, "file", "f", "", "Tracking file path")
	confCmd.Flags().BoolVar(&conflictsJSON, "json", false, "Output as JSON")
	confCmd.Flags().BoolVar(&conflictsMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&confCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"detect-conflicts", "-f", trackingPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("detect-conflicts failed: %v", err)
	}

	// Verify JSON output
	var result ConflictsResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "no_conflicts" {
		t.Errorf("expected status 'no_conflicts', got %s", result.Status)
	}
	if result.Note != "Not enough entries to detect conflicts" {
		t.Errorf("expected note about insufficient entries, got %s", result.Note)
	}
}
