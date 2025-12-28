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

func TestExportMemoryCommand(t *testing.T) {
	t.Run("requires source flag", func(t *testing.T) {
		cmd := NewExportMemoryCmd()
		cmd.SetArgs([]string{"--output", "test.yaml"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing source flag")
		}
	})

	t.Run("requires output flag", func(t *testing.T) {
		cmd := NewExportMemoryCmd()
		cmd.SetArgs([]string{"--source", "test.db"})
		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for missing output flag")
		}
	})

	t.Run("exports SQLite to YAML", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.db")
		outputPath := filepath.Join(dir, "output.yaml")

		// Create SQLite storage with entries
		ctx := context.Background()
		store, err := storage.NewSQLiteStorage(ctx, sourcePath)
		if err != nil {
			t.Fatalf("failed to create storage: %v", err)
		}

		entries := []tracking.Entry{
			{
				ID:                "export-1",
				CanonicalQuestion: "First question?",
				CurrentAnswer:     "First answer",
				Occurrences:       3,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-10",
				Status:            "active",
				Confidence:        "high",
				Variants:          []string{"variant 1", "variant 2"},
				ContextTags:       []string{"tag1", "tag2"},
				SprintsSeen:       []string{"sprint-1.0"},
			},
			{
				ID:                "export-2",
				CanonicalQuestion: "Second question?",
				CurrentAnswer:     "Second answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-05",
				LastSeen:          "2025-01-05",
				Status:            "pending",
				Confidence:        "medium",
			},
		}

		for _, e := range entries {
			store.Create(ctx, &e)
		}
		store.Close()

		// Execute export
		var out bytes.Buffer
		cmd := NewExportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--output", outputPath})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("export failed: %v", err)
		}

		// Verify YAML file exists
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("output file was not created")
		}

		// Verify YAML content
		yamlStore, err := storage.NewYAMLStorage(ctx, outputPath)
		if err != nil {
			t.Fatalf("failed to open exported YAML: %v", err)
		}
		defer yamlStore.Close()

		exportedEntries, err := yamlStore.List(ctx, storage.ListFilter{})
		if err != nil {
			t.Fatalf("failed to list entries: %v", err)
		}

		if len(exportedEntries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(exportedEntries))
		}
	})

	t.Run("preserves all fields including variants and tags", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.db")
		outputPath := filepath.Join(dir, "output.yaml")

		ctx := context.Background()
		store, _ := storage.NewSQLiteStorage(ctx, sourcePath)

		entry := &tracking.Entry{
			ID:                "full-fields",
			CanonicalQuestion: "What about all fields?",
			CurrentAnswer:     "All fields preserved",
			Occurrences:       5,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-15",
			Status:            "promoted",
			Confidence:        "high",
			Variants:          []string{"v1", "v2", "v3"},
			ContextTags:       []string{"frontend", "react"},
			SprintsSeen:       []string{"sprint-1.0", "sprint-2.0"},
			PromotedTo:        "docs/faq.md",
			PromotedDate:      "2025-01-15",
		}
		store.Create(ctx, entry)
		store.Close()

		var out bytes.Buffer
		cmd := NewExportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--output", outputPath})
		cmd.Execute()

		// Verify all fields
		yamlStore, _ := storage.NewYAMLStorage(ctx, outputPath)
		defer yamlStore.Close()

		exported, err := yamlStore.Read(ctx, "full-fields")
		if err != nil {
			t.Fatalf("failed to read exported entry: %v", err)
		}

		if exported.CanonicalQuestion != entry.CanonicalQuestion {
			t.Error("question not preserved")
		}
		if len(exported.Variants) != 3 {
			t.Errorf("variants not preserved: got %d", len(exported.Variants))
		}
		if len(exported.ContextTags) != 2 {
			t.Errorf("tags not preserved: got %d", len(exported.ContextTags))
		}
		if len(exported.SprintsSeen) != 2 {
			t.Errorf("sprints not preserved: got %d", len(exported.SprintsSeen))
		}
		if exported.PromotedTo != entry.PromotedTo {
			t.Error("promoted_to not preserved")
		}
	})

	t.Run("exports YAML to YAML", func(t *testing.T) {
		dir := t.TempDir()
		sourcePath := filepath.Join(dir, "source.yaml")
		outputPath := filepath.Join(dir, "output.yaml")

		ctx := context.Background()
		store, _ := storage.NewYAMLStorage(ctx, sourcePath)
		entry := &tracking.Entry{
			ID:                "yaml-to-yaml",
			CanonicalQuestion: "YAML to YAML?",
			CurrentAnswer:     "Yes",
			Occurrences:       1,
			FirstSeen:         "2025-01-01",
			LastSeen:          "2025-01-01",
			Status:            "pending",
			Confidence:        "medium",
		}
		store.Create(ctx, entry)
		store.Close()

		var out bytes.Buffer
		cmd := NewExportMemoryCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--source", sourcePath, "--output", outputPath})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("export failed: %v", err)
		}

		yamlStore, _ := storage.NewYAMLStorage(ctx, outputPath)
		defer yamlStore.Close()
		entries, _ := yamlStore.List(ctx, storage.ListFilter{})
		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}
	})
}
