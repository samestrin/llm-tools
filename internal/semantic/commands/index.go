package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
)

func indexCmd() *cobra.Command {
	var (
		includes   []string
		excludes   []string
		force      bool
		jsonOutput bool
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
			})
		},
	}

	cmd.Flags().StringSliceVarP(&includes, "include", "i", nil, "Glob patterns to include (e.g., '*.go')")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{"vendor", "node_modules", ".git"}, "Directories to exclude")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force re-index all files")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")

	return cmd
}

type indexOpts struct {
	includes   []string
	excludes   []string
	force      bool
	jsonOutput bool
}

func runIndex(ctx context.Context, path string, opts indexOpts) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Ensure index directory exists
	indexPath := resolveIndexPath(absPath)
	indexDir := filepath.Dir(indexPath)
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// Create storage based on storage type flag
	storage, err := createStorage(indexPath, 0)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Create embedder based on --embedder flag
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

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

	result, err := mgr.Index(ctx, absPath, semantic.IndexOptions{
		Includes: opts.includes,
		Excludes: opts.excludes,
		Force:    opts.force,
	})
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
		// Use Qdrant cloud storage
		return semantic.NewQdrantStorageFromEnv(embeddingDim)
	case "sqlite", "":
		// Default to SQLite
		return semantic.NewSQLiteStorage(indexPath, embeddingDim)
	default:
		return nil, fmt.Errorf("unknown storage type: %s (use 'sqlite' or 'qdrant')", storageType)
	}
}
