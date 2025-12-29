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

	indexPath := findIndexPath()
	if indexPath == "" {
		return fmt.Errorf("semantic index not found. Run 'llm-semantic index' first")
	}

	// Open storage
	storage, err := semantic.NewSQLiteStorage(indexPath, 0)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}
	defer storage.Close()

	// Create embedder
	cfg := semantic.EmbedderConfig{
		APIURL: apiURL,
		Model:  model,
		APIKey: getAPIKey(),
	}
	embedder, err := semantic.NewEmbedder(cfg)
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	// Create chunker factory
	factory := semantic.NewChunkerFactory()
	factory.Register("go", semantic.NewGoChunker())

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
