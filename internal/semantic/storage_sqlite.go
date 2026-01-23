package semantic

import (
	"container/heap"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
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
		domain TEXT DEFAULT 'code',
		embedding BLOB,
		file_mtime INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);
	CREATE INDEX IF NOT EXISTS idx_chunks_type ON chunks(type);
	CREATE INDEX IF NOT EXISTS idx_chunks_language ON chunks(language);
	CREATE INDEX IF NOT EXISTS idx_chunks_file_mtime ON chunks(file_mtime);

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

	CREATE TABLE IF NOT EXISTS index_metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Migrate existing databases: add file_mtime column if missing
	if err := s.migrateMtimeColumn(); err != nil {
		return err
	}

	// Migrate existing databases: add domain column if missing
	if err := s.migrateDomainColumn(); err != nil {
		return err
	}

	// Initialize FTS5 for lexical search
	if err := s.initFTS5Schema(); err != nil {
		return err
	}

	// Initialize memory stats tracking tables
	if err := s.initMemoryStatsSchema(); err != nil {
		return err
	}

	return nil
}

// migrateMtimeColumn adds file_mtime column to existing databases if missing.
func (s *SQLiteStorage) migrateMtimeColumn() error {
	// Check if column exists
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name = 'file_mtime'
	`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Column doesn't exist, add it
		_, err = s.db.Exec(`ALTER TABLE chunks ADD COLUMN file_mtime INTEGER`)
		if err != nil {
			return fmt.Errorf("failed to add file_mtime column: %w", err)
		}
		// Create index for the new column
		_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_chunks_file_mtime ON chunks(file_mtime)`)
		if err != nil {
			return fmt.Errorf("failed to create file_mtime index: %w", err)
		}
	}

	return nil
}

// migrateDomainColumn adds domain column to existing databases if missing.
func (s *SQLiteStorage) migrateDomainColumn() error {
	// Check if column exists
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('chunks')
		WHERE name = 'domain'
	`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Column doesn't exist, add it with default 'code' for existing rows
		_, err = s.db.Exec(`ALTER TABLE chunks ADD COLUMN domain TEXT DEFAULT 'code'`)
		if err != nil {
			return fmt.Errorf("failed to add domain column: %w", err)
		}
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

	// Default domain to "code" if not set
	domain := chunk.Domain
	if domain == "" {
		domain = "code"
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO chunks (id, file_path, type, name, signature, content, start_line, end_line, language, domain, embedding, file_mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.FilePath, chunk.Type.String(), chunk.Name, chunk.Signature,
		chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Language, domain, embeddingBytes,
		nullableInt64(chunk.FileMtime))

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
		INSERT INTO chunks (id, file_path, type, name, signature, content, start_line, end_line, language, domain, embedding, file_mtime)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

		// Default domain to "code" if not set
		domain := cwe.Chunk.Domain
		if domain == "" {
			domain = "code"
		}

		_, err = stmt.ExecContext(ctx,
			cwe.Chunk.ID, cwe.Chunk.FilePath, cwe.Chunk.Type.String(), cwe.Chunk.Name,
			cwe.Chunk.Signature, cwe.Chunk.Content, cwe.Chunk.StartLine, cwe.Chunk.EndLine,
			cwe.Chunk.Language, domain, embeddingBytes, nullableInt64(cwe.Chunk.FileMtime))
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
		SELECT id, file_path, type, name, signature, content, start_line, end_line, language, COALESCE(domain, 'code')
		FROM chunks WHERE id = ?
	`, id).Scan(&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name, &chunk.Signature,
		&chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Language, &chunk.Domain)

	if errors.Is(err, sql.ErrNoRows) {
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

	domain := chunk.Domain
	if domain == "" {
		domain = "code"
	}

	result, err := s.db.ExecContext(ctx, `
		UPDATE chunks SET
			file_path = ?, type = ?, name = ?, signature = ?, content = ?,
			start_line = ?, end_line = ?, language = ?, embedding = ?,
			file_mtime = ?, domain = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, chunk.FilePath, chunk.Type.String(), chunk.Name, chunk.Signature,
		chunk.Content, chunk.StartLine, chunk.EndLine, chunk.Language, embeddingBytes,
		chunk.FileMtime, domain, chunk.ID)

	if err != nil {
		return fmt.Errorf("failed to update chunk: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
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

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
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

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return int(rows), nil
}

func (s *SQLiteStorage) DeleteByDomain(ctx context.Context, domain string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStorageClosed
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE COALESCE(domain, 'code') = ?`, domain)
	if err != nil {
		return 0, fmt.Errorf("failed to delete chunks by domain: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return int(rows), nil
}

func (s *SQLiteStorage) List(ctx context.Context, opts ListOptions) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	query := `SELECT id, file_path, type, name, signature, content, start_line, end_line, language, COALESCE(domain, 'code') FROM chunks WHERE 1=1`
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
			&chunk.Signature, &chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Language, &chunk.Domain); err != nil {
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
	query := `SELECT id, file_path, type, name, signature, content, start_line, end_line, language, COALESCE(domain, 'code'), embedding, file_mtime FROM chunks WHERE 1=1`
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
		var fileMtime sql.NullInt64

		if err := rows.Scan(&chunk.ID, &chunk.FilePath, &typeStr, &chunk.Name,
			&chunk.Signature, &chunk.Content, &chunk.StartLine, &chunk.EndLine,
			&chunk.Language, &chunk.Domain, &embeddingBytes, &fileMtime); err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		chunkType, _ := ParseChunkType(typeStr)
		chunk.Type = chunkType
		if fileMtime.Valid {
			chunk.FileMtime = fileMtime.Int64
		}

		embedding, err := decodeEmbedding(embeddingBytes)
		if err != nil {
			slog.Debug("skipping chunk with invalid embedding", "chunk_id", chunk.ID, "file", chunk.FilePath, "error", err)
			continue
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

	if errors.Is(err, sql.ErrNoRows) {
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
// Uses batched processing to limit memory usage to O(batch_size + topK) instead of O(n)
func (s *SQLiteStorage) SearchMemory(ctx context.Context, queryEmbedding []float32, opts MemorySearchOptions) ([]MemorySearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// Determine TopK limit for heap sizing
	topK := opts.TopK
	if topK <= 0 {
		topK = 10 // Default
	}

	// Initialize min-heap to track topK results across all batches
	h := &resultHeap{}
	heap.Init(h)

	// Batch size: process 1000 entries at a time to bound memory usage
	const batchSize = 1000
	offset := 0

	for {
		// Fetch a batch of memory entries
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, question, answer, tags, source, status, occurrences, embedding, created_at, updated_at
			FROM memory
			ORDER BY id
			LIMIT ? OFFSET ?
		`, batchSize, offset)
		if err != nil {
			return nil, fmt.Errorf("failed to query memory batch: %w", err)
		}

		hasRows := false
		for rows.Next() {
			hasRows = true
			var entry MemoryEntry
			var tagsStr sql.NullString
			var source sql.NullString
			var embeddingBytes []byte

			err := rows.Scan(&entry.ID, &entry.Question, &entry.Answer, &tagsStr, &source, &entry.Status, &entry.Occurrences, &embeddingBytes, &entry.CreatedAt, &entry.UpdatedAt)
			if err != nil {
				rows.Close()
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
				slog.Debug("skipping memory entry with invalid embedding", "entry_id", entry.ID, "error", err)
				continue
			}

			// Calculate cosine similarity
			score := cosineSimilarity(queryEmbedding, embedding)

			// Apply threshold filter early (skip low scores entirely)
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

			result := MemorySearchResult{
				Entry:     entry,
				Score:     score,
				Embedding: embedding,
			}

			// Maintain topK: if heap is not full, add; otherwise, replace min if result is better
			if h.Len() < topK {
				heap.Push(h, result)
			} else if score > h.results[0].Score {
				// Result is better than the current minimum in topK
				heap.Pop(h)
				heap.Push(h, result)
			}
		}

		rows.Close()

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating rows: %w", err)
		}

		// If no rows in this batch, we've processed all entries
		if !hasRows {
			break
		}

		// Move to next batch
		offset += batchSize
	}

	// Extract results from heap and sort by score descending
	results := make([]MemorySearchResult, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(MemorySearchResult)
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

	if errors.Is(err, sql.ErrNoRows) {
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

// resultHeap is a min-heap for tracking topK memory search results
type resultHeap struct {
	results []MemorySearchResult
}

// Len implements heap.Interface
func (h resultHeap) Len() int { return len(h.results) }

// Less implements heap.Interface (min-heap by score)
func (h resultHeap) Less(i, j int) bool { return h.results[i].Score < h.results[j].Score }

// Swap implements heap.Interface
func (h resultHeap) Swap(i, j int) { h.results[i], h.results[j] = h.results[j], h.results[i] }

// Push implements heap.Interface
func (h *resultHeap) Push(x interface{}) { h.results = append(h.results, x.(MemorySearchResult)) }

// Pop implements heap.Interface
func (h *resultHeap) Pop() interface{} {
	n := len(h.results)
	x := h.results[n-1]
	h.results = h.results[:n-1]
	return x
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
		slog.Debug("decodeEmbedding using JSON fallback for legacy data", "data_len", len(data))
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
		slog.Debug("cosineSimilarity dimension mismatch", "len_a", len(a), "len_b", len(b))
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

// sortResultsByScore sorts results by score in descending order using O(n log n) sort
func sortResultsByScore(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}

// IndexPath returns the default index path for a repository
func IndexPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".index", "semantic.db")
}

// nullableInt64 returns nil if value is 0, otherwise returns the value.
// Used for optional INTEGER columns in SQLite.
func nullableInt64(v int64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}

// GetCalibrationMetadata retrieves stored calibration data.
// Returns (nil, nil) if no calibration has been performed yet.
func (s *SQLiteStorage) GetCalibrationMetadata(ctx context.Context) (*CalibrationMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM index_metadata WHERE key = 'calibration'`,
	).Scan(&value)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No calibration yet
		}
		return nil, fmt.Errorf("failed to get calibration metadata: %w", err)
	}

	var meta CalibrationMetadata
	if err := json.Unmarshal([]byte(value), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse calibration metadata: %w", err)
	}

	return &meta, nil
}

// SetCalibrationMetadata stores calibration data.
// Overwrites any existing calibration data.
func (s *SQLiteStorage) SetCalibrationMetadata(ctx context.Context, meta *CalibrationMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	value, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal calibration metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO index_metadata (key, value, updated_at)
		VALUES ('calibration', ?, CURRENT_TIMESTAMP)
	`, string(value))

	if err != nil {
		return fmt.Errorf("failed to set calibration metadata: %w", err)
	}

	return nil
}

// ===== Memory Stats Tracking =====

// initMemoryStatsSchema creates the memory stats tracking tables.
func (s *SQLiteStorage) initMemoryStatsSchema() error {
	// Create memory_stats table for tracking retrieval counts
	statsSchema := `
	CREATE TABLE IF NOT EXISTS memory_stats (
		memory_id TEXT PRIMARY KEY,
		retrieval_count INTEGER DEFAULT 0,
		last_retrieved TEXT,
		status TEXT DEFAULT 'active'
	);
	`
	if _, err := s.db.Exec(statsSchema); err != nil {
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
	if _, err := s.db.Exec(logSchema); err != nil {
		return fmt.Errorf("failed to create retrieval_log table: %w", err)
	}

	// Create indexes for efficient queries
	indexSchema := `
	CREATE INDEX IF NOT EXISTS idx_retrieval_log_timestamp ON retrieval_log(timestamp);
	CREATE INDEX IF NOT EXISTS idx_retrieval_log_memory_id ON retrieval_log(memory_id);
	`
	if _, err := s.db.Exec(indexSchema); err != nil {
		return fmt.Errorf("failed to create retrieval_log indexes: %w", err)
	}

	return nil
}

// TrackMemoryRetrieval records a memory retrieval event.
// This should be called after search results are returned for memory search.
func (s *SQLiteStorage) TrackMemoryRetrieval(ctx context.Context, memoryID string, query string, score float32) error {
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

	// Update or insert stats
	_, err = tx.ExecContext(ctx, `
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
	_, err = tx.ExecContext(ctx, `
		INSERT INTO retrieval_log (memory_id, query, score)
		VALUES (?, ?, ?)
	`, memoryID, query, score)
	if err != nil {
		return fmt.Errorf("failed to insert retrieval_log: %w", err)
	}

	return tx.Commit()
}

// TrackMemoryRetrievalBatch records multiple memory retrieval events in a single transaction.
func (s *SQLiteStorage) TrackMemoryRetrievalBatch(ctx context.Context, retrievals []MemoryRetrieval, query string) error {
	if len(retrievals) == 0 {
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

	// Prepare statements for batch operations
	stmtStats, err := tx.PrepareContext(ctx, `
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

	stmtLog, err := tx.PrepareContext(ctx, `
		INSERT INTO retrieval_log (memory_id, query, score)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare log statement: %w", err)
	}
	defer stmtLog.Close()

	for _, r := range retrievals {
		if _, err := stmtStats.ExecContext(ctx, r.MemoryID); err != nil {
			return fmt.Errorf("failed to update stats for %s: %w", r.MemoryID, err)
		}
		if _, err := stmtLog.ExecContext(ctx, r.MemoryID, query, r.Score); err != nil {
			return fmt.Errorf("failed to log retrieval for %s: %w", r.MemoryID, err)
		}
	}

	return tx.Commit()
}

// GetMemoryStats returns stats for a specific memory entry.
func (s *SQLiteStorage) GetMemoryStats(ctx context.Context, memoryID string) (*RetrievalStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	var stats RetrievalStats
	var lastRetrieved sql.NullString

	err := s.db.QueryRowContext(ctx, `
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
	err = s.db.QueryRowContext(ctx, `
		SELECT AVG(score) FROM retrieval_log WHERE memory_id = ?
	`, memoryID).Scan(&avgScore)
	if err == nil && avgScore.Valid {
		stats.AvgScore = float32(avgScore.Float64)
	}

	return &stats, nil
}

// GetAllMemoryStats returns stats for all tracked memories.
func (s *SQLiteStorage) GetAllMemoryStats(ctx context.Context) ([]RetrievalStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ms.memory_id,
			ms.retrieval_count,
			ms.last_retrieved,
			ms.status,
			COALESCE(AVG(rl.score), 0) as avg_score,
			m.question,
			m.tags,
			m.status as memory_status,
			m.created_at
		FROM memory_stats ms
		LEFT JOIN retrieval_log rl ON ms.memory_id = rl.memory_id
		LEFT JOIN memory m ON ms.memory_id = m.id
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
		var question sql.NullString
		var tags sql.NullString
		var memoryStatus sql.NullString
		var createdAt sql.NullString

		if err := rows.Scan(&stats.MemoryID, &stats.RetrievalCount, &lastRetrieved, &stats.Status, &avgScore, &question, &tags, &memoryStatus, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan memory stats: %w", err)
		}

		if lastRetrieved.Valid {
			stats.LastRetrieved = lastRetrieved.String
		}
		if avgScore.Valid {
			stats.AvgScore = float32(avgScore.Float64)
		}
		if question.Valid {
			stats.Question = question.String
		}
		if tags.Valid && tags.String != "" {
			stats.Tags = strings.Split(tags.String, ",")
		} else {
			stats.Tags = []string{}
		}
		// Use memory status (pending/promoted) instead of stats status
		if memoryStatus.Valid {
			stats.Status = memoryStatus.String
		}
		if createdAt.Valid {
			stats.CreatedAt = createdAt.String
		}

		results = append(results, stats)
	}

	return results, rows.Err()
}

// GetMemoryRetrievalHistory returns recent retrieval log entries for a memory.
func (s *SQLiteStorage) GetMemoryRetrievalHistory(ctx context.Context, memoryID string, limit int) ([]RetrievalLogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
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

// PruneMemoryRetrievalLog removes retrieval log entries older than the specified duration.
func (s *SQLiteStorage) PruneMemoryRetrievalLog(ctx context.Context, olderThanDays int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStorageClosed
	}

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM retrieval_log
		WHERE timestamp < datetime('now', '-' || ? || ' days')
	`, olderThanDays)
	if err != nil {
		return 0, fmt.Errorf("failed to prune retrieval log: %w", err)
	}

	return result.RowsAffected()
}

// UpdateMemoryStatsStatus updates the status of a memory entry (active, archived, etc.).
func (s *SQLiteStorage) UpdateMemoryStatsStatus(ctx context.Context, memoryID string, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_stats (memory_id, retrieval_count, status)
		VALUES (?, 0, ?)
		ON CONFLICT(memory_id) DO UPDATE SET status = ?
	`, memoryID, status, status)
	if err != nil {
		return fmt.Errorf("failed to update memory status: %w", err)
	}

	return nil
}
