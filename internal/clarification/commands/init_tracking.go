package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/pkg/output"

	"github.com/spf13/cobra"
)

var initTrackingCmd = &cobra.Command{
	Use:   "init-tracking",
	Short: "Initialize a new tracking file",
	Long:  `Initialize a new clarification tracking file with the proper schema.`,
	RunE:  runInitTracking,
}

var (
	initOutput  string
	initForce   bool
	initJSON    bool
	initMinimal bool
)

func init() {
	rootCmd.AddCommand(initTrackingCmd)
	initTrackingCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initTrackingCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	initTrackingCmd.Flags().BoolVar(&initJSON, "json", false, "Output as JSON")
	initTrackingCmd.Flags().BoolVar(&initMinimal, "min", false, "Output in minimal/token-optimized format")
	initTrackingCmd.MarkFlagRequired("output")
}

// InitResult represents the JSON output of the init-tracking command.
type InitResult struct {
	Status  string `json:"status"`
	File    string `json:"file"`
	Message string `json:"message"`
}

func runInitTracking(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Apply --db override if set
	outputPath := GetDBPath(initOutput)

	// Validate output path is not a directory
	if strings.HasSuffix(outputPath, "/") || strings.HasSuffix(outputPath, string(os.PathSeparator)) {
		return fmt.Errorf("output path must be a file, not a directory")
	}

	// Validate storage type is supported
	if _, err := storage.DetectStorageType(outputPath); err != nil {
		return fmt.Errorf("unsupported file extension: %w", err)
	}

	// Check if file already exists
	if FileOrDBExists(outputPath) {
		if !initForce {
			return fmt.Errorf("storage file already exists: %s (use --force to overwrite)", outputPath)
		}
		// With --force, backup existing file
		timestamp := time.Now().Format("20060102-150405")
		backupPath := fmt.Sprintf("%s.backup-%s", outputPath, timestamp)
		if err := os.Rename(outputPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing file: %w", err)
		}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create storage (this will initialize the database/file)
	store, err := storage.NewStorage(ctx, outputPath)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer store.Close()

	// Output result
	result := InitResult{
		Status:  "created",
		File:    outputPath,
		Message: "Storage initialized successfully",
	}

	formatter := output.New(initJSON, initMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(InitResult)
		fmt.Fprintf(w, "STATUS: %s\n", r.Status)
		fmt.Fprintf(w, "FILE: %s\n", r.File)
		fmt.Fprintf(w, "MESSAGE: %s\n", r.Message)
	})
}
