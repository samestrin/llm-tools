package tracking

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTrackingFile_Valid(t *testing.T) {
	tf, err := LoadTrackingFile("../../testdata/tracking-sample.yaml")
	if err != nil {
		t.Fatalf("LoadTrackingFile failed: %v", err)
	}

	if tf.Version != 1 {
		t.Errorf("expected Version 1, got %d", tf.Version)
	}

	if tf.Created != "2025-01-15" {
		t.Errorf("expected Created '2025-01-15', got %s", tf.Created)
	}

	if tf.LastUpdated != "2025-01-20" {
		t.Errorf("expected LastUpdated '2025-01-20', got %s", tf.LastUpdated)
	}

	if len(tf.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(tf.Entries))
	}

	// Check first entry
	e1 := tf.Entries[0]
	if e1.ID != "clr-20250115-abc123" {
		t.Errorf("expected ID 'clr-20250115-abc123', got %s", e1.ID)
	}
	if e1.CanonicalQuestion != "What testing framework should we use?" {
		t.Errorf("expected CanonicalQuestion 'What testing framework should we use?', got %s", e1.CanonicalQuestion)
	}
	if len(e1.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(e1.Variants))
	}
	if e1.Occurrences != 5 {
		t.Errorf("expected Occurrences 5, got %d", e1.Occurrences)
	}
	if e1.Status != "pending" {
		t.Errorf("expected Status 'pending', got %s", e1.Status)
	}

	// Check second entry has promoted fields
	e2 := tf.Entries[1]
	if e2.PromotedTo != "CLAUDE.md" {
		t.Errorf("expected PromotedTo 'CLAUDE.md', got %s", e2.PromotedTo)
	}
	if e2.PromotedDate != "2025-01-19" {
		t.Errorf("expected PromotedDate '2025-01-19', got %s", e2.PromotedDate)
	}
}

func TestLoadTrackingFile_NotFound(t *testing.T) {
	_, err := LoadTrackingFile("nonexistent.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadTrackingFile_Empty(t *testing.T) {
	tf, err := LoadTrackingFile("../../testdata/tracking-empty.yaml")
	if err != nil {
		t.Fatalf("LoadTrackingFile failed: %v", err)
	}

	if len(tf.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(tf.Entries))
	}

	// Entries should be empty slice, not nil
	if tf.Entries == nil {
		t.Error("Entries should be empty slice, not nil")
	}
}

func TestSaveTrackingFile_Valid(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tf := &TrackingFile{
		Version:     1,
		Created:     "2025-01-15",
		LastUpdated: "2025-01-20",
		Entries: []Entry{
			{
				ID:                "clr-test-123",
				CanonicalQuestion: "Test question?",
				CurrentAnswer:     "Test answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-15",
				LastSeen:          "2025-01-15",
				Status:            "pending",
				Confidence:        "medium",
			},
		},
	}

	outPath := filepath.Join(tmpDir, "output.yaml")
	if err := SaveTrackingFile(tf, outPath); err != nil {
		t.Fatalf("SaveTrackingFile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("output file was not created")
	}

	// Verify content can be loaded back
	loaded, err := LoadTrackingFile(outPath)
	if err != nil {
		t.Fatalf("failed to load saved file: %v", err)
	}

	if loaded.Version != tf.Version {
		t.Errorf("Version mismatch: expected %d, got %d", tf.Version, loaded.Version)
	}
	if len(loaded.Entries) != len(tf.Entries) {
		t.Errorf("Entries count mismatch: expected %d, got %d", len(tf.Entries), len(loaded.Entries))
	}
}

func TestSaveTrackingFile_CreateDirectory(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tf := NewTrackingFile("2025-01-15")
	outPath := filepath.Join(tmpDir, "subdir", "nested", "output.yaml")

	if err := SaveTrackingFile(tf, outPath); err != nil {
		t.Fatalf("SaveTrackingFile failed: %v", err)
	}

	// Verify file was created in nested directory
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("output file was not created in nested directory")
	}
}

func TestRoundTrip(t *testing.T) {
	// Load sample file
	original, err := LoadTrackingFile("../../testdata/tracking-sample.yaml")
	if err != nil {
		t.Fatalf("LoadTrackingFile failed: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save to temp file
	outPath := filepath.Join(tmpDir, "roundtrip.yaml")
	if err := SaveTrackingFile(original, outPath); err != nil {
		t.Fatalf("SaveTrackingFile failed: %v", err)
	}

	// Load again
	reloaded, err := LoadTrackingFile(outPath)
	if err != nil {
		t.Fatalf("LoadTrackingFile (reload) failed: %v", err)
	}

	// Verify data preserved
	if reloaded.Version != original.Version {
		t.Errorf("Version mismatch after round trip")
	}
	if reloaded.Created != original.Created {
		t.Errorf("Created mismatch after round trip")
	}
	if len(reloaded.Entries) != len(original.Entries) {
		t.Errorf("Entries count mismatch after round trip: expected %d, got %d", len(original.Entries), len(reloaded.Entries))
	}

	// Check first entry preserved
	if len(reloaded.Entries) > 0 {
		if reloaded.Entries[0].ID != original.Entries[0].ID {
			t.Errorf("Entry ID mismatch after round trip")
		}
		if reloaded.Entries[0].CanonicalQuestion != original.Entries[0].CanonicalQuestion {
			t.Errorf("Entry CanonicalQuestion mismatch after round trip")
		}
		if len(reloaded.Entries[0].Variants) != len(original.Entries[0].Variants) {
			t.Errorf("Entry Variants count mismatch after round trip")
		}
	}
}

func TestFileExists(t *testing.T) {
	if !FileExists("../../testdata/tracking-sample.yaml") {
		t.Error("FileExists returned false for existing file")
	}
	if FileExists("nonexistent.yaml") {
		t.Error("FileExists returned true for non-existing file")
	}
}

func TestLoadTrackingFile_InvalidYAML(t *testing.T) {
	// Create temp file with invalid YAML
	tmpDir, err := os.MkdirTemp("", "tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	err = os.WriteFile(invalidPath, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid YAML: %v", err)
	}

	_, err = LoadTrackingFile(invalidPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSaveTrackingFile_NilEntries(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create tracking file with nil entries
	tf := &TrackingFile{
		Version:     1,
		Created:     "2025-01-15",
		LastUpdated: "2025-01-15",
		Entries:     nil,
	}

	outPath := filepath.Join(tmpDir, "nil-entries.yaml")
	if err := SaveTrackingFile(tf, outPath); err != nil {
		t.Fatalf("SaveTrackingFile with nil entries failed: %v", err)
	}

	// Verify file was created and can be loaded
	loaded, err := LoadTrackingFile(outPath)
	if err != nil {
		t.Fatalf("failed to load saved file: %v", err)
	}

	// Entries should be empty slice, not nil
	if loaded.Entries == nil {
		t.Error("loaded Entries should not be nil")
	}
	if len(loaded.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(loaded.Entries))
	}
}

func TestLoadTrackingFile_NilEntriesInFile(t *testing.T) {
	// Create temp file with no entries field
	tmpDir, err := os.MkdirTemp("", "tracking-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	noEntriesPath := filepath.Join(tmpDir, "no-entries.yaml")
	yamlContent := `version: 1
created: "2025-01-15"
last_updated: "2025-01-15"
`
	err = os.WriteFile(noEntriesPath, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("failed to write YAML: %v", err)
	}

	tf, err := LoadTrackingFile(noEntriesPath)
	if err != nil {
		t.Fatalf("LoadTrackingFile failed: %v", err)
	}

	// Entries should be empty slice, not nil
	if tf.Entries == nil {
		t.Error("Entries should be empty slice, not nil")
	}
}
