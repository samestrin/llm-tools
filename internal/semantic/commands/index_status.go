package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
)

// indexStatusJSON represents the JSON output structure for index-status command
type indexStatusJSON struct {
	Indexed      bool                          `json:"indexed"`
	Storage      string                        `json:"storage"`
	FilesIndexed int                           `json:"files_indexed"`
	ChunksTotal  int                           `json:"chunks_total"`
	LastUpdated  string                        `json:"last_updated"`
	Path         string                        `json:"path,omitempty"`        // Only for sqlite
	Collection   string                        `json:"collection,omitempty"`  // Only for qdrant
	Calibration  *semantic.CalibrationMetadata `json:"calibration,omitempty"` // Optional
}

func indexStatusCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "index-status",
		Short: "Show semantic index status",
		Long:  `Display statistics about the current semantic index.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runIndexStatus(cmd.Context(), jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func runIndexStatus(ctx context.Context, jsonOutput bool) error {
	indexPath := findIndexPath()
	if indexPath == "" && storageType != "qdrant" {
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
				"error":   "index not found",
				"indexed": false,
			})
		}
		fmt.Println("Semantic index not found.")
		fmt.Println("Run 'llm-semantic index' to create one.")
		return nil
	}

	// For Qdrant, we need to probe the embedder to get dimensions
	embeddingDim := 0
	embedderOffline := false
	if storageType == "qdrant" {
		embedder, err := createEmbedder()
		if err != nil {
			slog.Debug("embedder unavailable, trying offline mode", "error", err)
			embedderOffline = true
		} else {
			testEmbed, err := embedder.Embed(ctx, "test")
			if err != nil {
				slog.Debug("embedder probe failed, trying offline mode", "error", err)
				embedderOffline = true
			} else {
				embeddingDim = len(testEmbed)
			}
		}

		// For offline Qdrant status, we can't create the storage without dimensions
		// Report limited info instead
		if embedderOffline {
			collection := resolveCollectionName()
			if jsonOutput {
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"indexed":          "unknown",
					"storage":          "qdrant",
					"collection":       collection,
					"embedder_offline": true,
					"message":          "Embedder service unavailable. Cannot retrieve full index status.",
				})
			}
			fmt.Printf("Semantic Index Status (Limited - Embedder Offline)\n")
			fmt.Printf("==================================================\n")
			fmt.Printf("Storage:       qdrant\n")
			fmt.Printf("Collection:    %s\n", collection)
			fmt.Printf("\nEmbedder service is offline. Cannot retrieve full index status.\n")
			fmt.Printf("Start the embedder service to see complete statistics.\n")
			return nil
		}
	}

	// Open storage
	storage, err := createStorage(indexPath, embeddingDim)
	if err != nil {
		return fmt.Errorf("failed to open index: %w", err)
	}
	defer storage.Close()

	// Get stats
	stats, err := storage.Stats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Get calibration metadata (optional - may not exist)
	calibration, calErr := storage.GetCalibrationMetadata(ctx)
	if calErr != nil {
		slog.Debug("calibration metadata not available", "error", calErr)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		result := indexStatusJSON{
			Indexed:      true,
			Storage:      storageType,
			FilesIndexed: stats.FilesIndexed,
			ChunksTotal:  stats.ChunksTotal,
			LastUpdated:  stats.LastUpdated,
			Calibration:  calibration,
		}
		if storageType == "qdrant" {
			result.Collection = resolveCollectionName()
		} else {
			result.Path = indexPath
		}
		return enc.Encode(result)
	}

	fmt.Printf("Semantic Index Status\n")
	fmt.Printf("=====================\n")
	if storageType == "qdrant" {
		fmt.Printf("Storage:       qdrant\n")
		fmt.Printf("Collection:    %s\n", resolveCollectionName())
	} else {
		fmt.Printf("Index path:    %s\n", indexPath)
	}
	fmt.Printf("Files indexed: %d\n", stats.FilesIndexed)
	fmt.Printf("Total chunks:  %d\n", stats.ChunksTotal)
	fmt.Printf("Last updated:  %s\n", stats.LastUpdated)

	// Display calibration info
	fmt.Printf("\nCalibration\n")
	fmt.Printf("-----------\n")
	if calibration != nil {
		fmt.Printf("Model:         %s\n", calibration.EmbeddingModel)
		fmt.Printf("Calibrated:    %s\n", calibration.CalibrationDate.Format("2006-01-02 15:04:05"))
		fmt.Printf("Perfect match: %.4f\n", calibration.PerfectMatchScore)
		fmt.Printf("Baseline:      %.4f\n", calibration.BaselineScore)
		fmt.Printf("Score range:   %.4f\n", calibration.ScoreRange)
		fmt.Printf("Thresholds:    high=%.4f, medium=%.4f, low=%.4f\n",
			calibration.HighThreshold, calibration.MediumThreshold, calibration.LowThreshold)
	} else {
		fmt.Printf("Status:        Not performed\n")
		fmt.Printf("Run 'llm-semantic index' to calibrate\n")
	}

	return nil
}
