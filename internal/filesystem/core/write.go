package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// WriteFileOptions contains input parameters for WriteFile
type WriteFileOptions struct {
	Path        string
	Content     string
	CreateDirs  bool
	Append      bool
	AllowedDirs []string
}

// WriteFileResult represents the result of a write operation
type WriteFileResult struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Created bool   `json:"created"`
	Backup  string `json:"backup,omitempty"`
	Message string `json:"message"`
}

// WriteFile writes content to a file
func WriteFile(opts WriteFileOptions) (*WriteFileResult, error) {
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

	// Create parent directories if needed
	if opts.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directories: %w", err)
		}
	}

	// Determine if file exists (for created flag)
	_, err = os.Stat(normalizedPath)
	created := os.IsNotExist(err)

	// Set file flags based on mode
	flags := os.O_WRONLY | os.O_CREATE
	if opts.Append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	// Write file
	file, err := os.OpenFile(normalizedPath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	n, err := file.WriteString(opts.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to write: %w", err)
	}

	return &WriteFileResult{
		Path:    normalizedPath,
		Size:    int64(n),
		Created: created,
		Message: "File written successfully",
	}, nil
}

// LargeWriteFileOptions contains input parameters for LargeWriteFile
type LargeWriteFileOptions struct {
	Path        string
	Content     string
	CreateDirs  bool
	Append      bool
	Backup      bool
	VerifyWrite bool
	AllowedDirs []string
}

// LargeWriteFile writes large files with backup and verification
func LargeWriteFile(opts LargeWriteFileOptions) (*WriteFileResult, error) {
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

	// Create parent directories if needed
	if opts.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directories: %w", err)
		}
	}

	var backupPath string
	_, err = os.Stat(normalizedPath)
	fileExists := err == nil

	// Create backup if file exists and backup requested
	if fileExists && opts.Backup && !opts.Append {
		backupPath = createBackupFile(normalizedPath)
	}

	// Write to temp file first (atomic write)
	tempPath := normalizedPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	n, err := tempFile.WriteString(opts.Content)
	if err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to write: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to sync: %w", err)
	}
	tempFile.Close()

	// Verify write if requested
	if opts.VerifyWrite {
		written, err := os.ReadFile(tempPath)
		if err != nil || string(written) != opts.Content {
			os.Remove(tempPath)
			return nil, fmt.Errorf("write verification failed")
		}
	}

	// Atomic rename
	if err := os.Rename(tempPath, normalizedPath); err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to rename temp file: %w", err)
	}

	return &WriteFileResult{
		Path:    normalizedPath,
		Size:    int64(n),
		Created: !fileExists,
		Backup:  backupPath,
		Message: "File written successfully",
	}, nil
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

// GetFileInfoOptions contains input parameters for GetFileInfo
type GetFileInfoOptions struct {
	Path        string
	AllowedDirs []string
}

// GetFileInfoResult represents file information
type GetFileInfoResult struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Mode     string `json:"mode"`
	Modified string `json:"modified"`
}

// GetFileInfo gets detailed information about a file or directory
func GetFileInfo(opts GetFileInfoOptions) (*GetFileInfoResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	return &GetFileInfoResult{
		Path:     normalizedPath,
		Name:     info.Name(),
		Size:     info.Size(),
		IsDir:    info.IsDir(),
		Mode:     info.Mode().String(),
		Modified: info.ModTime().Format(time.RFC3339),
	}, nil
}

// CreateDirectoryOptions contains input parameters for CreateDirectory
type CreateDirectoryOptions struct {
	Path        string
	Recursive   bool
	AllowedDirs []string
}

// CreateDirectoryResult represents the result of creating a directory
type CreateDirectoryResult struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
	Message string `json:"message"`
}

// CreateDirectory creates a directory
func CreateDirectory(opts CreateDirectoryOptions) (*CreateDirectoryResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	if opts.Recursive {
		err = os.MkdirAll(normalizedPath, 0755)
	} else {
		err = os.Mkdir(normalizedPath, 0755)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &CreateDirectoryResult{
		Path:    normalizedPath,
		Created: true,
		Message: "Directory created successfully",
	}, nil
}
