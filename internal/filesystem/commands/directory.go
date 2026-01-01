package commands

import (
	"fmt"
	"strings"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addDirectoryCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(listDirectoryCmd())
	rootCmd.AddCommand(getDirectoryTreeCmd())
}

func listDirectoryCmd() *cobra.Command {
	var path, pattern, sortBy string
	var showHidden, reverse bool
	var page, pageSize int

	cmd := &cobra.Command{
		Use:   "list-directory",
		Short: "List directory contents",
		Long:  "Lists the contents of a directory with filtering, sorting, and pagination",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.ListDirectory(core.ListDirectoryOptions{
				Path:        path,
				ShowHidden:  showHidden,
				Pattern:     pattern,
				SortBy:      sortBy,
				Reverse:     reverse,
				Page:        page,
				PageSize:    pageSize,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Directory: %s (%d entries)\n\n", result.Path, result.Total))
				for _, e := range result.Items {
					typeIndicator := " "
					if e.IsDir {
						typeIndicator = "/"
					}
					sb.WriteString(fmt.Sprintf("%s%s  %d  %s\n",
						e.Name, typeIndicator, e.Size, e.Modified))
				}
				if result.TotalPages > 0 {
					sb.WriteString(fmt.Sprintf("\nPage %d/%d", result.Page, result.TotalPages))
				}
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Directory path (required)")
	cmd.Flags().BoolVar(&showHidden, "show-hidden", false, "Show hidden files")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Filename filter pattern")
	cmd.Flags().StringVar(&sortBy, "sort-by", "name", "Sort by: name, size, modified")
	cmd.Flags().BoolVar(&reverse, "reverse", false, "Reverse sort order")
	cmd.Flags().IntVar(&page, "page", 0, "Page number for pagination")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Items per page")
	cmd.MarkFlagRequired("path")

	return cmd
}

func getDirectoryTreeCmd() *cobra.Command {
	var path, pattern string
	var maxDepth int
	var showHidden, includeFiles bool

	cmd := &cobra.Command{
		Use:   "get-directory-tree",
		Short: "Get directory tree structure",
		Long:  "Returns the directory tree structure with optional depth and filtering",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.GetDirectoryTree(core.GetDirectoryTreeOptions{
				Path:         path,
				MaxDepth:     maxDepth,
				ShowHidden:   showHidden,
				IncludeFiles: includeFiles,
				Pattern:      pattern,
				AllowedDirs:  GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("Tree: %s\n", result.Tree.Path))
				sb.WriteString(fmt.Sprintf("Dirs: %d, Files: %d, Size: %d bytes\n\n",
					result.TotalDirs, result.TotalFiles, result.TotalSize))
				printTree(&sb, result.Tree, "", true)
				return sb.String()
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Root directory path (required)")
	cmd.Flags().IntVar(&maxDepth, "depth", 5, "Maximum depth to traverse")
	cmd.Flags().BoolVar(&showHidden, "show-hidden", false, "Show hidden files")
	cmd.Flags().BoolVar(&includeFiles, "include-files", false, "Include files in tree")
	cmd.Flags().StringVar(&pattern, "pattern", "", "File pattern filter")
	cmd.MarkFlagRequired("path")

	return cmd
}

func printTree(sb *strings.Builder, node *core.TreeNode, prefix string, isLast bool) {
	if node == nil {
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}

	sb.WriteString(prefix + connector + node.Name)
	if node.IsDir {
		sb.WriteString("/")
	}
	sb.WriteString("\n")

	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	for i, child := range node.Children {
		printTree(sb, child, childPrefix, i == len(node.Children)-1)
	}
}
