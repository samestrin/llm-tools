package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestNewYAMLStorage(t *testing.T) {
	ctx := context.Background()

	t.Run("creates new file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.yaml")

		store, err := NewYAMLStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewYAMLStorage failed: %v", err)
		}
		defer store.Close()

		// Verify file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Expected file to be created")
		}
	})

	t.Run("loads existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "existing.yaml")

		// Create file with existing entry
		tf := tracking.NewTrackingFile("2025-01-01")
		tf.Entries = append(tf.Entries, *tracking.NewEntry("existing-1", "Question?", "Answer", "2025-01-01"))
		if err := tracking.SaveTrackingFile(tf, path); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		store, err := NewYAMLStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewYAMLStorage failed: %v", err)
		}
		defer store.Close()

		// Verify entry was loaded
		entry, err := store.Read(ctx, "existing-1")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if entry.CanonicalQuestion != "Question?" {
			t.Errorf("Expected 'Question?', got %q", entry.CanonicalQuestion)
		}
	})

	t.Run("rejects invalid extension", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.txt")

		_, err := NewYAMLStorage(ctx, path)
		if err == nil {
			t.Fatal("Expected error for invalid extension")
		}
		if !errors.Is(err, ErrUnsupportedBackend) {
			t.Errorf("Expected ErrUnsupportedBackend, got %v", err)
		}
	})

	t.Run("accepts .yml extension", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "test.yml")

		store, err := NewYAMLStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewYAMLStorage failed for .yml: %v", err)
		}
		store.Close()
	})

	t.Run("rejects empty path", func(t *testing.T) {
		_, err := NewYAMLStorage(ctx, "")
		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("Expected ErrInvalidPath, got %v", err)
		}
	})
}

func TestYAMLStorageCRUD(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "crud.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	// Test Create
	entry := tracking.NewEntry("crud-1", "Test question?", "Test answer", "2025-01-01")
	if err := store.Create(ctx, entry); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test duplicate Create
	if err := store.Create(ctx, entry); !errors.Is(err, ErrDuplicateEntry) {
		t.Errorf("Expected ErrDuplicateEntry, got %v", err)
	}

	// Test Read
	retrieved, err := store.Read(ctx, "crud-1")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if retrieved.CurrentAnswer != "Test answer" {
		t.Errorf("Expected 'Test answer', got %q", retrieved.CurrentAnswer)
	}

	// Test Read not found
	_, err = store.Read(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}

	// Test Update
	entry.CurrentAnswer = "Updated answer"
	if err := store.Update(ctx, entry); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, _ = store.Read(ctx, "crud-1")
	if retrieved.CurrentAnswer != "Updated answer" {
		t.Errorf("Expected 'Updated answer', got %q", retrieved.CurrentAnswer)
	}

	// Test Update not found
	notFound := tracking.NewEntry("notfound", "Q", "A", "2025-01-01")
	if err := store.Update(ctx, notFound); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}

	// Test Delete
	if err := store.Delete(ctx, "crud-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Read(ctx, "crud-1")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound after delete, got %v", err)
	}

	// Test Delete not found
	if err := store.Delete(ctx, "nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestYAMLStorageList(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "list.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	// Create test entries
	entries := []tracking.Entry{
		{ID: "l1", CanonicalQuestion: "Q1", Status: "pending", Occurrences: 1, ContextTags: []string{"frontend"}},
		{ID: "l2", CanonicalQuestion: "Q2", Status: "promoted", Occurrences: 5, ContextTags: []string{"backend"}},
		{ID: "l3", CanonicalQuestion: "Q3", Status: "pending", Occurrences: 3, SprintsSeen: []string{"sprint-1"}},
	}
	for i := range entries {
		store.Create(ctx, &entries[i])
	}

	t.Run("list all", func(t *testing.T) {
		list, err := store.List(ctx, ListFilter{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(list))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		list, err := store.List(ctx, ListFilter{Status: "pending"})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("Expected 2 pending entries, got %d", len(list))
		}
	})

	t.Run("filter by min occurrences", func(t *testing.T) {
		list, err := store.List(ctx, ListFilter{MinOccurrences: 3})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("Expected 2 entries with 3+ occurrences, got %d", len(list))
		}
	})

	t.Run("filter by tags", func(t *testing.T) {
		list, err := store.GetByTags(ctx, []string{"frontend"})
		if err != nil {
			t.Fatalf("GetByTags failed: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("Expected 1 frontend entry, got %d", len(list))
		}
	})

	t.Run("filter by sprint", func(t *testing.T) {
		list, err := store.GetBySprint(ctx, "sprint-1")
		if err != nil {
			t.Fatalf("GetBySprint failed: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("Expected 1 entry in sprint-1, got %d", len(list))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		list, err := store.List(ctx, ListFilter{Offset: 1, Limit: 1})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(list) != 1 {
			t.Errorf("Expected 1 entry with pagination, got %d", len(list))
		}
	})
}

func TestYAMLStorageFindByQuestion(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "find.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	entry := tracking.NewEntry("find-1", "How to configure auth?", "Use OAuth", "2025-01-01")
	store.Create(ctx, entry)

	// Test find existing
	found, err := store.FindByQuestion(ctx, "How to configure auth?")
	if err != nil {
		t.Fatalf("FindByQuestion failed: %v", err)
	}
	if found.ID != "find-1" {
		t.Errorf("Expected ID find-1, got %s", found.ID)
	}

	// Test find not found
	_, err = store.FindByQuestion(ctx, "Nonexistent question")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestYAMLStorageBulk(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bulk.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	entries := []tracking.Entry{
		*tracking.NewEntry("b1", "Q1", "A1", "2025-01-01"),
		*tracking.NewEntry("b2", "Q2", "A2", "2025-01-01"),
		*tracking.NewEntry("b3", "Q3", "A3", "2025-01-01"),
	}

	// Test BulkInsert
	result, err := store.BulkInsert(ctx, entries)
	if err != nil {
		t.Fatalf("BulkInsert failed: %v", err)
	}
	if result.Created != 3 {
		t.Errorf("Expected 3 created, got %d", result.Created)
	}

	// Test BulkInsert with existing
	result, err = store.BulkInsert(ctx, entries[:1])
	if err != nil {
		t.Fatalf("BulkInsert failed: %v", err)
	}
	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped, got %d", result.Skipped)
	}

	// Test BulkUpdate
	entries[0].CurrentAnswer = "Updated A1"
	result, err = store.BulkUpdate(ctx, entries[:1])
	if err != nil {
		t.Fatalf("BulkUpdate failed: %v", err)
	}
	if result.Updated != 1 {
		t.Errorf("Expected 1 updated, got %d", result.Updated)
	}

	// Test BulkDelete
	result, err = store.BulkDelete(ctx, []string{"b1", "b2"})
	if err != nil {
		t.Fatalf("BulkDelete failed: %v", err)
	}
	if result.Processed != 2 {
		t.Errorf("Expected 2 processed, got %d", result.Processed)
	}

	list, _ := store.List(ctx, ListFilter{})
	if len(list) != 1 {
		t.Errorf("Expected 1 remaining, got %d", len(list))
	}
}

func TestYAMLStorageImport(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	t.Run("append mode", func(t *testing.T) {
		path := filepath.Join(tmpDir, "import-append.yaml")
		store, _ := NewYAMLStorage(ctx, path)
		defer store.Close()

		// Create existing entry
		store.Create(ctx, tracking.NewEntry("existing", "Q", "A", "2025-01-01"))

		// Import new entries
		result, err := store.Import(ctx, []tracking.Entry{
			*tracking.NewEntry("new1", "Q1", "A1", "2025-01-01"),
			*tracking.NewEntry("existing", "Q2", "A2", "2025-01-01"), // duplicate
		}, ImportModeAppend)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}
		if result.Created != 1 || result.Skipped != 1 {
			t.Errorf("Expected 1 created, 1 skipped; got %d created, %d skipped", result.Created, result.Skipped)
		}
	})

	t.Run("overwrite mode", func(t *testing.T) {
		path := filepath.Join(tmpDir, "import-overwrite.yaml")
		store, _ := NewYAMLStorage(ctx, path)
		defer store.Close()

		// Create existing entries
		store.Create(ctx, tracking.NewEntry("old1", "Q1", "A1", "2025-01-01"))
		store.Create(ctx, tracking.NewEntry("old2", "Q2", "A2", "2025-01-01"))

		// Overwrite with new entries
		result, err := store.Import(ctx, []tracking.Entry{
			*tracking.NewEntry("new1", "N1", "NA1", "2025-01-01"),
		}, ImportModeOverwrite)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}

		list, _ := store.List(ctx, ListFilter{})
		if len(list) != 1 {
			t.Errorf("Expected 1 entry after overwrite, got %d", len(list))
		}
		if result.Created != 1 {
			t.Errorf("Expected 1 created, got %d", result.Created)
		}
	})

	t.Run("merge mode", func(t *testing.T) {
		path := filepath.Join(tmpDir, "import-merge.yaml")
		store, _ := NewYAMLStorage(ctx, path)
		defer store.Close()

		// Create existing entry
		store.Create(ctx, tracking.NewEntry("merge1", "Q1", "A1", "2025-01-01"))

		// Merge with new and updated entries
		result, err := store.Import(ctx, []tracking.Entry{
			{ID: "merge1", CanonicalQuestion: "Q1", CurrentAnswer: "Updated A1"},
			*tracking.NewEntry("merge2", "Q2", "A2", "2025-01-01"),
		}, ImportModeMerge)
		if err != nil {
			t.Fatalf("Import failed: %v", err)
		}
		if result.Updated != 1 || result.Created != 1 {
			t.Errorf("Expected 1 updated, 1 created; got %d updated, %d created", result.Updated, result.Created)
		}
	})
}

func TestYAMLStorageStats(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "stats.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	// Create entries with various attributes
	entry1 := tracking.NewEntry("s1", "Q1", "A1", "2025-01-01")
	entry1.Variants = []string{"V1", "V2"}
	entry1.ContextTags = []string{"frontend", "auth"}
	entry1.SprintsSeen = []string{"sprint-1"}
	store.Create(ctx, entry1)

	entry2 := tracking.NewEntry("s2", "Q2", "A2", "2025-01-01")
	entry2.Status = "promoted"
	entry2.ContextTags = []string{"backend"}
	entry2.SprintsSeen = []string{"sprint-1", "sprint-2"}
	store.Create(ctx, entry2)

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.TotalEntries != 2 {
		t.Errorf("Expected 2 entries, got %d", stats.TotalEntries)
	}
	if stats.TotalVariants != 2 {
		t.Errorf("Expected 2 variants, got %d", stats.TotalVariants)
	}
	if stats.TotalTags != 3 {
		t.Errorf("Expected 3 tags, got %d", stats.TotalTags)
	}
	if stats.TotalSprints != 2 {
		t.Errorf("Expected 2 sprints, got %d", stats.TotalSprints)
	}
	if stats.EntriesByStatus["pending"] != 1 {
		t.Errorf("Expected 1 pending, got %d", stats.EntriesByStatus["pending"])
	}
	if stats.EntriesByStatus["promoted"] != 1 {
		t.Errorf("Expected 1 promoted, got %d", stats.EntriesByStatus["promoted"])
	}
}

func TestYAMLStorageBackup(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "backup.yaml")
	backupPath := filepath.Join(tmpDir, "backup-copy.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	// Create entry
	store.Create(ctx, tracking.NewEntry("bk1", "Q", "A", "2025-01-01"))

	// Create backup
	if err := store.Backup(ctx, backupPath); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file not created")
	}

	// Load backup and verify content
	backupStore, err := NewYAMLStorage(ctx, backupPath)
	if err != nil {
		t.Fatalf("Failed to load backup: %v", err)
	}
	defer backupStore.Close()

	entry, err := backupStore.Read(ctx, "bk1")
	if err != nil {
		t.Fatalf("Failed to read from backup: %v", err)
	}
	if entry.CanonicalQuestion != "Q" {
		t.Error("Backup content mismatch")
	}
}

func TestYAMLStorageClose(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "close.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}

	// Close should succeed
	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail
	_, err = store.Read(ctx, "test")
	if !errors.Is(err, ErrStorageClosed) {
		t.Errorf("Expected ErrStorageClosed, got %v", err)
	}

	// Double close should not error
	if err := store.Close(); err != nil {
		t.Errorf("Double close should not error: %v", err)
	}
}

func TestYAMLStorageQueryFilter(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "query.yaml")

	store, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store.Close()

	// Create entry with searchable content
	entry := tracking.NewEntry("query-1", "How to configure authentication?", "Use OAuth2 with JWT tokens", "2025-01-01")
	store.Create(ctx, entry)

	// Test query filter
	list, err := store.List(ctx, ListFilter{Query: "authentication"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 match for 'authentication', got %d", len(list))
	}

	// Test query in answer
	list, err = store.List(ctx, ListFilter{Query: "OAuth2"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("Expected 1 match for 'OAuth2', got %d", len(list))
	}

	// Test no match
	list, err = store.List(ctx, ListFilter{Query: "nonexistent"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("Expected 0 matches for 'nonexistent', got %d", len(list))
	}
}

func TestYAMLStoragePersistence(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "persist.yaml")

	// Create store and add entry
	store1, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	store1.Create(ctx, tracking.NewEntry("persist-1", "Q", "A", "2025-01-01"))
	store1.Close()

	// Open new store and verify data persisted
	store2, err := NewYAMLStorage(ctx, path)
	if err != nil {
		t.Fatalf("NewYAMLStorage failed: %v", err)
	}
	defer store2.Close()

	entry, err := store2.Read(ctx, "persist-1")
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if entry.CanonicalQuestion != "Q" {
		t.Error("Data not persisted correctly")
	}
}
