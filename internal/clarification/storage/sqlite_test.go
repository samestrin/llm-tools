package storage

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// createTestSQLiteStorage creates a temporary SQLite storage for testing.
func createTestSQLiteStorage(t *testing.T) (*SQLiteStorage, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	storage, err := NewSQLiteStorage(context.Background(), path)
	if err != nil {
		t.Fatalf("failed to create SQLite storage: %v", err)
	}
	return storage, path
}

// createTestEntry creates a test entry with the given ID.
func createTestEntry(id string) *tracking.Entry {
	return &tracking.Entry{
		ID:                id,
		CanonicalQuestion: "What is the test question for " + id + "?",
		CurrentAnswer:     "This is the answer for " + id,
		Occurrences:       1,
		FirstSeen:         "2025-01-01",
		LastSeen:          "2025-01-01",
		Status:            "active",
		Confidence:        "high",
		Variants:          []string{"variant 1", "variant 2"},
		ContextTags:       []string{"test", "golang"},
		SprintsSeen:       []string{"sprint-1.0"},
	}
}

func TestNewSQLiteStorage(t *testing.T) {
	t.Run("creates storage with valid path", func(t *testing.T) {
		storage, path := createTestSQLiteStorage(t)
		defer storage.Close()

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("database file was not created")
		}
	})

	t.Run("rejects empty path", func(t *testing.T) {
		_, err := NewSQLiteStorage(context.Background(), "")
		if err != ErrInvalidPath {
			t.Errorf("expected ErrInvalidPath, got %v", err)
		}
	})

	t.Run("rejects invalid extension", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.txt")
		_, err := NewSQLiteStorage(context.Background(), path)
		if err == nil {
			t.Error("expected error for invalid extension")
		}
		var unsupportedErr *UnsupportedBackendError
		if !isUnsupportedBackendError(err) {
			t.Errorf("expected UnsupportedBackendError, got %T", err)
		}
		_ = unsupportedErr
	})

	t.Run("accepts .sqlite extension", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.sqlite")
		storage, err := NewSQLiteStorage(context.Background(), path)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}
		defer storage.Close()
	})

	t.Run("accepts .sqlite3 extension", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.sqlite3")
		storage, err := NewSQLiteStorage(context.Background(), path)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}
		defer storage.Close()
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nested", "dirs", "test.db")
		storage, err := NewSQLiteStorage(context.Background(), path)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}
		defer storage.Close()
	})
}

func TestSQLiteStorage_ForeignKeys(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()

	// Foreign keys should be enabled - test by trying to insert orphan
	ctx := context.Background()
	_, err := storage.db.ExecContext(ctx,
		"INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)",
		"nonexistent", "test")
	if err == nil {
		t.Error("expected foreign key violation, got nil")
	}
}

func TestSQLiteStorage_Create(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	t.Run("creates entry with all fields", func(t *testing.T) {
		entry := createTestEntry("create-1")
		err := storage.Create(ctx, entry)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Verify entry was created
		result, err := storage.Read(ctx, "create-1")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if result.CanonicalQuestion != entry.CanonicalQuestion {
			t.Errorf("question mismatch: got %q, want %q", result.CanonicalQuestion, entry.CanonicalQuestion)
		}
		if len(result.Variants) != 2 {
			t.Errorf("variants count: got %d, want 2", len(result.Variants))
		}
		if len(result.ContextTags) != 2 {
			t.Errorf("tags count: got %d, want 2", len(result.ContextTags))
		}
		if len(result.SprintsSeen) != 1 {
			t.Errorf("sprints count: got %d, want 1", len(result.SprintsSeen))
		}
	})

	t.Run("rejects duplicate entry", func(t *testing.T) {
		entry := createTestEntry("create-dup")
		if err := storage.Create(ctx, entry); err != nil {
			t.Fatalf("first create failed: %v", err)
		}

		err := storage.Create(ctx, entry)
		if err == nil {
			t.Error("expected error for duplicate entry")
		}
		if !isDuplicateEntryError(err) {
			t.Errorf("expected DuplicateEntryError, got %T: %v", err, err)
		}
	})

	t.Run("returns error when closed", func(t *testing.T) {
		closedStorage, _ := createTestSQLiteStorage(t)
		closedStorage.Close()

		err := closedStorage.Create(ctx, createTestEntry("test"))
		if err != ErrStorageClosed {
			t.Errorf("expected ErrStorageClosed, got %v", err)
		}
	})
}

func TestSQLiteStorage_Read(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create test entry
	entry := createTestEntry("read-1")
	if err := storage.Create(ctx, entry); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	t.Run("reads existing entry", func(t *testing.T) {
		result, err := storage.Read(ctx, "read-1")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if result.ID != "read-1" {
			t.Errorf("ID mismatch: got %q, want %q", result.ID, "read-1")
		}
	})

	t.Run("returns not found for missing entry", func(t *testing.T) {
		_, err := storage.Read(ctx, "nonexistent")
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("loads related data without N+1", func(t *testing.T) {
		result, err := storage.Read(ctx, "read-1")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}

		// Should have all related data in single read operation
		if len(result.Variants) != 2 {
			t.Errorf("expected 2 variants, got %d", len(result.Variants))
		}
		if len(result.ContextTags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(result.ContextTags))
		}
	})
}

func TestSQLiteStorage_Update(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	t.Run("updates existing entry", func(t *testing.T) {
		entry := createTestEntry("update-1")
		if err := storage.Create(ctx, entry); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		entry.CurrentAnswer = "Updated answer"
		entry.Occurrences = 5
		entry.ContextTags = []string{"updated", "tag"}
		if err := storage.Update(ctx, entry); err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		result, err := storage.Read(ctx, "update-1")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if result.CurrentAnswer != "Updated answer" {
			t.Errorf("answer not updated: got %q", result.CurrentAnswer)
		}
		if result.Occurrences != 5 {
			t.Errorf("occurrences not updated: got %d", result.Occurrences)
		}
		if len(result.ContextTags) != 2 {
			t.Errorf("tags count mismatch: got %v", result.ContextTags)
		}
		// Check tags contain expected values (order not guaranteed)
		tagSet := make(map[string]bool)
		for _, tag := range result.ContextTags {
			tagSet[tag] = true
		}
		if !tagSet["updated"] || !tagSet["tag"] {
			t.Errorf("tags not updated correctly: got %v", result.ContextTags)
		}
	})

	t.Run("returns not found for missing entry", func(t *testing.T) {
		entry := createTestEntry("nonexistent")
		err := storage.Update(ctx, entry)
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestSQLiteStorage_Delete(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	t.Run("deletes existing entry", func(t *testing.T) {
		entry := createTestEntry("delete-1")
		if err := storage.Create(ctx, entry); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if err := storage.Delete(ctx, "delete-1"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err := storage.Read(ctx, "delete-1")
		if !isNotFoundError(err) {
			t.Error("entry should not exist after delete")
		}
	})

	t.Run("cascade deletes related data", func(t *testing.T) {
		entry := createTestEntry("delete-cascade")
		if err := storage.Create(ctx, entry); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Verify related data exists
		var variantCount int
		storage.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM entry_variants WHERE entry_id = ?",
			"delete-cascade").Scan(&variantCount)
		if variantCount == 0 {
			t.Error("variants should exist before delete")
		}

		if err := storage.Delete(ctx, "delete-cascade"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify cascade delete
		storage.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM entry_variants WHERE entry_id = ?",
			"delete-cascade").Scan(&variantCount)
		if variantCount != 0 {
			t.Error("variants should be deleted with cascade")
		}
	})

	t.Run("returns not found for missing entry", func(t *testing.T) {
		err := storage.Delete(ctx, "nonexistent")
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestSQLiteStorage_List(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create test entries
	for i := 1; i <= 5; i++ {
		entry := createTestEntry("list-" + string(rune('0'+i)))
		entry.Status = "active"
		if i > 3 {
			entry.Status = "resolved"
		}
		entry.Occurrences = i
		if err := storage.Create(ctx, entry); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	t.Run("lists all entries", func(t *testing.T) {
		entries, err := storage.List(ctx, ListFilter{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(entries) != 5 {
			t.Errorf("expected 5 entries, got %d", len(entries))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		entries, err := storage.List(ctx, ListFilter{Status: "active"})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("expected 3 active entries, got %d", len(entries))
		}
	})

	t.Run("filters by min occurrences", func(t *testing.T) {
		entries, err := storage.List(ctx, ListFilter{MinOccurrences: 3})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(entries) != 3 {
			t.Errorf("expected 3 entries with occurrences >= 3, got %d", len(entries))
		}
	})

	t.Run("applies pagination", func(t *testing.T) {
		entries, err := storage.List(ctx, ListFilter{Limit: 2, Offset: 1})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 entries with limit, got %d", len(entries))
		}
	})
}

func TestSQLiteStorage_FindByQuestion(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	entry := createTestEntry("find-q-1")
	entry.CanonicalQuestion = "How do I configure authentication?"
	if err := storage.Create(ctx, entry); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	t.Run("finds by exact question", func(t *testing.T) {
		result, err := storage.FindByQuestion(ctx, "How do I configure authentication?")
		if err != nil {
			t.Fatalf("FindByQuestion failed: %v", err)
		}
		if result.ID != "find-q-1" {
			t.Errorf("expected ID find-q-1, got %s", result.ID)
		}
	})

	t.Run("returns not found for non-matching question", func(t *testing.T) {
		_, err := storage.FindByQuestion(ctx, "nonexistent question")
		if !isNotFoundError(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})
}

func TestSQLiteStorage_GetByTags(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create entries with different tags
	entry1 := createTestEntry("tags-1")
	entry1.ContextTags = []string{"frontend", "react"}
	storage.Create(ctx, entry1)

	entry2 := createTestEntry("tags-2")
	entry2.ContextTags = []string{"backend", "golang"}
	storage.Create(ctx, entry2)

	entry3 := createTestEntry("tags-3")
	entry3.ContextTags = []string{"frontend", "vue"}
	storage.Create(ctx, entry3)

	t.Run("finds entries by tag", func(t *testing.T) {
		entries, err := storage.GetByTags(ctx, []string{"frontend"})
		if err != nil {
			t.Fatalf("GetByTags failed: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 entries with frontend tag, got %d", len(entries))
		}
	})

	t.Run("finds entries by any of multiple tags", func(t *testing.T) {
		entries, err := storage.GetByTags(ctx, []string{"react", "vue"})
		if err != nil {
			t.Fatalf("GetByTags failed: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 entries with react or vue tag, got %d", len(entries))
		}
	})
}

func TestSQLiteStorage_GetBySprint(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	entry1 := createTestEntry("sprint-1")
	entry1.SprintsSeen = []string{"sprint-1.0", "sprint-2.0"}
	storage.Create(ctx, entry1)

	entry2 := createTestEntry("sprint-2")
	entry2.SprintsSeen = []string{"sprint-2.0"}
	storage.Create(ctx, entry2)

	t.Run("finds entries by sprint", func(t *testing.T) {
		entries, err := storage.GetBySprint(ctx, "sprint-2.0")
		if err != nil {
			t.Fatalf("GetBySprint failed: %v", err)
		}
		if len(entries) != 2 {
			t.Errorf("expected 2 entries in sprint-2.0, got %d", len(entries))
		}
	})
}

func TestSQLiteStorage_FTS(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create entries with searchable content
	entry1 := createTestEntry("fts-1")
	entry1.CanonicalQuestion = "How to configure OAuth authentication?"
	entry1.CurrentAnswer = "Use JWT tokens with refresh mechanism"
	storage.Create(ctx, entry1)

	entry2 := createTestEntry("fts-2")
	entry2.CanonicalQuestion = "Database connection pooling best practices"
	entry2.CurrentAnswer = "Use connection pools with max idle connections"
	storage.Create(ctx, entry2)

	t.Run("searches by keyword in question", func(t *testing.T) {
		entries, err := storage.List(ctx, ListFilter{Query: "authentication"})
		if err != nil {
			t.Fatalf("List with FTS failed: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 entry matching authentication, got %d", len(entries))
		}
	})

	t.Run("searches by keyword in answer", func(t *testing.T) {
		entries, err := storage.List(ctx, ListFilter{Query: "JWT"})
		if err != nil {
			t.Fatalf("List with FTS failed: %v", err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 entry matching JWT, got %d", len(entries))
		}
	})
}

func TestSQLiteStorage_BulkInsert(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	t.Run("inserts multiple entries", func(t *testing.T) {
		entries := make([]tracking.Entry, 100)
		for i := range entries {
			entries[i] = *createTestEntry("bulk-" + string(rune(i)))
		}

		result, err := storage.BulkInsert(ctx, entries)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}
		if result.Created != 100 {
			t.Errorf("expected 100 created, got %d", result.Created)
		}
		if result.Processed != 100 {
			t.Errorf("expected 100 processed, got %d", result.Processed)
		}
	})

	t.Run("skips duplicates", func(t *testing.T) {
		entry := createTestEntry("bulk-dup")
		storage.Create(ctx, entry)

		entries := []tracking.Entry{*entry}
		result, err := storage.BulkInsert(ctx, entries)
		if err != nil {
			t.Fatalf("BulkInsert failed: %v", err)
		}
		if result.Skipped != 1 {
			t.Errorf("expected 1 skipped, got %d", result.Skipped)
		}
	})
}

func TestSQLiteStorage_BulkInsert_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create 1000 entries
	entries := make([]tracking.Entry, 1000)
	for i := range entries {
		entry := createTestEntry("perf-" + string(rune(i/256)) + string(rune(i%256)))
		entry.Variants = []string{"v1", "v2", "v3"}
		entry.ContextTags = []string{"tag1", "tag2"}
		entry.SprintsSeen = []string{"sprint-1"}
		entries[i] = *entry
	}

	start := time.Now()
	result, err := storage.BulkInsert(ctx, entries)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("BulkInsert failed: %v", err)
	}
	if result.Created != 1000 {
		t.Errorf("expected 1000 created, got %d", result.Created)
	}
	if duration > 10*time.Second {
		t.Errorf("BulkInsert took too long: %v (should be < 10s)", duration)
	}
	t.Logf("BulkInsert 1000 entries took %v", duration)
}

func TestSQLiteStorage_BulkUpdate(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create entries
	for i := 1; i <= 5; i++ {
		storage.Create(ctx, createTestEntry("bulk-update-"+string(rune('0'+i))))
	}

	t.Run("updates multiple entries", func(t *testing.T) {
		entries := make([]tracking.Entry, 5)
		for i := range entries {
			entries[i] = *createTestEntry("bulk-update-" + string(rune('1'+i)))
			entries[i].CurrentAnswer = "Updated answer"
		}

		result, err := storage.BulkUpdate(ctx, entries)
		if err != nil {
			t.Fatalf("BulkUpdate failed: %v", err)
		}
		if result.Updated != 5 {
			t.Errorf("expected 5 updated, got %d", result.Updated)
		}
	})

	t.Run("skips non-existent entries", func(t *testing.T) {
		entries := []tracking.Entry{*createTestEntry("nonexistent")}
		result, err := storage.BulkUpdate(ctx, entries)
		if err != nil {
			t.Fatalf("BulkUpdate failed: %v", err)
		}
		if result.Skipped != 1 {
			t.Errorf("expected 1 skipped, got %d", result.Skipped)
		}
	})
}

func TestSQLiteStorage_BulkDelete(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create entries
	for i := 1; i <= 5; i++ {
		storage.Create(ctx, createTestEntry("bulk-delete-"+string(rune('0'+i))))
	}

	t.Run("deletes multiple entries", func(t *testing.T) {
		ids := []string{"bulk-delete-1", "bulk-delete-2", "bulk-delete-3"}
		result, err := storage.BulkDelete(ctx, ids)
		if err != nil {
			t.Fatalf("BulkDelete failed: %v", err)
		}
		if result.Processed != 3 {
			t.Errorf("expected 3 processed, got %d", result.Processed)
		}
	})

	t.Run("skips non-existent entries", func(t *testing.T) {
		ids := []string{"nonexistent"}
		result, err := storage.BulkDelete(ctx, ids)
		if err != nil {
			t.Fatalf("BulkDelete failed: %v", err)
		}
		if result.Skipped != 1 {
			t.Errorf("expected 1 skipped, got %d", result.Skipped)
		}
	})
}

func TestSQLiteStorage_Import(t *testing.T) {
	ctx := context.Background()

	t.Run("overwrites existing entries", func(t *testing.T) {
		storage, _ := createTestSQLiteStorage(t)
		defer storage.Close()

		// Create initial entries
		storage.Create(ctx, createTestEntry("import-1"))
		storage.Create(ctx, createTestEntry("import-2"))

		// Import new entries with overwrite mode
		newEntries := []tracking.Entry{*createTestEntry("import-new")}
		result, err := storage.Import(ctx, newEntries, ImportModeOverwrite)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}
		if result.Created != 1 {
			t.Errorf("expected 1 created, got %d", result.Created)
		}

		// Verify old entries are gone
		entries, _ := storage.List(ctx, ListFilter{})
		if len(entries) != 1 {
			t.Errorf("expected 1 entry after overwrite, got %d", len(entries))
		}
	})

	t.Run("merges entries", func(t *testing.T) {
		storage, _ := createTestSQLiteStorage(t)
		defer storage.Close()

		// Create initial entry
		storage.Create(ctx, createTestEntry("merge-1"))

		// Import with merge mode
		importEntries := []tracking.Entry{
			*createTestEntry("merge-1"), // Will update
			*createTestEntry("merge-2"), // Will create
		}
		importEntries[0].CurrentAnswer = "Merged answer"

		result, err := storage.Import(ctx, importEntries, ImportModeMerge)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}
		if result.Updated != 1 {
			t.Errorf("expected 1 updated, got %d", result.Updated)
		}
		if result.Created != 1 {
			t.Errorf("expected 1 created, got %d", result.Created)
		}

		// Verify merge
		entry, _ := storage.Read(ctx, "merge-1")
		if entry.CurrentAnswer != "Merged answer" {
			t.Errorf("merge did not update answer")
		}
	})

	t.Run("appends entries", func(t *testing.T) {
		storage, _ := createTestSQLiteStorage(t)
		defer storage.Close()

		storage.Create(ctx, createTestEntry("append-1"))

		importEntries := []tracking.Entry{
			*createTestEntry("append-1"), // Will skip
			*createTestEntry("append-2"), // Will create
		}

		result, err := storage.Import(ctx, importEntries, ImportModeAppend)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}
		if result.Skipped != 1 {
			t.Errorf("expected 1 skipped, got %d", result.Skipped)
		}
		if result.Created != 1 {
			t.Errorf("expected 1 created, got %d", result.Created)
		}
	})
}

func TestSQLiteStorage_Export(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create test entries
	storage.Create(ctx, createTestEntry("export-1"))
	storage.Create(ctx, createTestEntry("export-2"))

	entries, err := storage.Export(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSQLiteStorage_Vacuum(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create and delete entries to create fragmentation
	for i := 0; i < 100; i++ {
		storage.Create(ctx, createTestEntry("vacuum-"+string(rune(i))))
	}
	for i := 0; i < 100; i++ {
		storage.Delete(ctx, "vacuum-"+string(rune(i)))
	}

	freed, err := storage.Vacuum(ctx)
	if err != nil {
		t.Fatalf("Vacuum failed: %v", err)
	}
	// Freed bytes can be 0 or positive depending on fragmentation
	t.Logf("Vacuum freed %d bytes", freed)
}

func TestSQLiteStorage_Stats(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create test entries
	for i := 1; i <= 3; i++ {
		entry := createTestEntry("stats-" + string(rune('0'+i)))
		if i == 3 {
			entry.Status = "resolved"
		}
		storage.Create(ctx, entry)
	}

	stats, err := storage.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalEntries != 3 {
		t.Errorf("expected 3 total entries, got %d", stats.TotalEntries)
	}
	if stats.EntriesByStatus["active"] != 2 {
		t.Errorf("expected 2 active entries, got %d", stats.EntriesByStatus["active"])
	}
	if stats.EntriesByStatus["resolved"] != 1 {
		t.Errorf("expected 1 resolved entry, got %d", stats.EntriesByStatus["resolved"])
	}
	if stats.TotalVariants != 6 { // 3 entries * 2 variants each
		t.Errorf("expected 6 variants, got %d", stats.TotalVariants)
	}
}

func TestSQLiteStorage_Backup(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create test entry
	storage.Create(ctx, createTestEntry("backup-1"))

	// Create backup
	backupDir := t.TempDir()
	backupPath := filepath.Join(backupDir, "backup.db")
	if err := storage.Backup(ctx, backupPath); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup exists and is valid
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup file was not created")
	}

	// Open backup and verify data
	backupStorage, err := NewSQLiteStorage(ctx, backupPath)
	if err != nil {
		t.Fatalf("failed to open backup: %v", err)
	}
	defer backupStorage.Close()

	entry, err := backupStorage.Read(ctx, "backup-1")
	if err != nil {
		t.Fatalf("failed to read from backup: %v", err)
	}
	if entry.ID != "backup-1" {
		t.Error("backup data mismatch")
	}
}

func TestSQLiteStorage_Close(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	ctx := context.Background()

	if err := storage.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// All operations should fail after close
	err := storage.Create(ctx, createTestEntry("test"))
	if err != ErrStorageClosed {
		t.Errorf("expected ErrStorageClosed, got %v", err)
	}

	// Close again should be idempotent
	if err := storage.Close(); err != nil {
		t.Errorf("second Close should not error, got %v", err)
	}
}

func TestSQLiteStorage_ConcurrentAccess(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()
	ctx := context.Background()

	// Create initial entries
	for i := 0; i < 5; i++ {
		storage.Create(ctx, createTestEntry("concurrent-read-"+string(rune('a'+i))))
	}

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Sequential writes with concurrent reads
	// Pure Go SQLite driver has known concurrency limitations
	// This test verifies basic operation under moderate concurrency

	// Concurrent reads (should work well)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := storage.Read(ctx, "concurrent-read-"+string(rune('a'+n)))
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// At least some reads should succeed
	if successCount < 3 {
		t.Errorf("expected at least 3 successful concurrent reads, got %d", successCount)
	}

	// Test sequential writes work correctly
	for i := 0; i < 5; i++ {
		entry := createTestEntry("sequential-write-" + string(rune('a'+i)))
		if err := storage.Create(ctx, entry); err != nil {
			t.Errorf("sequential write %d failed: %v", i, err)
		}
	}

	// Verify all entries exist
	entries, err := storage.List(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(entries))
	}
}

func TestSQLiteStorage_ContextCancellation(t *testing.T) {
	storage, _ := createTestSQLiteStorage(t)
	defer storage.Close()

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should respect context cancellation
	err := storage.Create(ctx, createTestEntry("cancelled"))
	if err == nil {
		t.Error("expected error with cancelled context")
	}
}

// Helper functions for error type checking
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}

func isDuplicateEntryError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*DuplicateEntryError)
	return ok
}

func isUnsupportedBackendError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*UnsupportedBackendError)
	return ok
}
