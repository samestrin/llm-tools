package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

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
	if storageType == "qdrant" {
		embedder, err := createEmbedder()
		if err != nil {
			return fmt.Errorf("failed to create embedder: %w", err)
		}
		testEmbed, err := embedder.Embed(ctx, "test")
		if err != nil {
			return fmt.Errorf("failed to probe embedder for dimensions: %w", err)
		}
		embeddingDim = len(testEmbed)
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

	// Get calibration metadata
	calibration, _ := storage.GetCalibrationMetadata(ctx)

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		result := map[string]interface{}{
			"indexed":       true,
			"storage":       storageType,
			"files_indexed": stats.FilesIndexed,
			"chunks_total":  stats.ChunksTotal,
			"last_updated":  stats.LastUpdated,
		}
		if storageType == "qdrant" {
			result["collection"] = resolveCollectionName()
		} else {
			result["path"] = indexPath
		}
		if calibration != nil {
			result["calibration"] = calibration
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
