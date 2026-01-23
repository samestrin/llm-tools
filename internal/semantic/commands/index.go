package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	// progressReportInterval is the number of files between progress reports
	progressReportInterval = 100

	// Default maximum chunk sizes for different chunkers
	// These values are tuned for embedding models that typically have 512-8192 token contexts
	markdownMaxChunkSize = 4000 // ~1000 tokens, good for documentation sections
	htmlMaxChunkSize     = 4000 // not currently used for size-based chunking (HTML uses semantic boundaries)
	genericMaxChunkSize  = 2000 // smaller for generic code, ~500 tokens
)

func indexCmd() *cobra.Command {
	var (
		includes        []string
		excludes        []string
		excludeTests    bool
		force           bool
		jsonOutput      bool
		verbose         bool
		recalibrate     bool
		skipCalibration bool
		batchSize       int
		parallel        int
		embedBatchSize  int
	)

	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Build semantic index for a codebase",
		Long: `Build or rebuild the semantic index for a codebase.
Walks the directory, parses code files, and generates embeddings.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			// Expand path with glob support (e.g., "docs*/", "path/to/docs*")
			// If no glob wildcard found, use path as-is
			matches, err := filepath.Glob(path)
			if err != nil {
				return fmt.Errorf("invalid path pattern: %w", err)
			}
			if len(matches) == 0 {
				return fmt.Errorf("no matches for path pattern: %s", path)
			}

			// Filter out paths that contain excluded directories
			// This is needed because filepath.Glob() can expand to paths like ".stryker-tmp/docs/"
			// even when ".stryker-tmp" is in the excludes list
			matches = filterExcludedPaths(matches, excludes, excludeTests)

			if len(matches) == 0 && len(excludes) > 0 {
				return fmt.Errorf("all matched paths were excluded by exclude patterns")
			}

			// Index each matched path
			for _, match := range matches {
				if err := runIndex(cmd.Context(), match, indexOpts{
					includes:        includes,
					excludes:        excludes,
					excludeTests:    excludeTests,
					force:           force,
					jsonOutput:      jsonOutput,
					verbose:         verbose,
					recalibrate:     recalibrate,
					skipCalibration: skipCalibration,
					batchSize:       batchSize,
					parallel:        parallel,
					embedBatchSize:  embedBatchSize,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&includes, "include", "i", nil, "Glob patterns to include (e.g., '*.go')")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{"vendor", "node_modules", ".git"}, "Patterns to exclude (directories and files, e.g., 'vendor', '*_test.go')")
	cmd.Flags().BoolVar(&excludeTests, "exclude-tests", false, "Exclude common test files and directories")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force re-index all files")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show per-file progress instead of periodic summary")
	cmd.Flags().BoolVar(&recalibrate, "recalibrate", false, "Force recalibration of score thresholds")
	cmd.Flags().BoolVar(&skipCalibration, "skip-calibration", false, "Skip calibration step")
	cmd.Flags().IntVar(&batchSize, "batch-size", 0, "Number of vectors per upsert batch (0 = unlimited)")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "Number of parallel batch uploads (0 = sequential, requires --batch-size)")
	cmd.Flags().IntVar(&embedBatchSize, "embed-batch-size", 0, "Number of chunks to embed per API call across files (0 = per-file batching)")

	return cmd
}

type indexOpts struct {
	includes        []string
	excludes        []string
	excludeTests    bool
	force           bool
	jsonOutput      bool
	verbose         bool
	recalibrate     bool
	skipCalibration bool
	batchSize       int
	parallel        int
	embedBatchSize  int
}

func runIndex(ctx context.Context, path string, opts indexOpts) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Resolve index path (only needed for SQLite)
	indexPath := ""
	if storageType != "qdrant" {
		indexPath = resolveIndexPath(absPath)
		indexDir := filepath.Dir(indexPath)
		if err := os.MkdirAll(indexDir, 0700); err != nil {
			return fmt.Errorf("failed to create index directory %q: %w", indexDir, err)
		}
	}

	// Create embedder based on --embedder flag
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	// For Qdrant, we need to probe the embedder to get dimensions
	embeddingDim := 0
	if storageType == "qdrant" {
		if !opts.jsonOutput {
			fmt.Println("Probing embedding model for dimensions...")
		}
		testEmbed, err := embedder.Embed(ctx, "test")
		if err != nil {
			return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
		}
		embeddingDim = len(testEmbed)
		if embeddingDim == 0 {
			return fmt.Errorf("embedder returned zero-dimension embedding, check embedding model configuration")
		}
		if !opts.jsonOutput {
			fmt.Printf("Detected embedding dimension: %d\n", embeddingDim)
		}
	}

	// Create storage based on storage type flag
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Create chunker factory with all language support
	factory := semantic.NewChunkerFactory()
	RegisterAllChunkers(factory)

	// Create index manager
	mgr := semantic.NewIndexManager(storage, embedder, factory)

	// Run indexing
	if !opts.jsonOutput {
		fmt.Printf("Indexing %s...\n", absPath)
	}

	// Set up progress callbacks for non-JSON output
	var progressCallback semantic.ProgressCallback
	var uploadProgressCallback semantic.UploadProgressCallback
	var isTTY bool
	var verboseStartTime time.Time
	if !opts.jsonOutput {
		lastReported := 0
		isTTY = term.IsTerminal(int(os.Stdout.Fd()))
		if opts.verbose {
			verboseStartTime = time.Now()
		}

		// File processing progress callback
		progressCallback = func(event semantic.ProgressEvent) {
			if opts.verbose {
				// Verbose mode: single-line update with ETA
				status := "Processing"
				if event.Skipped {
					status = "Skipped"
				}

				// Truncate file path for display
				displayPath := truncatePath(event.FilePath, 50)

				// Calculate ETA after processing a few files
				var etaStr string
				if event.Current >= 3 && event.Current < event.Total {
					elapsed := time.Since(verboseStartTime)
					avgPerFile := elapsed.Seconds() / float64(event.Current)
					remaining := float64(event.Total-event.Current) * avgPerFile
					eta := time.Duration(remaining * float64(time.Second))
					etaStr = formatDuration(eta)
				}

				if isTTY {
					// Single-line update: clear line and overwrite
					if etaStr != "" {
						fmt.Printf("\r\033[K[%d/%d] %-10s ETA: %-8s %s", event.Current, event.Total, status, etaStr, displayPath)
					} else {
						fmt.Printf("\r\033[K[%d/%d] %-10s %s", event.Current, event.Total, status, displayPath)
					}
				} else {
					// Non-TTY: regular line output
					if etaStr != "" {
						fmt.Printf("[%d/%d] %s (ETA: %s) %s\n", event.Current, event.Total, status, etaStr, displayPath)
					} else {
						fmt.Printf("[%d/%d] %s %s\n", event.Current, event.Total, status, displayPath)
					}
				}
			} else {
				// Default mode: periodic summary
				if event.Current-lastReported >= progressReportInterval || event.Current == event.Total {
					fmt.Printf("  [%d/%d files] %d chunks...\n", event.Current, event.Total, event.ChunksTotal)
					lastReported = event.Current
				}
			}
		}

		// Upload/embedding phase progress callback
		uploadProgressCallback = func(event semantic.UploadProgressEvent) {
			phaseName := "Processing"
			switch event.Phase {
			case "embedding":
				phaseName = "Embedding"
			case "uploading":
				phaseName = "Uploading"
			case "indexing":
				phaseName = "Indexing"
			}

			// Format ETA
			var etaStr string
			if event.ETASeconds > 0 {
				eta := time.Duration(event.ETASeconds * float64(time.Second))
				etaStr = formatDuration(eta)
			}

			// Calculate batch percentage (more useful than chunk percentage in streaming mode)
			batchPct := 0
			if event.Total > 0 {
				batchPct = (event.Current * 100) / event.Total
			}

			if isTTY {
				// Single-line update: clear line and overwrite
				if event.ChunksTotal > 0 {
					// Known total - show chunk progress
					chunkPct := (event.ChunksUploaded * 100) / event.ChunksTotal
					if etaStr != "" {
						fmt.Printf("\r\033[K%s: [%d/%d batches] %d/%d chunks (%d%%) ETA: %s",
							phaseName, event.Current, event.Total, event.ChunksUploaded, event.ChunksTotal, chunkPct, etaStr)
					} else {
						fmt.Printf("\r\033[K%s: [%d/%d batches] %d/%d chunks (%d%%)",
							phaseName, event.Current, event.Total, event.ChunksUploaded, event.ChunksTotal, chunkPct)
					}
				} else {
					// Streaming mode - show batch progress and cumulative chunks
					if etaStr != "" {
						fmt.Printf("\r\033[K%s: [%d/%d batches] %d chunks (%d%%) ETA: %s",
							phaseName, event.Current, event.Total, event.ChunksUploaded, batchPct, etaStr)
					} else {
						fmt.Printf("\r\033[K%s: [%d/%d batches] %d chunks (%d%%)",
							phaseName, event.Current, event.Total, event.ChunksUploaded, batchPct)
					}
				}
			} else {
				// Non-TTY: log at meaningful intervals (every 10% or on completion)
				if event.Current == event.Total || batchPct%10 == 0 {
					if event.ChunksTotal > 0 {
						chunkPct := (event.ChunksUploaded * 100) / event.ChunksTotal
						if etaStr != "" {
							fmt.Printf("%s: [%d/%d batches] %d/%d chunks (%d%%) ETA: %s\n",
								phaseName, event.Current, event.Total, event.ChunksUploaded, event.ChunksTotal, chunkPct, etaStr)
						} else {
							fmt.Printf("%s: [%d/%d batches] %d/%d chunks (%d%%)\n",
								phaseName, event.Current, event.Total, event.ChunksUploaded, event.ChunksTotal, chunkPct)
						}
					} else {
						if etaStr != "" {
							fmt.Printf("%s: [%d/%d batches] %d chunks (%d%%) ETA: %s\n",
								phaseName, event.Current, event.Total, event.ChunksUploaded, batchPct, etaStr)
						} else {
							fmt.Printf("%s: [%d/%d batches] %d chunks (%d%%)\n",
								phaseName, event.Current, event.Total, event.ChunksUploaded, batchPct)
						}
					}
				}
			}
		}
	}

	result, err := mgr.Index(ctx, absPath, semantic.IndexOptions{
		Includes:         opts.includes,
		Excludes:         opts.excludes,
		ExcludeTests:     opts.excludeTests,
		Force:            opts.force,
		OnProgress:       progressCallback,
		OnUploadProgress: uploadProgressCallback,
		BatchSize:        opts.batchSize,
		Parallel:         opts.parallel,
		EmbedBatchSize:   opts.embedBatchSize,
	})

	// Print final newline after TTY progress
	if !opts.jsonOutput && isTTY {
		fmt.Println()
	}

	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	// Run calibration unless skipped
	var calibrationMeta *semantic.CalibrationMetadata
	var calibrationErr error
	if !opts.skipCalibration {
		calibrationMeta, calibrationErr = runCalibration(ctx, storage, embedder, opts.recalibrate, opts.jsonOutput)
	}

	// Output results
	if opts.jsonOutput {
		type jsonResult struct {
			*semantic.IndexResult
			Calibration *semantic.CalibrationMetadata `json:"calibration,omitempty"`
		}
		out := jsonResult{
			IndexResult: result,
			Calibration: calibrationMeta,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("Indexed %d files, created %d chunks\n", result.FilesProcessed, result.ChunksCreated)
	if result.FilesUnchanged > 0 {
		fmt.Printf("Skipped %d unchanged files (already indexed)\n", result.FilesUnchanged)
	}
	if result.FilesSkipped > 0 {
		fmt.Printf("Skipped %d files with errors\n", result.FilesSkipped)
	}
	if len(result.Errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}

	// Display calibration result
	if calibrationErr != nil {
		fmt.Printf("\nCalibration warning: %v\n", calibrationErr)
	} else if calibrationMeta != nil {
		fmt.Printf("\nCalibration: model=%s, thresholds high=%.4f/medium=%.4f/low=%.4f\n",
			calibrationMeta.EmbeddingModel,
			calibrationMeta.HighThreshold,
			calibrationMeta.MediumThreshold,
			calibrationMeta.LowThreshold)
	} else if opts.skipCalibration {
		fmt.Println("\nCalibration: skipped (--skip-calibration)")
	}

	return nil
}

// runCalibration handles the calibration workflow for the index command.
// It checks for existing calibration, runs calibration if needed, and stores results.
func runCalibration(ctx context.Context, storage semantic.Storage, embedder semantic.EmbedderInterface, forceRecalibrate, jsonOutput bool) (*semantic.CalibrationMetadata, error) {
	// Check for existing calibration
	existing, err := storage.GetCalibrationMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing calibration: %w", err)
	}

	// Skip if calibration exists and not forcing recalibration
	if existing != nil && !forceRecalibrate {
		if !jsonOutput {
			fmt.Printf("\nUsing existing calibration (model=%s, date=%s)\n",
				existing.EmbeddingModel,
				existing.CalibrationDate.Format("2006-01-02"))
		}
		return existing, nil
	}

	if !jsonOutput {
		if forceRecalibrate {
			fmt.Println("\nRecalibrating score thresholds...")
		} else {
			fmt.Println("\nRunning initial calibration...")
		}
	}

	// Run calibration
	modelName := embedder.Model()
	meta, err := semantic.RunCalibration(ctx, storage, embedder, modelName)
	if err != nil {
		// Calibration failure is a warning, not a fatal error
		return nil, err
	}

	// Store calibration results
	if err := storage.SetCalibrationMetadata(ctx, meta); err != nil {
		return nil, fmt.Errorf("failed to store calibration: %w", err)
	}

	return meta, nil
}

func resolveIndexPath(rootPath string) string {
	// If custom index dir specified
	if indexDir != "" && indexDir != ".index" {
		return filepath.Join(indexDir, "semantic.db")
	}

	// Try git root first
	gitRoot, err := findGitRootFrom(rootPath)
	if err == nil {
		return filepath.Join(gitRoot, ".index", "semantic.db")
	}

	// Fall back to the indexed directory
	return filepath.Join(rootPath, ".index", "semantic.db")
}

func findGitRootFrom(startPath string) (string, error) {
	dir := startPath
	// Limit traversal depth to prevent infinite loops on unusual filesystems
	const maxDepth = 256
	for i := 0; i < maxDepth; i++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a git repository")
		}
		dir = parent
	}
	return "", fmt.Errorf("not in a git repository (max depth %d exceeded)", maxDepth)
}

// createStorage creates a storage backend based on the --storage flag
func createStorage(indexPath string, embeddingDim int) (semantic.Storage, error) {
	switch storageType {
	case "qdrant":
		// Use Qdrant storage with resolved collection name
		collection := resolveCollectionName()
		config := semantic.QdrantConfig{
			APIKey:         strings.TrimSpace(os.Getenv("QDRANT_API_KEY")),
			URL:            strings.TrimSpace(os.Getenv("QDRANT_API_URL")),
			CollectionName: collection,
			EmbeddingDim:   embeddingDim,
			FTSDataDir:     filepath.Dir(indexPath),
		}
		return semantic.NewQdrantStorage(config)
	case "sqlite", "":
		// Default to SQLite
		return semantic.NewSQLiteStorage(indexPath, embeddingDim)
	default:
		return nil, fmt.Errorf("unknown storage type: %s (use 'sqlite' or 'qdrant')", storageType)
	}
}

// formatDuration formats a duration for display (e.g., "2m 30s", "45s")
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// truncatePath shortens a path to fit within maxLen characters (rune-safe for unicode paths)
func truncatePath(path string, maxLen int) string {
	runes := []rune(path)
	if len(runes) <= maxLen {
		return path
	}
	// Show beginning and end with ellipsis
	if maxLen < 10 {
		return string(runes[:maxLen])
	}
	half := (maxLen - 3) / 2
	return string(runes[:half]) + "..." + string(runes[len(runes)-half:])
}

// RegisterAllChunkers registers all supported language chunkers with the factory.
// This is a shared function used by both index and index-update commands.
func RegisterAllChunkers(factory *semantic.ChunkerFactory) {
	// Go chunker
	factory.Register("go", semantic.NewGoChunker())

	// JS/TS chunker
	jsChunker := semantic.NewJSChunker()
	for _, ext := range jsChunker.SupportedExtensions() {
		factory.Register(ext, jsChunker)
	}

	// Python chunker
	pyChunker := semantic.NewPythonChunker()
	for _, ext := range pyChunker.SupportedExtensions() {
		factory.Register(ext, pyChunker)
	}

	// PHP chunker
	phpChunker := semantic.NewPHPChunker()
	for _, ext := range phpChunker.SupportedExtensions() {
		factory.Register(ext, phpChunker)
	}

	// Rust chunker
	rustChunker := semantic.NewRustChunker()
	for _, ext := range rustChunker.SupportedExtensions() {
		factory.Register(ext, rustChunker)
	}

	// Markdown chunker for documentation files
	mdChunker := semantic.NewMarkdownChunker(markdownMaxChunkSize)
	for _, ext := range mdChunker.SupportedExtensions() {
		factory.Register(ext, mdChunker)
	}

	// HTML chunker for HTML documentation
	htmlChunker := semantic.NewHTMLChunker(htmlMaxChunkSize)
	for _, ext := range htmlChunker.SupportedExtensions() {
		factory.Register(ext, htmlChunker)
	}

	// Generic chunker for other file types
	generic := semantic.NewGenericChunker(genericMaxChunkSize)
	for _, ext := range generic.SupportedExtensions() {
		factory.Register(ext, generic)
	}
}

// filterExcludedPaths removes paths that contain excluded directories
// This is needed when path expansion (filepath.Glob) matches patterns like "docs*"
// and expands to include excluded paths like ".stryker-tmp/docs/"
func filterExcludedPaths(paths []string, excludes []string, excludeTests bool) []string {
	if len(excludes) == 0 && !excludeTests {
		return paths
	}

	// Add test exclude patterns if needed
	allExcludes := make([]string, 0, len(excludes)+10)
	allExcludes = append(allExcludes, excludes...)
	if excludeTests {
		allExcludes = append(allExcludes, semantic.TestDirPatterns...)
	}

	// Normalize paths for comparison (use forward slashes)
	var filtered []string
	for _, p := range paths {
		// Convert path to use forward slashes for consistent matching
		normalizedPath := filepath.ToSlash(p)

		// Check if any exclude pattern matches this path
		excluded := false
		for _, exc := range allExcludes {
			// Simple directory name check: if exclude is a simple pattern without special chars
			// check if it appears anywhere in the path as a directory component
			if !strings.ContainsAny(exc, "*?[") {
				// Check if exclude appears as a directory component in the path
				pathParts := strings.Split(normalizedPath, "/")
				for _, part := range pathParts {
					if part == exc {
						excluded = true
						break
					}
				}
				if excluded {
					break
				}
			}
			// Check using filepath.Match or substring
			if strings.Contains(normalizedPath, exc) {
				excluded = true
				break
			}
			// Check with glob-like matching
			if matched, _ := filepath.Match(exc, filepath.Base(p)); matched {
				excluded = true
				break
			}
		}

		if !excluded {
			filtered = append(filtered, p)
		}
	}

	return filtered
}
