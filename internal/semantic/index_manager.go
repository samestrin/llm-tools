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
	Includes             []string               // Glob patterns to include (e.g., "*.go")
	Excludes             []string               // Patterns to exclude (directories and files, e.g., "vendor", "*_test.go")
	ExcludeTests         bool                   // Exclude common test files and directories
	Force                bool                   // Re-index all files even if unchanged
	Domain               string                 // Domain tag for indexed chunks (e.g., "code", "docs", "memory") - defaults to "code"
	OnProgress           ProgressCallback       // Optional callback for progress updates
	OnUploadProgress     UploadProgressCallback // Optional callback for upload phase progress
	BatchSize            int                    // Number of vectors to send per upsert (0 = unlimited)
	Parallel             int                    // Number of parallel batch uploads (0 or 1 = sequential)
	EmbedBatchSize       int                    // Number of chunks to embed per API call across files (0 = per-file batching)
	OverlapLines         int                    // Lines of overlap between adjacent chunks (default 3, 0 = disabled)
	IncludeParentContext bool                   // Prepend enclosing scope (package/class) to embedding text (default true)
}

// UpdateOptions configures incremental update behavior
type UpdateOptions struct {
	Includes     []string // Glob patterns to include
	Excludes     []string // Patterns to exclude (directories and files)
	ExcludeTests bool     // Exclude common test files and directories
	Domain       string   // Domain tag for indexed chunks (e.g., "code", "docs", "memory") - defaults to "code"
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
	FilesUpdated  int    `json:"files_updated"`
	FilesRemoved  int    `json:"files_removed"`
	ChunksCreated int    `json:"chunks_created"`
	ChunksRemoved int    `json:"chunks_removed"`
	Mode          string `json:"mode"` // "git" or "full"
}

// IndexManager handles indexing operations
type IndexManager struct {
	storage       Storage
	embedder      EmbedderInterface
	factory       *ChunkerFactory
	overlapConfig OverlapConfig
}

// getOverlapConfig returns the current overlap configuration.
func (m *IndexManager) getOverlapConfig() OverlapConfig {
	return m.overlapConfig
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

	// Set overlap configuration
	m.overlapConfig = OverlapConfig{
		OverlapLines:         opts.OverlapLines,
		IncludeParentContext: opts.IncludeParentContext,
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
		processErr := m.processFile(ctx, file, result, opts.BatchSize, opts.Parallel, opts.Domain)
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

		// Tag chunks with domain
		if opts.Domain != "" {
			for i := range chunks {
				chunks[i].Domain = opts.Domain
			}
		}

		// Apply overlap and parent context enrichment
		overlapCfg := m.getOverlapConfig()
		if overlapCfg.OverlapLines > 0 || overlapCfg.IncludeParentContext {
			fileContent, _ := os.ReadFile(file)
			chunks = ApplyOverlap(chunks, fileContent, overlapCfg)
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
			texts[i] = EnrichForEmbedding(pc.chunk)
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

	// Compute content hashes for diff-based re-indexing
	for i := range chunks {
		chunks[i].ContentHash = chunks[i].ComputeContentHash()
	}

	return chunks, hashStr, nil
}

// processFileWithDiff re-indexes a file using content-aware diffing.
// Only chunks whose content actually changed are re-embedded. Moved but
// unchanged code has its embedding reused. Returns (chunksCreated, chunksRemoved, error).
func (m *IndexManager) processFileWithDiff(ctx context.Context, filePath string, domain string) (int, int, error) {
	// Step 1: Re-chunk the file
	chunks, fileHash, err := m.chunkFile(filePath)
	if err != nil {
		return 0, 0, err
	}

	// Tag domain and compute content hashes
	for i := range chunks {
		if domain != "" {
			chunks[i].Domain = domain
		}
		chunks[i].ContentHash = chunks[i].ComputeContentHash()
	}

	// Apply overlap and parent context enrichment
	overlapCfg := m.getOverlapConfig()
	if overlapCfg.OverlapLines > 0 || overlapCfg.IncludeParentContext {
		fileContent, _ := os.ReadFile(filePath)
		chunks = ApplyOverlap(chunks, fileContent, overlapCfg)
	}

	// Step 2: Get old chunk summaries from storage
	oldSummaries, err := m.storage.GetChunksByFilePath(ctx, filePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get old chunks: %w", err)
	}

	// Step 3: Diff old vs new
	diff := DiffChunks(oldSummaries, chunks)

	// Step 4: Delete old chunks that are no longer present
	chunksRemoved := 0
	for _, oldID := range diff.Delete {
		if err := m.storage.Delete(ctx, oldID); err != nil {
			return 0, 0, fmt.Errorf("failed to delete old chunk %s: %w", oldID, err)
		}
		chunksRemoved++
	}

	// Step 5: Fetch reusable embeddings
	reuseOldIDs := make([]string, 0, len(diff.Reuse))
	for _, oldID := range diff.Reuse {
		reuseOldIDs = append(reuseOldIDs, oldID)
	}
	reusedEmbeddings, err := m.storage.ReadEmbeddings(ctx, reuseOldIDs)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read old embeddings: %w", err)
	}

	// Step 6: Store reused chunks (delete old ID first if different, then create with new)
	for newIdx, oldID := range diff.Reuse {
		newChunk := chunks[newIdx]
		emb, ok := reusedEmbeddings[oldID]
		if !ok {
			// Embedding not found — fall back to re-embed
			diff.NeedEmbed = append(diff.NeedEmbed, newIdx)
			continue
		}
		// Delete old chunk if ID changed
		if newChunk.ID != oldID {
			_ = m.storage.Delete(ctx, oldID)
			chunksRemoved++
		}
		if err := m.storage.Create(ctx, newChunk, emb); err != nil {
			return 0, 0, fmt.Errorf("failed to store reused chunk: %w", err)
		}
	}

	// Step 7: Embed and store new/changed chunks
	chunksCreated := len(diff.Reuse) // reused counts as "created" in the new index
	if len(diff.NeedEmbed) > 0 {
		texts := make([]string, len(diff.NeedEmbed))
		embedChunks := make([]Chunk, len(diff.NeedEmbed))
		for i, idx := range diff.NeedEmbed {
			embedChunks[i] = chunks[idx]
			texts[i] = EnrichForEmbedding(chunks[idx])
		}

		embeddings, err := m.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to generate embeddings: %w", err)
		}

		cwes := make([]ChunkWithEmbedding, len(embedChunks))
		for i := range embedChunks {
			cwes[i] = ChunkWithEmbedding{
				Chunk:     embedChunks[i],
				Embedding: embeddings[i],
			}
		}

		if err := m.storage.CreateBatch(ctx, cwes); err != nil {
			return 0, 0, fmt.Errorf("failed to store new chunks: %w", err)
		}
		chunksCreated += len(cwes)
	}

	// Step 8: Update file hash
	if err := m.storage.SetFileHash(ctx, filePath, fileHash); err != nil {
		return 0, 0, fmt.Errorf("failed to set file hash: %w", err)
	}

	return chunksCreated, chunksRemoved, nil
}

// Update performs incremental index update
func (m *IndexManager) Update(ctx context.Context, rootPath string, opts UpdateOptions) (*UpdateResult, error) {
	result := &UpdateResult{}

	// Get all indexed file paths from storage (lightweight — no chunk content or embeddings)
	indexedFiles, err := m.storage.ListIndexedFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexed files: %w", err)
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
			// Use content-aware diff to only re-embed changed chunks
			created, removed, err := m.processFileWithDiff(ctx, file, opts.Domain)
			if err == nil {
				result.FilesUpdated++
				result.ChunksCreated += created
				result.ChunksRemoved += removed
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

// UpdateGit performs a git-aware incremental update. It uses git diff to find
// changed/added/deleted files since the given ref (e.g., "HEAD~1"), filters them
// against include/exclude patterns, and only re-indexes the affected files.
func (m *IndexManager) UpdateGit(ctx context.Context, rootPath string, gitRef string, opts UpdateOptions) (*UpdateResult, error) {
	result := &UpdateResult{}

	// Get changed files from git
	changed, deleted, err := gitChangedFiles(rootPath, gitRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get git changes: %w", err)
	}

	// Filter changed files against include/exclude patterns and supported extensions
	for _, relPath := range changed {
		absPath := filepath.Join(rootPath, relPath)

		if !m.fileMatchesFilters(rootPath, absPath, opts.Includes, opts.Excludes, opts.ExcludeTests) {
			continue
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		needsUpdate, err := m.fileNeedsUpdate(ctx, absPath)
		if err != nil || !needsUpdate {
			continue
		}

		// Remove old chunks and re-index
		removed, _ := m.storage.DeleteByFilePath(ctx, absPath)
		result.ChunksRemoved += removed

		indexResult := &IndexResult{}
		if err := m.processFile(ctx, absPath, indexResult, 0, 0, opts.Domain); err == nil {
			result.FilesUpdated++
			result.ChunksCreated += indexResult.ChunksCreated
		}
	}

	// Handle deleted files
	for _, relPath := range deleted {
		absPath := filepath.Join(rootPath, relPath)
		removed, _ := m.storage.DeleteByFilePath(ctx, absPath)
		if removed > 0 {
			result.ChunksRemoved += removed
			result.FilesRemoved++
		}
	}

	return result, nil
}

// fileMatchesFilters checks if a single file passes the include/exclude/extension filters.
func (m *IndexManager) fileMatchesFilters(rootPath, absPath string, includes, excludes []string, excludeTests bool) bool {
	fileName := filepath.Base(absPath)
	relPath := strings.TrimPrefix(absPath, rootPath)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

	allExcludes := excludes
	if excludeTests {
		allExcludes = append(allExcludes, TestDirPatterns...)
		allExcludes = append(allExcludes, TestFilePatterns...)
	}

	// Check excludes
	for _, pattern := range allExcludes {
		if strings.Contains(pattern, "**") {
			if m, _ := doublestar.Match(pattern, fileName); m {
				return false
			}
			if m, _ := doublestar.Match(pattern, relPath); m {
				return false
			}
		} else {
			if m, _ := filepath.Match(pattern, fileName); m {
				return false
			}
			// Check path components for exact directory matches
			if !strings.ContainsAny(pattern, "*?[") {
				for _, part := range strings.Split(relPath, string(filepath.Separator)) {
					if part == pattern {
						return false
					}
				}
			}
		}
	}

	// Check includes (if specified)
	if len(includes) > 0 {
		matched := false
		for _, pattern := range includes {
			if strings.Contains(pattern, "**") {
				if m, _ := doublestar.Match(pattern, fileName); m {
					matched = true
					break
				}
				if m, _ := doublestar.Match(pattern, relPath); m {
					matched = true
					break
				}
			} else {
				if m, _ := filepath.Match(pattern, fileName); m {
					matched = true
					break
				}
			}
		}
		if !matched {
			return false
		}
	}

	// Check if we have a chunker for this file type
	if _, ok := m.factory.GetByExtension(absPath); !ok {
		return false
	}

	return true
}

// Status returns index statistics
func (m *IndexManager) Status(ctx context.Context) (*IndexStats, error) {
	return m.storage.Stats(ctx)
}

// collectFiles walks the directory and collects files matching criteria
func (m *IndexManager) collectFiles(rootPath string, includes, excludes []string, excludeTests bool) ([]string, error) {
	var files []string

	// Build effective exclude list by combining user excludes with test patterns
	allExcludes := excludes
	if excludeTests {
		allExcludes = append(allExcludes, TestDirPatterns...)
		allExcludes = append(allExcludes, TestFilePatterns...)
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
	for _, exc := range allExcludes {
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
			// Check filename against pattern (handles patterns like *.test.ts)
			if m, _ := filepath.Match(pattern, d.Name()); m {
				return nil
			}
			// Check if any directory component exactly matches the pattern
			// This catches files inside excluded directories (e.g., .stryker-tmp/docs/file.md)
			// We use exact match for directories, not pattern matching
			// because patterns like *.test.ts should not match directory names
			if !strings.ContainsAny(pattern, "*?[") {
				// Only check exact name matches for patterns without wildcards
				pathParts := strings.Split(relPath, string(filepath.Separator))
				for _, part := range pathParts {
					// Use ToSlash for cross-platform consistency
					if part == pattern || part == filepath.ToSlash(pattern) {
						return nil
					}
				}
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
func (m *IndexManager) processFile(ctx context.Context, filePath string, result *IndexResult, batchSize, parallel int, domain string) error {
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

	// Tag chunks with domain and compute content hashes
	for i := range chunks {
		if domain != "" {
			chunks[i].Domain = domain
		}
		chunks[i].ContentHash = chunks[i].ComputeContentHash()
	}

	// Apply overlap and parent context enrichment
	overlapCfg := m.getOverlapConfig()
	if overlapCfg.OverlapLines > 0 || overlapCfg.IncludeParentContext {
		chunks = ApplyOverlap(chunks, content, overlapCfg)
	}

	// Collect all chunk contents for batch embedding
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = EnrichForEmbedding(chunk)
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

	// Extract and store chunk references if chunker supports it
	m.extractAndStoreRefs(ctx, chunker, filePath, content, chunks)

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

// extractAndStoreRefs checks if the chunker implements RefExtractor and if
// the storage implements RefStorage, then extracts and stores chunk references.
func (m *IndexManager) extractAndStoreRefs(ctx context.Context, chunker Chunker, filePath string, content []byte, chunks []Chunk) {
	re, ok := chunker.(RefExtractor)
	if !ok {
		return
	}
	rs, ok := m.storage.(RefStorage)
	if !ok {
		return
	}

	refs, err := re.ExtractRefs(filePath, content, chunks)
	if err != nil || len(refs) == 0 {
		return
	}

	// Delete old refs for these chunks first
	for _, ch := range chunks {
		_ = rs.DeleteRefsByChunk(ctx, ch.ID)
	}

	_ = rs.StoreRefs(ctx, refs)
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
