package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
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
	`

	_, err := s.db.Exec(schema)
	return err
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

func (s *SQLiteStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.db.Close()
}

// encodeEmbedding converts a float32 slice to bytes for storage
func encodeEmbedding(embedding []float32) ([]byte, error) {
	return json.Marshal(embedding)
}

// decodeEmbedding converts bytes back to a float32 slice
func decodeEmbedding(data []byte) ([]float32, error) {
	var embedding []float32
	err := json.Unmarshal(data, &embedding)
	return embedding, err
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
