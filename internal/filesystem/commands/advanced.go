package commands

import (
	"fmt"
	"strings"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addAdvancedCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(getDiskUsageCmd())
	rootCmd.AddCommand(findLargeFilesCmd())
	rootCmd.AddCommand(compressFilesCmd())
	rootCmd.AddCommand(extractArchiveCmd())
	rootCmd.AddCommand(syncDirectoriesCmd())
	rootCmd.AddCommand(listAllowedDirectoriesCmd())
}

func getDiskUsageCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "get-disk-usage",
		Short: "Get disk usage for a path",
		Long:  "Calculates total disk usage for a directory",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.GetDiskUsage(core.DiskUsageOptions{
				Path:        path,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Path: %s\nTotal Size: %d bytes\nFiles: %d\nDirectories: %d",
					result.Path, result.TotalSize, result.TotalFiles, result.TotalDirs)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Path to analyze (required)")
	cmd.MarkFlagRequired("path")

	return cmd
}

func findLargeFilesCmd() *cobra.Command {
	var path string
	var minSize int64
	var limit int

	cmd := &cobra.Command{
		Use:   "find-large-files",
		Short: "Find large files",
		Long:  "Finds files larger than a specified size",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.FindLargeFiles(core.FindLargeFilesOptions{
				Path:        path,
				MinSize:     minSize,
				Limit:       limit,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Found %d files >= %d bytes in %s\n\n",
					result.Total, result.MinSize, result.Path))
				for _, f := range result.Files {
					sb.WriteString(fmt.Sprintf("%s  %d bytes  %s\n", f.Path, f.Size, f.ModTime))
				}
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Directory to search (required)")
	cmd.Flags().Int64Var(&minSize, "min-size", 0, "Minimum file size in bytes")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum results to return")
	cmd.MarkFlagRequired("path")

	return cmd
}

func compressFilesCmd() *cobra.Command {
	var paths []string
	var output, format string

	cmd := &cobra.Command{
		Use:   "compress-files",
		Short: "Compress files into an archive",
		Long:  "Creates a zip or tar.gz archive from files and directories",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.CompressFiles(core.CompressFilesOptions{
				Paths:       paths,
				Output:      output,
				Format:      format,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Created %s (%s format)\n%d files added, %d bytes",
					result.Output, result.Format, result.FilesAdded, result.Size)
			})
		},
	}

	cmd.Flags().StringSliceVar(&paths, "paths", nil, "Paths to compress (required)")
	cmd.Flags().StringVar(&output, "output", "", "Output archive path (required)")
	cmd.Flags().StringVar(&format, "format", "zip", "Archive format: zip, tar.gz")
	cmd.MarkFlagRequired("paths")
	cmd.MarkFlagRequired("output")

	return cmd
}

func extractArchiveCmd() *cobra.Command {
	var archive, destination string

	cmd := &cobra.Command{
		Use:   "extract-archive",
		Short: "Extract an archive",
		Long:  "Extracts a zip or tar.gz archive to a destination",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.ExtractArchive(core.ExtractArchiveOptions{
				Archive:     archive,
				Destination: destination,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Extracted %s to %s\n%d files extracted",
					result.Archive, result.Destination, result.FilesExtracted)
			})
		},
	}

	cmd.Flags().StringVar(&archive, "archive", "", "Archive file path (required)")
	cmd.Flags().StringVar(&destination, "dest", "", "Destination directory (required)")
	cmd.MarkFlagRequired("archive")
	cmd.MarkFlagRequired("dest")

	return cmd
}

func syncDirectoriesCmd() *cobra.Command {
	var source, destination string

	cmd := &cobra.Command{
		Use:   "sync-directories",
		Short: "Synchronize two directories",
		Long:  "Synchronizes the contents of a source directory to a destination",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.SyncDirectories(core.SyncDirectoriesOptions{
				Source:      source,
				Destination: destination,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Synced %s to %s\n%d files copied, %d directories created",
					result.Source, result.Destination, result.FilesCopied, result.DirsCreated)
			})
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Source directory (required)")
	cmd.Flags().StringVar(&destination, "dest", "", "Destination directory (required)")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("dest")

	return cmd
}

func listAllowedDirectoriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-allowed-directories",
		Short: "List allowed directories",
		Long:  "Returns the list of directories the tool is allowed to access",
		Run: func(cmd *cobra.Command, args []string) {
			result := core.ListAllowedDirectories(GetAllowedDirs())
			OutputResult(result, func() string {
				if len(result.AllowedDirectories) == 0 {
					return "All directories allowed (no restrictions)"
				}
				return fmt.Sprintf("Allowed directories:\n%s",
					strings.Join(result.AllowedDirectories, "\n"))
			})
		},
	}

	return cmd
}
