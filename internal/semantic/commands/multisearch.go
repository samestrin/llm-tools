package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/samestrin/llm-tools/internal/semantic/config"
	"github.com/spf13/cobra"
)

func multisearchCmd() *cobra.Command {
	var (
		topK         int
		threshold    float64
		profiles     []string
		noBoost      bool
		noDedupe     bool
		outputFormat string
		jsonOutput   bool
		minOutput    bool
	)

	cmd := &cobra.Command{
		Use:   "multisearch <query1> [query2] [query3] ...",
		Short: "Execute multiple semantic queries with deduplication and boosting",
		Long: `Execute multiple semantic queries in parallel with intelligent deduplication and multi-match boosting.

Results matching multiple queries receive boosted scores (+0.05 per additional match).
Duplicate chunks are deduplicated, keeping the highest score.

Examples:
  llm-semantic multisearch "authentication" "JWT tokens"
  llm-semantic multisearch "error handling" "logging" --profiles code,docs
  llm-semantic multisearch "api endpoint" "REST handler" --top 20 --json
  llm-semantic multisearch "test" "spec" --output by_query`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			queries := args

			// Validate queries
			if len(queries) > semantic.MaxQueries {
				return fmt.Errorf("multisearch supports up to %d queries, got: %d", semantic.MaxQueries, len(queries))
			}

			// Filter empty queries
			var filteredQueries []string
			for _, q := range queries {
				q = strings.TrimSpace(q)
				if q != "" {
					filteredQueries = append(filteredQueries, q)
				}
			}
			if len(filteredQueries) == 0 {
				return fmt.Errorf("at least one non-empty query required")
			}

			// Validate output format
			if outputFormat != "" && !semantic.IsValidOutputFormat(outputFormat) {
				return fmt.Errorf("invalid output format: %s (valid: %s)", outputFormat, strings.Join(semantic.ValidOutputFormats(), ", "))
			}

			// Validate profile names
			for _, p := range profiles {
				if !config.IsValidProfile(p) {
					return fmt.Errorf("invalid profile: %s (valid: %s)", p, strings.Join(config.ValidProfiles(), ", "))
				}
			}

			return runMultisearch(cmd.Context(), filteredQueries, multisearchOpts{
				topK:         topK,
				threshold:    float32(threshold),
				profiles:     profiles,
				noBoost:      noBoost,
				noDedupe:     noDedupe,
				outputFormat: semantic.OutputFormat(outputFormat),
				jsonOutput:   jsonOutput,
				minOutput:    minOutput,
			})
		},
	}

	cmd.Flags().IntVarP(&topK, "top", "n", 15, "Maximum number of results to return (default 15 vs search's 10 to account for multi-query result merging)")
	cmd.Flags().Float64VarP(&threshold, "threshold", "t", 0.0, "Minimum similarity score (0.0-1.0)")
	cmd.Flags().StringSliceVar(&profiles, "profiles", nil, "Profiles to search across (comma-separated, e.g., code,docs)")
	cmd.Flags().BoolVar(&noBoost, "no-boost", false, "Disable multi-match score boosting")
	cmd.Flags().BoolVar(&noDedupe, "no-dedupe", false, "Disable result deduplication")
	cmd.Flags().StringVar(&outputFormat, "output", "", "Output format: blended (default), by_query, by_collection")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Output minimal JSON format")

	return cmd
}

type multisearchOpts struct {
	topK         int
	threshold    float32
	profiles     []string
	noBoost      bool
	noDedupe     bool
	outputFormat semantic.OutputFormat
	jsonOutput   bool
	minOutput    bool
}

func runMultisearch(ctx context.Context, queries []string, opts multisearchOpts) error {
	// Initialize common search components
	components, cleanup, err := initSearchComponents(ctx, createStorage)
	if err != nil {
		return err
	}
	defer cleanup()

	// Create searcher
	searcher := semantic.NewSearcher(components.Storage, components.Embedder)

	// Build multisearch options
	boostEnabled := !opts.noBoost
	msOpts := semantic.MultisearchOptions{
		Queries:         queries,
		TopK:            opts.topK,
		Threshold:       opts.threshold,
		Profiles:        opts.profiles,
		BoostMultiMatch: &boostEnabled,
		Output:          opts.outputFormat,
	}

	// Execute multisearch
	result, err := searcher.Multisearch(ctx, msOpts)
	if err != nil {
		return fmt.Errorf("multisearch failed: %w", err)
	}

	// Apply output format if specified
	if opts.outputFormat != "" && opts.outputFormat != semantic.OutputBlended {
		result = result.FormatAs(opts.outputFormat)
	}

	// Output results
	if opts.jsonOutput || opts.minOutput {
		return outputMultisearchJSON(result, opts.minOutput)
	}
	return outputMultisearchText(result)
}

func outputMultisearchJSON(result *semantic.MultisearchResult, minimal bool) error {
	if minimal {
		// Minimal format: just essential fields
		minResult := map[string]interface{}{
			"total_queries": result.TotalQueries,
			"total_results": result.TotalResults,
		}

		if result.Results != nil {
			minResults := make([]map[string]interface{}, len(result.Results))
			for i, r := range result.Results {
				minResults[i] = map[string]interface{}{
					"file":    r.Chunk.FilePath,
					"name":    r.Chunk.Name,
					"line":    r.Chunk.StartLine,
					"score":   r.Score,
					"boosted": r.BoostedScore,
					"matches": len(r.MatchedQueries),
				}
				if r.Chunk.Domain != "" {
					minResults[i]["profile"] = r.Chunk.Domain
				}
			}
			minResult["results"] = minResults
		}

		if result.ByQuery != nil {
			minByQuery := make(map[string][]map[string]interface{})
			for query, results := range result.ByQuery {
				minByQuery[query] = make([]map[string]interface{}, len(results))
				for i, r := range results {
					minByQuery[query][i] = map[string]interface{}{
						"file":    r.Chunk.FilePath,
						"name":    r.Chunk.Name,
						"line":    r.Chunk.StartLine,
						"score":   r.Score,
						"boosted": r.BoostedScore,
					}
				}
			}
			minResult["by_query"] = minByQuery
		}

		if result.ByCollection != nil {
			minByCollection := make(map[string][]map[string]interface{})
			for collection, results := range result.ByCollection {
				minByCollection[collection] = make([]map[string]interface{}, len(results))
				for i, r := range results {
					minByCollection[collection][i] = map[string]interface{}{
						"file":    r.Chunk.FilePath,
						"name":    r.Chunk.Name,
						"line":    r.Chunk.StartLine,
						"score":   r.Score,
						"boosted": r.BoostedScore,
					}
				}
			}
			minResult["by_collection"] = minByCollection
		}

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(minResult)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func outputMultisearchText(result *semantic.MultisearchResult) error {
	fmt.Printf("Multisearch: %d queries, %d results\n\n", result.TotalQueries, result.TotalResults)

	if len(result.Results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	for i, r := range result.Results {
		// Format match info
		matchInfo := ""
		if len(r.MatchedQueries) > 1 {
			matchInfo = fmt.Sprintf(" [%d matches]", len(r.MatchedQueries))
		}

		// Format profile info
		profileInfo := ""
		if r.Chunk.Domain != "" {
			profileInfo = fmt.Sprintf(" (%s)", r.Chunk.Domain)
		}

		fmt.Printf("%d. %s:%d - %s%s%s\n", i+1, r.Chunk.FilePath, r.Chunk.StartLine, r.Chunk.Name, profileInfo, matchInfo)
		fmt.Printf("   Score: %.4f (boosted: %.4f)\n", r.Score, r.BoostedScore)

		if len(r.MatchedQueries) > 0 {
			fmt.Printf("   Matched: %s\n", strings.Join(r.MatchedQueries, ", "))
		}

		// Show preview if available
		if r.Preview != "" {
			fmt.Printf("   %s\n", r.Preview)
		} else if r.Chunk.Signature != "" {
			fmt.Printf("   %s\n", r.Chunk.Signature)
		}
		fmt.Println()
	}

	return nil
}
