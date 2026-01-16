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

func indexUpdateCmd() *cobra.Command {
	var (
		includes   []string
		excludes   []string
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "index-update [path]",
		Short: "Incrementally update the semantic index",
		Long: `Update the semantic index with changes since last indexing.
Only processes new or modified files.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			return runIndexUpdate(cmd.Context(), path, updateOpts{
				includes:   includes,
				excludes:   excludes,
				jsonOutput: jsonOutput,
			})
		},
	}

	cmd.Flags().StringSliceVarP(&includes, "include", "i", nil, "Glob patterns to include")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{"vendor", "node_modules", ".git"}, "Directories to exclude")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

type updateOpts struct {
	includes   []string
	excludes   []string
	jsonOutput bool
}

func runIndexUpdate(ctx context.Context, path string, opts updateOpts) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Find index path (only needed for SQLite)
	indexPath := ""
	if storageType != "qdrant" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic index' first")
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
		testEmbed, err := embedder.Embed(ctx, "test")
		if err != nil {
			return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
		}
		embeddingDim = len(testEmbed)
	}

	// Open storage based on --storage flag
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
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

	// Register Markdown chunker for documentation files
	mdChunker := semantic.NewMarkdownChunker(4000)
	for _, ext := range mdChunker.SupportedExtensions() {
		factory.Register(ext, mdChunker)
	}

	// Register HTML chunker for HTML documentation
	htmlChunker := semantic.NewHTMLChunker(4000)
	for _, ext := range htmlChunker.SupportedExtensions() {
		factory.Register(ext, htmlChunker)
	}

	// Register generic chunker for other file types
	generic := semantic.NewGenericChunker(2000)
	for _, ext := range generic.SupportedExtensions() {
		factory.Register(ext, generic)
	}

	// Create index manager
	mgr := semantic.NewIndexManager(storage, embedder, factory)

	if !opts.jsonOutput {
		fmt.Printf("Updating index for %s...\n", absPath)
	}

	// Run update
	result, err := mgr.Update(ctx, absPath, semantic.UpdateOptions{
		Includes: opts.includes,
		Excludes: opts.excludes,
	})
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	if opts.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Updated %d files, removed %d files\n", result.FilesUpdated, result.FilesRemoved)
	fmt.Printf("Created %d chunks, removed %d chunks\n", result.ChunksCreated, result.ChunksRemoved)

	return nil
}
