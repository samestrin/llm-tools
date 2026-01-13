package semantic

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// stringToUUID converts an arbitrary string to a valid UUID format using SHA256
func stringToUUID(s string) string {
	hash := sha256.Sum256([]byte(s))
	// Format as UUID: 8-4-4-4-12 hex chars
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}

// QdrantConfig holds configuration for Qdrant storage
type QdrantConfig struct {
	APIKey         string
	URL            string // Full URL like https://abc123.qdrant.io:6334
	CollectionName string
	EmbeddingDim   int
}

// Validate checks if the config is valid
func (c *QdrantConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("QDRANT_API_KEY is required")
	}
	if c.URL == "" {
		return fmt.Errorf("QDRANT_API_URL is required")
	}
	if c.EmbeddingDim <= 0 {
		return fmt.Errorf("EmbeddingDim must be positive")
	}
	if c.CollectionName == "" {
		c.CollectionName = "llm_semantic"
	}
	return nil
}

// QdrantStorage implements the Storage interface using Qdrant cloud
type QdrantStorage struct {
	client         *qdrant.Client
	collectionName string
	embeddingDim   int
	mu             sync.RWMutex
	closed         bool
}

// NewQdrantStorage creates a new Qdrant-based storage
func NewQdrantStorage(config QdrantConfig) (*QdrantStorage, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	host, port, useTLS, err := parseQdrantURL(config.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Qdrant URL: %w", err)
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host:   host,
		Port:   port,
		APIKey: config.APIKey,
		UseTLS: useTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Qdrant client: %w", err)
	}

	storage := &QdrantStorage{
		client:         client,
		collectionName: config.CollectionName,
		embeddingDim:   config.EmbeddingDim,
	}

	// Ensure collection exists
	if err := storage.ensureCollection(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ensure collection: %w", err)
	}

	return storage, nil
}

// parseQdrantURL extracts host, port, and TLS setting from a URL
func parseQdrantURL(rawURL string) (host string, port int, useTLS bool, err error) {
	// Default port
	port = 6334

	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", 0, false, err
	}

	// Determine TLS
	useTLS = u.Scheme == "https"

	// Extract host
	host = u.Hostname()
	if host == "" {
		return "", 0, false, fmt.Errorf("missing host in URL")
	}

	// Extract port if specified
	if portStr := u.Port(); portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, false, fmt.Errorf("invalid port: %w", err)
		}
	}

	return host, port, useTLS, nil
}

func (s *QdrantStorage) ensureCollection() error {
	ctx := context.Background()

	// Check if collection exists
	collections, err := s.client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed to list collections: %w", err)
	}

	for _, c := range collections {
		if c == s.collectionName {
			return nil // Collection exists
		}
	}

	// Create collection with cosine similarity
	err = s.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: s.collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(s.embeddingDim),
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// DeleteCollection removes the collection (used for testing cleanup)
func (s *QdrantStorage) DeleteCollection() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.client.DeleteCollection(context.Background(), s.collectionName)
}

func (s *QdrantStorage) Create(ctx context.Context, chunk Chunk, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	point := s.chunkToPoint(chunk, embedding)
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Points:         []*qdrant.PointStruct{point},
	})
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

func (s *QdrantStorage) CreateBatch(ctx context.Context, chunks []ChunkWithEmbedding) error {
	if len(chunks) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Convert all chunks to points
	points := make([]*qdrant.PointStruct, len(chunks))
	for i, cwe := range chunks {
		points[i] = s.chunkToPoint(cwe.Chunk, cwe.Embedding)
	}

	// Batch upsert all points in a single request
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to batch upsert points: %w", err)
	}

	return nil
}

func (s *QdrantStorage) Read(ctx context.Context, id string) (*Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	points, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(stringToUUID(id))},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get point: %w", err)
	}

	if len(points) == 0 {
		return nil, ErrNotFound
	}

	chunk := s.pointToChunk(points[0])
	return &chunk, nil
}

func (s *QdrantStorage) Update(ctx context.Context, chunk Chunk, embedding []float32) error {
	// Qdrant upsert is idempotent, so update is the same as create
	return s.Create(ctx, chunk, embedding)
}

func (s *QdrantStorage) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// First check if the point exists
	points, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(stringToUUID(id))},
	})
	if err != nil {
		return fmt.Errorf("failed to check point: %w", err)
	}
	if len(points) == 0 {
		return ErrNotFound
	}

	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: []*qdrant.PointId{qdrant.NewID(stringToUUID(id))},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete point: %w", err)
	}

	return nil
}

func (s *QdrantStorage) DeleteByFilePath(ctx context.Context, filePath string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrStorageClosed
	}

	// Count matching points first
	scrollResult, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collectionName,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("file_path", filePath),
			},
		},
		WithPayload: qdrant.NewWithPayload(false),
		Limit:       qdrant.PtrOf(uint32(10000)),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to scroll points: %w", err)
	}

	count := len(scrollResult)
	if count == 0 {
		return 0, nil
	}

	// Delete by filter
	_, err = s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{
						qdrant.NewMatch("file_path", filePath),
					},
				},
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete points: %w", err)
	}

	return count, nil
}

func (s *QdrantStorage) List(ctx context.Context, opts ListOptions) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// Build filter conditions
	var conditions []*qdrant.Condition
	if opts.FilePath != "" {
		conditions = append(conditions, qdrant.NewMatch("file_path", opts.FilePath))
	}
	if opts.Type != "" {
		conditions = append(conditions, qdrant.NewMatch("type", opts.Type))
	}
	if opts.Language != "" {
		conditions = append(conditions, qdrant.NewMatch("language", opts.Language))
	}

	var filter *qdrant.Filter
	if len(conditions) > 0 {
		filter = &qdrant.Filter{Must: conditions}
	}

	limit := uint32(10000)
	if opts.Limit > 0 {
		limit = uint32(opts.Limit + opts.Offset)
	}

	scrollResult, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collectionName,
		Filter:         filter,
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          qdrant.PtrOf(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scroll points: %w", err)
	}

	// Apply offset
	start := opts.Offset
	if start > len(scrollResult) {
		start = len(scrollResult)
	}

	// Apply limit
	end := len(scrollResult)
	if opts.Limit > 0 && start+opts.Limit < end {
		end = start + opts.Limit
	}

	chunks := make([]Chunk, 0, end-start)
	for _, point := range scrollResult[start:end] {
		chunks = append(chunks, s.pointToChunk(point))
	}

	return chunks, nil
}

func (s *QdrantStorage) Search(ctx context.Context, queryEmbedding []float32, opts SearchOptions) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	// Build filter
	var conditions []*qdrant.Condition
	if opts.Type != "" {
		conditions = append(conditions, qdrant.NewMatch("type", opts.Type))
	}
	if opts.PathFilter != "" {
		// Use prefix matching for path filter
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "file_path",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Text{
							Text: opts.PathFilter,
						},
					},
				},
			},
		})
	}

	var filter *qdrant.Filter
	if len(conditions) > 0 {
		filter = &qdrant.Filter{Must: conditions}
	}

	limit := uint64(10)
	if opts.TopK > 0 {
		limit = uint64(opts.TopK)
	}

	searchResult, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collectionName,
		Query:          qdrant.NewQuery(queryEmbedding...),
		Filter:         filter,
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          qdrant.PtrOf(limit),
		ScoreThreshold: qdrant.PtrOf(opts.Threshold),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	results := make([]SearchResult, 0, len(searchResult))
	for _, point := range searchResult {
		chunk := s.scoredPointToChunk(point)
		results = append(results, SearchResult{
			Chunk: chunk,
			Score: point.Score,
		})
	}

	return results, nil
}

func (s *QdrantStorage) Stats(ctx context.Context) (*IndexStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrStorageClosed
	}

	info, err := s.client.GetCollectionInfo(ctx, s.collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	// Count unique file paths by scrolling (expensive but necessary)
	filesIndexed := 0
	scrollResult, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collectionName,
		WithPayload:    qdrant.NewWithPayloadInclude("file_path"),
		Limit:          qdrant.PtrOf(uint32(10000)),
	})
	if err == nil {
		fileSet := make(map[string]bool)
		for _, point := range scrollResult {
			if fp, ok := point.Payload["file_path"]; ok {
				if str := fp.GetStringValue(); str != "" {
					fileSet[str] = true
				}
			}
		}
		filesIndexed = len(fileSet)
	}

	chunksTotal := 0
	if info.PointsCount != nil {
		chunksTotal = int(*info.PointsCount)
	}

	return &IndexStats{
		ChunksTotal:  chunksTotal,
		FilesIndexed: filesIndexed,
		LastUpdated:  time.Now().Format(time.RFC3339),
	}, nil
}

// Clear removes all points from the Qdrant collection (for force re-index)
func (s *QdrantStorage) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Delete all points by using an empty filter that matches everything
	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: &qdrant.Filter{
					Must: []*qdrant.Condition{}, // Empty filter matches all
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to clear collection: %w", err)
	}

	return nil
}

func (s *QdrantStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.client.Close()
}

// ===== Memory Entry Methods =====
// TODO: Full implementation in Task-03

// StoreMemory stores a memory entry with its embedding
func (s *QdrantStorage) StoreMemory(ctx context.Context, entry MemoryEntry, embedding []float32) error {
	return fmt.Errorf("StoreMemory not implemented yet")
}

// StoreMemoryBatch stores multiple memory entries with their embeddings
func (s *QdrantStorage) StoreMemoryBatch(ctx context.Context, entries []MemoryWithEmbedding) error {
	return fmt.Errorf("StoreMemoryBatch not implemented yet")
}

// SearchMemory finds memory entries similar to the query embedding
func (s *QdrantStorage) SearchMemory(ctx context.Context, queryEmbedding []float32, opts MemorySearchOptions) ([]MemorySearchResult, error) {
	return nil, fmt.Errorf("SearchMemory not implemented yet")
}

// GetMemory retrieves a memory entry by ID
func (s *QdrantStorage) GetMemory(ctx context.Context, id string) (*MemoryEntry, error) {
	return nil, fmt.Errorf("GetMemory not implemented yet")
}

// DeleteMemory removes a memory entry by ID
func (s *QdrantStorage) DeleteMemory(ctx context.Context, id string) error {
	return fmt.Errorf("DeleteMemory not implemented yet")
}

// ListMemory retrieves memory entries based on filter options
func (s *QdrantStorage) ListMemory(ctx context.Context, opts MemoryListOptions) ([]MemoryEntry, error) {
	return nil, fmt.Errorf("ListMemory not implemented yet")
}

// fileHashID generates a unique ID for file hash storage
func fileHashID(filePath string) string {
	return stringToUUID("file_hash:" + filePath)
}

// GetFileHash retrieves the stored content hash for a file path
func (s *QdrantStorage) GetFileHash(ctx context.Context, filePath string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return "", ErrStorageClosed
	}

	points, err := s.client.Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collectionName,
		Ids:            []*qdrant.PointId{qdrant.NewID(fileHashID(filePath))},
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file hash: %w", err)
	}

	if len(points) == 0 {
		return "", nil // Not indexed yet
	}

	if v, ok := points[0].Payload["content_hash"]; ok {
		return v.GetStringValue(), nil
	}

	return "", nil
}

// SetFileHash stores the content hash for a file path
func (s *QdrantStorage) SetFileHash(ctx context.Context, filePath string, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrStorageClosed
	}

	// Create a dummy embedding (zeros) for the file hash point
	dummyEmbedding := make([]float32, s.embeddingDim)

	point := &qdrant.PointStruct{
		Id:      qdrant.NewID(fileHashID(filePath)),
		Vectors: qdrant.NewVectors(dummyEmbedding...),
		Payload: map[string]*qdrant.Value{
			"type":         qdrant.NewValueString("file_hash"),
			"file_path":    qdrant.NewValueString(filePath),
			"content_hash": qdrant.NewValueString(hash),
		},
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Points:         []*qdrant.PointStruct{point},
	})
	if err != nil {
		return fmt.Errorf("failed to set file hash: %w", err)
	}

	return nil
}

// chunkToPoint converts a Chunk to a Qdrant point
func (s *QdrantStorage) chunkToPoint(chunk Chunk, embedding []float32) *qdrant.PointStruct {
	return &qdrant.PointStruct{
		Id:      qdrant.NewID(stringToUUID(chunk.ID)),
		Vectors: qdrant.NewVectors(embedding...),
		Payload: map[string]*qdrant.Value{
			"chunk_id":   qdrant.NewValueString(chunk.ID), // Store original ID in payload
			"file_path":  qdrant.NewValueString(chunk.FilePath),
			"type":       qdrant.NewValueString(chunk.Type.String()),
			"name":       qdrant.NewValueString(chunk.Name),
			"signature":  qdrant.NewValueString(chunk.Signature),
			"content":    qdrant.NewValueString(chunk.Content),
			"start_line": qdrant.NewValueInt(int64(chunk.StartLine)),
			"end_line":   qdrant.NewValueInt(int64(chunk.EndLine)),
			"language":   qdrant.NewValueString(chunk.Language),
		},
	}
}

// pointToChunk converts a Qdrant point to a Chunk
func (s *QdrantStorage) pointToChunk(point *qdrant.RetrievedPoint) Chunk {
	chunk := Chunk{}

	payload := point.Payload
	if payload == nil {
		return chunk
	}

	// Get original chunk ID from payload (preferred over UUID)
	if v, ok := payload["chunk_id"]; ok {
		chunk.ID = v.GetStringValue()
	} else if point.Id != nil {
		// Fallback to UUID if chunk_id not in payload
		if str := point.Id.GetUuid(); str != "" {
			chunk.ID = str
		} else if num := point.Id.GetNum(); num != 0 {
			chunk.ID = fmt.Sprintf("%d", num)
		}
	}

	if v, ok := payload["file_path"]; ok {
		chunk.FilePath = v.GetStringValue()
	}
	if v, ok := payload["type"]; ok {
		chunkType, _ := ParseChunkType(v.GetStringValue())
		chunk.Type = chunkType
	}
	if v, ok := payload["name"]; ok {
		chunk.Name = v.GetStringValue()
	}
	if v, ok := payload["signature"]; ok {
		chunk.Signature = v.GetStringValue()
	}
	if v, ok := payload["content"]; ok {
		chunk.Content = v.GetStringValue()
	}
	if v, ok := payload["start_line"]; ok {
		chunk.StartLine = int(v.GetIntegerValue())
	}
	if v, ok := payload["end_line"]; ok {
		chunk.EndLine = int(v.GetIntegerValue())
	}
	if v, ok := payload["language"]; ok {
		chunk.Language = v.GetStringValue()
	}

	return chunk
}

// scoredPointToChunk converts a ScoredPoint to a Chunk (for search results)
func (s *QdrantStorage) scoredPointToChunk(point *qdrant.ScoredPoint) Chunk {
	chunk := Chunk{}

	payload := point.Payload
	if payload == nil {
		return chunk
	}

	// Get original chunk ID from payload (preferred over UUID)
	if v, ok := payload["chunk_id"]; ok {
		chunk.ID = v.GetStringValue()
	} else if point.Id != nil {
		// Fallback to UUID if chunk_id not in payload
		if str := point.Id.GetUuid(); str != "" {
			chunk.ID = str
		} else if num := point.Id.GetNum(); num != 0 {
			chunk.ID = fmt.Sprintf("%d", num)
		}
	}

	if v, ok := payload["file_path"]; ok {
		chunk.FilePath = v.GetStringValue()
	}
	if v, ok := payload["type"]; ok {
		chunkType, _ := ParseChunkType(v.GetStringValue())
		chunk.Type = chunkType
	}
	if v, ok := payload["name"]; ok {
		chunk.Name = v.GetStringValue()
	}
	if v, ok := payload["signature"]; ok {
		chunk.Signature = v.GetStringValue()
	}
	if v, ok := payload["content"]; ok {
		chunk.Content = v.GetStringValue()
	}
	if v, ok := payload["start_line"]; ok {
		chunk.StartLine = int(v.GetIntegerValue())
	}
	if v, ok := payload["end_line"]; ok {
		chunk.EndLine = int(v.GetIntegerValue())
	}
	if v, ok := payload["language"]; ok {
		chunk.Language = v.GetStringValue()
	}

	return chunk
}

// NewQdrantStorageFromEnv creates a QdrantStorage from environment variables
// Env vars: QDRANT_API_KEY, QDRANT_API_URL, QDRANT_COLLECTION (optional, default: llm_semantic)
func NewQdrantStorageFromEnv(embeddingDim int) (*QdrantStorage, error) {
	collectionName := strings.TrimSpace(os.Getenv("QDRANT_COLLECTION"))
	if collectionName == "" {
		collectionName = "llm_semantic"
	}

	config := QdrantConfig{
		APIKey:         strings.TrimSpace(os.Getenv("QDRANT_API_KEY")),
		URL:            strings.TrimSpace(os.Getenv("QDRANT_API_URL")),
		CollectionName: collectionName,
		EmbeddingDim:   embeddingDim,
	}
	return NewQdrantStorage(config)
}
