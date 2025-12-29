package commands

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	importSource  string
	importTarget  string
	importMode    string
	importQuiet   bool
	importJSON    bool
	importMinimal bool
)

// ImportMemoryResult holds the import result
type ImportMemoryResult struct {
	Source    string `json:"source,omitempty"`
	Src       string `json:"src,omitempty"`
	Target    string `json:"target,omitempty"`
	Tgt       string `json:"tgt,omitempty"`
	Mode      string `json:"mode,omitempty"`
	M         string `json:"m,omitempty"`
	Processed int    `json:"processed,omitempty"`
	Pr        *int   `json:"pr,omitempty"`
	Created   int    `json:"created,omitempty"`
	Cr        *int   `json:"cr,omitempty"`
	Updated   int    `json:"updated,omitempty"`
	Upd       *int   `json:"upd,omitempty"`
	Skipped   int    `json:"skipped,omitempty"`
	Sk        *int   `json:"sk,omitempty"`
	Errors    int    `json:"errors,omitempty"`
	Er        *int   `json:"er,omitempty"`
}

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
	cmd.Flags().BoolVar(&importJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&importMinimal, "min", false, "Output in minimal/token-optimized format")
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

	// Build output result
	errorCount := len(result.Errors)
	var outputResult ImportMemoryResult
	if importMinimal {
		outputResult = ImportMemoryResult{
			Src: importSource,
			Tgt: importTarget,
			M:   importMode,
			Pr:  &result.Processed,
			Cr:  &result.Created,
			Upd: &result.Updated,
			Sk:  &result.Skipped,
			Er:  &errorCount,
		}
	} else {
		outputResult = ImportMemoryResult{
			Source:    importSource,
			Target:    importTarget,
			Mode:      importMode,
			Processed: result.Processed,
			Created:   result.Created,
			Updated:   result.Updated,
			Skipped:   result.Skipped,
			Errors:    errorCount,
		}
	}

	if importQuiet && !importJSON && !importMinimal {
		return nil
	}

	formatter := output.New(importJSON, importMinimal, cmd.OutOrStdout())
	return formatter.Print(outputResult, func(w io.Writer, data interface{}) {
		if !importQuiet {
			fmt.Fprintf(w, "Import complete:\n")
			fmt.Fprintf(w, "  Processed: %d\n", result.Processed)
			fmt.Fprintf(w, "  Created:   %d\n", result.Created)
			fmt.Fprintf(w, "  Updated:   %d\n", result.Updated)
			fmt.Fprintf(w, "  Skipped:   %d\n", result.Skipped)
			if errorCount > 0 {
				fmt.Fprintf(w, "  Errors:    %d\n", errorCount)
			}
		}
	})
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
