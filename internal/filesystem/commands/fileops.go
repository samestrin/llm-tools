package commands

import (
	"encoding/json"
	"fmt"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addFileOpsCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(copyFileCmd())
	rootCmd.AddCommand(moveFileCmd())
	rootCmd.AddCommand(deleteFileCmd())
	rootCmd.AddCommand(batchFileOperationsCmd())
}

func copyFileCmd() *cobra.Command {
	var source, destination string

	cmd := &cobra.Command{
		Use:   "copy-file",
		Short: "Copy a file or directory",
		Long:  "Copies a file or directory to a new location",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.CopyFile(core.CopyFileOptions{
				Source:      source,
				Destination: destination,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Copied %s to %s", result.Source, result.Destination)
			})
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Source path (required)")
	cmd.Flags().StringVar(&destination, "dest", "", "Destination path (required)")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("dest")

	return cmd
}

func moveFileCmd() *cobra.Command {
	var source, destination string

	cmd := &cobra.Command{
		Use:   "move-file",
		Short: "Move or rename a file or directory",
		Long:  "Moves or renames a file or directory",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.MoveFile(core.MoveFileOptions{
				Source:      source,
				Destination: destination,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Moved %s to %s", result.Source, result.Destination)
			})
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Source path (required)")
	cmd.Flags().StringVar(&destination, "dest", "", "Destination path (required)")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("dest")

	return cmd
}

func deleteFileCmd() *cobra.Command {
	var path string
	var recursive bool

	cmd := &cobra.Command{
		Use:   "delete-file",
		Short: "Delete a file or directory",
		Long:  "Deletes a file or directory, optionally recursively",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.DeleteFile(core.DeleteFileOptions{
				Path:        path,
				Recursive:   recursive,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Deleted %s", result.Path)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Path to delete (required)")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "Delete directories recursively")
	cmd.MarkFlagRequired("path")

	return cmd
}

func batchFileOperationsCmd() *cobra.Command {
	var operationsJSON string

	cmd := &cobra.Command{
		Use:   "batch-file-operations",
		Short: "Perform batch file operations",
		Long:  "Performs multiple file operations in a batch",
		Run: func(cmd *cobra.Command, args []string) {
			var operations []core.BatchOperation
			if err := json.Unmarshal([]byte(operationsJSON), &operations); err != nil {
				OutputError(fmt.Errorf("invalid operations JSON: %w", err))
			}

			result, err := core.BatchFileOperations(core.BatchFileOperationsOptions{
				Operations:  operations,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Batch complete: %d success, %d failed",
					result.Success, result.Failed)
			})
		},
	}

	cmd.Flags().StringVar(&operationsJSON, "operations", "",
		`JSON array of operations: [{"operation":"copy","source":"a","destination":"b"}]`)
	cmd.MarkFlagRequired("operations")

	return cmd
}
