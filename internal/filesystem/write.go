package filesystem

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/samestrin/llm-tools/internal/filesystem/core"
	"github.com/samestrin/llm-tools/pkg/pathvalidation"
)

// WriteFileResult represents the result of a write operation
type WriteFileResult struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Created bool   `json:"created"`
	Backup  string `json:"backup,omitempty"`
	Message string `json:"message"`
}

func (s *Server) handleWriteFile(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	content := GetString(args, "content", "")
	createDirs := GetBool(args, "create_dirs", true)
	appendMode := GetBool(args, "append", false)

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

	// Create parent directories if needed
	if createDirs {
		if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// Determine if file exists (for created flag)
	_, err = os.Stat(normalizedPath)
	created := os.IsNotExist(err)

	// Set file flags based on mode
	flags := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	// Write file
	file, err := os.OpenFile(normalizedPath, flags, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	n, err := file.WriteString(content)
	if err != nil {
		return "", fmt.Errorf("failed to write: %w", err)
	}

	result := WriteFileResult{
		Path:    normalizedPath,
		Size:    int64(n),
		Created: created,
		Message: "File written successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleLargeWriteFile(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	content := GetString(args, "content", "")
	createDirs := GetBool(args, "create_dirs", true)
	appendMode := GetBool(args, "append", false)
	backup := GetBool(args, "backup", true)
	verifyWrite := GetBool(args, "verify_write", true)

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

	// Create parent directories if needed
	if createDirs {
		if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create directories: %w", err)
		}
	}

	var backupPath string
	_, err = os.Stat(normalizedPath)
	fileExists := err == nil

	// Create backup if file exists and backup requested
	if fileExists && backup && !appendMode {
		backupPath = createBackupFile(normalizedPath)
	}

	// Write to temp file first (atomic write)
	tempPath := normalizedPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	n, err := tempFile.WriteString(content)
	if err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to write: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to sync: %w", err)
	}
	tempFile.Close()

	// Verify write if requested
	if verifyWrite {
		written, err := os.ReadFile(tempPath)
		if err != nil || string(written) != content {
			os.Remove(tempPath)
			return "", fmt.Errorf("write verification failed")
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, normalizedPath); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to rename temp file: %w", err)
	}

	result := WriteFileResult{
		Path:    normalizedPath,
		Size:    int64(n),
		Created: !fileExists,
		Backup:  backupPath,
		Message: "File written successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func createBackupFile(path string) string {
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.bak.%s", path, timestamp)

	// Copy original to backup
	src, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return ""
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(backupPath)
		return ""
	}

	return backupPath
}

func (s *Server) handleGetFileInfo(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat: %w", err)
	}

	// Determine type
	entryType := "file"
	if info.IsDir() {
		entryType = "directory"
	}

	// Get extension and MIME type for files
	var ext, mimeType string
	if !info.IsDir() {
		ext = filepath.Ext(info.Name())
		if ext != "" {
			mimeType = mime.TypeByExtension(ext)
		}
	}

	// Check access permissions
	isReadable, isWritable := checkAccess(normalizedPath)

	// Get all timestamps
	created, accessed, modified := core.GetFileTimestamps(info)

	result := map[string]interface{}{
		"path":          normalizedPath,
		"name":          info.Name(),
		"type":          entryType,
		"size":          info.Size(),
		"size_readable": humanize.Bytes(uint64(info.Size())),
		"is_dir":        info.IsDir(),
		"mode":          info.Mode().String(),
		"permissions":   uint32(info.Mode().Perm()),
		"modified":      modified.Format(time.RFC3339),
		"created":       created.Format(time.RFC3339),
		"accessed":      accessed.Format(time.RFC3339),
		"extension":     ext,
		"mime_type":     mimeType,
		"is_readable":   isReadable,
		"is_writable":   isWritable,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleCreateDirectory(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	recursive := GetBool(args, "recursive", true)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Check for unresolved template variables before normalizing
	if err := pathvalidation.ValidatePathForCreation(path); err != nil {
		return "", err
	}

	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	if recursive {
		err = os.MkdirAll(normalizedPath, 0755)
	} else {
		err = os.Mkdir(normalizedPath, 0755)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	result := map[string]interface{}{
		"path":    normalizedPath,
		"created": true,
		"message": "Directory created successfully",
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleCreateDirectories(args map[string]interface{}) (string, error) {
	paths := GetStringSlice(args, "paths")
	recursive := GetBool(args, "recursive", true)

	if len(paths) == 0 {
		return "", fmt.Errorf("paths is required")
	}

	result, err := core.CreateDirectories(core.CreateDirectoriesOptions{
		Paths:       paths,
		Recursive:   recursive,
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
