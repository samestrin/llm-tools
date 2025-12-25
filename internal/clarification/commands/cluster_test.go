package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestClusterClarificationsCmd_Clustered(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cluster-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with entries to cluster
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
		{
			ID:                "clr-20250117-ghi789",
			CanonicalQuestion: "What database should we use?",
			CurrentAnswer:     "PostgreSQL",
			Status:            "pending",
		},
	}
	if err := tracking.SaveTrackingFile(tf, trackingPath); err != nil {
		t.Fatalf("failed to create tracking file: %v", err)
	}

	// Reset flags
	clusterFile = ""

	cmd := newTestRootCmd()
	clusterCmd := *clusterClarificationsCmd
	clusterCmd.ResetFlags()
	clusterCmd.Flags().StringVarP(&clusterFile, "file", "f", "", "Tracking file path")
	cmd.AddCommand(&clusterCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"cluster-clarifications", "-f", trackingPath})

	// Set mock LLM client
	SetLLMClient(&MockLLMClient{
		Response: `{"clusters": [{"label": "Testing framework", "question_indices": [1, 2]}, {"label": "Database choice", "question_indices": [3]}], "cluster_count": 2}`,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster-clarifications failed: %v", err)
	}

	// Verify JSON output
	var result ClusterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "clustered" {
		t.Errorf("expected status 'clustered', got %s", result.Status)
	}
	if result.ClusterCount != 2 {
		t.Errorf("expected 2 clusters, got %d", result.ClusterCount)
	}
	if len(result.Clusters) != 2 {
		t.Errorf("expected 2 clusters in array, got %d", len(result.Clusters))
	}
}

func TestClusterClarificationsCmd_TooFewEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "cluster-test")
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
	clusterFile = ""

	cmd := newTestRootCmd()
	clusterCmd := *clusterClarificationsCmd
	clusterCmd.ResetFlags()
	clusterCmd.Flags().StringVarP(&clusterFile, "file", "f", "", "Tracking file path")
	cmd.AddCommand(&clusterCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"cluster-clarifications", "-f", trackingPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cluster-clarifications failed: %v", err)
	}

	// Verify JSON output
	var result ClusterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "insufficient_data" {
		t.Errorf("expected status 'insufficient_data', got %s", result.Status)
	}
	if result.Note != "Not enough questions to cluster" {
		t.Errorf("expected note about insufficient data, got %s", result.Note)
	}
}

func TestClusterClarificationsCmd_FileNotFound(t *testing.T) {
	// Reset flags
	clusterFile = ""

	cmd := newTestRootCmd()
	clusterCmd := *clusterClarificationsCmd
	clusterCmd.ResetFlags()
	clusterCmd.Flags().StringVarP(&clusterFile, "file", "f", "", "Tracking file path")
	cmd.AddCommand(&clusterCmd)

	cmd.SetArgs([]string{"cluster-clarifications", "-f", "/nonexistent/tracking.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}
