package storage

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

func TestRoundTripIntegrity(t *testing.T) {
	// Create a complete entry with all 12 fields populated
	originalEntry := &tracking.Entry{
		ID:                "roundtrip-test",
		CanonicalQuestion: "What is the answer to life, the universe, and everything?",
		CurrentAnswer:     "42, according to The Hitchhiker's Guide to the Galaxy",
		Occurrences:       7,
		FirstSeen:         "2025-01-01",
		LastSeen:          "2025-01-28",
		SprintsSeen:       []string{"sprint-1.0", "sprint-2.0", "sprint-3.0"},
		Status:            "promoted",
		ContextTags:       []string{"philosophy", "humor", "deep-thought"},
		Confidence:        "high",
		PromotedTo:        "docs/faq.md",
		PromotedDate:      "2025-01-15",
		Variants:          []string{"What's the meaning of life?", "The ultimate question?", "42?"},
	}

	t.Run("YAML -> SQLite -> YAML preserves all fields", func(t *testing.T) {
		dir := t.TempDir()
		yamlPath1 := filepath.Join(dir, "original.yaml")
		sqlitePath := filepath.Join(dir, "intermediate.db")
		yamlPath2 := filepath.Join(dir, "roundtrip.yaml")

		ctx := context.Background()

		// Step 1: Create YAML storage and add entry
		yamlStore1, err := NewYAMLStorage(ctx, yamlPath1)
		if err != nil {
			t.Fatalf("failed to create YAML storage: %v", err)
		}
		if err := yamlStore1.Create(ctx, originalEntry); err != nil {
			t.Fatalf("failed to create entry in YAML: %v", err)
		}
		yamlStore1.Close()

		// Step 2: Read from YAML and import to SQLite
		yamlStore1, _ = NewYAMLStorage(ctx, yamlPath1)
		entries, _ := yamlStore1.Export(ctx, ListFilter{})
		yamlStore1.Close()

		sqliteStore, err := NewSQLiteStorage(ctx, sqlitePath)
		if err != nil {
			t.Fatalf("failed to create SQLite storage: %v", err)
		}
		for _, e := range entries {
			if err := sqliteStore.Create(ctx, &e); err != nil {
				t.Fatalf("failed to create entry in SQLite: %v", err)
			}
		}
		sqliteStore.Close()

		// Step 3: Read from SQLite and export to YAML
		sqliteStore, _ = NewSQLiteStorage(ctx, sqlitePath)
		sqliteEntries, _ := sqliteStore.Export(ctx, ListFilter{})
		sqliteStore.Close()

		yamlStore2, err := NewYAMLStorage(ctx, yamlPath2)
		if err != nil {
			t.Fatalf("failed to create second YAML storage: %v", err)
		}
		for _, e := range sqliteEntries {
			if err := yamlStore2.Create(ctx, &e); err != nil {
				t.Fatalf("failed to create entry in second YAML: %v", err)
			}
		}
		yamlStore2.Close()

		// Step 4: Read final YAML and verify
		yamlStore2, _ = NewYAMLStorage(ctx, yamlPath2)
		defer yamlStore2.Close()

		finalEntry, err := yamlStore2.Read(ctx, "roundtrip-test")
		if err != nil {
			t.Fatalf("failed to read final entry: %v", err)
		}

		// Verify all fields
		verifyEntryFields(t, originalEntry, finalEntry)
	})

	t.Run("SQLite -> YAML -> SQLite preserves all fields", func(t *testing.T) {
		dir := t.TempDir()
		sqlitePath1 := filepath.Join(dir, "original.db")
		yamlPath := filepath.Join(dir, "intermediate.yaml")
		sqlitePath2 := filepath.Join(dir, "roundtrip.db")

		ctx := context.Background()

		// Step 1: Create SQLite storage and add entry
		sqliteStore1, err := NewSQLiteStorage(ctx, sqlitePath1)
		if err != nil {
			t.Fatalf("failed to create SQLite storage: %v", err)
		}
		if err := sqliteStore1.Create(ctx, originalEntry); err != nil {
			t.Fatalf("failed to create entry in SQLite: %v", err)
		}
		sqliteStore1.Close()

		// Step 2: Read from SQLite and export to YAML
		sqliteStore1, _ = NewSQLiteStorage(ctx, sqlitePath1)
		entries, _ := sqliteStore1.Export(ctx, ListFilter{})
		sqliteStore1.Close()

		yamlStore, err := NewYAMLStorage(ctx, yamlPath)
		if err != nil {
			t.Fatalf("failed to create YAML storage: %v", err)
		}
		for _, e := range entries {
			if err := yamlStore.Create(ctx, &e); err != nil {
				t.Fatalf("failed to create entry in YAML: %v", err)
			}
		}
		yamlStore.Close()

		// Step 3: Read from YAML and import to second SQLite
		yamlStore, _ = NewYAMLStorage(ctx, yamlPath)
		yamlEntries, _ := yamlStore.Export(ctx, ListFilter{})
		yamlStore.Close()

		sqliteStore2, err := NewSQLiteStorage(ctx, sqlitePath2)
		if err != nil {
			t.Fatalf("failed to create second SQLite storage: %v", err)
		}
		for _, e := range yamlEntries {
			if err := sqliteStore2.Create(ctx, &e); err != nil {
				t.Fatalf("failed to create entry in second SQLite: %v", err)
			}
		}
		sqliteStore2.Close()

		// Step 4: Read final SQLite and verify
		sqliteStore2, _ = NewSQLiteStorage(ctx, sqlitePath2)
		defer sqliteStore2.Close()

		finalEntry, err := sqliteStore2.Read(ctx, "roundtrip-test")
		if err != nil {
			t.Fatalf("failed to read final entry: %v", err)
		}

		// Verify all fields
		verifyEntryFields(t, originalEntry, finalEntry)
	})

	t.Run("multiple entries preserve order and data", func(t *testing.T) {
		dir := t.TempDir()
		yamlPath := filepath.Join(dir, "source.yaml")
		sqlitePath := filepath.Join(dir, "target.db")

		ctx := context.Background()

		// Create multiple entries
		entries := []tracking.Entry{
			{
				ID:                "entry-1",
				CanonicalQuestion: "First question?",
				CurrentAnswer:     "First answer",
				Occurrences:       1,
				FirstSeen:         "2025-01-01",
				LastSeen:          "2025-01-01",
				Status:            "pending",
				Confidence:        "medium",
				Variants:          []string{"Q1 variant"},
				ContextTags:       []string{"tag1"},
				SprintsSeen:       []string{"sprint-1"},
			},
			{
				ID:                "entry-2",
				CanonicalQuestion: "Second question?",
				CurrentAnswer:     "Second answer",
				Occurrences:       5,
				FirstSeen:         "2025-01-02",
				LastSeen:          "2025-01-10",
				Status:            "active",
				Confidence:        "high",
				Variants:          []string{"Q2 v1", "Q2 v2"},
				ContextTags:       []string{"tag2", "tag3"},
				SprintsSeen:       []string{"sprint-1", "sprint-2"},
			},
			{
				ID:                "entry-3",
				CanonicalQuestion: "Third question?",
				CurrentAnswer:     "Third answer",
				Occurrences:       3,
				FirstSeen:         "2025-01-03",
				LastSeen:          "2025-01-05",
				Status:            "resolved",
				Confidence:        "low",
				PromotedTo:        "docs/readme.md",
				PromotedDate:      "2025-01-05",
			},
		}

		// Create YAML storage and add entries
		yamlStore, _ := NewYAMLStorage(ctx, yamlPath)
		for _, e := range entries {
			yamlStore.Create(ctx, &e)
		}
		yamlStore.Close()

		// Import to SQLite
		yamlStore, _ = NewYAMLStorage(ctx, yamlPath)
		yamlEntries, _ := yamlStore.Export(ctx, ListFilter{})
		yamlStore.Close()

		sqliteStore, _ := NewSQLiteStorage(ctx, sqlitePath)
		for _, e := range yamlEntries {
			sqliteStore.Create(ctx, &e)
		}
		sqliteStore.Close()

		// Verify each entry
		sqliteStore, _ = NewSQLiteStorage(ctx, sqlitePath)
		defer sqliteStore.Close()

		for _, original := range entries {
			stored, err := sqliteStore.Read(ctx, original.ID)
			if err != nil {
				t.Errorf("failed to read entry %s: %v", original.ID, err)
				continue
			}
			verifyEntryFields(t, &original, stored)
		}
	})
}

func verifyEntryFields(t *testing.T, original, final *tracking.Entry) {
	t.Helper()

	if original.ID != final.ID {
		t.Errorf("ID mismatch: got %q, want %q", final.ID, original.ID)
	}
	if original.CanonicalQuestion != final.CanonicalQuestion {
		t.Errorf("CanonicalQuestion mismatch: got %q, want %q", final.CanonicalQuestion, original.CanonicalQuestion)
	}
	if original.CurrentAnswer != final.CurrentAnswer {
		t.Errorf("CurrentAnswer mismatch: got %q, want %q", final.CurrentAnswer, original.CurrentAnswer)
	}
	if original.Occurrences != final.Occurrences {
		t.Errorf("Occurrences mismatch: got %d, want %d", final.Occurrences, original.Occurrences)
	}
	if original.FirstSeen != final.FirstSeen {
		t.Errorf("FirstSeen mismatch: got %q, want %q", final.FirstSeen, original.FirstSeen)
	}
	if original.LastSeen != final.LastSeen {
		t.Errorf("LastSeen mismatch: got %q, want %q", final.LastSeen, original.LastSeen)
	}
	if original.Status != final.Status {
		t.Errorf("Status mismatch: got %q, want %q", final.Status, original.Status)
	}
	if original.Confidence != final.Confidence {
		t.Errorf("Confidence mismatch: got %q, want %q", final.Confidence, original.Confidence)
	}
	if original.PromotedTo != final.PromotedTo {
		t.Errorf("PromotedTo mismatch: got %q, want %q", final.PromotedTo, original.PromotedTo)
	}
	if original.PromotedDate != final.PromotedDate {
		t.Errorf("PromotedDate mismatch: got %q, want %q", final.PromotedDate, original.PromotedDate)
	}

	// Compare slices (order may vary)
	if !equalStringSlices(original.Variants, final.Variants) {
		t.Errorf("Variants mismatch: got %v, want %v", final.Variants, original.Variants)
	}
	if !equalStringSlices(original.ContextTags, final.ContextTags) {
		t.Errorf("ContextTags mismatch: got %v, want %v", final.ContextTags, original.ContextTags)
	}
	if !equalStringSlices(original.SprintsSeen, final.SprintsSeen) {
		t.Errorf("SprintsSeen mismatch: got %v, want %v", final.SprintsSeen, original.SprintsSeen)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for comparison (order-independent)
	aMap := make(map[string]int)
	for _, s := range a {
		aMap[s]++
	}
	bMap := make(map[string]int)
	for _, s := range b {
		bMap[s]++
	}

	return reflect.DeepEqual(aMap, bMap)
}
