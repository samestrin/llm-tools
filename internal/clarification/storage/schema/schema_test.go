package schema

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestDB opens an in-memory SQLite database for testing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	return db
}

func TestInitialize(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Test initial creation
	version, err := Initialize(ctx, db)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if version != CurrentVersion {
		t.Errorf("Expected version %d, got %d", CurrentVersion, version)
	}

	// Test idempotent creation (run twice)
	version, err = Initialize(ctx, db)
	if err != nil {
		t.Fatalf("Second Initialize failed: %v", err)
	}
	if version != CurrentVersion {
		t.Errorf("Expected version %d after second init, got %d", CurrentVersion, version)
	}
}

func TestEntriesTable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Test insert
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, current_answer, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?)
	`, "test-1", "What is the answer?", "42", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Test select
	var question, answer string
	err = db.QueryRowContext(ctx, "SELECT canonical_question, current_answer FROM entries WHERE id = ?", "test-1").Scan(&question, &answer)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if question != "What is the answer?" || answer != "42" {
		t.Errorf("Unexpected values: question=%q, answer=%q", question, answer)
	}
}

func TestVariantsTable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create parent entry
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, first_seen, last_seen)
		VALUES (?, ?, ?, ?)
	`, "test-1", "Question?", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert entry failed: %v", err)
	}

	// Insert variants
	_, err = db.ExecContext(ctx, `
		INSERT INTO entry_variants (entry_id, variant) VALUES (?, ?)
	`, "test-1", "Variant 1")
	if err != nil {
		t.Fatalf("Insert variant failed: %v", err)
	}

	// Test cascade delete
	_, err = db.ExecContext(ctx, "DELETE FROM entries WHERE id = ?", "test-1")
	if err != nil {
		t.Fatalf("Delete entry failed: %v", err)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entry_variants WHERE entry_id = ?", "test-1").Scan(&count)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 variants after cascade delete, got %d", count)
	}
}

func TestTagsTable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create parent entry
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, first_seen, last_seen)
		VALUES (?, ?, ?, ?)
	`, "test-1", "Question?", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert entry failed: %v", err)
	}

	// Insert tags
	_, err = db.ExecContext(ctx, `
		INSERT INTO entry_tags (entry_id, tag) VALUES (?, ?)
	`, "test-1", "frontend")
	if err != nil {
		t.Fatalf("Insert tag failed: %v", err)
	}

	// Query by tag
	var entryID string
	err = db.QueryRowContext(ctx, "SELECT entry_id FROM entry_tags WHERE tag = ?", "frontend").Scan(&entryID)
	if err != nil {
		t.Fatalf("Select by tag failed: %v", err)
	}
	if entryID != "test-1" {
		t.Errorf("Expected entry_id test-1, got %s", entryID)
	}
}

func TestSprintsTable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Create parent entry
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, first_seen, last_seen)
		VALUES (?, ?, ?, ?)
	`, "test-1", "Question?", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert entry failed: %v", err)
	}

	// Insert sprint reference
	_, err = db.ExecContext(ctx, `
		INSERT INTO entry_sprints (entry_id, sprint) VALUES (?, ?)
	`, "test-1", "sprint-1.0")
	if err != nil {
		t.Fatalf("Insert sprint failed: %v", err)
	}

	// Query by sprint
	var entryID string
	err = db.QueryRowContext(ctx, "SELECT entry_id FROM entry_sprints WHERE sprint = ?", "sprint-1.0").Scan(&entryID)
	if err != nil {
		t.Fatalf("Select by sprint failed: %v", err)
	}
	if entryID != "test-1" {
		t.Errorf("Expected entry_id test-1, got %s", entryID)
	}
}

func TestIndexes(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Check that expected indexes exist
	indexes := []string{
		"idx_entries_status",
		"idx_entries_occurrences",
		"idx_entries_last_seen",
		"idx_entries_created_at",
		"idx_entries_updated_at",
		"idx_tags_tag",
		"idx_sprints_sprint",
	}

	for _, idx := range indexes {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?",
			idx).Scan(&count)
		if err != nil {
			t.Errorf("Failed to check index %s: %v", idx, err)
			continue
		}
		if count == 0 {
			t.Errorf("Index %s does not exist", idx)
		}
	}
}

func TestFTS5(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Insert entry (trigger should populate FTS)
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, current_answer, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?)
	`, "fts-1", "How do I configure authentication?", "Use OAuth2 with JWT tokens", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert entry failed: %v", err)
	}

	// Test FTS MATCH query
	var id string
	err = db.QueryRowContext(ctx,
		"SELECT id FROM entries_fts WHERE entries_fts MATCH ?",
		"authentication").Scan(&id)
	if err != nil {
		t.Fatalf("FTS query failed: %v", err)
	}
	if id != "fts-1" {
		t.Errorf("Expected id fts-1, got %s", id)
	}

	// Test FTS with another term
	err = db.QueryRowContext(ctx,
		"SELECT id FROM entries_fts WHERE entries_fts MATCH ?",
		"OAuth2").Scan(&id)
	if err != nil {
		t.Fatalf("FTS query for OAuth2 failed: %v", err)
	}
	if id != "fts-1" {
		t.Errorf("Expected id fts-1, got %s", id)
	}
}

func TestFTSUpdate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Insert entry
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, current_answer, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?)
	`, "fts-2", "Original question", "Original answer", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Update entry
	_, err = db.ExecContext(ctx,
		"UPDATE entries SET canonical_question = ? WHERE id = ?",
		"Updated authentication question", "fts-2")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify FTS reflects update
	var id string
	err = db.QueryRowContext(ctx,
		"SELECT id FROM entries_fts WHERE entries_fts MATCH ?",
		"authentication").Scan(&id)
	if err != nil {
		t.Fatalf("FTS query after update failed: %v", err)
	}
	if id != "fts-2" {
		t.Errorf("Expected id fts-2, got %s", id)
	}
}

func TestFTSDelete(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Insert entry
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, current_answer, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?)
	`, "fts-3", "Delete test question", "Answer", "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Delete entry
	_, err = db.ExecContext(ctx, "DELETE FROM entries WHERE id = ?", "fts-3")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify FTS reflects delete
	var count int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM entries_fts WHERE entries_fts MATCH ?",
		"Delete").Scan(&count)
	if err != nil {
		t.Fatalf("FTS count query failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 FTS results after delete, got %d", count)
	}
}

func TestValidate(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Should pass validation
	if err := Validate(ctx, db); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestSchemaVersion(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Before initialization, version should error
	_, err := GetVersion(ctx, db)
	if err == nil {
		t.Error("Expected error before initialization")
	}

	// After initialization
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	version, err := GetVersion(ctx, db)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if version != CurrentVersion {
		t.Errorf("Expected version %d, got %d", CurrentVersion, version)
	}
}

func TestUnicodeHandling(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	if _, err := Initialize(ctx, db); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Insert entry with unicode
	unicodeQuestion := "Comment configurer l'authentification?"
	unicodeAnswer := "Utilisez OAuth2 avec les jetons JWT"
	_, err := db.ExecContext(ctx, `
		INSERT INTO entries (id, canonical_question, current_answer, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?)
	`, "unicode-1", unicodeQuestion, unicodeAnswer, "2025-01-01", "2025-01-01")
	if err != nil {
		t.Fatalf("Insert unicode entry failed: %v", err)
	}

	// Verify retrieval
	var question, answer string
	err = db.QueryRowContext(ctx, "SELECT canonical_question, current_answer FROM entries WHERE id = ?", "unicode-1").Scan(&question, &answer)
	if err != nil {
		t.Fatalf("Select unicode entry failed: %v", err)
	}
	if question != unicodeQuestion || answer != unicodeAnswer {
		t.Errorf("Unicode not preserved: got question=%q, answer=%q", question, answer)
	}
}
