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

			return runIndex(cmd.Context(), path, indexOpts{
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
			})
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

	// Set up progress callback for non-JSON output
	var progressCallback semantic.ProgressCallback
	var verboseIsTTY bool
	var verboseStartTime time.Time
	if !opts.jsonOutput {
		lastReported := 0
		if opts.verbose {
			verboseIsTTY = term.IsTerminal(int(os.Stdout.Fd()))
			verboseStartTime = time.Now()
		}
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

				if verboseIsTTY {
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
	}

	result, err := mgr.Index(ctx, absPath, semantic.IndexOptions{
		Includes:     opts.includes,
		Excludes:     opts.excludes,
		ExcludeTests: opts.excludeTests,
		Force:        opts.force,
		OnProgress:   progressCallback,
		BatchSize:    opts.batchSize,
		Parallel:     opts.parallel,
	})

	// Print final newline after verbose TTY progress
	if opts.verbose && !opts.jsonOutput && verboseIsTTY {
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
