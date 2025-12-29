package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestPromoteCmd_Success(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "promote-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with entry to promote
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What testing framework should we use?",
			CurrentAnswer:     "Use Vitest for unit tests",
			Occurrences:       5,
			FirstSeen:         "2025-01-15",
			LastSeen:          "2025-01-20",
			Status:            "pending",
			Confidence:        "high",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Create target file
	targetPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(targetPath, []byte("# Project Guidelines\n\n"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Reset flags
	promoteFile = ""
	promoteID = ""
	promoteTarget = ""
	promoteForce = false

	cmd := newTestRootCmd()
	promoteCmd := *promoteClarificationCmd
	promoteCmd.ResetFlags()
	promoteCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path")
	promoteCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote")
	promoteCmd.Flags().StringVar(&promoteTarget, "target", "", "Target file (default: CLAUDE.md)")
	promoteCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion")
	promoteCmd.Flags().BoolVar(&promoteJSON, "json", false, "Output as JSON")
	promoteCmd.Flags().BoolVar(&promoteMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&promoteCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"promote-clarification", "-f", trackingPath, "--id", "clr-20250115-abc123", "--target", targetPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote-clarification failed: %v", err)
	}

	// Verify JSON output
	var result PromoteResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "promoted" {
		t.Errorf("expected status 'promoted', got %s", result.Status)
	}

	// Verify entry was updated in tracking file
	loaded, err := tracking.LoadTrackingFile(trackingPath)
	if err != nil {
		t.Fatalf("failed to load tracking file: %v", err)
	}
	if loaded.Entries[0].Status != "promoted" {
		t.Errorf("expected entry status 'promoted', got %s", loaded.Entries[0].Status)
	}
	if loaded.Entries[0].PromotedTo == "" {
		t.Error("expected PromotedTo to be set")
	}

	// Verify content was appended to target file
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}
	if !strings.Contains(string(content), "What testing framework should we use?") {
		t.Error("expected question to be in target file")
	}
	if !strings.Contains(string(content), "Use Vitest for unit tests") {
		t.Error("expected answer to be in target file")
	}
}

func TestPromoteCmd_AlreadyPromoted(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "promote-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with already promoted entry
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What database?",
			CurrentAnswer:     "PostgreSQL",
			Occurrences:       3,
			FirstSeen:         "2025-01-15",
			LastSeen:          "2025-01-20",
			Status:            "promoted",
			PromotedTo:        "CLAUDE.md",
			PromotedDate:      "2025-01-19",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	promoteFile = ""
	promoteID = ""
	promoteTarget = ""
	promoteForce = false

	cmd := newTestRootCmd()
	promoteCmd := *promoteClarificationCmd
	promoteCmd.ResetFlags()
	promoteCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path")
	promoteCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote")
	promoteCmd.Flags().StringVar(&promoteTarget, "target", "", "Target file")
	promoteCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion")
	cmd.AddCommand(&promoteCmd)

	cmd.SetArgs([]string{"promote-clarification", "-f", trackingPath, "--id", "clr-20250115-abc123"})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when promoting already promoted entry")
	}
	if err != nil && !strings.Contains(err.Error(), "already promoted") {
		t.Errorf("expected 'already promoted' error, got: %v", err)
	}
}

func TestPromoteCmd_ForceRePromote(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "promote-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with already promoted entry
	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What database?",
			CurrentAnswer:     "PostgreSQL with Prisma",
			Occurrences:       3,
			FirstSeen:         "2025-01-15",
			LastSeen:          "2025-01-20",
			Status:            "promoted",
			PromotedTo:        "CLAUDE.md",
			PromotedDate:      "2025-01-19",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Create target file
	targetPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(targetPath, []byte("# Guidelines\n\n"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Reset flags
	promoteFile = ""
	promoteID = ""
	promoteTarget = ""
	promoteForce = false

	cmd := newTestRootCmd()
	promoteCmd := *promoteClarificationCmd
	promoteCmd.ResetFlags()
	promoteCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path")
	promoteCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote")
	promoteCmd.Flags().StringVar(&promoteTarget, "target", "", "Target file")
	promoteCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion")
	promoteCmd.Flags().BoolVar(&promoteJSON, "json", false, "Output as JSON")
	promoteCmd.Flags().BoolVar(&promoteMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&promoteCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"promote-clarification", "-f", trackingPath, "--id", "clr-20250115-abc123", "--target", targetPath, "--force", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote-clarification --force failed: %v", err)
	}

	// Verify JSON output
	var result PromoteResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "re-promoted" {
		t.Errorf("expected status 're-promoted', got %s", result.Status)
	}

	// Verify content was appended to target file
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read target file: %v", err)
	}
	if !strings.Contains(string(content), "What database?") {
		t.Error("expected question to be in target file")
	}
}

func TestPromoteCmd_EntryNotFound(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "promote-test")
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
	promoteFile = ""
	promoteID = ""

	cmd := newTestRootCmd()
	promoteCmd := *promoteClarificationCmd
	promoteCmd.ResetFlags()
	promoteCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path")
	promoteCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote")
	promoteCmd.Flags().StringVar(&promoteTarget, "target", "", "Target file")
	promoteCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion")
	cmd.AddCommand(&promoteCmd)

	cmd.SetArgs([]string{"promote-clarification", "-f", trackingPath, "--id", "nonexistent-id"})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent entry")
	}
}

func TestPromoteCmd_JSONOutput(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "promote-test")
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
			CanonicalQuestion: "Test question?",
			CurrentAnswer:     "Test answer",
			Occurrences:       1,
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Create target file
	targetPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(targetPath, []byte("# Guidelines\n\n"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Reset flags
	promoteFile = ""
	promoteID = ""
	promoteTarget = ""
	promoteForce = false

	cmd := newTestRootCmd()
	promoteCmd := *promoteClarificationCmd
	promoteCmd.ResetFlags()
	promoteCmd.Flags().StringVarP(&promoteFile, "file", "f", "", "Tracking file path")
	promoteCmd.Flags().StringVar(&promoteID, "id", "", "Entry ID to promote")
	promoteCmd.Flags().StringVar(&promoteTarget, "target", "", "Target file")
	promoteCmd.Flags().BoolVar(&promoteForce, "force", false, "Force re-promotion")
	promoteCmd.Flags().BoolVar(&promoteJSON, "json", false, "Output as JSON")
	promoteCmd.Flags().BoolVar(&promoteMinimal, "min", false, "Output in minimal format")
	cmd.AddCommand(&promoteCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"promote-clarification", "-f", trackingPath, "--id", "clr-20250115-abc123", "--target", targetPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote-clarification failed: %v", err)
	}

	// Verify JSON is valid with expected fields
	output := stdout.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	requiredFields := []string{"status", "id", "target", "message"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}
