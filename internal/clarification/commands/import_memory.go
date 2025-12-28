package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/spf13/cobra"
)

var (
	importSource string
	importTarget string
	importMode   string
	importQuiet  bool
)

// NewImportMemoryCmd creates a new import-memory command.
func NewImportMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-memory",
		Short: "Import clarification data from YAML to storage",
		Long: `Import clarification data from a YAML file to any supported storage format
(SQLite or YAML). Supports different import modes for handling existing data.

Import modes:
  append    - Add new entries, skip existing (default)
  overwrite - Replace all data with imported entries
  merge     - Update existing entries, add new ones`,
		RunE: runImportMemory,
	}

	cmd.Flags().StringVarP(&importSource, "source", "s", "", "Source YAML file (required)")
	cmd.Flags().StringVarP(&importTarget, "target", "t", "", "Target storage file (required)")
	cmd.Flags().StringVarP(&importMode, "mode", "m", "append", "Import mode: append, overwrite, merge")
	cmd.Flags().BoolVarP(&importQuiet, "quiet", "q", false, "Suppress output")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("target")

	return cmd
}

var importMemoryCmd = NewImportMemoryCmd()

func init() {
	rootCmd.AddCommand(importMemoryCmd)
}

func runImportMemory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse import mode
	mode, err := parseImportMode(importMode)
	if err != nil {
		return err
	}

	// Open source storage (YAML)
	sourceStore, err := storage.NewStorage(ctx, importSource)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceStore.Close()

	// Export all entries from source
	entries, err := sourceStore.Export(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to read source entries: %w", err)
	}

	if !importQuiet {
		fmt.Fprintf(cmd.OutOrStdout(), "Found %d entries to import\n", len(entries))
	}

	// Open target storage
	targetStore, err := storage.NewStorage(ctx, importTarget)
	if err != nil {
		return fmt.Errorf("failed to open target storage: %w", err)
	}
	defer targetStore.Close()

	// Import entries
	result, err := targetStore.Import(ctx, entries, mode)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	if !importQuiet {
		fmt.Fprintf(cmd.OutOrStdout(), "Import complete:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Processed: %d\n", result.Processed)
		fmt.Fprintf(cmd.OutOrStdout(), "  Created:   %d\n", result.Created)
		fmt.Fprintf(cmd.OutOrStdout(), "  Updated:   %d\n", result.Updated)
		fmt.Fprintf(cmd.OutOrStdout(), "  Skipped:   %d\n", result.Skipped)
		if len(result.Errors) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Errors:    %d\n", len(result.Errors))
		}
	}

	return nil
}

func parseImportMode(mode string) (storage.ImportMode, error) {
	switch strings.ToLower(mode) {
	case "append":
		return storage.ImportModeAppend, nil
	case "overwrite":
		return storage.ImportModeOverwrite, nil
	case "merge":
		return storage.ImportModeMerge, nil
	default:
		return 0, fmt.Errorf("invalid import mode: %s (use: append, overwrite, merge)", mode)
	}
}
