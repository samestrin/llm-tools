package commands

import (
	"context"
	"fmt"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
	"github.com/spf13/cobra"
)

var (
	exportSource string
	exportOutput string
	exportQuiet  bool
)

// NewExportMemoryCmd creates a new export-memory command.
func NewExportMemoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export-memory",
		Short: "Export clarification data to YAML format",
		Long: `Export clarification data from any supported storage format (SQLite or YAML)
to a human-readable YAML file for editing or backup.`,
		RunE: runExportMemory,
	}

	cmd.Flags().StringVarP(&exportSource, "source", "s", "", "Source storage file (required)")
	cmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output YAML file path (required)")
	cmd.Flags().BoolVarP(&exportQuiet, "quiet", "q", false, "Suppress output")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("output")

	return cmd
}

var exportMemoryCmd = NewExportMemoryCmd()

func init() {
	rootCmd.AddCommand(exportMemoryCmd)
}

func runExportMemory(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Open source storage
	sourceStore, err := storage.NewStorage(ctx, exportSource)
	if err != nil {
		return fmt.Errorf("failed to open source storage: %w", err)
	}
	defer sourceStore.Close()

	// Export all entries
	entries, err := sourceStore.Export(ctx, storage.ListFilter{})
	if err != nil {
		return fmt.Errorf("failed to export entries: %w", err)
	}

	if !exportQuiet {
		fmt.Fprintf(cmd.OutOrStdout(), "Exporting %d entries...\n", len(entries))
	}

	// Create YAML tracking file structure
	tf := tracking.NewTrackingFile(entries[0].FirstSeen)
	if len(entries) > 0 {
		// Find the earliest first_seen date
		for _, e := range entries {
			if e.FirstSeen < tf.Created {
				tf.Created = e.FirstSeen
			}
		}
	}

	// Add entries
	for _, e := range entries {
		tf.Entries = append(tf.Entries, e)
	}

	// Save to output file
	if err := tracking.SaveTrackingFile(tf, exportOutput); err != nil {
		return fmt.Errorf("failed to save YAML file: %w", err)
	}

	if !exportQuiet {
		fmt.Fprintf(cmd.OutOrStdout(), "Successfully exported to: %s\n", exportOutput)
	}

	return nil
}
