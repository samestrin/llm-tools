package semantic

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode"
)

// LexicalIndex provides a standalone FTS5 full-text search index.
// It can be used independently or as a parallel index for vector storage backends.
type LexicalIndex struct {
	db           *sql.DB
	embeddingDim int
	mu           sync.RWMutex
	closed       bool
}

// NewLexicalIndex creates a new lexical index at the specified path.
// Use ":memory:" for an in-memory index (useful for testing).
// Parent directories are created if they don't exist.
func NewLexicalIndex(dbPath string, embeddingDim int) (*LexicalIndex, error) {
	// Create parent directories if needed (skip for in-memory)
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if dbPath != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
		}
	}

	idx := &LexicalIndex{
		db:           db,
		embeddingDim: embeddingDim,
	}

	// Initialize schema
	if err := idx.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return idx, nil
}

// initSchema creates the FTS5 virtual table and supporting tables.
func (idx *LexicalIndex) initSchema() error {
	// Create chunks table to store chunk metadata (mirrors SQLiteStorage schema)
	// Note: file_mtime column is added via migration for backward compatibility
	chunksSchema := `
	CREATE TABLE IF NOT EXISTS chunks (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT UNIQUE NOT NULL,
		file_path TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT,
		signature TEXT,
		content TEXT,
		start_line INTEGER,
		end_line INTEGER,
		language TEXT,
		file_mtime INTEGER
	);
	`

	if _, err := idx.db.Exec(chunksSchema); err != nil {
		return fmt.Errorf("failed to create chunks table: %w", err)
	}

	// Migrate existing databases: add file_mtime column if missing
	// This must run BEFORE creating indexes that reference file_mtime
	if err := idx.migrateMtimeColumn(); err != nil {
		return err
	}

	// Create indexes (after migration ensures file_mtime column exists)
	indexSchema := `
	CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);
	CREATE INDEX IF NOT EXISTS idx_chunks_type ON chunks(type);
	CREATE INDEX IF NOT EXISTS idx_chunks_file_mtime ON chunks(file_mtime);
	`

	if _, err := idx.db.Exec(indexSchema); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	// Create FTS5 virtual table
	fts5Schema := `
	CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
		name,
		content,
		tokenize='porter unicode61'
	);
	`

	if _, err := idx.db.Exec(fts5Schema); err != nil {
		return fmt.Errorf("failed to create FTS5 virtual table: %w", err)
	}

	// Create INSERT trigger
	insertTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_insert
	AFTER INSERT ON chunks
	BEGIN
		INSERT INTO chunks_fts(rowid, name, content)
		VALUES (NEW.rowid, COALESCE(NEW.name, ''), COALESCE(NEW.content, ''));
	END;
	`

	if _, err := idx.db.Exec(insertTrigger); err != nil {
		return fmt.Errorf("failed to create INSERT trigger: %w", err)
	}

	// Create UPDATE trigger
	updateTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_update
	AFTER UPDATE ON chunks
	BEGIN
		DELETE FROM chunks_fts WHERE rowid = OLD.rowid;
		INSERT INTO chunks_fts(rowid, name, content)
		VALUES (NEW.rowid, COALESCE(NEW.name, ''), COALESCE(NEW.content, ''));
	END;
	`

	if _, err := idx.db.Exec(updateTrigger); err != nil {
		return fmt.Errorf("failed to create UPDATE trigger: %w", err)
	}

	// Create DELETE trigger
	deleteTrigger := `
	CREATE TRIGGER IF NOT EXISTS chunks_fts_delete
	AFTER DELETE ON chunks
	BEGIN
		DELETE FROM chunks_fts WHERE rowid = OLD.rowid;
	END;
	`

	if _, err := idx.db.Exec(deleteTrigger); err != nil {
		return fmt.Errorf("failed to create DELETE trigger: %w", err)
	}

	// Backfill existing chunks if FTS is empty but chunks exist
	if err := idx.backfillFTS(); err != nil {
		return err
	}

	// Create memory stats tracking tables
	return idx.initStatsSchema()
}

// backfillFTS populates the FTS5 index from existing chunks.
func (idx *LexicalIndex) backfillFTS() error {
	var ftsCount, chunksCount int

	if err := idx.db.QueryRow(`SELECT COUNT(*) FROM chunks_fts`).Scan(&ftsCount); err != nil {
		return fmt.Errorf("failed to count FTS entries: %w", err)
	}

	if err := idx.db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunksCount); err != nil {
		return fmt.Errorf("failed to count chunks: %w", err)
	}

	if chunksCount > 0 && ftsCount == 0 {
		_, err := idx.db.Exec(`
			INSERT INTO chunks_fts(rowid, name, content)
			SELECT rowid, COALESCE(name, ''), COALESCE(content, '')
			FROM chunks
		`)
		if err != nil {
			return fmt.Errorf("failed to backfill FTS: %w", err)
		}
	}

	return nil
}

// IndexChunk adds a single chunk to the index.
func (idx *LexicalIndex) IndexChunk(ctx context.Context, chunk Chunk) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	_, err := idx.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO chunks (id, file_path, type, name, signature, content, start_line, end_line, language, file_mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.FilePath, chunk.Type.String(), chunk.Name, chunk.Signature,
		chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Language, nullableInt64Lexical(chunk.FileMtime))

	if err != nil {
		return fmt.Errorf("failed to index chunk: %w", err)
	}

	return nil
}

// IndexBatch adds multiple chunks to the index in a single transaction.
func (idx *LexicalIndex) IndexBatch(ctx context.Context, chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO chunks (id, file_path, type, name, signature, content, start_line, end_line, language, file_mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		if _, err := stmt.ExecContext(ctx, chunk.ID, chunk.FilePath, chunk.Type.String(),
			chunk.Name, chunk.Signature, chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Language,
			nullableInt64Lexical(chunk.FileMtime)); err != nil {
			return fmt.Errorf("failed to index chunk %s: %w", chunk.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteChunk removes a chunk from the index by ID.
func (idx *LexicalIndex) DeleteChunk(ctx context.Context, id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	_, err := idx.db.ExecContext(ctx, `DELETE FROM chunks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete chunk: %w", err)
	}

	return nil
}

// DeleteByFilePath removes all chunks with the given file path.
// Returns the number of chunks deleted.
func (idx *LexicalIndex) DeleteByFilePath(ctx context.Context, filePath string) (int, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return 0, ErrStorageClosed
	}

	// Count first
	var count int
	if err := idx.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks WHERE file_path = ?`, filePath).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count chunks: %w", err)
	}

	if count == 0 {
		return 0, nil
	}

	// Delete
	_, err := idx.db.ExecContext(ctx, `DELETE FROM chunks WHERE file_path = ?`, filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to delete chunks: %w", err)
	}

	return count, nil
}

// Search performs a full-text search and returns matching chunks.
func (idx *LexicalIndex) Search(ctx context.Context, query string, opts LexicalSearchOptions) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, ErrStorageClosed
	}

	if query == "" {
		return []SearchResult{}, nil
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}

	// Build query with optional filters
	sqlQuery := `
		SELECT c.id, c.file_path, c.type, c.name, c.signature, c.content,
		       c.start_line, c.end_line, c.language, c.file_mtime, bm25(chunks_fts) as score
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

	rows, err := idx.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var chunk Chunk
		var typeStr string
		var bm25Score float64
		var fileMtime sql.NullInt64

		if err := rows.Scan(
			&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name, &chunk.Signature,
			&chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Language, &fileMtime, &bm25Score,
		); err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		chunkType, _ := ParseChunkType(typeStr)
		chunk.Type = chunkType
		if fileMtime.Valid {
			chunk.FileMtime = fileMtime.Int64
		}

		// Convert BM25 score (more negative = better) to positive scale
		results = append(results, SearchResult{
			Chunk: chunk,
			Score: float32(-bm25Score),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	if results == nil {
		results = []SearchResult{}
	}

	return results, nil
}

// Clear removes all chunks from the index.
func (idx *LexicalIndex) Clear(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return ErrStorageClosed
	}

	// Delete from chunks table - the DELETE trigger will sync to FTS automatically
	if _, err := idx.db.ExecContext(ctx, `DELETE FROM chunks`); err != nil {
		return fmt.Errorf("failed to clear chunks: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (idx *LexicalIndex) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return nil
	}

	idx.closed = true
	return idx.db.Close()
}

// ===== Helper functions for Qdrant parallel FTS =====

// sanitizeCollectionName removes unsafe characters from collection names.
// Only alphanumeric, hyphen, and underscore are preserved.
func sanitizeCollectionName(name string) string {
	if name == "" {
		return "default"
	}

	var result []rune
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			result = append(result, r)
		}
	}

	if len(result) == 0 {
		return "default"
	}

	return string(result)
}

// getFTSPath returns the path for a parallel FTS database.
// dataDir should be the project's .index directory (e.g., {gitRoot}/.index/).
// Returns empty string if dataDir is empty (caller should handle).
func getFTSPath(collection string, dataDir string) string {
	if dataDir == "" {
		// Return empty string to signal error - caller should handle
		return ""
	}

	return filepath.Join(dataDir, "qdrant_fts.db")
}

// nullableInt64Lexical returns nil if value is 0, otherwise returns the value.
// Used for optional INTEGER columns in SQLite. Separate from storage_sqlite.go
// to avoid package-level name collision.
func nullableInt64Lexical(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

// migrateMtimeColumn adds file_mtime column to existing LexicalIndex databases if missing.
func (idx *LexicalIndex) migrateMtimeColumn() error {
	// Check if column exists
	var count int
	err := idx.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name = 'file_mtime'
	`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Column doesn't exist, add it
		_, err = idx.db.Exec(`ALTER TABLE chunks ADD COLUMN file_mtime INTEGER`)
		if err != nil {
			return fmt.Errorf("failed to add file_mtime column: %w", err)
		}
		// Create index for the new column
		_, err = idx.db.Exec(`CREATE INDEX IF NOT EXISTS idx_chunks_file_mtime ON chunks(file_mtime)`)
		if err != nil {
			return fmt.Errorf("failed to create file_mtime index: %w", err)
		}
	}

	return nil
}

// ===== Memory Stats Tracking =====

// initStatsSchema creates the memory stats tracking tables.
func (idx *LexicalIndex) initStatsSchema() error {
	// Create memory_stats table for tracking retrieval counts
	statsSchema := `
	CREATE TABLE IF NOT EXISTS memory_stats (
		memory_id TEXT PRIMARY KEY,
		retrieval_count INTEGER DEFAULT 0,
		last_retrieved TEXT,
		status TEXT DEFAULT 'active'
	);
	`
	if _, err := idx.db.Exec(statsSchema); err != nil {
		return fmt.Errorf("failed to create memory_stats table: %w", err)
	}

	// Create retrieval_log table for detailed tracking
	logSchema := `
	CREATE TABLE IF NOT EXISTS retrieval_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		memory_id TEXT NOT NULL,
		query TEXT,
		score REAL,
		timestamp TEXT DEFAULT (datetime('now'))
	);
	`
	if _, err := idx.db.Exec(logSchema); err != nil {
		return fmt.Errorf("failed to create retrieval_log table: %w", err)
	}

	// Create index for pruning old log entries
	indexSchema := `
	CREATE INDEX IF NOT EXISTS idx_retrieval_log_timestamp ON retrieval_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_retrieval_log_memory_id ON retrieval_log(memory_id);
	`
	if _, err := idx.db.Exec(indexSchema); err != nil {
		return fmt.Errorf("failed to create retrieval_log indexes: %w", err)
	}

	return nil
}

// TrackRetrieval records a memory retrieval event.
// This should be called after search results are returned for profile=memory.
func (idx *LexicalIndex) TrackRetrieval(memoryID string, query string, score float32) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return fmt.Errorf("index is closed")
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update or insert stats
	_, err = tx.Exec(`
		INSERT INTO memory_stats (memory_id, retrieval_count, last_retrieved, status)
		VALUES (?, 1, datetime('now'), 'active')
		ON CONFLICT(memory_id) DO UPDATE SET
			retrieval_count = retrieval_count + 1,
			last_retrieved = datetime('now')
	`, memoryID)
	if err != nil {
		return fmt.Errorf("failed to update memory_stats: %w", err)
	}

	// Log the retrieval
	_, err = tx.Exec(`
		INSERT INTO retrieval_log (memory_id, query, score)
		VALUES (?, ?, ?)
	`, memoryID, query, score)
	if err != nil {
		return fmt.Errorf("failed to insert retrieval_log: %w", err)
	}

	return tx.Commit()
}

// TrackRetrievalBatch records multiple memory retrieval events in a single transaction.
// More efficient than calling TrackRetrieval multiple times.
func (idx *LexicalIndex) TrackRetrievalBatch(retrievals []struct {
	MemoryID string
	Score    float32
}, query string) error {
	if len(retrievals) == 0 {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return fmt.Errorf("index is closed")
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements for batch operations
	stmtStats, err := tx.Prepare(`
		INSERT INTO memory_stats (memory_id, retrieval_count, last_retrieved, status)
		VALUES (?, 1, datetime('now'), 'active')
		ON CONFLICT(memory_id) DO UPDATE SET
			retrieval_count = retrieval_count + 1,
			last_retrieved = datetime('now')
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare stats statement: %w", err)
	}
	defer stmtStats.Close()

	stmtLog, err := tx.Prepare(`
		INSERT INTO retrieval_log (memory_id, query, score)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare log statement: %w", err)
	}
	defer stmtLog.Close()

	for _, r := range retrievals {
		if _, err := stmtStats.Exec(r.MemoryID); err != nil {
			return fmt.Errorf("failed to update stats for %s: %w", r.MemoryID, err)
		}
		if _, err := stmtLog.Exec(r.MemoryID, query, r.Score); err != nil {
			return fmt.Errorf("failed to log retrieval for %s: %w", r.MemoryID, err)
		}
	}

	return tx.Commit()
}

// GetMemoryStats returns stats for a specific memory entry.
func (idx *LexicalIndex) GetMemoryStats(memoryID string) (*RetrievalStats, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, fmt.Errorf("index is closed")
	}

	var stats RetrievalStats
	var lastRetrieved sql.NullString

	err := idx.db.QueryRow(`
		SELECT memory_id, retrieval_count, last_retrieved, status
		FROM memory_stats
		WHERE memory_id = ?
	`, memoryID).Scan(&stats.MemoryID, &stats.RetrievalCount, &lastRetrieved, &stats.Status)

	if err == sql.ErrNoRows {
		return nil, nil // No stats yet
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	if lastRetrieved.Valid {
		stats.LastRetrieved = lastRetrieved.String
	}

	// Calculate average score from log
	var avgScore sql.NullFloat64
	err = idx.db.QueryRow(`
		SELECT AVG(score) FROM retrieval_log WHERE memory_id = ?
	`, memoryID).Scan(&avgScore)
	if err == nil && avgScore.Valid {
		stats.AvgScore = float32(avgScore.Float64)
	}

	return &stats, nil
}

// GetAllMemoryStats returns stats for all tracked memories.
func (idx *LexicalIndex) GetAllMemoryStats() ([]RetrievalStats, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, fmt.Errorf("index is closed")
	}

	rows, err := idx.db.Query(`
		SELECT
			ms.memory_id,
			ms.retrieval_count,
			ms.last_retrieved,
			ms.status,
			COALESCE(AVG(rl.score), 0) as avg_score
		FROM memory_stats ms
		LEFT JOIN retrieval_log rl ON ms.memory_id = rl.memory_id
		GROUP BY ms.memory_id
		ORDER BY ms.retrieval_count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory stats: %w", err)
	}
	defer rows.Close()

	var results []RetrievalStats
	for rows.Next() {
		var stats RetrievalStats
		var lastRetrieved sql.NullString
		var avgScore sql.NullFloat64

		if err := rows.Scan(&stats.MemoryID, &stats.RetrievalCount, &lastRetrieved, &stats.Status, &avgScore); err != nil {
			return nil, fmt.Errorf("failed to scan memory stats: %w", err)
		}

		if lastRetrieved.Valid {
			stats.LastRetrieved = lastRetrieved.String
		}
		if avgScore.Valid {
			stats.AvgScore = float32(avgScore.Float64)
		}

		results = append(results, stats)
	}

	return results, rows.Err()
}

// GetRetrievalHistory returns recent retrieval log entries for a memory.
func (idx *LexicalIndex) GetRetrievalHistory(memoryID string, limit int) ([]RetrievalLogEntry, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, fmt.Errorf("index is closed")
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := idx.db.Query(`
		SELECT id, memory_id, query, score, timestamp
		FROM retrieval_log
		WHERE memory_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`, memoryID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query retrieval history: %w", err)
	}
	defer rows.Close()

	var results []RetrievalLogEntry
	for rows.Next() {
		var entry RetrievalLogEntry
		var query sql.NullString
		var score sql.NullFloat64

		if err := rows.Scan(&entry.ID, &entry.MemoryID, &query, &score, &entry.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan retrieval log entry: %w", err)
		}

		if query.Valid {
			entry.Query = query.String
		}
		if score.Valid {
			entry.Score = float32(score.Float64)
		}

		results = append(results, entry)
	}

	return results, rows.Err()
}

// PruneRetrievalLog removes retrieval log entries older than the specified duration.
func (idx *LexicalIndex) PruneRetrievalLog(olderThanDays int) (int64, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return 0, fmt.Errorf("index is closed")
	}

	result, err := idx.db.Exec(`
		DELETE FROM retrieval_log
		WHERE timestamp < datetime('now', '-' || ? || ' days')
	`, olderThanDays)
	if err != nil {
		return 0, fmt.Errorf("failed to prune retrieval log: %w", err)
	}

	return result.RowsAffected()
}

// UpdateMemoryStatus updates the status of a memory entry (active, archived, etc.).
func (idx *LexicalIndex) UpdateMemoryStatus(memoryID string, status string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return fmt.Errorf("index is closed")
	}

	_, err := idx.db.Exec(`
		INSERT INTO memory_stats (memory_id, retrieval_count, status)
		VALUES (?, 0, ?)
		ON CONFLICT(memory_id) DO UPDATE SET status = ?
	`, memoryID, status, status)
	if err != nil {
		return fmt.Errorf("failed to update memory status: %w", err)
	}

	return nil
}
