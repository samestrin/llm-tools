package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestImportMemoryCommand(t *testing.T) {
	t.Run("requires source flag", func(t *testing.T) {
		cmd := NewImportMemoryCmd()
		cmd.SetArgs([]string{"--target", "test.db"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing source flag")
		}
	})

	t.Run("requires target flag", func(t *testing.T) {
		cmd := NewImportMemoryCmd()
		cmd.SetArgs([]string{"--source", "test.yaml"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing target flag")
		}
	})

	t.Run("imports YAML to SQLite with append mode", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.yaml")
		targetPath := filepath.Join(dir, "target.db")

		// Create YAML source with entries
		ctx := context.Background()
		yamlStore, err := storage.NewYAMLStorage(ctx, sourcePath)
		if err != nil {
			t.Fatalf("failed to create yaml storage: %v", err)
		}

		entries := []tracking.Entry{
			{
				ID:                "import-1",
				CanonicalQuestion: "First question?",
				CurrentAnswer:     "First answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "pending",
				Confidence:        "medium",
			},
			{
				ID:                "import-2",
				CanonicalQuestion: "Second question?",
				CurrentAnswer:     "Second answer",
				Occurrences:       2,
				FirstSeen:         "2025-01-02",
				LastSeen:          "2025-01-02",
				Status:            "active",
				Confidence:        "high",
			},
		}

		for _, e := range entries {
			yamlStore.Create(ctx, &e)
		}
		yamlStore.Close()

		// Execute import
		var out bytes.Buffer
		cmd := NewImportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--target", targetPath, "--mode", "append"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("import failed: %v", err)
		}

		// Verify SQLite content
		sqliteStore, err := storage.NewSQLiteStorage(ctx, targetPath)
		if err != nil {
			t.Fatalf("failed to open sqlite: %v", err)
		}
		defer sqliteStore.Close()

		importedEntries, err := sqliteStore.List(ctx, storage.ListFilter{})
		if err != nil {
			t.Fatalf("failed to list entries: %v", err)
		}

		if len(importedEntries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(importedEntries))
		}
	})

	t.Run("append mode adds without overwriting", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.yaml")
		targetPath := filepath.Join(dir, "target.db")

		ctx := context.Background()

		// Create initial SQLite entry
		sqliteStore, _ := storage.NewSQLiteStorage(ctx, targetPath)
		sqliteStore.Create(ctx, &tracking.Entry{
			ID:                "existing",
			CanonicalQuestion: "Existing question?",
			CurrentAnswer:     "Existing answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		})
		sqliteStore.Close()

		// Create YAML source
		yamlStore, _ := storage.NewYAMLStorage(ctx, sourcePath)
		yamlStore.Create(ctx, &tracking.Entry{
			ID:                "new-entry",
			CanonicalQuestion: "New question?",
			CurrentAnswer:     "New answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-02",
			LastSeen:          "2025-01-02",
			Status:            "pending",
			Confidence:        "medium",
		})
		yamlStore.Close()

		// Import with append mode
		var out bytes.Buffer
		cmd := NewImportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--target", targetPath, "--mode", "append"})
		cmd.Execute()

		// Verify both entries exist
		sqliteStore, _ = storage.NewSQLiteStorage(ctx, targetPath)
		defer sqliteStore.Close()

		entries, _ := sqliteStore.List(ctx, storage.ListFilter{})
		if len(entries) != 2 {
			t.Errorf("expected 2 entries (existing + new), got %d", len(entries))
		}
	})

	t.Run("overwrite mode replaces all data", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.yaml")
		targetPath := filepath.Join(dir, "target.db")

		ctx := context.Background()

		// Create initial SQLite entries
		sqliteStore, _ := storage.NewSQLiteStorage(ctx, targetPath)
		sqliteStore.Create(ctx, &tracking.Entry{
			ID:                "will-be-removed",
			CanonicalQuestion: "Will be removed?",
			CurrentAnswer:     "Yes",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		})
		sqliteStore.Close()

		// Create YAML source with different entry
		yamlStore, _ := storage.NewYAMLStorage(ctx, sourcePath)
		yamlStore.Create(ctx, &tracking.Entry{
			ID:                "replacement",
			CanonicalQuestion: "Replacement question?",
			CurrentAnswer:     "Replacement answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-02",
			LastSeen:          "2025-01-02",
			Status:            "pending",
			Confidence:        "medium",
		})
		yamlStore.Close()

		// Import with overwrite mode
		var out bytes.Buffer
		cmd := NewImportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--target", targetPath, "--mode", "overwrite"})
		cmd.Execute()

		// Verify only new entry exists
		sqliteStore, _ = storage.NewSQLiteStorage(ctx, targetPath)
		defer sqliteStore.Close()

		entries, _ := sqliteStore.List(ctx, storage.ListFilter{})
		if len(entries) != 1 {
			t.Errorf("expected 1 entry after overwrite, got %d", len(entries))
		}
		if entries[0].ID != "replacement" {
			t.Errorf("expected entry 'replacement', got %s", entries[0].ID)
		}
	})

	t.Run("merge mode combines with conflict resolution", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.yaml")
		targetPath := filepath.Join(dir, "target.db")

		ctx := context.Background()

		// Create initial SQLite entry
		sqliteStore, _ := storage.NewSQLiteStorage(ctx, targetPath)
		sqliteStore.Create(ctx, &tracking.Entry{
			ID:                "existing",
			CanonicalQuestion: "Existing question?",
			CurrentAnswer:     "Old answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		})
		sqliteStore.Close()

		// Create YAML source with same ID but different answer
		yamlStore, _ := storage.NewYAMLStorage(ctx, sourcePath)
		yamlStore.Create(ctx, &tracking.Entry{
			ID:                "existing",
			CanonicalQuestion: "Existing question?",
			CurrentAnswer:     "Updated answer", // Different answer
			Occurrences:       3,                // Higher occurrences
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-05",
			Status:            "active",
			Confidence:        "high",
		})
		yamlStore.Create(ctx, &tracking.Entry{
			ID:                "new-entry",
			CanonicalQuestion: "New question?",
			CurrentAnswer:     "New answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-02",
			LastSeen:          "2025-01-02",
			Status:            "pending",
			Confidence:        "medium",
		})
		yamlStore.Close()

		// Import with merge mode
		var out bytes.Buffer
		cmd := NewImportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--target", targetPath, "--mode", "merge"})
		cmd.Execute()

		// Verify both entries exist and existing is updated
		sqliteStore, _ = storage.NewSQLiteStorage(ctx, targetPath)
		defer sqliteStore.Close()

		entries, _ := sqliteStore.List(ctx, storage.ListFilter{})
		if len(entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(entries))
		}

		existing, _ := sqliteStore.Read(ctx, "existing")
		if existing.CurrentAnswer != "Updated answer" {
			t.Errorf("merge should update existing entry, got answer: %s", existing.CurrentAnswer)
		}
	})
}
