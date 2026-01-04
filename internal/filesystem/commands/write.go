package commands

import (
	"fmt"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addWriteCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(writeFileCmd())
	rootCmd.AddCommand(largeWriteFileCmd())
	rootCmd.AddCommand(getFileInfoCmd())
	rootCmd.AddCommand(createDirectoryCmd())
	rootCmd.AddCommand(createDirectoriesCmd())
}

func writeFileCmd() *cobra.Command {
	var path, content string
	var createDirs, appendMode bool

	cmd := &cobra.Command{
		Use:   "write-file",
		Short: "Write content to a file",
		Long:  "Writes or modifies a file with the specified content",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.WriteFile(core.WriteFileOptions{
				Path:        path,
				Content:     content,
				CreateDirs:  createDirs,
				Append:      appendMode,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				action := "Updated"
				if result.Created {
					action = "Created"
				}
				return fmt.Sprintf("%s %s (%d bytes)", action, result.Path, result.Size)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&content, "content", "", "Content to write (required)")
	cmd.Flags().BoolVar(&createDirs, "create-dirs", true, "Create parent directories if needed")
	cmd.Flags().BoolVar(&appendMode, "append", false, "Append to file instead of overwrite")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("content")

	return cmd
}

func largeWriteFileCmd() *cobra.Command {
	var path, content string
	var createDirs, appendMode, backup, verifyWrite bool

	cmd := &cobra.Command{
		Use:   "large-write-file",
		Short: "Write large files with backup and verification",
		Long:  "Reliably writes large files with streaming, backup, and verification",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.LargeWriteFile(core.LargeWriteFileOptions{
				Path:        path,
				Content:     content,
				CreateDirs:  createDirs,
				Append:      appendMode,
				Backup:      backup,
				VerifyWrite: verifyWrite,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				action := "Updated"
				if result.Created {
					action = "Created"
				}
				msg := fmt.Sprintf("%s %s (%d bytes)", action, result.Path, result.Size)
				if result.Backup != "" {
					msg += fmt.Sprintf("\nBackup: %s", result.Backup)
				}
				return msg
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&content, "content", "", "Content to write (required)")
	cmd.Flags().BoolVar(&createDirs, "create-dirs", true, "Create parent directories if needed")
	cmd.Flags().BoolVar(&appendMode, "append", false, "Append to file instead of overwrite")
	cmd.Flags().BoolVar(&backup, "backup", true, "Create backup before writing")
	cmd.Flags().BoolVar(&verifyWrite, "verify", true, "Verify write after completion")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("content")

	return cmd
}

func getFileInfoCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "get-file-info",
		Short: "Get detailed file information",
		Long:  "Gets detailed information about a file or directory",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.GetFileInfo(core.GetFileInfoOptions{
				Path:        path,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				fileType := "File"
				if result.IsDir {
					fileType = "Directory"
				}
				return fmt.Sprintf("%s: %s\nSize: %d\nMode: %s\nModified: %s",
					fileType, result.Path, result.Size, result.Mode, result.Modified)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Path to get info for (required)")
	cmd.MarkFlagRequired("path")

	return cmd
}

func createDirectoryCmd() *cobra.Command {
	var path string
	var recursive bool

	cmd := &cobra.Command{
		Use:   "create-directory",
		Short: "Create a directory",
		Long:  "Creates a directory, optionally with parent directories",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.CreateDirectory(core.CreateDirectoryOptions{
				Path:        path,
				Recursive:   recursive,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Created directory: %s", result.Path)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Directory path to create (required)")
	cmd.Flags().BoolVar(&recursive, "recursive", true, "Create parent directories")
	cmd.MarkFlagRequired("path")

	return cmd
}

func createDirectoriesCmd() *cobra.Command {
	var paths []string
	var recursive bool

	cmd := &cobra.Command{
		Use:   "create-directories",
		Short: "Create multiple directories",
		Long:  "Creates multiple directories in a single operation, optionally with parent directories",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.CreateDirectories(core.CreateDirectoriesOptions{
				Paths:       paths,
				Recursive:   recursive,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Created %d directories, %d failed", result.Success, result.Failed)
			})
		},
	}

	cmd.Flags().StringArrayVar(&paths, "paths", nil, "Directory paths to create (required, can be specified multiple times)")
	cmd.Flags().BoolVar(&recursive, "recursive", true, "Create parent directories")
	cmd.MarkFlagRequired("paths")

	return cmd
}
