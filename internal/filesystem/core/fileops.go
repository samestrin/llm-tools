package core

import (
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

// CopyFileOptions contains input parameters for CopyFile
type CopyFileOptions struct {
	Source      string
	Destination string
	AllowedDirs []string
}

// CopyFile copies a file or directory
func CopyFile(opts CopyFileOptions) (*FileOpResult, error) {
	if opts.Source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if opts.Destination == "" {
		return nil, fmt.Errorf("destination is required")
	}

	// Normalize and validate paths
	srcPath, err := NormalizePath(opts.Source)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}

	dstPath, err := NormalizePath(opts.Destination)
	if err != nil {
		return nil, fmt.Errorf("invalid destination path: %w", err)
	}

	if err := ValidatePath(srcPath, opts.AllowedDirs); err != nil {
		return nil, err
	}
	if err := ValidatePath(dstPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Check source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("source does not exist: %w", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	if srcInfo.IsDir() {
		if err := copyDir(srcPath, dstPath); err != nil {
			return nil, fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := copyFile(srcPath, dstPath); err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}
	}

	return &FileOpResult{
		Source:      srcPath,
		Destination: dstPath,
		Success:     true,
		Operation:   "copy",
		Message:     "Copy completed successfully",
	}, nil
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

// MoveFileOptions contains input parameters for MoveFile
type MoveFileOptions struct {
	Source      string
	Destination string
	AllowedDirs []string
}

// MoveFile moves or renames a file or directory
func MoveFile(opts MoveFileOptions) (*FileOpResult, error) {
	if opts.Source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if opts.Destination == "" {
		return nil, fmt.Errorf("destination is required")
	}

	// Normalize and validate paths
	srcPath, err := NormalizePath(opts.Source)
	if err != nil {
		return nil, fmt.Errorf("invalid source path: %w", err)
	}

	dstPath, err := NormalizePath(opts.Destination)
	if err != nil {
		return nil, fmt.Errorf("invalid destination path: %w", err)
	}

	if err := ValidatePath(srcPath, opts.AllowedDirs); err != nil {
		return nil, err
	}
	if err := ValidatePath(dstPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Check source exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("source does not exist: %w", err)
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Try rename first (works across same filesystem)
	if err := os.Rename(srcPath, dstPath); err != nil {
		// Fallback to copy + delete (cross-filesystem)
		if srcInfo.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return nil, fmt.Errorf("failed to move directory: %w", err)
			}
			os.RemoveAll(srcPath)
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return nil, fmt.Errorf("failed to move file: %w", err)
			}
			os.Remove(srcPath)
		}
	}

	return &FileOpResult{
		Source:      srcPath,
		Destination: dstPath,
		Success:     true,
		Operation:   "move",
		Message:     "Move completed successfully",
	}, nil
}

// DeleteFileOptions contains input parameters for DeleteFile
type DeleteFileOptions struct {
	Path        string
	Recursive   bool
	AllowedDirs []string
}

// DeleteFile deletes a file or directory
func DeleteFile(opts DeleteFileOptions) (*FileOpResult, error) {
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

	// Check if exists
	info, err := os.Stat(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	if info.IsDir() {
		if opts.Recursive {
			if err := os.RemoveAll(normalizedPath); err != nil {
				return nil, fmt.Errorf("failed to delete directory: %w", err)
			}
		} else {
			if err := os.Remove(normalizedPath); err != nil {
				return nil, fmt.Errorf("failed to delete directory (use recursive=true for non-empty): %w", err)
			}
		}
	} else {
		if err := os.Remove(normalizedPath); err != nil {
			return nil, fmt.Errorf("failed to delete file: %w", err)
		}
	}

	return &FileOpResult{
		Path:      normalizedPath,
		Success:   true,
		Operation: "delete",
		Message:   "Delete completed successfully",
	}, nil
}

// BatchOperation represents a single batch operation
type BatchOperation struct {
	Operation   string `json:"operation"`
	Source      string `json:"source"`
	Destination string `json:"destination,omitempty"`
	Recursive   bool   `json:"recursive,omitempty"`
}

// BatchFileOperationsOptions contains input parameters for BatchFileOperations
type BatchFileOperationsOptions struct {
	Operations  []BatchOperation
	AllowedDirs []string
}

// BatchOpResult represents the result of batch operations
type BatchOpResult struct {
	Success    int            `json:"success"`
	Failed     int            `json:"failed"`
	Operations []FileOpResult `json:"operations"`
}

// BatchFileOperations performs multiple file operations
func BatchFileOperations(opts BatchFileOperationsOptions) (*BatchOpResult, error) {
	var results []FileOpResult
	success := 0
	failed := 0

	for _, op := range opts.Operations {
		var result *FileOpResult
		var err error

		switch op.Operation {
		case "copy":
			result, err = CopyFile(CopyFileOptions{
				Source:      op.Source,
				Destination: op.Destination,
				AllowedDirs: opts.AllowedDirs,
			})
		case "move":
			result, err = MoveFile(MoveFileOptions{
				Source:      op.Source,
				Destination: op.Destination,
				AllowedDirs: opts.AllowedDirs,
			})
		case "delete":
			result, err = DeleteFile(DeleteFileOptions{
				Path:        op.Source,
				Recursive:   op.Recursive,
				AllowedDirs: opts.AllowedDirs,
			})
		default:
			err = fmt.Errorf("unknown operation: %s", op.Operation)
		}

		opResult := FileOpResult{
			Operation: op.Operation,
		}

		if err != nil {
			opResult.Success = false
			opResult.Message = err.Error()
			failed++
		} else {
			opResult = *result
			success++
		}

		results = append(results, opResult)
	}

	return &BatchOpResult{
		Success:    success,
		Failed:     failed,
		Operations: results,
	}, nil
}
