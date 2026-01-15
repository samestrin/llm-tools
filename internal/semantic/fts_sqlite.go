package semantic

import (
	"database/sql"
	"fmt"
)

// FTS5 schema and trigger definitions for full-text search support.
// Uses a content-sync FTS5 table that mirrors the chunks table.

// initFTS5Schema creates the FTS5 virtual table and sync triggers.
// This is called during storage initialization.
func (s *SQLiteStorage) initFTS5Schema() error {
	// First check if FTS5 is available
	var fts5Available int
	err := s.db.QueryRow(`SELECT 1 FROM pragma_compile_options WHERE compile_options = 'ENABLE_FTS5'`).Scan(&fts5Available)
	if err != nil {
		// Try creating a test FTS5 table to check availability
		_, testErr := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS _fts5_test USING fts5(test)`)
		if testErr != nil {
			return fmt.Errorf("FTS5 extension is not available in this SQLite build: %w", testErr)
		}
		// Clean up test table
		s.db.Exec(`DROP TABLE IF EXISTS _fts5_test`)
	}

	// Create FTS5 virtual table with content sync
	// We use a content table approach where FTS5 stores its own copy of the data
	// This is more reliable than external content tables for trigger-based sync
	fts5Schema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
		name,
		content,
		tokenize='porter unicode61'
	);
	`

	if _, err := s.db.Exec(fts5Schema); err != nil {
		return fmt.Errorf("failed to create FTS5 virtual table: %w", err)
	}

	// Create INSERT trigger to sync new chunks to FTS
	insertTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_insert
	AFTER INSERT ON chunks
	BEGIN
		INSERT INTO chunks_fts(rowid, name, content)
		VALUES (NEW.rowid, NEW.name, COALESCE(NEW.content, ''));
	END;
	`

	if _, err := s.db.Exec(insertTrigger); err != nil {
		return fmt.Errorf("failed to create INSERT trigger: %w", err)
	}

	// Create UPDATE trigger to sync chunk changes to FTS
	// For FTS5, we need to delete the old entry and insert the new one
	updateTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_update
	AFTER UPDATE ON chunks
	BEGIN
		DELETE FROM chunks_fts WHERE rowid = OLD.rowid;
		INSERT INTO chunks_fts(rowid, name, content)
		VALUES (NEW.rowid, NEW.name, COALESCE(NEW.content, ''));
	END;
	`

	if _, err := s.db.Exec(updateTrigger); err != nil {
		return fmt.Errorf("failed to create UPDATE trigger: %w", err)
	}

	// Create DELETE trigger to remove chunks from FTS
	deleteTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_delete
	AFTER DELETE ON chunks
	BEGIN
		DELETE FROM chunks_fts WHERE rowid = OLD.rowid;
	END;
	`

	if _, err := s.db.Exec(deleteTrigger); err != nil {
		return fmt.Errorf("failed to create DELETE trigger: %w", err)
	}

	// Backfill existing chunks if FTS is empty but chunks exist
	// This handles migration from pre-FTS databases
	if err := s.backfillFTS5(); err != nil {
		return fmt.Errorf("failed to backfill FTS5 index: %w", err)
	}

	return nil
}

// backfillFTS5 populates the FTS5 index from existing chunks.
// This is used for migration when upgrading an existing database.
func (s *SQLiteStorage) backfillFTS5() error {
	// Check if we need to backfill
	var ftsCount, chunksCount int

	err := s.db.QueryRow(`SELECT COUNT(*) FROM chunks_fts`).Scan(&ftsCount)
	if err != nil {
		return fmt.Errorf("failed to count FTS entries: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunksCount)
	if err != nil {
		return fmt.Errorf("failed to count chunks: %w", err)
	}

	// Only backfill if chunks exist but FTS is empty
	if chunksCount > 0 && ftsCount == 0 {
		// Use INSERT...SELECT for efficient batch backfill
		_, err := s.db.Exec(`
			INSERT INTO chunks_fts(rowid, name, content)
			SELECT rowid, name, COALESCE(content, '')
			FROM chunks
		`)
		if err != nil {
			return fmt.Errorf("failed to backfill FTS5 from chunks: %w", err)
		}
	}

	return nil
}

// RebuildFTSIndex rebuilds the FTS5 index from scratch.
// Use this if the FTS index becomes corrupted or out of sync.
func (s *SQLiteStorage) RebuildFTSIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Clear the FTS index
	if _, err := s.db.Exec(`DELETE FROM chunks_fts`); err != nil {
		return fmt.Errorf("failed to clear FTS index: %w", err)
	}

	// Rebuild from chunks table
	if _, err := s.db.Exec(`
		INSERT INTO chunks_fts(rowid, name, content)
		SELECT rowid, name, COALESCE(content, '')
		FROM chunks
	`); err != nil {
		return fmt.Errorf("failed to rebuild FTS index: %w", err)
	}

	// Optimize the FTS index
	if _, err := s.db.Exec(`INSERT INTO chunks_fts(chunks_fts) VALUES('optimize')`); err != nil {
		// Optimization failure is not critical, just log and continue
		return nil
	}

	return nil
}

// FTSIntegrityCheck verifies the FTS5 index integrity.
// Returns an error if the index is corrupted.
func (s *SQLiteStorage) FTSIntegrityCheck() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Run FTS5 integrity check
	rows, err := s.db.Query(`INSERT INTO chunks_fts(chunks_fts) VALUES('integrity-check')`)
	if err != nil {
		return fmt.Errorf("FTS5 index integrity check failed: %w", err)
	}
	defer rows.Close()

	return nil
}

// clearFTS5 removes all entries from the FTS5 index.
// Called by Clear() to maintain consistency.
func (s *SQLiteStorage) clearFTS5() error {
	_, err := s.db.Exec(`DELETE FROM chunks_fts`)
	if err != nil {
		return fmt.Errorf("failed to clear FTS5 index: %w", err)
	}
	return nil
}

// LexicalSearchResult represents a result from lexical (FTS5) search.
type LexicalSearchResult struct {
	ChunkID string
	Name    string
	Content string
	Score   float64
}

// LexicalSearchOptions configures lexical search behavior.
type LexicalSearchOptions struct {
	TopK       int     // Maximum results to return (default: 10)
	Type       string  // Filter by chunk type
	PathFilter string  // Filter by file path prefix
	Threshold  float64 // Minimum BM25 score (more negative = more relevant)
}

// LexicalSearch performs a full-text search using FTS5.
// Returns results ranked by BM25 relevance score.
func (s *SQLiteStorage) LexicalSearch(query string, opts LexicalSearchOptions) ([]LexicalSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	if query == "" {
		return []LexicalSearchResult{}, nil
	}

	// Set defaults
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	// Build the query with optional filters
	// Join with chunks table to get full chunk info and apply filters
	sqlQuery := `
		SELECT c.id, f.name, f.content, bm25(chunks_fts) as score
		FROM chunks_fts f
		JOIN chunks c ON c.rowid = f.rowid
		WHERE chunks_fts MATCH ?
	`
	args := []interface{}{query}

	if opts.Type != "" {
		sqlQuery += ` AND c.type = ?`
		args = append(args, opts.Type)
	}

	if opts.PathFilter != "" {
		sqlQuery += ` AND c.file_path LIKE ?`
		args = append(args, opts.PathFilter+"%")
	}

	sqlQuery += ` ORDER BY bm25(chunks_fts) LIMIT ?`
	args = append(args, topK)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		// Check for FTS5 syntax errors and return a clearer message
		if isFTS5SyntaxError(err) {
			return nil, fmt.Errorf("fts5 query syntax error: %w", err)
		}
		return nil, fmt.Errorf("failed to execute lexical search: %w", err)
	}
	defer rows.Close()

	var results []LexicalSearchResult
	for rows.Next() {
		var r LexicalSearchResult
		if err := rows.Scan(&r.ChunkID, &r.Name, &r.Content, &r.Score); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return results, nil
}

// isFTS5SyntaxError checks if the error is an FTS5 syntax error.
func isFTS5SyntaxError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "fts5: syntax error") ||
		contains(errStr, "malformed MATCH expression")
}

// contains checks if s contains substr (simple implementation to avoid strings import).
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// EscapeFTS5Query escapes special FTS5 characters in a query string.
// This prevents injection and syntax errors from user input.
func EscapeFTS5Query(query string) string {
	// FTS5 special characters that need escaping: " * - + ( ) : ^
	var result []byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch c {
		case '"', '*', '-', '+', '(', ')', ':', '^':
			result = append(result, '"')
			result = append(result, c)
			result = append(result, '"')
		default:
			result = append(result, c)
		}
	}
	return string(result)
}

// Ensure FTS5 is not being explicitly checked (for migration)
func ftsTableExists(db *sql.DB) bool {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='chunks_fts'
	`).Scan(&count)
	return err == nil && count > 0
}
