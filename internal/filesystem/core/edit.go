package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// EditResult represents the result of an edit operation
type EditResult struct {
	Path       string `json:"path"`
	Success    bool   `json:"success"`
	Changes    int    `json:"changes"`
	Backup     string `json:"backup,omitempty"`
	Message    string `json:"message"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

// EditBlockOptions contains input parameters for EditBlock
type EditBlockOptions struct {
	Path        string
	OldString   string
	NewString   string
	AllowedDirs []string
}

// EditBlock replaces a single block of text in a file
func EditBlock(opts EditBlockOptions) (*EditResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if opts.OldString == "" {
		return nil, fmt.Errorf("old_string is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if old_string exists
	if !strings.Contains(string(content), opts.OldString) {
		return nil, fmt.Errorf("old_string not found in file")
	}

	// Replace content
	newContent := strings.Replace(string(content), opts.OldString, opts.NewString, 1)

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &EditResult{
		Path:    normalizedPath,
		Success: true,
		Changes: 1,
		Message: "Edit completed successfully",
	}, nil
}

// EditBlocksOptions contains input parameters for EditBlocks
type EditBlocksOptions struct {
	Path        string
	Edits       []EditPair
	AllowedDirs []string
}

// EditPair represents an old/new string pair for editing
type EditPair struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// EditBlocks applies multiple edits to a single file
func EditBlocks(opts EditBlocksOptions) (*EditResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	changes := 0

	for _, edit := range opts.Edits {
		if edit.OldString == "" {
			continue
		}

		if strings.Contains(contentStr, edit.OldString) {
			contentStr = strings.Replace(contentStr, edit.OldString, edit.NewString, 1)
			changes++
		}
	}

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(contentStr), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &EditResult{
		Path:    normalizedPath,
		Success: true,
		Changes: changes,
		Message: fmt.Sprintf("Applied %d edits successfully", changes),
	}, nil
}

// SafeEditOptions contains input parameters for SafeEdit
type SafeEditOptions struct {
	Path        string
	OldString   string
	NewString   string
	Backup      bool
	DryRun      bool
	AllowedDirs []string
}

// SafeEdit performs a safe edit with backup and dry-run support
func SafeEdit(opts SafeEditOptions) (*EditResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if opts.OldString == "" {
		return nil, fmt.Errorf("old_string is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)

	// Check if old_string exists
	if !strings.Contains(contentStr, opts.OldString) {
		return nil, fmt.Errorf("old_string not found in file")
	}

	newContent := strings.Replace(contentStr, opts.OldString, opts.NewString, 1)

	result := &EditResult{
		Path:       normalizedPath,
		Success:    true,
		Changes:    1,
		OldContent: contentStr,
		NewContent: newContent,
	}

	if opts.DryRun {
		result.Message = "Dry run - no changes made"
		return result, nil
	}

	// Create backup if requested
	if opts.Backup {
		backupPath := createEditBackup(normalizedPath)
		result.Backup = backupPath
	}

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	result.Message = "Edit completed successfully"
	return result, nil
}

func createEditBackup(path string) string {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.bak.%s", path, timestamp)

	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return ""
	}

	return backupPath
}

// EditFileOptions contains input parameters for EditFile
type EditFileOptions struct {
	Path        string
	Operation   string
	Line        int
	Content     string
	AllowedDirs []string
}

// EditFile performs line-based file editing
func EditFile(opts EditFileOptions) (*EditResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if opts.Operation == "" {
		return nil, fmt.Errorf("operation is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Read file content
	fileContent, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(fileContent), "\n")

	switch opts.Operation {
	case "insert":
		if opts.Line < 1 || opts.Line > len(lines)+1 {
			return nil, fmt.Errorf("line number out of range")
		}
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:opts.Line-1]...)
		newLines = append(newLines, opts.Content)
		newLines = append(newLines, lines[opts.Line-1:]...)
		lines = newLines

	case "delete":
		if opts.Line < 1 || opts.Line > len(lines) {
			return nil, fmt.Errorf("line number out of range")
		}
		lines = append(lines[:opts.Line-1], lines[opts.Line:]...)

	case "replace":
		if opts.Line < 1 || opts.Line > len(lines) {
			return nil, fmt.Errorf("line number out of range")
		}
		lines[opts.Line-1] = opts.Content

	default:
		return nil, fmt.Errorf("unknown operation: %s", opts.Operation)
	}

	newContent := strings.Join(lines, "\n")

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	return &EditResult{
		Path:    normalizedPath,
		Success: true,
		Changes: 1,
		Message: fmt.Sprintf("Operation '%s' completed successfully", opts.Operation),
	}, nil
}

// SearchReplaceOptions contains input parameters for SearchAndReplace
type SearchReplaceOptions struct {
	Path        string
	Pattern     string
	Replacement string
	Regex       bool
	DryRun      bool
	FileTypes   []string
	AllowedDirs []string
}

// SearchReplaceResult represents results from search and replace
type SearchReplaceResult struct {
	Path          string       `json:"path"`
	FilesModified int          `json:"files_modified"`
	TotalChanges  int          `json:"total_changes"`
	Results       []EditResult `json:"results"`
}

// SearchAndReplace performs search and replace across files
func SearchAndReplace(opts SearchReplaceOptions) (*SearchReplaceResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	var replaceFn func(s string) string

	if opts.Regex {
		re, err := regexp.Compile(opts.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		replaceFn = func(s string) string {
			return re.ReplaceAllString(s, opts.Replacement)
		}
	} else {
		replaceFn = func(s string) string {
			return strings.ReplaceAll(s, opts.Pattern, opts.Replacement)
		}
	}

	var results []EditResult
	totalChanges := 0

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check file type filter
		if len(opts.FileTypes) > 0 {
			ext := filepath.Ext(info.Name())
			matched := false
			for _, ft := range opts.FileTypes {
				if ext == ft || "."+ext == ft || ext == "."+ft {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// Skip binary files
		if isBinaryFile(filePath) {
			return nil
		}

		// Read file
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		contentStr := string(content)
		newContent := replaceFn(contentStr)

		if contentStr == newContent {
			return nil // No changes
		}

		changes := countChanges(contentStr, newContent, opts.Pattern, opts.Regex)
		totalChanges += changes

		if !opts.DryRun {
			if err := os.WriteFile(filePath, []byte(newContent), info.Mode()); err != nil {
				return nil
			}
		}

		results = append(results, EditResult{
			Path:    filePath,
			Success: true,
			Changes: changes,
			Message: "Modified",
		})

		return nil
	}

	if err := filepath.Walk(normalizedPath, walkFn); err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return &SearchReplaceResult{
		Path:          normalizedPath,
		FilesModified: len(results),
		TotalChanges:  totalChanges,
		Results:       results,
	}, nil
}

func countChanges(old, new, pattern string, useRegex bool) int {
	if useRegex {
		re, _ := regexp.Compile(pattern)
		return len(re.FindAllString(old, -1))
	}
	return strings.Count(old, pattern)
}

// MultiEditOperation represents a single edit operation in EditMultipleBlocks
type MultiEditOperation struct {
	OldText    string `json:"old_text,omitempty"`
	NewText    string `json:"new_text,omitempty"`
	LineNumber int    `json:"line_number,omitempty"`
	Mode       string `json:"mode,omitempty"` // "replace", "insert_before", "insert_after", "delete_line"
}

// MultiEditResult represents the detailed result of EditMultipleBlocks (fast-filesystem parity)
type MultiEditResult struct {
	Message         string           `json:"message"`
	Path            string           `json:"path"`
	TotalEdits      int              `json:"total_edits"`
	SuccessfulEdits int              `json:"successful_edits"`
	TotalChanges    int              `json:"total_changes"`
	OriginalLines   int              `json:"original_lines"`
	NewLines        int              `json:"new_lines"`
	EditResults     []EditOpResult   `json:"edit_results"`
	BackupCreated   *string          `json:"backup_created"`
	BackupEnabled   bool             `json:"backup_enabled"`
	Size            int64            `json:"size"`
	SizeReadable    string           `json:"size_readable"`
	Timestamp       string           `json:"timestamp"`
}

// EditOpResult represents the result of a single edit operation
type EditOpResult struct {
	EditIndex    int    `json:"edit_index"`
	Mode         string `json:"mode"`
	Status       string `json:"status"`
	LineNumber   int    `json:"line_number,omitempty"`
	OldText      string `json:"old_text_preview,omitempty"`
	NewText      string `json:"new_text_preview,omitempty"`
	InsertedLine string `json:"inserted_line,omitempty"`
	DeletedLine  string `json:"deleted_line,omitempty"`
	ChangesMade  int    `json:"changes_made"`
}

// EditMultipleBlocksOptions contains input parameters for EditMultipleBlocks
type EditMultipleBlocksOptions struct {
	Path        string
	Edits       []MultiEditOperation
	Backup      bool
	AllowedDirs []string
}

// EditMultipleBlocks applies multiple edits to a file with support for different modes
// Modes: replace (text matching), insert_before (line-based), insert_after (line-based), delete_line (line-based)
func EditMultipleBlocks(opts EditMultipleBlocksOptions) (*MultiEditResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var backupPath *string
	if opts.Backup {
		bp := createEditBackup(normalizedPath)
		if bp != "" {
			backupPath = &bp
		}
	}

	contentStr := string(content)
	originalLines := len(strings.Split(contentStr, "\n"))
	totalChanges := 0
	successfulEdits := 0
	editResults := make([]EditOpResult, 0, len(opts.Edits))

	// Process each edit
	for i, edit := range opts.Edits {
		mode := edit.Mode
		if mode == "" {
			mode = "replace"
		}

		opResult := EditOpResult{
			EditIndex:  i + 1,
			Mode:       mode,
			LineNumber: edit.LineNumber,
		}

		switch mode {
		case "replace":
			if edit.OldText == "" {
				opResult.Status = "skipped"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			if strings.Contains(contentStr, edit.OldText) {
				contentStr = strings.Replace(contentStr, edit.OldText, edit.NewText, 1)
				opResult.Status = "success"
				opResult.ChangesMade = 1
				opResult.OldText = truncatePreview(edit.OldText, 50)
				opResult.NewText = truncatePreview(edit.NewText, 50)
				totalChanges++
				successfulEdits++
			} else {
				opResult.Status = "not_found"
				opResult.ChangesMade = 0
			}

		case "insert_before":
			if edit.LineNumber < 1 {
				opResult.Status = "invalid_line"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			lines := strings.Split(contentStr, "\n")
			if edit.LineNumber > len(lines)+1 {
				opResult.Status = "line_out_of_range"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:edit.LineNumber-1]...)
			newLines = append(newLines, edit.NewText)
			newLines = append(newLines, lines[edit.LineNumber-1:]...)
			contentStr = strings.Join(newLines, "\n")
			opResult.Status = "success"
			opResult.ChangesMade = 1
			opResult.InsertedLine = truncatePreview(edit.NewText, 50)
			totalChanges++
			successfulEdits++

		case "insert_after":
			if edit.LineNumber < 1 {
				opResult.Status = "invalid_line"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			lines := strings.Split(contentStr, "\n")
			if edit.LineNumber > len(lines) {
				opResult.Status = "line_out_of_range"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:edit.LineNumber]...)
			newLines = append(newLines, edit.NewText)
			newLines = append(newLines, lines[edit.LineNumber:]...)
			contentStr = strings.Join(newLines, "\n")
			opResult.Status = "success"
			opResult.ChangesMade = 1
			opResult.InsertedLine = truncatePreview(edit.NewText, 50)
			totalChanges++
			successfulEdits++

		case "delete_line":
			if edit.LineNumber < 1 {
				opResult.Status = "invalid_line"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			lines := strings.Split(contentStr, "\n")
			if edit.LineNumber > len(lines) {
				opResult.Status = "line_out_of_range"
				opResult.ChangesMade = 0
				editResults = append(editResults, opResult)
				continue
			}
			opResult.DeletedLine = truncatePreview(lines[edit.LineNumber-1], 50)
			lines = append(lines[:edit.LineNumber-1], lines[edit.LineNumber:]...)
			contentStr = strings.Join(lines, "\n")
			opResult.Status = "success"
			opResult.ChangesMade = 1
			totalChanges++
			successfulEdits++
		}

		editResults = append(editResults, opResult)
	}

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(contentStr), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Get final file info
	finalInfo, _ := os.Stat(normalizedPath)
	var finalSize int64
	if finalInfo != nil {
		finalSize = finalInfo.Size()
	}

	newLines := len(strings.Split(contentStr, "\n"))

	return &MultiEditResult{
		Message:         "Safe multiple blocks edited successfully",
		Path:            normalizedPath,
		TotalEdits:      len(opts.Edits),
		SuccessfulEdits: successfulEdits,
		TotalChanges:    totalChanges,
		OriginalLines:   originalLines,
		NewLines:        newLines,
		EditResults:     editResults,
		BackupCreated:   backupPath,
		BackupEnabled:   opts.Backup,
		Size:            finalSize,
		SizeReadable:    formatBytes(finalSize),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// truncatePreview truncates a string to maxLen characters with ellipsis
func truncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatBytes formats bytes as human-readable string
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
