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
