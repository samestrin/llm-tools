package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/samestrin/llm-tools/internal/semantic/config"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var (
		topK          int
		threshold     float64
		typeFilter    string
		pathFilter    string
		jsonOutput    bool
		minOutput     bool
		hybrid        bool
		fusionK       int
		fusionAlpha   float64
		recencyBoost  bool
		recencyFactor float64
		recencyDecay  float64
		profiles      []string // Multi-profile search
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the semantic index",
		Long: `Search the semantic code index using natural language queries.
Returns ranked results based on semantic similarity.

With --hybrid, performs combined dense (vector) and lexical (FTS5) search
using Reciprocal Rank Fusion (RRF) for improved recall.

With --profiles, searches across multiple collections (e.g., code,docs) and
merges results sorted by score. Each result includes its source profile.`,
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

			// Validate hybrid parameters
			if hybrid {
				if fusionK <= 0 {
					return fmt.Errorf("fusion-k must be a positive integer, got: %d", fusionK)
				}
				if fusionAlpha < 0.0 || fusionAlpha > 1.0 {
					return fmt.Errorf("fusion-alpha must be between 0.0 and 1.0, got: %f", fusionAlpha)
				}
			}

			// Validate recency parameters
			if recencyBoost {
				if recencyFactor < 0 {
					return fmt.Errorf("recency-factor must be >= 0, got: %f", recencyFactor)
				}
				if recencyDecay <= 0 {
					return fmt.Errorf("recency-decay must be > 0, got: %f", recencyDecay)
				}
			}

			// Validate profile names
			for _, p := range profiles {
				if !config.IsValidProfile(p) {
					return fmt.Errorf("invalid profile: %s (valid: %s)", p, strings.Join(config.ValidProfiles(), ", "))
				}
			}

			return runSearch(cmd.Context(), query, searchOpts{
				topK:          topK,
				threshold:     float32(threshold),
				typeFilter:    typeFilter,
				pathFilter:    pathFilter,
				jsonOutput:    jsonOutput,
				minOutput:     minOutput,
				hybrid:        hybrid,
				fusionK:       fusionK,
				fusionAlpha:   fusionAlpha,
				recencyBoost:  recencyBoost,
				recencyFactor: recencyFactor,
				recencyDecay:  recencyDecay,
				profiles:      profiles,
			})
		},
	}

	cmd.Flags().IntVarP(&topK, "top", "n", 10, "Number of results to return")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 0.0, "Minimum similarity score (0.0-1.0)")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by chunk type (function, method, struct, interface)")
	cmd.Flags().StringVarP(&pathFilter, "path", "p", "", "Filter by path prefix")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Output minimal JSON format")
	cmd.Flags().BoolVar(&hybrid, "hybrid", false, "Enable hybrid search (dense + lexical with RRF fusion)")
	cmd.Flags().IntVar(&fusionK, "fusion-k", 60, "RRF fusion k parameter (higher = smoother ranking)")
	cmd.Flags().Float64Var(&fusionAlpha, "fusion-alpha", 0.7, "Fusion weight: 1.0 = dense only, 0.0 = lexical only")
	cmd.Flags().BoolVar(&recencyBoost, "recency-boost", false, "Enable recency boost (recently modified files ranked higher)")
	cmd.Flags().Float64Var(&recencyFactor, "recency-factor", 0.5, "Recency boost factor (max boost = 1+factor)")
	cmd.Flags().Float64Var(&recencyDecay, "recency-decay", 7, "Recency half-life in days (higher = slower decay)")
	cmd.Flags().StringSliceVar(&profiles, "profiles", nil, "Profiles to search across (comma-separated, e.g., code,docs)")

	return cmd
}

type searchOpts struct {
	topK          int
	threshold     float32
	typeFilter    string
	pathFilter    string
	jsonOutput    bool
	minOutput     bool
	hybrid        bool
	fusionK       int
	fusionAlpha   float64
	recencyBoost  bool
	recencyFactor float64
	recencyDecay  float64
	profiles      []string
}

func runSearch(ctx context.Context, query string, opts searchOpts) error {
	// Initialize common search components
	components, cleanup, err := initSearchComponents(ctx, createStorage)
	if err != nil {
		return err
	}
	defer cleanup()

	// Create searcher
	searcher := semantic.NewSearcher(components.Storage, components.Embedder)

	// Perform search (hybrid or dense-only)
	var results []semantic.SearchResult
	searchOpts := semantic.SearchOptions{
		TopK:       opts.topK,
		Threshold:  opts.threshold,
		Type:       opts.typeFilter,
		PathFilter: opts.pathFilter,
		Profiles:   opts.profiles,
	}

	if opts.hybrid {
		results, err = searcher.HybridSearch(ctx, query, semantic.HybridSearchOptions{
			SearchOptions: searchOpts,
			FusionK:       opts.fusionK,
			FusionAlpha:   opts.fusionAlpha,
		})
	} else {
		results, err = searcher.Search(ctx, query, searchOpts)
	}
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
			// Include relevance if set
			if r.Relevance != "" {
				minResults[i]["r"] = r.Relevance
			}
			// Include preview if set
			if r.Preview != "" {
				minResults[i]["pr"] = r.Preview
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
		// Format relevance label
		relevanceLabel := ""
		if r.Relevance != "" {
			relevanceLabel = fmt.Sprintf(" [%s]", strings.ToUpper(r.Relevance))
		}

		fmt.Printf("%d. %s:%d - %s (%s)%s\n", i+1, r.Chunk.FilePath, r.Chunk.StartLine, r.Chunk.Name, r.Chunk.Type, relevanceLabel)
		fmt.Printf("   Score: %.4f\n", r.Score)

		// Show preview if available, otherwise fall back to signature
		if r.Preview != "" {
			fmt.Printf("   %s\n", r.Preview)
		} else if r.Chunk.Signature != "" {
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
