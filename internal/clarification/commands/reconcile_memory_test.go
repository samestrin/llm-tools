package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestReconcileMemoryCommand(t *testing.T) {
	t.Run("requires file flag", func(t *testing.T) {
		cmd := NewReconcileMemoryCmd()
		cmd.SetArgs([]string{"--project-root", "."})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing file flag")
		}
	})

	t.Run("requires project-root flag", func(t *testing.T) {
		cmd := NewReconcileMemoryCmd()
		cmd.SetArgs([]string{"--file", "test.db"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing project-root flag")
		}
	})

	t.Run("identifies stale file references", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")
		projectRoot := filepath.Join(dir, "project")
		os.MkdirAll(projectRoot, 0755)

		// Create an existing file
		existingFile := filepath.Join(projectRoot, "existing.go")
		os.WriteFile(existingFile, []byte("package main"), 0644)

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		// Entry with reference to existing file
		store.Create(ctx, &tracking.Entry{
			ID:                "valid-ref",
			CanonicalQuestion: "How do I use existing.go?",
			CurrentAnswer:     "Check src/existing.go for details",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			ContextTags:       []string{"existing.go"},
		})

		// Entry with reference to non-existent file
		store.Create(ctx, &tracking.Entry{
			ID:                "stale-ref",
			CanonicalQuestion: "How do I use deleted.go?",
			CurrentAnswer:     "Check src/deleted.go for details",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			ContextTags:       []string{"deleted.go"},
		})
		store.Close()

		var out bytes.Buffer
		cmd := NewReconcileMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--project-root", projectRoot, "--dry-run"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("reconcile failed: %v", err)
		}

		output := out.String()
		if !bytes.Contains([]byte(output), []byte("deleted.go")) {
			t.Errorf("output should mention stale reference: %s", output)
		}
	})

	t.Run("dry-run shows changes without applying", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")
		projectRoot := filepath.Join(dir, "project")
		os.MkdirAll(projectRoot, 0755)

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		store.Create(ctx, &tracking.Entry{
			ID:                "stale-entry",
			CanonicalQuestion: "About nonexistent.go?",
			CurrentAnswer:     "See nonexistent.go",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			ContextTags:       []string{"nonexistent.go"},
		})
		store.Close()

		var out bytes.Buffer
		cmd := NewReconcileMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--project-root", projectRoot, "--dry-run"})
		cmd.Execute()

		output := out.String()
		if !bytes.Contains([]byte(output), []byte("dry-run")) && !bytes.Contains([]byte(output), []byte("DRY RUN")) {
			t.Errorf("output should indicate dry-run mode: %s", output)
		}

		// Entry should still exist
		store, _ = storage.NewSQLiteStorage(ctx, path)
		defer store.Close()
		_, err := store.Read(ctx, "stale-entry")
		if err != nil {
			t.Error("entry should still exist in dry-run mode")
		}
	})

	t.Run("marks stale references when not dry-run", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")
		projectRoot := filepath.Join(dir, "project")
		os.MkdirAll(projectRoot, 0755)

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		store.Create(ctx, &tracking.Entry{
			ID:                "to-mark",
			CanonicalQuestion: "About gone.go?",
			CurrentAnswer:     "See gone.go",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "active",
			Confidence:        "high",
			ContextTags:       []string{"gone.go"},
		})
		store.Close()

		var out bytes.Buffer
		cmd := NewReconcileMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--project-root", projectRoot})
		cmd.Execute()

		// Entry should be marked as stale
		store, _ = storage.NewSQLiteStorage(ctx, path)
		defer store.Close()
		entry, err := store.Read(ctx, "to-mark")
		if err != nil {
			t.Fatalf("entry should exist: %v", err)
		}
		if entry.Status != "stale" {
			t.Errorf("entry should be marked as stale, got: %s", entry.Status)
		}
	})

	t.Run("reports count of stale references found", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")
		projectRoot := filepath.Join(dir, "project")
		os.MkdirAll(projectRoot, 0755)

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		// Create multiple stale entries
		for i := 0; i < 3; i++ {
			store.Create(ctx, &tracking.Entry{
				ID:                "stale-" + string(rune('a'+i)),
				CanonicalQuestion: "Question?",
				CurrentAnswer:     "Answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "active",
				Confidence:        "high",
				ContextTags:       []string{"missing.go"},
			})
		}
		store.Close()

		var out bytes.Buffer
		cmd := NewReconcileMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--project-root", projectRoot, "--dry-run"})
		cmd.Execute()

		output := out.String()
		// Should report count
		if !bytes.Contains([]byte(output), []byte("3")) {
			t.Errorf("output should report count of stale references: %s", output)
		}
	})
}
