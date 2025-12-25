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

func createTestTrackingFile(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "list-entries-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	tf.Entries = []tracking.Entry{
		{
			ID:                "clr-20250115-abc123",
			CanonicalQuestion: "What framework?",
			CurrentAnswer:     "Use React",
			Occurrences:       5,
			FirstSeen:         "2025-01-15",
			LastSeen:          "2025-01-20",
			Status:            "pending",
			Confidence:        "high",
		},
		{
			ID:                "clr-20250116-def456",
			CanonicalQuestion: "What database?",
			CurrentAnswer:     "PostgreSQL",
			Occurrences:       3,
			FirstSeen:         "2025-01-16",
			LastSeen:          "2025-01-18",
			Status:            "promoted",
			Confidence:        "medium",
		},
		{
			ID:                "clr-20250117-ghi789",
			CanonicalQuestion: "What test runner?",
			CurrentAnswer:     "Vitest",
			Occurrences:       1,
			FirstSeen:         "2025-01-17",
			LastSeen:          "2025-01-17",
			Status:            "pending",
			Confidence:        "low",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create tracking file: %v", err)
	}

	return trackingPath, func() { os.RemoveAll(tmpDir) }
}

func TestListEntriesCmd_ListAll(t *testing.T) {
	trackingPath, cleanup := createTestTrackingFile(t)
	defer cleanup()

	// Reset flags
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	cmd.AddCommand(&listCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"list-entries", "-f", trackingPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-entries failed: %v", err)
	}

	// Verify JSON output
	var result ListEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Count != 3 {
		t.Errorf("expected 3 entries, got %d", result.Count)
	}
	if len(result.Entries) != 3 {
		t.Errorf("expected 3 entries in list, got %d", len(result.Entries))
	}
}

func TestListEntriesCmd_FilterByStatus(t *testing.T) {
	trackingPath, cleanup := createTestTrackingFile(t)
	defer cleanup()

	// Reset flags
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	cmd.AddCommand(&listCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"list-entries", "-f", trackingPath, "--status", "pending", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-entries failed: %v", err)
	}

	var result ListEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("expected 2 pending entries, got %d", result.Count)
	}

	for _, entry := range result.Entries {
		if entry.Status != "pending" {
			t.Errorf("expected status 'pending', got %s", entry.Status)
		}
	}
}

func TestListEntriesCmd_FilterByMinOccurrences(t *testing.T) {
	trackingPath, cleanup := createTestTrackingFile(t)
	defer cleanup()

	// Reset flags
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	cmd.AddCommand(&listCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"list-entries", "-f", trackingPath, "--min-occurrences", "3", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-entries failed: %v", err)
	}

	var result ListEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Count != 2 {
		t.Errorf("expected 2 entries with 3+ occurrences, got %d", result.Count)
	}

	for _, entry := range result.Entries {
		if entry.Occurrences < 3 {
			t.Errorf("expected occurrences >= 3, got %d", entry.Occurrences)
		}
	}
}

func TestListEntriesCmd_CombinedFilters(t *testing.T) {
	trackingPath, cleanup := createTestTrackingFile(t)
	defer cleanup()

	// Reset flags
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	cmd.AddCommand(&listCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	// Pending entries with 3+ occurrences = 1 (only the "What framework?" entry)
	cmd.SetArgs([]string{"list-entries", "-f", trackingPath, "--status", "pending", "--min-occurrences", "3", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-entries failed: %v", err)
	}

	var result ListEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Count != 1 {
		t.Errorf("expected 1 entry matching combined filters, got %d", result.Count)
	}
}

func TestListEntriesCmd_TableOutput(t *testing.T) {
	trackingPath, cleanup := createTestTrackingFile(t)
	defer cleanup()

	// Reset flags
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	cmd.AddCommand(&listCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"list-entries", "-f", trackingPath}) // No --json

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-entries failed: %v", err)
	}

	// Verify table output contains headers and data
	output := stdout.String()
	if !strings.Contains(output, "ID") {
		t.Error("expected table output to contain 'ID' header")
	}
	if !strings.Contains(output, "clr-20250115-abc123") {
		t.Error("expected table output to contain entry ID")
	}
}

func TestListEntriesCmd_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "list-entries-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	trackingPath := filepath.Join(tmpDir, "tracking.yaml")
	tf := tracking.NewTrackingFile("2025-01-15")
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	listCmd.Flags().StringVar(&listStatus, "status", "", "Filter by status")
	listCmd.Flags().IntVar(&listMinOccurrences, "min-occurrences", 0, "Filter by minimum occurrences")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	cmd.AddCommand(&listCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"list-entries", "-f", trackingPath, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list-entries failed: %v", err)
	}

	var result ListEntriesResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Count != 0 {
		t.Errorf("expected 0 entries, got %d", result.Count)
	}
}

func TestListEntriesCmd_FileNotFound(t *testing.T) {
	// Reset flags
	listFile = ""

	cmd := newTestRootCmd()
	listCmd := *listEntriesCmd
	listCmd.ResetFlags()
	listCmd.Flags().StringVarP(&listFile, "file", "f", "", "Tracking file path")
	cmd.AddCommand(&listCmd)

	cmd.SetArgs([]string{"list-entries", "-f", "/nonexistent/tracking.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
