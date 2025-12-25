package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/clarification/tracking"

	"github.com/spf13/cobra"
)

var initTrackingCmd = &cobra.Command{
	Use:   "init-tracking",
	Short: "Initialize a new tracking file",
	Long:  `Initialize a new clarification tracking file with the proper schema.`,
	RunE:  runInitTracking,
}

var (
	initOutput string
	initForce  bool
)

func init() {
	rootCmd.AddCommand(initTrackingCmd)
	initTrackingCmd.Flags().StringVarP(&initOutput, "output", "o", "", "Output file path (required)")
	initTrackingCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite if exists")
	initTrackingCmd.MarkFlagRequired("output")
}

// InitResult represents the JSON output of the init-tracking command.
type InitResult struct {
	Status  string `json:"status"`
	File    string `json:"file"`
	Message string `json:"message"`
}

func runInitTracking(cmd *cobra.Command, args []string) error {
	// Validate output path is not a directory
	if strings.HasSuffix(initOutput, "/") || strings.HasSuffix(initOutput, string(os.PathSeparator)) {
		return fmt.Errorf("output path must be a file, not a directory")
	}

	// Check if file already exists
	if tracking.FileExists(initOutput) {
		if !initForce {
			return fmt.Errorf("tracking file already exists: %s (use --force to overwrite)", initOutput)
		}
		// With --force, backup existing file
		timestamp := time.Now().Format("20060102-150405")
		backupPath := fmt.Sprintf("%s.backup-%s", initOutput, timestamp)
		if err := os.Rename(initOutput, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing file: %w", err)
		}
	}

	// Ensure parent directory exists
	dir := filepath.Dir(initOutput)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create new tracking file
	today := time.Now().Format("2006-01-02")
	tf := tracking.NewTrackingFile(today)

	if err := tracking.SaveTrackingFile(tf, initOutput); err != nil {
		return fmt.Errorf("failed to save tracking file: %w", err)
	}

	// Output JSON result
	result := InitResult{
		Status:  "created",
		File:    initOutput,
		Message: "Tracking file initialized successfully",
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}
