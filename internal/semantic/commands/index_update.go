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
		noGit      bool
		since      string
	)

	cmd := &cobra.Command{
		Use:   "index-update [path]",
		Short: "Incrementally update the semantic index",
		Long: `Update the semantic index with changes since last indexing.

By default, uses git to detect changed files (fast path). Falls back to
full directory scan if not in a git repo or if --no-git is specified.

Use --since to specify a custom git ref (default: HEAD~1 for post-commit hooks).`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			// Expand path with glob support (e.g., "docs*/", "path/to/docs*")
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
			matches = filterExcludedPaths(matches, excludes, false)

			// Update each matched path
			for _, match := range matches {
				if err := runIndexUpdate(cmd.Context(), match, updateOpts{
					includes:   includes,
					excludes:   excludes,
					jsonOutput: jsonOutput,
					noGit:      noGit,
					since:      since,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&includes, "include", "i", nil, "Glob patterns to include")
	cmd.Flags().StringSliceVarP(&excludes, "exclude", "e", []string{"vendor", "node_modules", ".git"}, "Directories to exclude")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&noGit, "no-git", false, "Force full directory scan instead of git-based detection")
	cmd.Flags().StringVar(&since, "since", "HEAD~1", "Git ref to diff against (default: HEAD~1)")

	return cmd
}

type updateOpts struct {
	includes   []string
	excludes   []string
	jsonOutput bool
	noGit      bool
	since      string
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

	// Acquire cross-process lock (non-blocking for incremental update)
	// If another index/update is running, skip silently — it will pick up our changes
	idxLock := semantic.NewIndexLock(indexPath, storageType, resolveCollectionName())
	locked, err := idxLock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to check index lock: %w", err)
	}
	if !locked {
		if !opts.jsonOutput {
			fmt.Println("Another index operation is in progress, skipping update")
		}
		return nil
	}
	defer idxLock.Unlock()

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

	// Create chunker factory with all language support
	factory := semantic.NewChunkerFactory()
	RegisterAllChunkers(factory)

	// Create index manager
	mgr := semantic.NewIndexManager(storage, embedder, factory)

	// Resolve domain from --profile flag (defaults to "code" if empty)
	domain := profile
	if domain == "" {
		domain = "code"
	}

	updateOpts := semantic.UpdateOptions{
		Includes: opts.includes,
		Excludes: opts.excludes,
		Domain:   domain,
	}

	// Determine update mode: git-aware (default) or full scan
	useGit := !opts.noGit && semantic.IsGitRepo(absPath)

	if useGit {
		// Git-aware mode: only process files changed since the given ref
		gitRoot, err := semantic.GitRepoRoot(absPath)
		if err != nil {
			// Fall back to full scan if we can't determine git root
			useGit = false
		} else {
			if !opts.jsonOutput {
				fmt.Printf("Updating index for %s (git mode, since %s)...\n", absPath, opts.since)
			}

			result, err := mgr.UpdateGit(ctx, gitRoot, opts.since, updateOpts)
			if err != nil {
				return fmt.Errorf("update failed: %w", err)
			}
			result.Mode = "git"

			return reportUpdateResult(result, opts.jsonOutput)
		}
	}

	// Full scan mode
	if !opts.jsonOutput {
		fmt.Printf("Updating index for %s (full scan)...\n", absPath)
	}

	result, err := mgr.Update(ctx, absPath, updateOpts)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	result.Mode = "full"

	return reportUpdateResult(result, opts.jsonOutput)
}

func reportUpdateResult(result *semantic.UpdateResult, jsonOutput bool) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Updated %d files, removed %d files (%s mode)\n", result.FilesUpdated, result.FilesRemoved, result.Mode)
	fmt.Printf("Created %d chunks, removed %d chunks\n", result.ChunksCreated, result.ChunksRemoved)
	return nil
}
