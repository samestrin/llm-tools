package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"golang.org/x/sync/errgroup"
)

// ProgressEvent represents a progress update during indexing
type ProgressEvent struct {
	Current     int    // Current file number (1-based)
	Total       int    // Total files to process
	FilePath    string // Current file being processed
	ChunksTotal int    // Total chunks created so far
	Skipped     bool   // True if this file was skipped (already indexed)
}

// ProgressCallback is called during indexing to report progress
type ProgressCallback func(event ProgressEvent)

// UploadProgressEvent represents a progress update during upload to storage
type UploadProgressEvent struct {
	Phase          string  // "embedding" or "uploading"
	Current        int     // Current batch number (1-based)
	Total          int     // Total batches
	ChunksUploaded int     // Total chunks uploaded so far
	ChunksTotal    int     // Total chunks to upload
	BatchSize      int     // Size of current batch
	ElapsedSeconds float64 // Time elapsed since upload phase started
	ETASeconds     float64 // Estimated seconds remaining (0 if not calculable)
}

// UploadProgressCallback is called during storage upload to report progress
type UploadProgressCallback func(event UploadProgressEvent)

// DefaultBatchSize is the default number of vectors to send per upsert operation
const DefaultBatchSize = 100

// Common test file patterns used by ExcludeTests option
var TestFilePatterns = []string{
	"*_test.go",  // Go
	"*_test.ts",  // TypeScript
	"*_test.js",  // JavaScript
	"*_test.tsx", // React TypeScript
	"*_test.jsx", // React JavaScript
	"*.test.ts",  // TypeScript (Jest style)
	"*.test.js",  // JavaScript (Jest style)
	"*.test.tsx", // React TypeScript (Jest style)
	"*.test.jsx", // React JavaScript (Jest style)
	"*.spec.ts",  // TypeScript (Jasmine/Mocha style)
	"*.spec.js",  // JavaScript (Jasmine/Mocha style)
	"*.spec.tsx", // React TypeScript
	"*.spec.jsx", // React JavaScript
	"test_*.py",  // Python (prefix style)
	"*_test.py",  // Python (suffix style)
	"*_test.rs",  // Rust
	"*_test.php", // PHP
	"*Test.php",  // PHP (PHPUnit style)
	"*_spec.rb",  // Ruby (RSpec)
	"test_*.rb",  // Ruby (prefix style)
}

// Common test directory patterns used by ExcludeTests option
var TestDirPatterns = []string{
	"__tests__",    // Jest
	"__mocks__",    // Jest mocks
	"test",         // Generic
	"tests",        // Generic
	"spec",         // RSpec/Jasmine
	"specs",        // RSpec/Jasmine
	"testdata",     // Go
	"test_data",    // Python
	"fixtures",     // Test fixtures
	"__fixtures__", // Jest fixtures
}

// IndexOptions configures indexing behavior
type IndexOptions struct {
	Includes         []string               // Glob patterns to include (e.g., "*.go")
	Excludes         []string               // Patterns to exclude (directories and files, e.g., "vendor", "*_test.go")
	ExcludeTests     bool                   // Exclude common test files and directories
	Force            bool                   // Re-index all files even if unchanged
	OnProgress       ProgressCallback       // Optional callback for progress updates
	OnUploadProgress UploadProgressCallback // Optional callback for upload phase progress
	BatchSize        int                    // Number of vectors to send per upsert (0 = unlimited)
	Parallel         int                    // Number of parallel batch uploads (0 or 1 = sequential)
	EmbedBatchSize   int                    // Number of chunks to embed per API call across files (0 = per-file batching)
}

// UpdateOptions configures incremental update behavior
type UpdateOptions struct {
	Includes     []string // Glob patterns to include
	Excludes     []string // Patterns to exclude (directories and files)
	ExcludeTests bool     // Exclude common test files and directories
}

// IndexResult contains statistics from an indexing operation
type IndexResult struct {
	FilesProcessed int      // Files that were indexed
	FilesSkipped   int      // Files skipped due to errors
	FilesUnchanged int      // Files skipped because already indexed and unchanged
	ChunksCreated  int      // Total chunks created
	Errors         []string // Error messages
}

// UpdateResult contains statistics from an update operation
type UpdateResult struct {
	FilesUpdated  int
	FilesRemoved  int
	ChunksCreated int
	ChunksRemoved int
}

// IndexManager handles indexing operations
type IndexManager struct {
	storage  Storage
	embedder EmbedderInterface
	factory  *ChunkerFactory
}

// NewIndexManager creates a new IndexManager
func NewIndexManager(storage Storage, embedder EmbedderInterface, factory *ChunkerFactory) *IndexManager {
	return &IndexManager{
		storage:  storage,
		embedder: embedder,
		factory:  factory,
	}
}

// pendingChunk holds a chunk waiting to be embedded, along with its file metadata
type pendingChunk struct {
	chunk       Chunk
	filePath    string
	contentHash string
}

// Index builds or rebuilds the semantic index for a directory
func (m *IndexManager) Index(ctx context.Context, rootPath string, opts IndexOptions) (*IndexResult, error) {
	result := &IndexResult{}

	// Verify path exists
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rootPath)
	}

	// Clear existing index if Force is set
	if opts.Force {
		if err := m.storage.Clear(ctx); err != nil {
			return nil, fmt.Errorf("failed to clear existing index: %w", err)
		}
	}

	// Walk directory and collect files
	files, err := m.collectFiles(rootPath, opts.Includes, opts.Excludes, opts.ExcludeTests)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	// Use cross-file batching if EmbedBatchSize is set
	if opts.EmbedBatchSize > 0 {
		return m.indexWithCrossFileBatching(ctx, files, opts, result)
	}

	// Original per-file approach
	return m.indexPerFile(ctx, files, opts, result)
}

// indexPerFile processes files one at a time (original behavior)
func (m *IndexManager) indexPerFile(ctx context.Context, files []string, opts IndexOptions, result *IndexResult) (*IndexResult, error) {
	totalFiles := len(files)

	for i, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Check if file is already indexed and unchanged (for resume capability)
		if !opts.Force {
			needsIndex, _ := m.fileNeedsUpdate(ctx, file)
			if !needsIndex {
				result.FilesUnchanged++
				// Report progress for skipped file
				if opts.OnProgress != nil {
					opts.OnProgress(ProgressEvent{
						Current:     i + 1,
						Total:       totalFiles,
						FilePath:    file,
						ChunksTotal: result.ChunksCreated,
						Skipped:     true,
					})
				}
				continue
			}
		}

		// Process the file
		processErr := m.processFile(ctx, file, result, opts.BatchSize, opts.Parallel)
		if processErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file, processErr))
			result.FilesSkipped++
		} else {
			result.FilesProcessed++
		}

		// Report progress
		if opts.OnProgress != nil {
			opts.OnProgress(ProgressEvent{
				Current:     i + 1,
				Total:       totalFiles,
				FilePath:    file,
				ChunksTotal: result.ChunksCreated,
				Skipped:     false,
			})
		}
	}

	return result, nil
}

// indexWithCrossFileBatching processes files in batches with incremental commits.
// Each batch: chunk files → embed chunks → store to DB → commit file hashes.
// This allows resuming interrupted indexing - files with stored hashes are skipped.
func (m *IndexManager) indexWithCrossFileBatching(ctx context.Context, files []string, opts IndexOptions, result *IndexResult) (*IndexResult, error) {
	totalFiles := len(files)
	embedBatchSize := opts.EmbedBatchSize

	// First pass: filter files that need indexing and group into batches
	// Each batch targets ~embedBatchSize chunks for efficient API usage
	type fileBatch struct {
		files  []string
		hashes map[string]string // filePath -> contentHash
		chunks []pendingChunk
	}

	var batches []fileBatch
	currentBatch := fileBatch{hashes: make(map[string]string)}
	fileIndex := 0

	for _, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		fileIndex++

		// Check if file is already indexed and unchanged
		if !opts.Force {
			needsIndex, _ := m.fileNeedsUpdate(ctx, file)
			if !needsIndex {
				result.FilesUnchanged++
				if opts.OnProgress != nil {
					opts.OnProgress(ProgressEvent{
						Current:     fileIndex,
						Total:       totalFiles,
						FilePath:    file,
						ChunksTotal: result.ChunksCreated,
						Skipped:     true,
					})
				}
				continue
			}
		}

		// Chunk the file
		chunks, contentHash, err := m.chunkFile(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file, err))
			result.FilesSkipped++
			continue
		}

		// Add to current batch
		currentBatch.files = append(currentBatch.files, file)
		currentBatch.hashes[file] = contentHash
		for _, chunk := range chunks {
			currentBatch.chunks = append(currentBatch.chunks, pendingChunk{
				chunk:       chunk,
				filePath:    file,
				contentHash: contentHash,
			})
		}

		// Report chunking progress
		if opts.OnProgress != nil {
			opts.OnProgress(ProgressEvent{
				Current:     fileIndex,
				Total:       totalFiles,
				FilePath:    file,
				ChunksTotal: result.ChunksCreated + len(currentBatch.chunks),
				Skipped:     false,
			})
		}

		// When batch reaches target size, finalize it
		if len(currentBatch.chunks) >= embedBatchSize {
			batches = append(batches, currentBatch)
			currentBatch = fileBatch{hashes: make(map[string]string)}
		}
	}

	// Don't forget the last partial batch
	if len(currentBatch.chunks) > 0 {
		batches = append(batches, currentBatch)
	}

	if len(batches) == 0 {
		return result, nil
	}

	// Process each batch: embed → store → commit hashes
	totalBatches := len(batches)
	startTime := time.Now()

	for batchIdx, batch := range batches {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		batchNum := batchIdx + 1

		// Step 1: Embed all chunks in this batch
		texts := make([]string, len(batch.chunks))
		for i, pc := range batch.chunks {
			texts[i] = pc.chunk.Content
		}

		embeddings, err := m.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return result, fmt.Errorf("failed to generate embeddings (batch %d): %w", batchNum, err)
		}

		// Pair chunks with embeddings
		chunksWithEmbeddings := make([]ChunkWithEmbedding, len(batch.chunks))
		for i, pc := range batch.chunks {
			chunksWithEmbeddings[i] = ChunkWithEmbedding{
				Chunk:     pc.chunk,
				Embedding: embeddings[i],
			}
		}

		// Step 2: Store chunks to database (with optional parallel sub-batching)
		storeBatchSize := opts.BatchSize
		if storeBatchSize <= 0 || opts.Parallel <= 1 {
			// Sequential: store all chunks in one call (storage layer handles sub-batching)
			if err := m.storage.CreateBatch(ctx, chunksWithEmbeddings); err != nil {
				return result, fmt.Errorf("failed to store chunks (batch %d): %w", batchNum, err)
			}
		} else {
			// Parallel: split into sub-batches and upload concurrently
			g, gctx := errgroup.WithContext(ctx)
			g.SetLimit(opts.Parallel)

			var uploadedChunks int64
			totalChunksInBatch := len(chunksWithEmbeddings)
			uploadStartTime := time.Now()

			for i := 0; i < len(chunksWithEmbeddings); i += storeBatchSize {
				start := i
				end := i + storeBatchSize
				if end > len(chunksWithEmbeddings) {
					end = len(chunksWithEmbeddings)
				}
				subBatch := chunksWithEmbeddings[start:end]

				g.Go(func() error {
					if err := m.storage.CreateBatch(gctx, subBatch); err != nil {
						return fmt.Errorf("failed to store chunks (batch %d, sub-batch %d-%d): %w", batchNum, start, end, err)
					}
					// Track progress for potential future use
					atomic.AddInt64(&uploadedChunks, int64(len(subBatch)))
					return nil
				})
			}

			if err := g.Wait(); err != nil {
				return result, err
			}

			// Log parallel upload stats at debug level
			if totalChunksInBatch > storeBatchSize {
				uploadDuration := time.Since(uploadStartTime)
				_ = uploadDuration // Available for debug logging if needed
			}
		}

		// Step 3: Commit file hashes (this is the durability checkpoint)
		// After this, these files will be skipped on resume
		for filePath, hash := range batch.hashes {
			_ = m.storage.SetFileHash(ctx, filePath, hash)
		}

		// Update results
		result.FilesProcessed += len(batch.files)
		result.ChunksCreated += len(chunksWithEmbeddings)

		// Report progress
		if opts.OnUploadProgress != nil {
			elapsed := time.Since(startTime).Seconds()
			var eta float64
			if batchNum >= 2 && batchNum < totalBatches {
				avgPerBatch := elapsed / float64(batchNum)
				eta = avgPerBatch * float64(totalBatches-batchNum)
			}
			opts.OnUploadProgress(UploadProgressEvent{
				Phase:          "indexing",
				Current:        batchNum,
				Total:          totalBatches,
				ChunksUploaded: result.ChunksCreated,
				ChunksTotal:    0, // Unknown total in streaming mode
				BatchSize:      len(chunksWithEmbeddings),
				ElapsedSeconds: elapsed,
				ETASeconds:     eta,
			})
		}
	}

	return result, nil
}

// chunkFile reads and chunks a single file, returning chunks and content hash
func (m *IndexManager) chunkFile(filePath string) ([]Chunk, string, error) {
	// Get appropriate chunker
	chunker, ok := m.factory.GetByExtension(filePath)
	if !ok {
		return nil, "", fmt.Errorf("no chunker for file type")
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %w", err)
	}

	// Calculate hash
	contentHash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(contentHash[:])

	// Parse into chunks
	chunks, err := chunker.Chunk(filePath, content)
	if err != nil {
		return nil, "", fmt.Errorf("failed to chunk file: %w", err)
	}

	return chunks, hashStr, nil
}

// Update performs incremental index update
func (m *IndexManager) Update(ctx context.Context, rootPath string, opts UpdateOptions) (*UpdateResult, error) {
	result := &UpdateResult{}

	// Get all indexed files from storage
	indexedFiles := make(map[string]bool)
	chunks, err := m.storage.List(ctx, ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list indexed chunks: %w", err)
	}
	for _, chunk := range chunks {
		indexedFiles[chunk.FilePath] = true
	}

	// Collect current files
	files, err := m.collectFiles(rootPath, opts.Includes, opts.Excludes, opts.ExcludeTests)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	currentFiles := make(map[string]bool)
	for _, file := range files {
		currentFiles[file] = true
	}

	// Check for modified files
	for _, file := range files {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		needsUpdate, err := m.fileNeedsUpdate(ctx, file)
		if err != nil {
			continue
		}

		if needsUpdate {
			// Remove old chunks for this file
			removed, _ := m.storage.DeleteByFilePath(ctx, file)
			result.ChunksRemoved += removed

			// Re-index the file (use 0 for unlimited batch size in updates, sequential)
			indexResult := &IndexResult{}
			if err := m.processFile(ctx, file, indexResult, 0, 0); err == nil {
				result.FilesUpdated++
				result.ChunksCreated += indexResult.ChunksCreated
			}
		}
	}

	// Check for removed files
	for indexed := range indexedFiles {
		if !currentFiles[indexed] {
			removed, _ := m.storage.DeleteByFilePath(ctx, indexed)
			result.ChunksRemoved += removed
			result.FilesRemoved++
		}
	}

	return result, nil
}

// Status returns index statistics
func (m *IndexManager) Status(ctx context.Context) (*IndexStats, error) {
	return m.storage.Stats(ctx)
}

// collectFiles walks the directory and collects files matching criteria
func (m *IndexManager) collectFiles(rootPath string, includes, excludes []string, excludeTests bool) ([]string, error) {
	var files []string

	// Build effective exclude lists
	dirExcludes := excludes
	fileExcludes := excludes
	if excludeTests {
		dirExcludes = append(dirExcludes, TestDirPatterns...)
		fileExcludes = append(fileExcludes, TestFilePatterns...)
	}

	// Separate patterns into simple (no **) and doublestar (with **) lists
	// Simple patterns use filepath.Match for backward compatibility
	// Doublestar patterns use doublestar.Match for recursive matching
	var simpleIncludes, simpleExcludes []string
	var doublestarIncludes, doublestarExcludes []string
	for _, inc := range includes {
		if strings.Contains(inc, "**") {
			doublestarIncludes = append(doublestarIncludes, inc)
		} else {
			simpleIncludes = append(simpleIncludes, inc)
		}
	}
	for _, exc := range excludes {
		if strings.Contains(exc, "**") {
			doublestarExcludes = append(doublestarExcludes, exc)
		} else {
			simpleExcludes = append(simpleExcludes, exc)
		}
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, rootPath)
		relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
		if relPath == "" {
			relPath = "."
		}

		// Skip excluded directories
		if d.IsDir() {
			// Check simple patterns (against directory name)
			for _, pattern := range simpleExcludes {
				if m, _ := filepath.Match(pattern, d.Name()); m {
					return filepath.SkipDir
				}
				if strings.Contains(relPath, pattern) {
					return filepath.SkipDir
				}
			}
			// Check doublestar patterns (against relative path)
			for _, pattern := range doublestarExcludes {
				if m, _ := doublestar.Match(pattern, d.Name()); m {
					return filepath.SkipDir
				}
				if m, _ := doublestar.Match(pattern, relPath); m {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file matches excludes
		for _, pattern := range simpleExcludes {
			if m, _ := filepath.Match(pattern, d.Name()); m {
				return nil
			}
		}
		for _, pattern := range doublestarExcludes {
			if m, _ := doublestar.Match(pattern, d.Name()); m {
				return nil
			}
			if m, _ := doublestar.Match(pattern, relPath); m {
				return nil
			}
		}

		// Check if file matches includes (if specified)
		if len(includes) > 0 {
			matched := false
			// Check simple patterns (against filename)
			for _, pattern := range simpleIncludes {
				if m, _ := filepath.Match(pattern, d.Name()); m {
					matched = true
					break
				}
			}
			// Check doublestar patterns (against relative path)
			if !matched {
				for _, pattern := range doublestarIncludes {
					if m, _ := doublestar.Match(pattern, d.Name()); m {
						matched = true
						break
					}
					if m, _ := doublestar.Match(pattern, relPath); m {
						matched = true
						break
					}
				}
			}
			if !matched {
				return nil
			}
		}

		// Check if we have a chunker for this file type
		if _, ok := m.factory.GetByExtension(path); !ok {
			return nil
		}

		files = append(files, path)
		return nil
	})

	return files, err
}

// processFile chunks and embeds a single file
// batchSize controls how many vectors are sent per upsert (0 = unlimited)
// parallel controls how many batch uploads run concurrently (0 or 1 = sequential)
func (m *IndexManager) processFile(ctx context.Context, filePath string, result *IndexResult, batchSize, parallel int) error {
	// Get appropriate chunker
	chunker, ok := m.factory.GetByExtension(filePath)
	if !ok {
		return fmt.Errorf("no chunker for file type")
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse into chunks
	chunks, err := chunker.Chunk(filePath, content)
	if err != nil {
		return fmt.Errorf("failed to chunk file: %w", err)
	}

	// Collect all chunk contents for batch embedding
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// Check for cancellation before batch embedding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Generate embeddings for all chunks in a single batch request
	embeddings, err := m.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// Pair chunks with their embeddings
	chunksWithEmbeddings := make([]ChunkWithEmbedding, len(chunks))
	for i, chunk := range chunks {
		chunksWithEmbeddings[i] = ChunkWithEmbedding{
			Chunk:     chunk,
			Embedding: embeddings[i],
		}
	}

	// Store chunks in batches
	if batchSize <= 0 {
		// Send all chunks in a single batch operation
		if err := m.storage.CreateBatch(ctx, chunksWithEmbeddings); err != nil {
			return fmt.Errorf("failed to store chunks: %w", err)
		}
	} else if parallel <= 1 {
		// Sequential batch uploads
		for i := 0; i < len(chunksWithEmbeddings); i += batchSize {
			// Check for cancellation between batches
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			end := i + batchSize
			if end > len(chunksWithEmbeddings) {
				end = len(chunksWithEmbeddings)
			}
			if err := m.storage.CreateBatch(ctx, chunksWithEmbeddings[i:end]); err != nil {
				return fmt.Errorf("failed to store chunks (batch %d-%d): %w", i, end, err)
			}
		}
	} else {
		// Parallel batch uploads using errgroup
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(parallel)

		for i := 0; i < len(chunksWithEmbeddings); i += batchSize {
			start := i
			end := i + batchSize
			if end > len(chunksWithEmbeddings) {
				end = len(chunksWithEmbeddings)
			}
			batch := chunksWithEmbeddings[start:end]

			g.Go(func() error {
				if err := m.storage.CreateBatch(gctx, batch); err != nil {
					return fmt.Errorf("failed to store chunks (batch %d-%d): %w", start, end, err)
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return err
		}
	}
	result.ChunksCreated += len(chunksWithEmbeddings)

	// Store file hash after successfully processing all chunks
	// This enables resume capability - files with matching hashes will be skipped
	contentHash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(contentHash[:])
	if err := m.storage.SetFileHash(ctx, filePath, hashStr); err != nil {
		// Log but don't fail - the file was indexed successfully
		// Resume will just re-index this file next time
		return nil
	}

	return nil
}

// fileNeedsUpdate checks if a file has been modified since indexing
func (m *IndexManager) fileNeedsUpdate(ctx context.Context, filePath string) (bool, error) {
	// Get file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}

	// Calculate current content hash
	hash := sha256.Sum256(content)
	currentHash := hex.EncodeToString(hash[:])

	// Get stored hash for this file
	storedHash, err := m.storage.GetFileHash(ctx, filePath)
	if err != nil {
		return true, nil // Assume needs update on error
	}

	if storedHash == "" {
		return true, nil // No stored hash = needs indexing
	}

	// Compare hashes - file needs update if hashes differ
	return storedHash != currentHash, nil
}
