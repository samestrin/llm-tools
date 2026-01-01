package filesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
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

// SearchReplaceResult represents results from search and replace
type SearchReplaceResult struct {
	Path          string       `json:"path"`
	FilesModified int          `json:"files_modified"`
	TotalChanges  int          `json:"total_changes"`
	Results       []EditResult `json:"results"`
}

func (s *Server) handleEditBlock(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	oldString := GetString(args, "old_string", "")
	newString := GetString(args, "new_string", "")

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldString == "" {
		return "", fmt.Errorf("old_string is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Check if old_string exists
	if !strings.Contains(string(content), oldString) {
		return "", fmt.Errorf("old_string not found in file")
	}

	// Replace content
	newContent := strings.Replace(string(content), oldString, newString, 1)

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	result := EditResult{
		Path:    normalizedPath,
		Success: true,
		Changes: 1,
		Message: "Edit completed successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleEditBlocks(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	edits := args["edits"]

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Parse edits
	editList, ok := edits.([]interface{})
	if !ok {
		return "", fmt.Errorf("edits must be an array")
	}

	contentStr := string(content)
	changes := 0

	for _, e := range editList {
		edit, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		oldString := GetString(edit, "old_string", "")
		newString := GetString(edit, "new_string", "")

		if oldString == "" {
			continue
		}

		if strings.Contains(contentStr, oldString) {
			contentStr = strings.Replace(contentStr, oldString, newString, 1)
			changes++
		}
	}

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(contentStr), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	result := EditResult{
		Path:    normalizedPath,
		Success: true,
		Changes: changes,
		Message: fmt.Sprintf("Applied %d edits successfully", changes),
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleEditMultipleBlocks(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	backup := GetBool(args, "backup", true)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Parse edits array
	editsRaw, ok := args["edits"].([]interface{})
	if !ok {
		return "", fmt.Errorf("edits is required and must be an array")
	}

	var edits []core.MultiEditOperation
	for _, e := range editsRaw {
		editMap, ok := e.(map[string]interface{})
		if !ok {
			continue
		}

		edit := core.MultiEditOperation{
			OldText:    GetString(editMap, "old_text", ""),
			NewText:    GetString(editMap, "new_text", ""),
			LineNumber: GetInt(editMap, "line_number", 0),
			Mode:       GetString(editMap, "mode", "replace"),
		}

		// Also support old_string/new_string for backwards compatibility
		if edit.OldText == "" {
			edit.OldText = GetString(editMap, "old_string", "")
		}
		if edit.NewText == "" {
			edit.NewText = GetString(editMap, "new_string", "")
		}

		edits = append(edits, edit)
	}

	result, err := core.EditMultipleBlocks(core.EditMultipleBlocksOptions{
		Path:        path,
		Edits:       edits,
		Backup:      backup,
		AllowedDirs: s.allowedDirs,
	})
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleSafeEdit(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	oldString := GetString(args, "old_string", "")
	newString := GetString(args, "new_string", "")
	backup := GetBool(args, "backup", true)
	dryRun := GetBool(args, "dry_run", false)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldString == "" {
		return "", fmt.Errorf("old_string is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Read file content
	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)

	// Check if old_string exists
	if !strings.Contains(contentStr, oldString) {
		return "", fmt.Errorf("old_string not found in file")
	}

	newContent := strings.Replace(contentStr, oldString, newString, 1)

	result := EditResult{
		Path:       normalizedPath,
		Success:    true,
		Changes:    1,
		OldContent: contentStr,
		NewContent: newContent,
	}

	if dryRun {
		result.Message = "Dry run - no changes made"
		jsonBytes, _ := json.Marshal(result)
		return string(jsonBytes), nil
	}

	// Create backup if requested
	var backupPath string
	if backup {
		backupPath = createEditBackup(normalizedPath)
		result.Backup = backupPath
	}

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	result.Message = "Edit completed successfully"

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
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

func (s *Server) handleEditFile(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	operation := GetString(args, "operation", "")
	line := GetInt(args, "line", 0)
	content := GetString(args, "content", "")

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if operation == "" {
		return "", fmt.Errorf("operation is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Read file content
	fileContent, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(fileContent), "\n")

	switch operation {
	case "insert":
		if line < 1 || line > len(lines)+1 {
			return "", fmt.Errorf("line number out of range")
		}
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:line-1]...)
		newLines = append(newLines, content)
		newLines = append(newLines, lines[line-1:]...)
		lines = newLines

	case "delete":
		if line < 1 || line > len(lines) {
			return "", fmt.Errorf("line number out of range")
		}
		lines = append(lines[:line-1], lines[line:]...)

	case "replace":
		if line < 1 || line > len(lines) {
			return "", fmt.Errorf("line number out of range")
		}
		lines[line-1] = content

	default:
		return "", fmt.Errorf("unknown operation: %s", operation)
	}

	newContent := strings.Join(lines, "\n")

	// Write file
	if err := os.WriteFile(normalizedPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	result := EditResult{
		Path:    normalizedPath,
		Success: true,
		Changes: 1,
		Message: fmt.Sprintf("Operation '%s' completed successfully", operation),
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleSearchAndReplace(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	pattern := GetString(args, "pattern", "")
	replacement := GetString(args, "replacement", "")
	useRegex := GetBool(args, "regex", false)
	dryRun := GetBool(args, "dry_run", false)
	fileTypes := GetStringSlice(args, "file_types")

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	var replaceFn func(s string) string

	if useRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex pattern: %w", err)
		}
		replaceFn = func(s string) string {
			return re.ReplaceAllString(s, replacement)
		}
	} else {
		replaceFn = func(s string) string {
			return strings.ReplaceAll(s, pattern, replacement)
		}
	}

	var results []EditResult
	totalChanges := 0

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check file type filter
		if len(fileTypes) > 0 {
			ext := filepath.Ext(info.Name())
			matched := false
			for _, ft := range fileTypes {
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

		changes := countChanges(contentStr, newContent, pattern, useRegex)
		totalChanges += changes

		if !dryRun {
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
		return "", fmt.Errorf("failed to walk directory: %w", err)
	}

	result := SearchReplaceResult{
		Path:          normalizedPath,
		FilesModified: len(results),
		TotalChanges:  totalChanges,
		Results:       results,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func countChanges(old, new, pattern string, useRegex bool) int {
	if useRegex {
		re, _ := regexp.Compile(pattern)
		return len(re.FindAllString(old, -1))
	}
	return strings.Count(old, pattern)
}
