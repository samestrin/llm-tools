package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestDeleteClarificationCommand(t *testing.T) {
	t.Run("requires id flag", func(t *testing.T) {
		cmd := NewDeleteClarificationCmd()
		cmd.SetArgs([]string{"--file", "test.yaml"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing id flag")
		}
	})

	t.Run("requires file flag", func(t *testing.T) {
		cmd := NewDeleteClarificationCmd()
		cmd.SetArgs([]string{"--id", "test-1"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing file flag")
		}
	})

	t.Run("deletes entry with force flag", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.yaml")

		// Create storage with entry
		ctx := context.Background()
		store, err := storage.NewYAMLStorage(ctx, path)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}

		entry := &tracking.Entry{
			ID:                "delete-me",
			CanonicalQuestion: "What should I delete?",
			CurrentAnswer:     "This entry",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		}
		if err := store.Create(ctx, entry); err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}
		store.Close()

		// Execute delete command with force
		var out bytes.Buffer
		cmd := NewDeleteClarificationCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--id", "delete-me", "--force"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		// Verify deletion
		store, _ = storage.NewYAMLStorage(ctx, path)
		defer store.Close()
		_, err = store.Read(ctx, "delete-me")
		if err == nil {
			t.Error("entry should be deleted")
		}
	})

	t.Run("shows entry details before confirmation", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.yaml")

		ctx := context.Background()
		store, err := storage.NewYAMLStorage(ctx, path)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}

		entry := &tracking.Entry{
			ID:                "show-details",
			CanonicalQuestion: "What are the details?",
			CurrentAnswer:     "These are the details",
			Occurrences:       5,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-15",
			Status:            "active",
			Confidence:        "high",
		}
		store.Create(ctx, entry)
		store.Close()

		var out bytes.Buffer
		cmd := NewDeleteClarificationCmd()
		cmd.SetOut(&out)
		// Use force to avoid interactive prompt
		cmd.SetArgs([]string{"--file", path, "--id", "show-details", "--force"})
		cmd.Execute()

		output := out.String()
		if !bytes.Contains([]byte(output), []byte("show-details")) {
			t.Error("output should contain entry ID")
		}
		if !bytes.Contains([]byte(output), []byte("What are the details?")) {
			t.Error("output should contain question")
		}
	})

	t.Run("returns error for non-existent id", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.yaml")

		ctx := context.Background()
		store, _ := storage.NewYAMLStorage(ctx, path)
		store.Close()

		var out bytes.Buffer
		cmd := NewDeleteClarificationCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--id", "nonexistent", "--force"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent id")
		}
	})

	t.Run("works with sqlite storage", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		ctx := context.Background()
		store, err := storage.NewSQLiteStorage(ctx, path)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}

		entry := &tracking.Entry{
			ID:                "sqlite-delete",
			CanonicalQuestion: "SQLite question?",
			CurrentAnswer:     "SQLite answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		}
		store.Create(ctx, entry)
		store.Close()

		var out bytes.Buffer
		cmd := NewDeleteClarificationCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--id", "sqlite-delete", "--force"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("delete from sqlite failed: %v", err)
		}

		// Verify deletion
		store, _ = storage.NewSQLiteStorage(ctx, path)
		defer store.Close()
		_, err = store.Read(ctx, "sqlite-delete")
		if err == nil {
			t.Error("entry should be deleted from sqlite")
		}
	})

	t.Run("quiet flag suppresses output", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.yaml")

		ctx := context.Background()
		store, _ := storage.NewYAMLStorage(ctx, path)
		entry := &tracking.Entry{
			ID:                "quiet-delete",
			CanonicalQuestion: "Quiet question?",
			CurrentAnswer:     "Quiet answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		}
		store.Create(ctx, entry)
		store.Close()

		var out bytes.Buffer
		cmd := NewDeleteClarificationCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--id", "quiet-delete", "--force", "--quiet"})
		cmd.Execute()

		if out.Len() != 0 {
			t.Errorf("expected no output with --quiet, got: %s", out.String())
		}
	})
}
