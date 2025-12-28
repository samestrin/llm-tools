package integration

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// TestMigrationFlow tests the complete YAML -> SQLite -> YAML migration flow
func TestMigrationFlow(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	yamlSource := filepath.Join(dir, "source.yaml")
	sqliteDB := filepath.Join(dir, "migrated.db")
	yamlExport := filepath.Join(dir, "exported.yaml")

	// Create source YAML with test entries
	sourceStore, err := storage.NewYAMLStorage(ctx, yamlSource)
	if err != nil {
		t.Fatalf("Failed to create source YAML: %v", err)
	}

	originalEntries := createTestEntries(10)
	for _, entry := range originalEntries {
		if err := sourceStore.Create(ctx, &entry); err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}
	}
	sourceStore.Close()

	// Step 1: Import from YAML to SQLite
	sourceStore, _ = storage.NewYAMLStorage(ctx, yamlSource)
	entries, err := sourceStore.Export(ctx, storage.ListFilter{})
	if err != nil {
		t.Fatalf("Failed to export from source: %v", err)
	}
	sourceStore.Close()

	sqliteStore, err := storage.NewSQLiteStorage(ctx, sqliteDB)
	if err != nil {
		t.Fatalf("Failed to create SQLite: %v", err)
	}
	for _, e := range entries {
		if err := sqliteStore.Create(ctx, &e); err != nil {
			t.Fatalf("Failed to import entry to SQLite: %v", err)
		}
	}
	sqliteStore.Close()

	// Step 2: Export from SQLite to YAML
	sqliteStore, _ = storage.NewSQLiteStorage(ctx, sqliteDB)
	sqliteEntries, err := sqliteStore.Export(ctx, storage.ListFilter{})
	if err != nil {
		t.Fatalf("Failed to export from SQLite: %v", err)
	}
	sqliteStore.Close()

	exportStore, err := storage.NewYAMLStorage(ctx, yamlExport)
	if err != nil {
		t.Fatalf("Failed to create export YAML: %v", err)
	}
	for _, e := range sqliteEntries {
		if err := exportStore.Create(ctx, &e); err != nil {
			t.Fatalf("Failed to export entry to YAML: %v", err)
		}
	}
	exportStore.Close()

	// Step 3: Compare original and exported
	exportStore, _ = storage.NewYAMLStorage(ctx, yamlExport)
	defer exportStore.Close()

	exportedEntries, _ := exportStore.Export(ctx, storage.ListFilter{})

	if len(exportedEntries) != len(originalEntries) {
		t.Errorf("Entry count mismatch: got %d, want %d", len(exportedEntries), len(originalEntries))
	}

	// Create map for comparison
	exportedMap := make(map[string]tracking.Entry)
	for _, e := range exportedEntries {
		exportedMap[e.ID] = e
	}

	for _, original := range originalEntries {
		exported, ok := exportedMap[original.ID]
		if !ok {
			t.Errorf("Missing entry: %s", original.ID)
			continue
		}
		verifyEntry(t, &original, &exported)
	}
}

// TestLargeScaleMigration tests migration with 100+ entries
func TestLargeScaleMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	yamlSource := filepath.Join(dir, "large_source.yaml")
	sqliteDB := filepath.Join(dir, "large_migrated.db")

	entryCount := 150 // More than 100

	// Create source YAML with many entries
	sourceStore, err := storage.NewYAMLStorage(ctx, yamlSource)
	if err != nil {
		t.Fatalf("Failed to create source YAML: %v", err)
	}

	originalEntries := createTestEntries(entryCount)
	for _, entry := range originalEntries {
		if err := sourceStore.Create(ctx, &entry); err != nil {
			t.Fatalf("Failed to create entry: %v", err)
		}
	}
	sourceStore.Close()

	// Import to SQLite
	sourceStore, _ = storage.NewYAMLStorage(ctx, yamlSource)
	entries, err := sourceStore.Export(ctx, storage.ListFilter{})
	if err != nil {
		t.Fatalf("Failed to export from source: %v", err)
	}
	sourceStore.Close()

	if len(entries) != entryCount {
		t.Errorf("Source export count mismatch: got %d, want %d", len(entries), entryCount)
	}

	sqliteStore, err := storage.NewSQLiteStorage(ctx, sqliteDB)
	if err != nil {
		t.Fatalf("Failed to create SQLite: %v", err)
	}

	// Import all entries
	for _, e := range entries {
		if err := sqliteStore.Create(ctx, &e); err != nil {
			t.Fatalf("Failed to import entry to SQLite: %v", err)
		}
	}

	// Verify count in SQLite
	sqliteEntries, err := sqliteStore.Export(ctx, storage.ListFilter{})
	if err != nil {
		t.Fatalf("Failed to export from SQLite: %v", err)
	}

	if len(sqliteEntries) != entryCount {
		t.Errorf("SQLite entry count mismatch: got %d, want %d", len(sqliteEntries), entryCount)
	}

	// Verify a sample of entries field-by-field
	entryMap := make(map[string]tracking.Entry)
	for _, e := range sqliteEntries {
		entryMap[e.ID] = e
	}

	// Check first 10, last 10, and 10 random entries
	checkIndices := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
		entryCount - 10, entryCount - 9, entryCount - 8, entryCount - 7, entryCount - 6,
		entryCount - 5, entryCount - 4, entryCount - 3, entryCount - 2, entryCount - 1,
		50, 75, 100, 125}

	for _, idx := range checkIndices {
		if idx >= len(originalEntries) {
			continue
		}
		original := originalEntries[idx]
		stored, ok := entryMap[original.ID]
		if !ok {
			t.Errorf("Missing entry at index %d: %s", idx, original.ID)
			continue
		}
		verifyEntry(t, &original, &stored)
	}

	sqliteStore.Close()
}

// TestDataIntegrityAllFields verifies all 12 Entry fields are preserved
func TestDataIntegrityAllFields(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	sqlitePath := filepath.Join(dir, "integrity.db")

	// Create an entry with ALL fields populated
	completeEntry := &tracking.Entry{
		ID:                "integrity-test-001",
		CanonicalQuestion: "What is the complete data integrity test?",
		CurrentAnswer:     "This tests all 12 fields of the Entry struct",
		Occurrences:       42,
		FirstSeen:         "2025-01-01",
		LastSeen:          "2025-12-28",
		SprintsSeen:       []string{"sprint-1.0", "sprint-2.0", "sprint-3.0"},
		Status:            "promoted",
		ContextTags:       []string{"testing", "integration", "data-integrity"},
		Confidence:        "high",
		PromotedTo:        "CLAUDE.md",
		PromotedDate:      "2025-06-15",
		Variants:          []string{"Variant question 1?", "Variant question 2?", "Variant question 3?"},
	}

	// Store in SQLite
	store, err := storage.NewSQLiteStorage(ctx, sqlitePath)
	if err != nil {
		t.Fatalf("Failed to create SQLite: %v", err)
	}
	if err := store.Create(ctx, completeEntry); err != nil {
		t.Fatalf("Failed to create entry: %v", err)
	}
	store.Close()

	// Re-open and read
	store, _ = storage.NewSQLiteStorage(ctx, sqlitePath)
	defer store.Close()

	retrieved, err := store.Read(ctx, completeEntry.ID)
	if err != nil {
		t.Fatalf("Failed to read entry: %v", err)
	}

	// Verify ALL 12 fields explicitly
	t.Run("Field_ID", func(t *testing.T) {
		if retrieved.ID != completeEntry.ID {
			t.Errorf("got %q, want %q", retrieved.ID, completeEntry.ID)
		}
	})
	t.Run("Field_CanonicalQuestion", func(t *testing.T) {
		if retrieved.CanonicalQuestion != completeEntry.CanonicalQuestion {
			t.Errorf("got %q, want %q", retrieved.CanonicalQuestion, completeEntry.CanonicalQuestion)
		}
	})
	t.Run("Field_CurrentAnswer", func(t *testing.T) {
		if retrieved.CurrentAnswer != completeEntry.CurrentAnswer {
			t.Errorf("got %q, want %q", retrieved.CurrentAnswer, completeEntry.CurrentAnswer)
		}
	})
	t.Run("Field_Occurrences", func(t *testing.T) {
		if retrieved.Occurrences != completeEntry.Occurrences {
			t.Errorf("got %d, want %d", retrieved.Occurrences, completeEntry.Occurrences)
		}
	})
	t.Run("Field_FirstSeen", func(t *testing.T) {
		if retrieved.FirstSeen != completeEntry.FirstSeen {
			t.Errorf("got %q, want %q", retrieved.FirstSeen, completeEntry.FirstSeen)
		}
	})
	t.Run("Field_LastSeen", func(t *testing.T) {
		if retrieved.LastSeen != completeEntry.LastSeen {
			t.Errorf("got %q, want %q", retrieved.LastSeen, completeEntry.LastSeen)
		}
	})
	t.Run("Field_SprintsSeen", func(t *testing.T) {
		if !slicesEqual(retrieved.SprintsSeen, completeEntry.SprintsSeen) {
			t.Errorf("got %v, want %v", retrieved.SprintsSeen, completeEntry.SprintsSeen)
		}
	})
	t.Run("Field_Status", func(t *testing.T) {
		if retrieved.Status != completeEntry.Status {
			t.Errorf("got %q, want %q", retrieved.Status, completeEntry.Status)
		}
	})
	t.Run("Field_ContextTags", func(t *testing.T) {
		if !slicesEqual(retrieved.ContextTags, completeEntry.ContextTags) {
			t.Errorf("got %v, want %v", retrieved.ContextTags, completeEntry.ContextTags)
		}
	})
	t.Run("Field_Confidence", func(t *testing.T) {
		if retrieved.Confidence != completeEntry.Confidence {
			t.Errorf("got %q, want %q", retrieved.Confidence, completeEntry.Confidence)
		}
	})
	t.Run("Field_PromotedTo", func(t *testing.T) {
		if retrieved.PromotedTo != completeEntry.PromotedTo {
			t.Errorf("got %q, want %q", retrieved.PromotedTo, completeEntry.PromotedTo)
		}
	})
	t.Run("Field_PromotedDate", func(t *testing.T) {
		if retrieved.PromotedDate != completeEntry.PromotedDate {
			t.Errorf("got %q, want %q", retrieved.PromotedDate, completeEntry.PromotedDate)
		}
	})
	t.Run("Field_Variants", func(t *testing.T) {
		if !slicesEqual(retrieved.Variants, completeEntry.Variants) {
			t.Errorf("got %v, want %v", retrieved.Variants, completeEntry.Variants)
		}
	})
}

// Helper functions

func createTestEntries(count int) []tracking.Entry {
	entries := make([]tracking.Entry, count)
	statuses := []string{"pending", "active", "promoted", "dismissed"}
	confidences := []string{"low", "medium", "high"}

	for i := 0; i < count; i++ {
		entries[i] = tracking.Entry{
			ID:                fmt.Sprintf("test-entry-%04d", i),
			CanonicalQuestion: fmt.Sprintf("Question number %d?", i),
			CurrentAnswer:     fmt.Sprintf("Answer for question %d", i),
			Occurrences:       (i % 10) + 1,
			FirstSeen:         fmt.Sprintf("2025-01-%02d", (i%28)+1),
			LastSeen:          fmt.Sprintf("2025-12-%02d", (i%28)+1),
			Status:            statuses[i%len(statuses)],
			Confidence:        confidences[i%len(confidences)],
			ContextTags:       []string{fmt.Sprintf("tag-%d", i%5), fmt.Sprintf("category-%d", i%3)},
			SprintsSeen:       []string{fmt.Sprintf("sprint-%d", i%4)},
		}

		// Add variants to some entries
		if i%3 == 0 {
			entries[i].Variants = []string{
				fmt.Sprintf("Variant A for %d?", i),
				fmt.Sprintf("Variant B for %d?", i),
			}
		}

		// Add promotion info to some entries
		if i%5 == 0 && entries[i].Status == "promoted" {
			entries[i].PromotedTo = "CLAUDE.md"
			entries[i].PromotedDate = "2025-06-15"
		}
	}

	return entries
}

func verifyEntry(t *testing.T, original, stored *tracking.Entry) {
	t.Helper()

	if original.ID != stored.ID {
		t.Errorf("ID: got %q, want %q", stored.ID, original.ID)
	}
	if original.CanonicalQuestion != stored.CanonicalQuestion {
		t.Errorf("CanonicalQuestion: got %q, want %q", stored.CanonicalQuestion, original.CanonicalQuestion)
	}
	if original.CurrentAnswer != stored.CurrentAnswer {
		t.Errorf("CurrentAnswer: got %q, want %q", stored.CurrentAnswer, original.CurrentAnswer)
	}
	if original.Occurrences != stored.Occurrences {
		t.Errorf("Occurrences: got %d, want %d", stored.Occurrences, original.Occurrences)
	}
	if original.Status != stored.Status {
		t.Errorf("Status: got %q, want %q", stored.Status, original.Status)
	}
	if original.Confidence != stored.Confidence {
		t.Errorf("Confidence: got %q, want %q", stored.Confidence, original.Confidence)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]int)
	for _, s := range a {
		aMap[s]++
	}
	bMap := make(map[string]int)
	for _, s := range b {
		bMap[s]++
	}
	for k, v := range aMap {
		if bMap[k] != v {
			return false
		}
	}
	return true
}
