package commands

import (
	"encoding/json"
	"fmt"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/spf13/cobra"
)

func addEditCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(editBlockCmd())
	rootCmd.AddCommand(editBlocksCmd())
	rootCmd.AddCommand(safeEditCmd())
	rootCmd.AddCommand(editFileCmd())
	rootCmd.AddCommand(searchAndReplaceCmd())
}

func editBlockCmd() *cobra.Command {
	var path, oldString, newString string

	cmd := &cobra.Command{
		Use:   "edit-block",
		Short: "Replace a block of text in a file",
		Long:  "Precisely replaces the first occurrence of old_string with new_string",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.EditBlock(core.EditBlockOptions{
				Path:        path,
				OldString:   oldString,
				NewString:   newString,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Edited %s: %d change(s)", result.Path, result.Changes)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&oldString, "old", "", "Text to find and replace (required)")
	cmd.Flags().StringVar(&newString, "new", "", "Replacement text")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("old")

	return cmd
}

func editBlocksCmd() *cobra.Command {
	var path, editsJSON string

	cmd := &cobra.Command{
		Use:   "edit-blocks",
		Short: "Apply multiple edits to a file",
		Long:  "Applies multiple block edits to a single file",
		Run: func(cmd *cobra.Command, args []string) {
			var edits []core.EditPair
			if err := json.Unmarshal([]byte(editsJSON), &edits); err != nil {
				OutputError(fmt.Errorf("invalid edits JSON: %w", err))
			}

			result, err := core.EditBlocks(core.EditBlocksOptions{
				Path:        path,
				Edits:       edits,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("Edited %s: %d change(s)", result.Path, result.Changes)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&editsJSON, "edits", "", `JSON array of edits: [{"old_string":"x","new_string":"y"}]`)
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("edits")

	return cmd
}

func safeEditCmd() *cobra.Command {
	var path, oldString, newString string
	var backup, dryRun bool

	cmd := &cobra.Command{
		Use:   "safe-edit",
		Short: "Safe edit with backup and dry-run support",
		Long:  "Performs a safe edit with optional backup creation and dry-run mode",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.SafeEdit(core.SafeEditOptions{
				Path:        path,
				OldString:   oldString,
				NewString:   newString,
				Backup:      backup,
				DryRun:      dryRun,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				msg := fmt.Sprintf("Edited %s: %d change(s)", result.Path, result.Changes)
				if result.Backup != "" {
					msg += fmt.Sprintf("\nBackup: %s", result.Backup)
				}
				if dryRun {
					msg = "[DRY RUN] " + msg
				}
				return msg
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&oldString, "old", "", "Text to find and replace (required)")
	cmd.Flags().StringVar(&newString, "new", "", "Replacement text")
	cmd.Flags().BoolVar(&backup, "backup", true, "Create backup before editing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("old")

	return cmd
}

func editFileCmd() *cobra.Command {
	var path, operation, content string
	var line int

	cmd := &cobra.Command{
		Use:   "edit-file",
		Short: "Line-based file editing",
		Long:  "Performs line-based operations: insert, replace, or delete",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.EditFile(core.EditFileOptions{
				Path:        path,
				Operation:   operation,
				Line:        line,
				Content:     content,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				return fmt.Sprintf("%s at line %d: %s", operation, line, result.Message)
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "File path (required)")
	cmd.Flags().StringVar(&operation, "operation", "", "Operation: insert, replace, delete (required)")
	cmd.Flags().IntVar(&line, "line", 0, "Line number to operate on (required)")
	cmd.Flags().StringVar(&content, "content", "", "Content for insert/replace operations")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("operation")
	cmd.MarkFlagRequired("line")

	return cmd
}

func searchAndReplaceCmd() *cobra.Command {
	var path, pattern, replacement string
	var regex, dryRun bool
	var fileTypes []string

	cmd := &cobra.Command{
		Use:   "search-and-replace",
		Short: "Search and replace across files",
		Long:  "Performs search and replace across multiple files with regex support",
		Run: func(cmd *cobra.Command, args []string) {
			result, err := core.SearchAndReplace(core.SearchReplaceOptions{
				Path:        path,
				Pattern:     pattern,
				Replacement: replacement,
				Regex:       regex,
				DryRun:      dryRun,
				FileTypes:   fileTypes,
				AllowedDirs: GetAllowedDirs(),
			})
			if err != nil {
				OutputError(err)
			}
			OutputResult(result, func() string {
				msg := fmt.Sprintf("Modified %d files, %d total changes",
					result.FilesModified, result.TotalChanges)
				if dryRun {
					msg = "[DRY RUN] " + msg
				}
				return msg
			})
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Directory to search in (required)")
	cmd.Flags().StringVar(&pattern, "pattern", "", "Search pattern (required)")
	cmd.Flags().StringVar(&replacement, "replacement", "", "Replacement text (required)")
	cmd.Flags().BoolVar(&regex, "regex", false, "Use regex pattern matching")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying")
	cmd.Flags().StringSliceVar(&fileTypes, "file-types", nil, "File extensions to include")
	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("pattern")
	cmd.MarkFlagRequired("replacement")

	return cmd
}
