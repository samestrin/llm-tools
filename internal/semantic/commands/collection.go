package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	_ "github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
)

func collectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Manage semantic index collections",
		Long: `Collection commands for managing semantic index collections.
Collections allow you to organize indexed content by domain (code, docs, memory, etc.).

Commands:
  delete - Delete all chunks for a specific domain`,
	}

	cmd.AddCommand(collectionDeleteCmd())

	return cmd
}

// ===== COLLECTION DELETE COMMAND =====

func collectionDeleteCmd() *cobra.Command {
	var (
		profile    string
		force      bool
		jsonOutput bool
		minOutput  bool
	)

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete all chunks for a specific profile",
		Long: `Delete all chunks for a specific profile from the semantic index.
This removes all embedded content for the given profile (e.g., "code", "docs", "memory", "sprints").

Example:
  llm-semantic collection delete --profile memory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if profile == "" {
				return fmt.Errorf("--profile is required")
			}
			return runCollectionDelete(cmd.Context(), collectionDeleteOpts{
				domain:     profile, // domain = profile internally
				force:      force,
				jsonOutput: jsonOutput,
				minOutput:  minOutput,
			})
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile to delete (code, docs, memory, sprints)")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output format")

	cmd.MarkFlagRequired("profile")

	return cmd
}

type collectionDeleteOpts struct {
	domain     string
	force      bool
	jsonOutput bool
	minOutput  bool
}

func runCollectionDelete(ctx context.Context, opts collectionDeleteOpts) error {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("semantic index not found. Run 'llm-semantic index' first")
		}
	}

	// Create embedder for storage dimension
	embedder, err := createEmbedder()
	if err != nil {
		return fmt.Errorf("failed to create embedder: %w", err)
	}

	embeddingDim := 0
	testEmbed, err := embedder.Embed(ctx, "test")
	if err != nil {
		return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
	}
	embeddingDim = len(testEmbed)

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	// Confirm deletion
	if !opts.force {
		fmt.Printf("This will delete all chunks for domain '%s'.\n", opts.domain)
		fmt.Print("Are you sure? [y/N]: ")

		var response string
		_, err = fmt.Scanln(&response)
		// If stdin is closed/redirected, abort
		if err != nil {
			fmt.Println("Aborted (non-interactive input). Use --force to confirm automatically.")
			return nil
		}
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Delete by domain
	count, err := storage.DeleteByDomain(ctx, opts.domain)
	if err != nil {
		return fmt.Errorf("failed to delete domain: %w", err)
	}

	// Output result
	if opts.jsonOutput || opts.minOutput {
		result := map[string]interface{}{
			"status":  "deleted",
			"domain":  opts.domain,
			"deleted": count,
			"count":   count,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if count == 0 {
		fmt.Printf("No chunks found for domain '%s'\n", opts.domain)
	} else {
		fmt.Printf("Deleted %d chunk(s) for domain '%s'\n", count, opts.domain)
	}
	return nil
}
