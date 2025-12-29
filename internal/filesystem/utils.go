package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath checks if a path is allowed based on the allowed directories list
func ValidatePath(path string, allowedDirs []string) error {
	// Empty allowed dirs means all paths are allowed
	if len(allowedDirs) == 0 {
		return nil
	}

	// Normalize the path
	normalized, err := NormalizePath(path)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}

	// Check if path is within any allowed directory
	if !IsPathAllowed(normalized, allowedDirs) {
		return fmt.Errorf("path %q is not within allowed directories", path)
	}

	return nil
}

// NormalizePath cleans and normalizes a file path
func NormalizePath(path string) (string, error) {
	// Expand home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Clean the path (resolves . and .., removes double slashes)
	cleaned := filepath.Clean(path)

	// Convert to absolute path if relative
	if !filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		cleaned = abs
	}

	return cleaned, nil
}

// IsPathAllowed checks if a path is within any of the allowed directories
func IsPathAllowed(path string, allowedDirs []string) bool {
	// Empty allowed dirs means all paths are allowed
	if len(allowedDirs) == 0 {
		return true
	}

	// Normalize the path for comparison
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return false
	}

	for _, dir := range allowedDirs {
		normalizedDir, err := NormalizePath(dir)
		if err != nil {
			continue
		}

		// Check if path is exactly the allowed dir or starts with it followed by separator
		if normalizedPath == normalizedDir {
			return true
		}
		if strings.HasPrefix(normalizedPath, normalizedDir+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// ResolveSymlink resolves a symlink to its target path
func ResolveSymlink(path string) (string, error) {
	// Get file info following symlinks
	info, err := os.Lstat(path)
	if err != nil {
		return path, nil // Return original path if can't stat
	}

	// If it's a symlink, resolve it
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve symlink: %w", err)
		}
		return target, nil
	}

	return path, nil
}

// CreateBackupPath generates a backup path with timestamp
func CreateBackupPath(path string) string {
	// Format: filename.bak.YYYYMMDD_HHMMSS
	// We'll use the current time when the backup is actually created
	return path + ".bak"
}

// EnsureDir creates directory and parents if needed
func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

// GetString extracts a string from args map with default value
func GetString(args map[string]interface{}, key string, defaultVal string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return defaultVal
}

// GetBool extracts a bool from args map with default value
func GetBool(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}

// GetInt extracts an int from args map with default value
func GetInt(args map[string]interface{}, key string, defaultVal int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return defaultVal
}

// GetStringSlice extracts a string slice from args map
func GetStringSlice(args map[string]interface{}, key string) []string {
	if v, ok := args[key].([]interface{}); ok {
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}
