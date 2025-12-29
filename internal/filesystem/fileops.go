package filesystem

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileOpResult represents the result of a file operation
type FileOpResult struct {
	Path        string `json:"path"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	Success     bool   `json:"success"`
	Operation   string `json:"operation"`
	Message     string `json:"message"`
}

// BatchOpResult represents the result of batch operations
type BatchOpResult struct {
	Success    int            `json:"success"`
	Failed     int            `json:"failed"`
	Operations []FileOpResult `json:"operations"`
}

func (s *Server) handleCopyFile(args map[string]interface{}) (string, error) {
	source := GetString(args, "source", "")
	destination := GetString(args, "destination", "")

	if source == "" {
		return "", fmt.Errorf("source is required")
	}
	if destination == "" {
		return "", fmt.Errorf("destination is required")
	}

	// Normalize and validate paths
	srcPath, err := NormalizePath(source)
	if err != nil {
		return "", fmt.Errorf("invalid source path: %w", err)
	}

	dstPath, err := NormalizePath(destination)
	if err != nil {
		return "", fmt.Errorf("invalid destination path: %w", err)
	}

	if err := ValidatePath(srcPath, s.allowedDirs); err != nil {
		return "", err
	}
	if err := ValidatePath(dstPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Check source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return "", fmt.Errorf("source does not exist: %w", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	if srcInfo.IsDir() {
		if err := copyDir(srcPath, dstPath); err != nil {
			return "", fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := copyFile(srcPath, dstPath); err != nil {
			return "", fmt.Errorf("failed to copy file: %w", err)
		}
	}

	result := FileOpResult{
		Source:      srcPath,
		Destination: dstPath,
		Success:     true,
		Operation:   "copy",
		Message:     "Copy completed successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Server) handleMoveFile(args map[string]interface{}) (string, error) {
	source := GetString(args, "source", "")
	destination := GetString(args, "destination", "")

	if source == "" {
		return "", fmt.Errorf("source is required")
	}
	if destination == "" {
		return "", fmt.Errorf("destination is required")
	}

	// Normalize and validate paths
	srcPath, err := NormalizePath(source)
	if err != nil {
		return "", fmt.Errorf("invalid source path: %w", err)
	}

	dstPath, err := NormalizePath(destination)
	if err != nil {
		return "", fmt.Errorf("invalid destination path: %w", err)
	}

	if err := ValidatePath(srcPath, s.allowedDirs); err != nil {
		return "", err
	}
	if err := ValidatePath(dstPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Check source exists
	if _, err := os.Stat(srcPath); err != nil {
		return "", fmt.Errorf("source does not exist: %w", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Try rename first (works across same filesystem)
	if err := os.Rename(srcPath, dstPath); err != nil {
		// Fallback to copy + delete (cross-filesystem)
		srcInfo, _ := os.Stat(srcPath)
		if srcInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return "", fmt.Errorf("failed to move directory: %w", err)
			}
			os.RemoveAll(srcPath)
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return "", fmt.Errorf("failed to move file: %w", err)
			}
			os.Remove(srcPath)
		}
	}

	result := FileOpResult{
		Source:      srcPath,
		Destination: dstPath,
		Success:     true,
		Operation:   "move",
		Message:     "Move completed successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleDeleteFile(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	recursive := GetBool(args, "recursive", false)

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

	// Check if exists
	info, err := os.Stat(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %w", err)
	}

	if info.IsDir() {
		if recursive {
			if err := os.RemoveAll(normalizedPath); err != nil {
				return "", fmt.Errorf("failed to delete directory: %w", err)
			}
		} else {
			if err := os.Remove(normalizedPath); err != nil {
				return "", fmt.Errorf("failed to delete directory (use recursive=true for non-empty): %w", err)
			}
		}
	} else {
		if err := os.Remove(normalizedPath); err != nil {
			return "", fmt.Errorf("failed to delete file: %w", err)
		}
	}

	result := FileOpResult{
		Path:      normalizedPath,
		Success:   true,
		Operation: "delete",
		Message:   "Delete completed successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleBatchFileOperations(args map[string]interface{}) (string, error) {
	operations, ok := args["operations"].([]interface{})
	if !ok {
		return "", fmt.Errorf("operations is required and must be an array")
	}

	var results []FileOpResult
	success := 0
	failed := 0

	for _, op := range operations {
		opMap, ok := op.(map[string]interface{})
		if !ok {
			continue
		}

		operation := GetString(opMap, "operation", "")
		var result string
		var err error

		switch operation {
		case "copy":
			result, err = s.handleCopyFile(opMap)
		case "move":
			result, err = s.handleMoveFile(opMap)
		case "delete":
			result, err = s.handleDeleteFile(opMap)
		default:
			err = fmt.Errorf("unknown operation: %s", operation)
		}

		opResult := FileOpResult{
			Operation: operation,
		}

		if err != nil {
			opResult.Success = false
			opResult.Message = err.Error()
			failed++
		} else {
			opResult.Success = true
			opResult.Message = result
			success++
		}

		results = append(results, opResult)
	}

	batchResult := BatchOpResult{
		Success:    success,
		Failed:     failed,
		Operations: results,
	}

	jsonBytes, err := json.Marshal(batchResult)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}
