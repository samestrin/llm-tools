package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStorage implements the Storage interface using SQLite
type SQLiteStorage struct {
	db           *sql.DB
	embeddingDim int
	mu           sync.RWMutex
	closed       bool
}

// NewSQLiteStorage creates a new SQLite-based storage
func NewSQLiteStorage(dbPath string, embeddingDim int) (*SQLiteStorage, error) {
	if dbPath == "" {
		dbPath = ":memory:"
	}

	// Ensure parent directory exists for file-based databases
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	storage := &SQLiteStorage{
		db:           db,
		embeddingDim: embeddingDim,
	}

	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return storage, nil
}

func (s *SQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS chunks (
		id TEXT PRIMARY KEY,
		file_path TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		signature TEXT,
		content TEXT,
		start_line INTEGER,
		end_line INTEGER,
		language TEXT,
		embedding BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);
	CREATE INDEX IF NOT EXISTS idx_chunks_type ON chunks(type);
	CREATE INDEX IF NOT EXISTS idx_chunks_language ON chunks(language);

	CREATE TABLE IF NOT EXISTS file_hashes (
		file_path TEXT PRIMARY KEY,
		content_hash TEXT NOT NULL,
		indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS memory (
		id TEXT PRIMARY KEY,
		question TEXT NOT NULL,
		answer TEXT NOT NULL,
		tags TEXT,
		source TEXT,
		status TEXT DEFAULT 'pending',
		occurrences INTEGER DEFAULT 1,
		embedding BLOB,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_memory_tags ON memory(tags);
	CREATE INDEX IF NOT EXISTS idx_memory_status ON memory(status);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Initialize FTS5 for lexical search
	if err := s.initFTS5Schema(); err != nil {
		return err
	}

	return nil
}

func (s *SQLiteStorage) Create(ctx context.Context, chunk Chunk, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	embeddingBytes, err := encodeEmbedding(embedding)
	if err != nil {
		return fmt.Errorf("failed to encode embedding: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chunks (id, file_path, type, name, signature, content, start_line, end_line, language, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.FilePath, chunk.Type.String(), chunk.Name, chunk.Signature,
		chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Language, embeddingBytes)

	if err != nil {
		return fmt.Errorf("failed to insert chunk: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) CreateBatch(ctx context.Context, chunks []ChunkWithEmbedding) error {
	if len(chunks) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (id, file_path, type, name, signature, content, start_line, end_line, language, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, cwe := range chunks {
		embeddingBytes, err := encodeEmbedding(cwe.Embedding)
		if err != nil {
			return fmt.Errorf("failed to encode embedding: %w", err)
		}

		_, err = stmt.ExecContext(ctx,
			cwe.Chunk.ID, cwe.Chunk.FilePath, cwe.Chunk.Type.String(), cwe.Chunk.Name,
			cwe.Chunk.Signature, cwe.Chunk.Content, cwe.Chunk.StartLine, cwe.Chunk.EndLine,
			cwe.Chunk.Language, embeddingBytes)
		if err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) Read(ctx context.Context, id string) (*Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	var chunk Chunk
	var typeStr string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, file_path, type, name, signature, content, start_line, end_line, language
		FROM chunks WHERE id = ?
	`, id).Scan(&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name, &chunk.Signature,
		&chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Language)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk: %w", err)
	}

	chunkType, _ := ParseChunkType(typeStr)
	chunk.Type = chunkType

	return &chunk, nil
}

func (s *SQLiteStorage) Update(ctx context.Context, chunk Chunk, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	embeddingBytes, err := encodeEmbedding(embedding)
	if err != nil {
		return fmt.Errorf("failed to encode embedding: %w", err)
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE chunks SET
			file_path = ?, type = ?, name = ?, signature = ?, content = ?,
			start_line = ?, end_line = ?, language = ?, embedding = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, chunk.FilePath, chunk.Type.String(), chunk.Name, chunk.Signature,
		chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Language, embeddingBytes, chunk.ID)

	if err != nil {
		return fmt.Errorf("failed to update chunk: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *SQLiteStorage) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete chunk: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *SQLiteStorage) DeleteByFilePath(ctx context.Context, filePath string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStorageClosed
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE file_path = ?`, filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to delete chunks by file path: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

func (s *SQLiteStorage) List(ctx context.Context, opts ListOptions) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	query := `SELECT id, file_path, type, name, signature, content, start_line, end_line, language FROM chunks WHERE 1=1`
	args := []interface{}{}

	if opts.FilePath != "" {
		query += ` AND file_path = ?`
		args = append(args, opts.FilePath)
	}
	if opts.Type != "" {
		query += ` AND type = ?`
		args = append(args, opts.Type)
	}
	if opts.Language != "" {
		query += ` AND language = ?`
		args = append(args, opts.Language)
	}

	query += ` ORDER BY file_path, start_line`

	if opts.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(` OFFSET %d`, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list chunks: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var chunk Chunk
		var typeStr string

		if err := rows.Scan(&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name,
			&chunk.Signature, &chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Language); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		chunkType, _ := ParseChunkType(typeStr)
		chunk.Type = chunkType
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

func (s *SQLiteStorage) Search(ctx context.Context, queryEmbedding []float32, opts SearchOptions) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// For now, we do brute-force cosine similarity search
	// In production, consider using sqlite-vss or FAISS
	query := `SELECT id, file_path, type, name, signature, content, start_line, end_line, language, embedding FROM chunks WHERE 1=1`
	args := []interface{}{}

	if opts.Type != "" {
		query += ` AND type = ?`
		args = append(args, opts.Type)
	}
	if opts.PathFilter != "" {
		// Simple prefix matching for path filter
		query += ` AND file_path LIKE ?`
		args = append(args, opts.PathFilter+"%")
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var chunk Chunk
		var typeStr string
		var embeddingBytes []byte

		if err := rows.Scan(&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name,
			&chunk.Signature, &chunk.Content, &chunk.StartLine, &chunk.EndLine,
			&chunk.Language, &embeddingBytes); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		chunkType, _ := ParseChunkType(typeStr)
		chunk.Type = chunkType

		embedding, err := decodeEmbedding(embeddingBytes)
		if err != nil {
			continue // Skip chunks with invalid embeddings
		}

		score := cosineSimilarity(queryEmbedding, embedding)

		if opts.Threshold > 0 && score < opts.Threshold {
			continue
		}

		results = append(results, SearchResult{
			Chunk: chunk,
			Score: score,
		})
	}

	// Sort by score descending
	sortResultsByScore(results)

	// Apply TopK limit
	if opts.TopK > 0 && len(results) > opts.TopK {
		results = results[:opts.TopK]
	}

	return results, nil
}

func (s *SQLiteStorage) Stats(ctx context.Context) (*IndexStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	var chunksTotal int
	var filesIndexed int
	var lastUpdated string

	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chunks`).Scan(&chunksTotal)
	if err != nil {
		return nil, fmt.Errorf("failed to count chunks: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT file_path) FROM chunks`).Scan(&filesIndexed)
	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(updated_at), '') FROM chunks`).Scan(&lastUpdated)
	if err != nil {
		lastUpdated = time.Now().Format(time.RFC3339)
	}

	return &IndexStats{
		ChunksTotal:  chunksTotal,
		FilesIndexed: filesIndexed,
		LastUpdated:  lastUpdated,
	}, nil
}

// Clear removes all chunks from storage (for force re-index)
func (s *SQLiteStorage) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Clear chunks (triggers will clear FTS automatically)
	if _, err := s.db.ExecContext(ctx, "DELETE FROM chunks"); err != nil {
		return err
	}

	// Also explicitly clear FTS to ensure consistency
	if err := s.clearFTS5(); err != nil {
		return err
	}

	// Clear file hashes
	_, err := s.db.ExecContext(ctx, "DELETE FROM file_hashes")
	return err
}

// GetFileHash retrieves the stored content hash for a file path
func (s *SQLiteStorage) GetFileHash(ctx context.Context, filePath string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return "", ErrStorageClosed
	}

	var hash string
	err := s.db.QueryRowContext(ctx,
		`SELECT content_hash FROM file_hashes WHERE file_path = ?`,
		filePath).Scan(&hash)

	if err == sql.ErrNoRows {
		return "", nil // Not indexed yet
	}
	if err != nil {
		return "", fmt.Errorf("failed to get file hash: %w", err)
	}

	return hash, nil
}

// SetFileHash stores the content hash for a file path
func (s *SQLiteStorage) SetFileHash(ctx context.Context, filePath string, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO file_hashes (file_path, content_hash, indexed_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(file_path) DO UPDATE SET
			content_hash = excluded.content_hash,
			indexed_at = CURRENT_TIMESTAMP
	`, filePath, hash)

	if err != nil {
		return fmt.Errorf("failed to set file hash: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}

// ===== Memory Entry Methods =====

// StoreMemory stores a memory entry with its embedding
func (s *SQLiteStorage) StoreMemory(ctx context.Context, entry MemoryEntry, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	embeddingBytes, err := encodeEmbedding(embedding)
	if err != nil {
		return fmt.Errorf("failed to encode embedding: %w", err)
	}

	// Store tags as comma-separated string
	tagsStr := strings.Join(entry.Tags, ",")

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memory (id, question, answer, tags, source, status, occurrences, embedding, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			question = excluded.question,
			answer = excluded.answer,
			tags = excluded.tags,
			source = excluded.source,
			status = excluded.status,
			occurrences = excluded.occurrences,
			embedding = excluded.embedding,
			updated_at = excluded.updated_at
	`, entry.ID, entry.Question, entry.Answer, tagsStr, entry.Source, entry.Status, entry.Occurrences, embeddingBytes, entry.CreatedAt, entry.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to store memory: %w", err)
	}

	return nil
}

// StoreMemoryBatch stores multiple memory entries with their embeddings
func (s *SQLiteStorage) StoreMemoryBatch(ctx context.Context, entries []MemoryWithEmbedding) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO memory (id, question, answer, tags, source, status, occurrences, embedding, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			question = excluded.question,
			answer = excluded.answer,
			tags = excluded.tags,
			source = excluded.source,
			status = excluded.status,
			occurrences = excluded.occurrences,
			embedding = excluded.embedding,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, mwe := range entries {
		embeddingBytes, err := encodeEmbedding(mwe.Embedding)
		if err != nil {
			return fmt.Errorf("failed to encode embedding: %w", err)
		}

		tagsStr := strings.Join(mwe.Entry.Tags, ",")

		_, err = stmt.ExecContext(ctx,
			mwe.Entry.ID, mwe.Entry.Question, mwe.Entry.Answer, tagsStr,
			mwe.Entry.Source, mwe.Entry.Status, mwe.Entry.Occurrences,
			embeddingBytes, mwe.Entry.CreatedAt, mwe.Entry.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to store memory %s: %w", mwe.Entry.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SearchMemory finds memory entries similar to the query embedding
func (s *SQLiteStorage) SearchMemory(ctx context.Context, queryEmbedding []float32, opts MemorySearchOptions) ([]MemorySearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// Load all memory entries with embeddings
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, question, answer, tags, source, status, occurrences, embedding, created_at, updated_at
		FROM memory
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query memory: %w", err)
	}
	defer rows.Close()

	var results []MemorySearchResult

	for rows.Next() {
		var entry MemoryEntry
		var tagsStr sql.NullString
		var source sql.NullString
		var embeddingBytes []byte

		err := rows.Scan(&entry.ID, &entry.Question, &entry.Answer, &tagsStr, &source, &entry.Status, &entry.Occurrences, &embeddingBytes, &entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse tags
		if tagsStr.Valid && tagsStr.String != "" {
			entry.Tags = strings.Split(tagsStr.String, ",")
		}
		if source.Valid {
			entry.Source = source.String
		}

		// Decode embedding
		embedding, err := decodeEmbedding(embeddingBytes)
		if err != nil {
			continue // Skip entries with invalid embeddings
		}

		// Calculate cosine similarity
		score := cosineSimilarity(queryEmbedding, embedding)

		// Apply threshold filter
		if opts.Threshold > 0 && score < opts.Threshold {
			continue
		}

		// Apply tag filter
		if len(opts.Tags) > 0 {
			hasMatch := false
			for _, filterTag := range opts.Tags {
				for _, entryTag := range entry.Tags {
					if strings.EqualFold(filterTag, entryTag) {
						hasMatch = true
						break
					}
				}
				if hasMatch {
					break
				}
			}
			if !hasMatch {
				continue
			}
		}

		// Apply source filter
		if opts.Source != "" && entry.Source != opts.Source {
			continue
		}

		// Apply status filter
		if opts.Status != "" && entry.Status != opts.Status {
			continue
		}

		results = append(results, MemorySearchResult{
			Entry:     entry,
			Score:     score,
			Embedding: embedding,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Sort by score descending
	sortMemoryResultsByScore(results)

	// Apply TopK limit
	topK := opts.TopK
	if topK <= 0 {
		topK = 10 // Default
	}
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// GetMemory retrieves a memory entry by ID
func (s *SQLiteStorage) GetMemory(ctx context.Context, id string) (*MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	var entry MemoryEntry
	var tagsStr sql.NullString
	var source sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, question, answer, tags, source, status, occurrences, created_at, updated_at
		FROM memory
		WHERE id = ?
	`, id).Scan(&entry.ID, &entry.Question, &entry.Answer, &tagsStr, &source, &entry.Status, &entry.Occurrences, &entry.CreatedAt, &entry.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	if tagsStr.Valid && tagsStr.String != "" {
		entry.Tags = strings.Split(tagsStr.String, ",")
	}
	if source.Valid {
		entry.Source = source.String
	}

	return &entry, nil
}

// DeleteMemory removes a memory entry by ID
func (s *SQLiteStorage) DeleteMemory(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM memory WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrMemoryNotFound
	}

	return nil
}

// ListMemory retrieves memory entries based on filter options
func (s *SQLiteStorage) ListMemory(ctx context.Context, opts MemoryListOptions) ([]MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	query := `SELECT id, question, answer, tags, source, status, occurrences, created_at, updated_at FROM memory WHERE 1=1`
	var args []interface{}

	if opts.Status != "" {
		query += ` AND status = ?`
		args = append(args, opts.Status)
	}
	if opts.Source != "" {
		query += ` AND source = ?`
		args = append(args, opts.Source)
	}

	query += ` ORDER BY created_at DESC`

	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list memory: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry

	for rows.Next() {
		var entry MemoryEntry
		var tagsStr sql.NullString
		var source sql.NullString

		err := rows.Scan(&entry.ID, &entry.Question, &entry.Answer, &tagsStr, &source, &entry.Status, &entry.Occurrences, &entry.CreatedAt, &entry.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if tagsStr.Valid && tagsStr.String != "" {
			entry.Tags = strings.Split(tagsStr.String, ",")
		}
		if source.Valid {
			entry.Source = source.String
		}

		// Apply tag filter (after loading since SQLite LIKE on comma-separated is complex)
		if len(opts.Tags) > 0 {
			hasMatch := false
			for _, filterTag := range opts.Tags {
				for _, entryTag := range entry.Tags {
					if strings.EqualFold(filterTag, entryTag) {
						hasMatch = true
						break
					}
				}
				if hasMatch {
					break
				}
			}
			if !hasMatch {
				continue
			}
		}

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return entries, nil
}

// sortMemoryResultsByScore sorts memory results by score in descending order
func sortMemoryResultsByScore(results []MemorySearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// encodeEmbedding converts a float32 slice to binary for storage (~3x smaller than JSON)
func encodeEmbedding(embedding []float32) ([]byte, error) {
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := math.Float32bits(v)
		buf[i*4] = byte(bits)
		buf[i*4+1] = byte(bits >> 8)
		buf[i*4+2] = byte(bits >> 16)
		buf[i*4+3] = byte(bits >> 24)
	}
	return buf, nil
}

// decodeEmbedding converts bytes back to a float32 slice
func decodeEmbedding(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		// Fallback: try JSON decode for legacy data
		var embedding []float32
		if err := json.Unmarshal(data, &embedding); err != nil {
			return nil, fmt.Errorf("invalid embedding data")
		}
		return embedding, nil
	}
	embedding := make([]float32, len(data)/4)
	for i := range embedding {
		bits := uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		embedding[i] = math.Float32frombits(bits)
	}
	return embedding, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// sortResultsByScore sorts results by score in descending order
func sortResultsByScore(results []SearchResult) {
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// IndexPath returns the default index path for a repository
func IndexPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".llm-index", "semantic.db")
}
