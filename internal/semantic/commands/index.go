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

func indexCmd() *cobra.Command {
	var (
		includes   []string
		excludes   []string
		force      bool
		jsonOutput bool
		verbose    bool
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
				includes:   includes,
				excludes:   excludes,
				force:      force,
				jsonOutput: jsonOutput,
				verbose:    verbose,
			})
		},
	}

	cmd.Flags().StringSliceVarP(&includes, "include", "i", nil, "Glob patterns to include (e.g., '*.go')")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{"vendor", "node_modules", ".git"}, "Directories to exclude")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force re-index all files")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show per-file progress instead of periodic summary")

	return cmd
}

type indexOpts struct {
	includes   []string
	excludes   []string
	force      bool
	jsonOutput bool
	verbose    bool
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
		if err := os.MkdirAll(indexDir, 0755); err != nil {
			return fmt.Errorf("failed to create index directory: %w", err)
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

	// Create chunker factory with language support
	factory := semantic.NewChunkerFactory()
	factory.Register("go", semantic.NewGoChunker())

	// Register JS/TS chunker
	jsChunker := semantic.NewJSChunker()
	for _, ext := range jsChunker.SupportedExtensions() {
		factory.Register(ext, jsChunker)
	}

	// Register Python chunker
	pyChunker := semantic.NewPythonChunker()
	for _, ext := range pyChunker.SupportedExtensions() {
		factory.Register(ext, pyChunker)
	}

	// Register PHP chunker
	phpChunker := semantic.NewPHPChunker()
	for _, ext := range phpChunker.SupportedExtensions() {
		factory.Register(ext, phpChunker)
	}

	// Register Rust chunker
	rustChunker := semantic.NewRustChunker()
	for _, ext := range rustChunker.SupportedExtensions() {
		factory.Register(ext, rustChunker)
	}

	// Register generic chunker for other file types
	generic := semantic.NewGenericChunker(2000)
	for _, ext := range generic.SupportedExtensions() {
		factory.Register(ext, generic)
	}

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
				// Default mode: periodic summary every 100 files
				if event.Current-lastReported >= 100 || event.Current == event.Total {
					fmt.Printf("  [%d/%d files] %d chunks...\n", event.Current, event.Total, event.ChunksTotal)
					lastReported = event.Current
				}
			}
		}
	}

	result, err := mgr.Index(ctx, absPath, semantic.IndexOptions{
		Includes:   opts.includes,
		Excludes:   opts.excludes,
		Force:      opts.force,
		OnProgress: progressCallback,
	})

	// Print final newline after verbose TTY progress
	if opts.verbose && !opts.jsonOutput && verboseIsTTY {
		fmt.Println()
	}

	if err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}

	// Output results
	if opts.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
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

	return nil
}

func resolveIndexPath(rootPath string) string {
	// If custom index dir specified
	if indexDir != "" && indexDir != ".llm-index" {
		return filepath.Join(indexDir, "semantic.db")
	}

	// Try git root first
	gitRoot, err := findGitRootFrom(rootPath)
	if err == nil {
		return filepath.Join(gitRoot, ".llm-index", "semantic.db")
	}

	// Fall back to the indexed directory
	return filepath.Join(rootPath, ".llm-index", "semantic.db")
}

func findGitRootFrom(startPath string) (string, error) {
	dir := startPath
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a git repository")
		}
		dir = parent
	}
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

// truncatePath shortens a path to fit within maxLen characters
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	// Show beginning and end with ellipsis
	if maxLen < 10 {
		return path[:maxLen]
	}
	half := (maxLen - 3) / 2
	return path[:half] + "..." + path[len(path)-half:]
}
