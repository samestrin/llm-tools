package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestOptimizeMemoryCommand(t *testing.T) {
	t.Run("requires file flag", func(t *testing.T) {
		cmd := NewOptimizeMemoryCmd()
		cmd.SetArgs([]string{"--vacuum"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing file flag")
		}
	})

	t.Run("vacuum runs SQLite VACUUM", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		// Create and delete entries to create fragmentation
		for i := 0; i < 100; i++ {
			entry := &tracking.Entry{
				ID:                "entry-" + string(rune(i)),
				CanonicalQuestion: "Question " + string(rune(i)),
				CurrentAnswer:     "Answer " + string(rune(i)),
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "pending",
				Confidence:        "medium",
			}
			store.Create(ctx, entry)
		}
		for i := 0; i < 50; i++ {
			store.Delete(ctx, "entry-"+string(rune(i)))
		}
		store.Close()

		var out bytes.Buffer
		cmd := NewOptimizeMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--vacuum"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("optimize failed: %v", err)
		}

		output := out.String()
		if !bytes.Contains([]byte(output), []byte("VACUUM")) {
			t.Error("output should mention VACUUM")
		}
	})

	t.Run("reports space reclaimed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		// Create entries
		for i := 0; i < 100; i++ {
			entry := &tracking.Entry{
				ID:                "entry-" + string(rune(i)),
				CanonicalQuestion: "A very long question that takes up space " + string(rune(i)),
				CurrentAnswer:     "A very long answer that takes up even more space " + string(rune(i)),
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "pending",
				Confidence:        "medium",
				Variants:          []string{"var1", "var2", "var3"},
				ContextTags:       []string{"tag1", "tag2"},
			}
			store.Create(ctx, entry)
		}
		// Delete some entries
		for i := 0; i < 50; i++ {
			store.Delete(ctx, "entry-"+string(rune(i)))
		}
		store.Close()

		var out bytes.Buffer
		cmd := NewOptimizeMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--vacuum"})
		cmd.Execute()

		output := out.String()
		if !bytes.Contains([]byte(output), []byte("bytes")) {
			t.Errorf("output should report space: %s", output)
		}
	})

	t.Run("errors on YAML backend", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.yaml")

		ctx := context.Background()
		store, _ := storage.NewYAMLStorage(ctx, path)
		store.Close()

		var out bytes.Buffer
		cmd := NewOptimizeMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--vacuum"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for YAML backend with vacuum")
		}
	})

	t.Run("prune-stale removes old entries", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)

		// Create old entries
		oldDate := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
		recentDate := time.Now().Format("2006-01-02")

		store.Create(ctx, &tracking.Entry{
			ID:                "old-entry",
			CanonicalQuestion: "Old question?",
			CurrentAnswer:     "Old answer",
			Occurrences:       1,
			FirstSeen:         oldDate,
			LastSeen:          oldDate,
			Status:            "pending",
			Confidence:        "medium",
		})
		store.Create(ctx, &tracking.Entry{
			ID:                "recent-entry",
			CanonicalQuestion: "Recent question?",
			CurrentAnswer:     "Recent answer",
			Occurrences:       1,
			FirstSeen:         recentDate,
			LastSeen:          recentDate,
			Status:            "pending",
			Confidence:        "medium",
		})
		store.Close()

		var out bytes.Buffer
		cmd := NewOptimizeMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--prune-stale", "30d"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("optimize failed: %v", err)
		}

		// Verify old entry removed
		store, _ = storage.NewSQLiteStorage(ctx, path)
		defer store.Close()

		_, err := store.Read(ctx, "old-entry")
		if err == nil {
			t.Error("old entry should be removed")
		}

		_, err = store.Read(ctx, "recent-entry")
		if err != nil {
			t.Error("recent entry should still exist")
		}
	})

	t.Run("stats shows storage statistics", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, path)
		store.Create(ctx, &tracking.Entry{
			ID:                "stats-entry",
			CanonicalQuestion: "Stats question?",
			CurrentAnswer:     "Stats answer",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		})
		store.Close()

		var out bytes.Buffer
		cmd := NewOptimizeMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--file", path, "--stats"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("optimize failed: %v", err)
		}

		output := out.String()
		if !bytes.Contains([]byte(output), []byte("Total Entries")) {
			t.Errorf("output should contain stats: %s", output)
		}
	})
}
