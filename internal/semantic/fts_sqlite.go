package semantic

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	// Use COALESCE for both name and content to handle NULLs safely
	insertTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_insert
	AFTER INSERT ON chunks
	BEGIN
		INSERT INTO chunks_fts(rowid, name, content)
		VALUES (NEW.rowid, COALESCE(NEW.name, ''), COALESCE(NEW.content, ''));
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
		VALUES (NEW.rowid, COALESCE(NEW.name, ''), COALESCE(NEW.content, ''));
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
			SELECT rowid, COALESCE(name, ''), COALESCE(content, '')
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

	// Run FTS5 integrity check - this returns rows with any issues found
	rows, err := s.db.Query(`SELECT * FROM chunks_fts WHERE chunks_fts MATCH 'integrity-check'`)
	if err != nil {
		// Try the insert command for older SQLite versions
		_, execErr := s.db.Exec(`INSERT INTO chunks_fts(chunks_fts) VALUES('integrity-check')`)
		if execErr != nil {
			return fmt.Errorf("FTS5 index integrity check failed: %w", execErr)
		}
		return nil
	}
	defer rows.Close()

	// If any rows are returned, there are integrity issues
	if rows.Next() {
		var issue string
		if err := rows.Scan(&issue); err == nil && issue != "" {
			return fmt.Errorf("FTS5 index integrity check failed: %s", issue)
		}
	}

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

// LexicalSearch performs a full-text search using FTS5.
// Returns results ranked by BM25 relevance score.
// The score is converted from BM25 (where more negative is better) to a
// positive scale where higher is better for consistency with vector search.
func (s *SQLiteStorage) LexicalSearch(ctx context.Context, query string, opts LexicalSearchOptions) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	if query == "" {
		return []SearchResult{}, nil
	}

	// Set defaults
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	// Build the query with optional filters
	// Join with chunks table to get full chunk info and apply filters
	sqlQuery := `
		SELECT c.id, c.file_path, c.type, c.name, c.signature, c.content,
		       c.start_line, c.end_line, c.language, bm25(chunks_fts) as score
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

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		// Check for FTS5 syntax errors and return a clearer message
		if isFTS5SyntaxError(err) {
			return nil, fmt.Errorf("fts5 query syntax error: %w", err)
		}
		return nil, fmt.Errorf("failed to execute lexical search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var chunk Chunk
		var typeStr string
		var bm25Score float64

		if err := rows.Scan(
			&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name, &chunk.Signature,
			&chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Language, &bm25Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		chunkType, _ := ParseChunkType(typeStr)
		chunk.Type = chunkType

		// Convert BM25 score (more negative = better) to positive scale
		// We use -bm25Score so higher is better, matching vector search convention
		results = append(results, SearchResult{
			Chunk: chunk,
			Score: float32(-bm25Score),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	// Ensure we return empty slice instead of nil for consistency
	if results == nil {
		results = []SearchResult{}
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

// contains checks if s contains substr.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// EscapeFTS5Query escapes special FTS5 characters in a query string.
// This prevents FTS5 syntax errors from user input containing operators.
// Note: This converts FTS5 operators to literal text - if you want to
// allow users to use FTS5 operators (AND, OR, NOT, *), don't escape.
func EscapeFTS5Query(query string) string {
	// FTS5 special characters that need quoting: " * - + ( ) : ^ \
	// We wrap each special char in quotes to treat it literally
	var result []byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch c {
		case '"':
			// Double quotes need to be doubled inside quoted strings
			result = append(result, '"', '"')
		case '*', '-', '+', '(', ')', ':', '^', '\\':
			result = append(result, '"')
			result = append(result, c)
			result = append(result, '"')
		default:
			result = append(result, c)
		}
	}
	return string(result)
}

// SafeLexicalSearch performs a lexical search with automatic query escaping.
// Use this when the query comes from untrusted user input.
// For advanced users who want FTS5 operators, use LexicalSearch directly.
func (s *SQLiteStorage) SafeLexicalSearch(ctx context.Context, query string, opts LexicalSearchOptions) ([]SearchResult, error) {
	return s.LexicalSearch(ctx, EscapeFTS5Query(query), opts)
}

// LexicalSearchMemory performs a full-text search on memory entries using FTS5.
// Returns results ranked by BM25 relevance score (higher = more relevant).
func (s *SQLiteStorage) LexicalSearchMemory(ctx context.Context, query string, opts MemoryLexicalSearchOptions) ([]MemorySearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	if query == "" {
		return []MemorySearchResult{}, nil
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	// Build query joining memory_fts with memory table
	sqlQuery := `
		SELECT m.id, m.question, m.answer, m.tags, m.source, m.status,
		       m.occurrences, m.file_path, m.sprints, m.files,
		       m.created_at, m.updated_at, bm25(memory_fts) as score
		FROM memory_fts f
		JOIN memory m ON m.rowid = f.rowid
		WHERE memory_fts MATCH ?
	`
	args := []interface{}{EscapeFTS5Query(query)}

	if opts.Status != "" {
		sqlQuery += ` AND m.status = ?`
		args = append(args, string(opts.Status))
	}

	if opts.Source != "" {
		sqlQuery += ` AND m.source = ?`
		args = append(args, opts.Source)
	}

	sqlQuery += ` ORDER BY bm25(memory_fts) LIMIT ?`
	args = append(args, topK)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		if isFTS5SyntaxError(err) {
			return nil, fmt.Errorf("fts5 query syntax error: %w", err)
		}
		return nil, fmt.Errorf("failed to execute lexical memory search: %w", err)
	}
	defer rows.Close()

	var results []MemorySearchResult
	for rows.Next() {
		var entry MemoryEntry
		var tagsStr, sprintsStr, filesStr sql.NullString
		var filePath, source sql.NullString
		var bm25Score float64

		if err := rows.Scan(
			&entry.ID, &entry.Question, &entry.Answer,
			&tagsStr, &source, &entry.Status,
			&entry.Occurrences, &filePath, &sprintsStr, &filesStr,
			&entry.CreatedAt, &entry.UpdatedAt, &bm25Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan memory result: %w", err)
		}

		if source.Valid {
			entry.Source = source.String
		}
		if filePath.Valid {
			entry.FilePath = filePath.String
		}
		if tagsStr.Valid && tagsStr.String != "" {
			entry.Tags = splitCSV(tagsStr.String)
		}
		if sprintsStr.Valid && sprintsStr.String != "" {
			entry.Sprints = splitCSV(sprintsStr.String)
		}
		if filesStr.Valid && filesStr.String != "" {
			entry.Files = splitCSV(filesStr.String)
		}

		// Apply tag filter in-memory (FTS5 can't filter CSV columns)
		if len(opts.Tags) > 0 && !matchesTags(entry.Tags, opts.Tags) {
			continue
		}

		results = append(results, MemorySearchResult{
			Entry: entry,
			Score: float32(-bm25Score),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating memory results: %w", err)
	}

	if results == nil {
		results = []MemorySearchResult{}
	}

	return results, nil
}

// splitCSV splits a comma-separated string into trimmed parts.
func splitCSV(s string) []string {
	parts := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// matchesTags returns true if any of the filter tags match any entry tags.
func matchesTags(entryTags, filterTags []string) bool {
	for _, ft := range filterTags {
		for _, et := range entryTags {
			if et == ft {
				return true
			}
		}
	}
	return false
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

// ===== MEMORY FTS5 =====

// initMemoryFTS5Schema creates the FTS5 virtual table and sync triggers for the memory table.
func (s *SQLiteStorage) initMemoryFTS5Schema() error {
	// Create FTS5 virtual table for memory entries
	fts5Schema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
		question,
		answer,
		tokenize='porter unicode61'
	);
	`
	if _, err := s.db.Exec(fts5Schema); err != nil {
		return fmt.Errorf("failed to create memory FTS5 virtual table: %w", err)
	}

	// INSERT trigger
	insertTrigger := `
	CREATE TRIGGER IF NOT EXISTS memory_fts_insert
	AFTER INSERT ON memory
	BEGIN
		INSERT INTO memory_fts(rowid, question, answer)
		VALUES (NEW.rowid, COALESCE(NEW.question, ''), COALESCE(NEW.answer, ''));
	END;
	`
	if _, err := s.db.Exec(insertTrigger); err != nil {
		return fmt.Errorf("failed to create memory FTS5 INSERT trigger: %w", err)
	}

	// UPDATE trigger (delete old + insert new)
	updateTrigger := `
	CREATE TRIGGER IF NOT EXISTS memory_fts_update
	AFTER UPDATE ON memory
	BEGIN
		DELETE FROM memory_fts WHERE rowid = OLD.rowid;
		INSERT INTO memory_fts(rowid, question, answer)
		VALUES (NEW.rowid, COALESCE(NEW.question, ''), COALESCE(NEW.answer, ''));
	END;
	`
	if _, err := s.db.Exec(updateTrigger); err != nil {
		return fmt.Errorf("failed to create memory FTS5 UPDATE trigger: %w", err)
	}

	// DELETE trigger
	deleteTrigger := `
	CREATE TRIGGER IF NOT EXISTS memory_fts_delete
	AFTER DELETE ON memory
	BEGIN
		DELETE FROM memory_fts WHERE rowid = OLD.rowid;
	END;
	`
	if _, err := s.db.Exec(deleteTrigger); err != nil {
		return fmt.Errorf("failed to create memory FTS5 DELETE trigger: %w", err)
	}

	// Backfill existing memories if FTS is empty but memories exist
	if err := s.backfillMemoryFTS5(); err != nil {
		return fmt.Errorf("failed to backfill memory FTS5 index: %w", err)
	}

	return nil
}

// backfillMemoryFTS5 populates the memory FTS5 index from existing memory entries.
func (s *SQLiteStorage) backfillMemoryFTS5() error {
	var ftsCount, memoryCount int

	err := s.db.QueryRow(`SELECT COUNT(*) FROM memory_fts`).Scan(&ftsCount)
	if err != nil {
		return fmt.Errorf("failed to count memory FTS entries: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM memory`).Scan(&memoryCount)
	if err != nil {
		return fmt.Errorf("failed to count memory entries: %w", err)
	}

	if memoryCount > 0 && ftsCount == 0 {
		_, err := s.db.Exec(`
			INSERT INTO memory_fts(rowid, question, answer)
			SELECT rowid, COALESCE(question, ''), COALESCE(answer, '')
			FROM memory
		`)
		if err != nil {
			return fmt.Errorf("failed to backfill memory FTS5: %w", err)
		}
	}

	return nil
}

// RebuildMemoryFTSIndex rebuilds the memory FTS5 index from scratch.
func (s *SQLiteStorage) RebuildMemoryFTSIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	if _, err := s.db.Exec(`DELETE FROM memory_fts`); err != nil {
		return fmt.Errorf("failed to clear memory FTS index: %w", err)
	}

	if _, err := s.db.Exec(`
		INSERT INTO memory_fts(rowid, question, answer)
		SELECT rowid, COALESCE(question, ''), COALESCE(answer, '')
		FROM memory
	`); err != nil {
		return fmt.Errorf("failed to rebuild memory FTS index: %w", err)
	}

	// Optimize
	s.db.Exec(`INSERT INTO memory_fts(memory_fts) VALUES('optimize')`)

	return nil
}
