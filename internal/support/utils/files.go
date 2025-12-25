package utils

import (
	"os"
	"path/filepath"
)

// IsValidFilePath checks if a path points to an existing file
func IsValidFilePath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsValidDirPath checks if a path points to an existing directory
func IsValidDirPath(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ExpandPath expands ~ to home directory and resolves to absolute path
func ExpandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Abs(path)
}
