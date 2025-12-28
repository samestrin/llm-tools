package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// resetAllCommandFlags resets all global command flags and command outputs to their defaults
func resetAllCommandFlags() {
	globalDBPath = ""
	listFile = ""
	listStatus = ""
	listMinOccurrences = 0
	listJSON = false
	addFile = ""
	addQuestion = ""
	addAnswer = ""
	addID = ""
	addSprint = ""
	addTags = nil
	addCheckMatch = false
	initOutput = ""
	initForce = false
	os.Unsetenv("CLARIFY_DB_PATH")

	// Reset command outputs to nil (they default to os.Stdout)
	listEntriesCmd.SetOut(nil)
	addClarificationCmd.SetOut(nil)
	initTrackingCmd.SetOut(nil)
}

// TestCommandsWithSQLite tests that existing commands work with SQLite storage
func TestCommandsWithSQLite(t *testing.T) {
	// Reset global state at start and end
	resetAllCommandFlags()
	defer resetAllCommandFlags()

	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "commands_sqlite_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Initialize SQLite storage with test data
	store, err := storage.NewStorage(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Add test entries
	testEntries := []tracking.Entry{
		{
			ID:                "test-001",
			CanonicalQuestion: "How do we handle errors?",
			CurrentAnswer:     "Use wrapped errors with context",
			Status:            "pending",
			Occurrences:       3,
			Confidence:        "high",
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-15",
			ContextTags:       []string{"error-handling", "go"},
			SprintsSeen:       []string{"sprint-1"},
		},
		{
			ID:                "test-002",
			CanonicalQuestion: "What testing framework to use?",
			CurrentAnswer:     "Standard Go testing package",
			Status:            "promoted",
			Occurrences:       5,
			Confidence:        "high",
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-20",
			ContextTags:       []string{"testing", "go"},
			SprintsSeen:       []string{"sprint-1", "sprint-2"},
			PromotedTo:        "CLAUDE.md",
			PromotedDate:      "2025-01-18",
		},
		{
			ID:                "test-003",
			CanonicalQuestion: "What is the preferred code style?",
			CurrentAnswer:     "Follow Go formatting conventions",
			Status:            "pending",
			Occurrences:       2,
			Confidence:        "medium",
			FirstSeen:         "2025-01-10",
			LastSeen:          "2025-01-10",
		},
	}

	for _, entry := range testEntries {
		if err := store.Create(ctx, &entry); err != nil {
			t.Fatalf("Failed to create test entry: %v", err)
		}
	}
	store.Close()

	// Test list-entries with SQLite
	t.Run("ListEntriesWithSQLite", func(t *testing.T) {
		// Reset all flags
		globalDBPath = ""
		listFile = dbPath
		listStatus = ""
		listMinOccurrences = 0
		listJSON = true

		var buf bytes.Buffer
		listEntriesCmd.SetOut(&buf)

		err := runListEntries(listEntriesCmd, nil)
		if err != nil {
			t.Errorf("list-entries failed with SQLite: %v", err)
			return
		}

		var result ListEntriesResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("Failed to parse output: %v", err)
			return
		}

		if result.Count != 3 {
			t.Errorf("Expected 3 entries, got %d", result.Count)
		}
	})

	// Test list-entries with status filter
	t.Run("ListEntriesWithStatusFilter", func(t *testing.T) {
		globalDBPath = ""
		listFile = dbPath
		listStatus = "pending"
		listMinOccurrences = 0
		listJSON = true

		var buf bytes.Buffer
		listEntriesCmd.SetOut(&buf)

		err := runListEntries(listEntriesCmd, nil)
		if err != nil {
			t.Errorf("list-entries with filter failed: %v", err)
			return
		}

		var result ListEntriesResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("Failed to parse output: %v", err)
			return
		}

		if result.Count != 2 {
			t.Errorf("Expected 2 pending entries, got %d", result.Count)
		}
	})

	// Test add-clarification with SQLite (create new)
	t.Run("AddClarificationCreateNew", func(t *testing.T) {
		globalDBPath = ""
		addFile = dbPath
		addQuestion = "What database should we use?"
		addAnswer = "SQLite for local storage"
		addID = ""
		addSprint = "sprint-3"
		addTags = []string{"database", "sqlite"}
		addCheckMatch = false

		var buf bytes.Buffer
		addClarificationCmd.SetOut(&buf)

		err := runAddClarification(addClarificationCmd, nil)
		if err != nil {
			t.Errorf("add-clarification create failed: %v", err)
			return
		}

		var result AddClarificationResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("Failed to parse output: %v", err)
			return
		}

		if result.Status != "created" {
			t.Errorf("Expected status 'created', got '%s'", result.Status)
		}
	})

	// Test add-clarification with SQLite (update existing)
	t.Run("AddClarificationUpdateExisting", func(t *testing.T) {
		globalDBPath = ""
		addFile = dbPath
		addQuestion = ""
		addAnswer = "Use wrapped errors with stack traces"
		addID = "test-001"
		addSprint = "sprint-3"
		addTags = nil
		addCheckMatch = false

		var buf bytes.Buffer
		addClarificationCmd.SetOut(&buf)

		err := runAddClarification(addClarificationCmd, nil)
		if err != nil {
			t.Errorf("add-clarification update failed: %v", err)
			return
		}

		var result AddClarificationResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("Failed to parse output: %v", err)
			return
		}

		if result.Status != "updated" {
			t.Errorf("Expected status 'updated', got '%s'", result.Status)
		}

		// Verify the update
		store, _ := storage.NewStorage(ctx, dbPath)
		defer store.Close()
		entry, err := store.Read(ctx, "test-001")
		if err != nil {
			t.Errorf("Failed to read updated entry: %v", err)
			return
		}
		if entry.CurrentAnswer != "Use wrapped errors with stack traces" {
			t.Errorf("Answer not updated correctly")
		}
		if entry.Occurrences != 4 {
			t.Errorf("Expected occurrences 4, got %d", entry.Occurrences)
		}
	})

	// Test init-tracking creates SQLite database
	t.Run("InitTrackingCreatesSQLite", func(t *testing.T) {
		globalDBPath = ""
		newDBPath := filepath.Join(tmpDir, "new.db")
		initOutput = newDBPath
		initForce = false

		var buf bytes.Buffer
		initTrackingCmd.SetOut(&buf)

		err := runInitTracking(initTrackingCmd, nil)
		if err != nil {
			t.Errorf("init-tracking failed: %v", err)
			return
		}

		var result InitResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Errorf("Failed to parse output: %v", err)
			return
		}

		if result.Status != "created" {
			t.Errorf("Expected status 'created', got '%s'", result.Status)
		}

		// Verify the database was created
		if _, err := os.Stat(newDBPath); os.IsNotExist(err) {
			t.Error("SQLite database file not created")
		}

		// Verify it's a valid SQLite database
		store, err := storage.NewStorage(ctx, newDBPath)
		if err != nil {
			t.Errorf("Created database is not valid: %v", err)
			return
		}
		store.Close()
	})
}

// TestDBFlagOverride tests that --db flag overrides --file
func TestDBFlagOverride(t *testing.T) {
	// Reset all command flags at start and end
	resetAllCommandFlags()
	defer resetAllCommandFlags()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "db_flag_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	dbPath := filepath.Join(tmpDir, "override.db")

	// Create a SQLite database with test data
	store, err := storage.NewStorage(ctx, dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	store.Create(ctx, &tracking.Entry{
		ID:                "override-test",
		CanonicalQuestion: "Test question",
		CurrentAnswer:     "Test answer",
		Status:            "pending",
		Occurrences:       1,
	})
	store.Close()

	// Test that GetDBPath returns --db when set
	t.Run("GetDBPathWithGlobalDB", func(t *testing.T) {
		globalDBPath = dbPath

		result := GetDBPath("/some/other/file.yaml")
		if result != dbPath {
			t.Errorf("Expected %s, got %s", dbPath, result)
		}
		globalDBPath = "" // cleanup immediately
	})

	// Test that GetDBPath returns --file when --db not set
	t.Run("GetDBPathWithoutGlobalDB", func(t *testing.T) {
		globalDBPath = ""

		result := GetDBPath("/some/file.yaml")
		if result != "/some/file.yaml" {
			t.Errorf("Expected /some/file.yaml, got %s", result)
		}
	})

	// Test environment variable override
	t.Run("GetDBPathWithEnvVar", func(t *testing.T) {
		globalDBPath = ""
		os.Setenv("CLARIFY_DB_PATH", dbPath)
		defer os.Unsetenv("CLARIFY_DB_PATH")

		result := GetDBPath("/some/file.yaml")
		if result != dbPath {
			t.Errorf("Expected %s (from env), got %s", dbPath, result)
		}
	})
}

// TestStorageHelperFunctions tests the storage helper functions
func TestStorageHelperFunctions(t *testing.T) {
	// Reset all command flags at start and end
	resetAllCommandFlags()
	defer resetAllCommandFlags()

	tmpDir, err := os.MkdirTemp("", "storage_helper_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	t.Run("GetStorageWithSQLite", func(t *testing.T) {
		dbPath := filepath.Join(tmpDir, "helper.db")
		globalDBPath = ""

		store, err := GetStorage(ctx, dbPath)
		if err != nil {
			t.Errorf("GetStorage failed: %v", err)
			return
		}
		defer store.Close()

		// Should be SQLite storage
		_, ok := store.(*storage.SQLiteStorage)
		if !ok {
			t.Error("Expected SQLiteStorage")
		}
	})

	t.Run("GetStorageOrErrorWithMissingYAML", func(t *testing.T) {
		globalDBPath = ""
		yamlPath := filepath.Join(tmpDir, "missing.yaml")

		_, err := GetStorageOrError(ctx, yamlPath)
		if err == nil {
			t.Error("Expected error for missing YAML file")
		}
	})

	t.Run("FileOrDBExists", func(t *testing.T) {
		// Create a file
		existingPath := filepath.Join(tmpDir, "exists.db")
		os.WriteFile(existingPath, []byte{}, 0644)

		if !FileOrDBExists(existingPath) {
			t.Error("Expected file to exist")
		}

		missingPath := filepath.Join(tmpDir, "missing.db")
		if FileOrDBExists(missingPath) {
			t.Error("Expected file to not exist")
		}
	})
}
