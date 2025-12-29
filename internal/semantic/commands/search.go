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

func searchCmd() *cobra.Command {
	var (
		topK       int
		threshold  float64
		typeFilter string
		pathFilter string
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the semantic index",
		Long: `Search the semantic code index using natural language queries.
Returns ranked results based on semantic similarity.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if len(args) > 1 {
				// Join multiple args as the query
				query = ""
				for _, arg := range args {
					query += arg + " "
				}
			}

			return runSearch(cmd.Context(), query, searchOpts{
				topK:       topK,
				threshold:  float32(threshold),
				typeFilter: typeFilter,
				pathFilter: pathFilter,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().IntVarP(&topK, "top", "n", 10, "Number of results to return")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 0.0, "Minimum similarity score (0.0-1.0)")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by chunk type (function, method, struct, interface)")
	cmd.Flags().StringVarP(&pathFilter, "path", "p", "", "Filter by path prefix")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Output minimal JSON format")

	return cmd
}

type searchOpts struct {
	topK       int
	threshold  float32
	typeFilter string
	pathFilter string
	jsonOutput bool
	minOutput  bool
}

func runSearch(ctx context.Context, query string, opts searchOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic index' first")
		}
	}

	// Open storage
	storage, err := createStorage(indexPath, 0)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}
	defer storage.Close()

	// Create embedder based on --embedder flag
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	// Create searcher
	searcher := semantic.NewSearcher(storage, embedder)

	// Perform search
	results, err := searcher.Search(ctx, query, semantic.SearchOptions{
		TopK:       opts.topK,
		Threshold:  opts.threshold,
		Type:       opts.typeFilter,
		PathFilter: opts.pathFilter,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Output results
	if opts.jsonOutput || opts.minOutput {
		return outputJSON(results, opts.minOutput)
	}
	return outputText(results)
}

func outputJSON(results []semantic.SearchResult, minimal bool) error {
	if minimal {
		minResults := make([]map[string]interface{}, len(results))
		for i, r := range results {
			minResults[i] = map[string]interface{}{
				"file":  r.Chunk.FilePath,
				"name":  r.Chunk.Name,
				"line":  r.Chunk.StartLine,
				"score": r.Score,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(minResults)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func outputText(results []semantic.SearchResult) error {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	for i, r := range results {
		fmt.Printf("%d. %s:%d - %s (%s)\n", i+1, r.Chunk.FilePath, r.Chunk.StartLine, r.Chunk.Name, r.Chunk.Type)
		fmt.Printf("   Score: %.4f\n", r.Score)
		if r.Chunk.Signature != "" {
			fmt.Printf("   %s\n", r.Chunk.Signature)
		}
		fmt.Println()
	}

	return nil
}

func findIndexPath() string {
	// Try specified index directory
	if indexDir != "" {
		path := filepath.Join(indexDir, "semantic.db")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Try .llm-index in current directory
	path := filepath.Join(".llm-index", "semantic.db")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Try to find git root and check there
	if gitRoot, err := findGitRoot(); err == nil {
		path := filepath.Join(gitRoot, ".llm-index", "semantic.db")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func findGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

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
