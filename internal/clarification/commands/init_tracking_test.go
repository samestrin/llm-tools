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

func TestInitTrackingCmd_HappyPath(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "init-tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "tracking.yaml")

	// Create fresh command
	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	cmd.AddCommand(&initCmd)

	// Capture output
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"init-tracking", "-o", outPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init-tracking failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("tracking file was not created")
	}

	// Verify JSON output
	var result InitResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.Status != "created" {
		t.Errorf("expected status 'created', got %s", result.Status)
	}
	if result.File != outPath {
		t.Errorf("expected file %s, got %s", outPath, result.File)
	}

	// Verify file content is valid YAML
	tf, err := tracking.LoadTrackingFile(outPath)
	if err != nil {
		t.Fatalf("failed to load created file: %v", err)
	}
	if tf.Version != 1 {
		t.Errorf("expected version 1, got %d", tf.Version)
	}
	if len(tf.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(tf.Entries))
	}
}

func TestInitTrackingCmd_CreateNestedDirectory(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "init-tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "deep", "nested", "dir", "tracking.yaml")

	// Reset flags for fresh test
	initOutput = ""
	initForce = false

	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	cmd.AddCommand(&initCmd)
	cmd.SetArgs([]string{"init-tracking", "-o", outPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init-tracking failed: %v", err)
	}

	// Verify file was created in nested directory
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("tracking file was not created in nested directory")
	}
}

func TestInitTrackingCmd_FileExistsNoForce(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "init-tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "tracking.yaml")

	// Create existing file
	if err := os.WriteFile(outPath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Reset flags
	initOutput = ""
	initForce = false

	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	cmd.AddCommand(&initCmd)

	cmd.SetArgs([]string{"init-tracking", "-o", outPath})

	// Execute - should return an error
	err = cmd.Execute()
	if err == nil {
		t.Error("expected error when file exists without --force")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	// Verify original file content is preserved
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "existing content" {
		t.Error("original file was modified without --force flag")
	}
}

func TestInitTrackingCmd_ForceOverwrite(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "init-tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "tracking.yaml")

	// Create existing file
	if err := os.WriteFile(outPath, []byte("existing content"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Reset flags
	initOutput = ""
	initForce = false

	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	cmd.AddCommand(&initCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"init-tracking", "-o", outPath, "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init-tracking --force failed: %v", err)
	}

	// Verify file was overwritten with valid tracking file
	tf, err := tracking.LoadTrackingFile(outPath)
	if err != nil {
		t.Fatalf("failed to load overwritten file: %v", err)
	}
	if tf.Version != 1 {
		t.Errorf("expected version 1, got %d", tf.Version)
	}

	// Verify backup was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	backupFound := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "tracking.yaml.backup-") {
			backupFound = true
			break
		}
	}
	if !backupFound {
		t.Error("backup file was not created")
	}
}

func TestInitTrackingCmd_RequiredFlag(t *testing.T) {
	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	initCmd.MarkFlagRequired("output")
	cmd.AddCommand(&initCmd)

	cmd.SetArgs([]string{"init-tracking"}) // No -o flag

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when -o flag is missing")
	}
}

func TestInitTrackingCmd_DirectoryPath(t *testing.T) {
	// Reset flags
	initOutput = ""
	initForce = false

	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	cmd.AddCommand(&initCmd)

	cmd.SetArgs([]string{"init-tracking", "-o", "/tmp/somedir/"}) // Ends with /

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when output path is a directory")
	}
	if err != nil && !strings.Contains(err.Error(), "must be a file") {
		t.Errorf("expected 'must be a file' error, got: %v", err)
	}
}

func TestInitTrackingCmd_JSONOutput(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "init-tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outPath := filepath.Join(tmpDir, "tracking.yaml")

	// Reset flags
	initOutput = ""
	initForce = false

	cmd := newTestRootCmd()
	initCmd := *initTrackingCmd
	initCmd.ResetFlags()
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	cmd.AddCommand(&initCmd)

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"init-tracking", "-o", outPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("init-tracking failed: %v", err)
	}

	// Verify JSON is properly formatted
	output := stdout.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify required fields
	requiredFields := []string{"status", "file", "message"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}
