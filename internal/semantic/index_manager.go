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

// IndexOptions configures indexing behavior
type IndexOptions struct {
	Includes   []string         // Glob patterns to include (e.g., "*.go")
	Excludes   []string         // Directory/pattern names to exclude (e.g., "vendor", "node_modules")
	Force      bool             // Re-index all files even if unchanged
	OnProgress ProgressCallback // Optional callback for progress updates
}

// UpdateOptions configures incremental update behavior
type UpdateOptions struct {
	Includes []string // Glob patterns to include
	Excludes []string // Patterns to exclude
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
	files, err := m.collectFiles(rootPath, opts.Includes, opts.Excludes)
	if err != nil {
		return nil, fmt.Errorf("failed to collect files: %w", err)
	}

	totalFiles := len(files)

	// Process each file
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
		processErr := m.processFile(ctx, file, result)
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
				Skipped:     false, // Not skipped - we tried to process it
			})
		}
	}

	return result, nil
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
	files, err := m.collectFiles(rootPath, opts.Includes, opts.Excludes)
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

			// Re-index the file
			indexResult := &IndexResult{}
			if err := m.processFile(ctx, file, indexResult); err == nil {
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
func (m *IndexManager) collectFiles(rootPath string, includes, excludes []string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		if d.IsDir() {
			relPath := strings.TrimPrefix(path, rootPath)
			relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

			for _, exclude := range excludes {
				if matched, _ := filepath.Match(exclude, d.Name()); matched {
					return filepath.SkipDir
				}
				if strings.Contains(relPath, exclude) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check if file matches includes (if specified)
		if len(includes) > 0 {
			matched := false
			for _, include := range includes {
				if m, _ := filepath.Match(include, d.Name()); m {
					matched = true
					break
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
func (m *IndexManager) processFile(ctx context.Context, filePath string, result *IndexResult) error {
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

	// Store all chunks in a single batch operation
	if err := m.storage.CreateBatch(ctx, chunksWithEmbeddings); err != nil {
		return fmt.Errorf("failed to store chunks: %w", err)
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
