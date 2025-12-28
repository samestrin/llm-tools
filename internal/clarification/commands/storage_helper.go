package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/samestrin/llm-tools/internal/clarification/storage"
	"github.com/samestrin/llm-tools/internal/clarification/tracking"
)

// GetStorage creates a Storage instance for the given file path.
// It uses the factory pattern to detect storage type from file extension.
// The caller is responsible for calling Close() on the returned Storage.
func GetStorage(ctx context.Context, filePath string) (storage.Storage, error) {
	effectivePath := GetDBPath(filePath)
	if effectivePath == "" {
		return nil, fmt.Errorf("no storage path specified (use --file or --db flag)")
	}

	return storage.NewStorage(ctx, effectivePath)
}

// GetStorageOrError creates a Storage instance, checking file existence first.
// For YAML files, it checks if the file exists.
// For SQLite files, it creates the database if it doesn't exist.
func GetStorageOrError(ctx context.Context, filePath string) (storage.Storage, error) {
	effectivePath := GetDBPath(filePath)
	if effectivePath == "" {
		return nil, fmt.Errorf("no storage path specified (use --file or --db flag)")
	}

	// Detect storage type
	storageType, err := storage.DetectStorageType(effectivePath)
	if err != nil {
		return nil, err
	}

	// For YAML, check file exists first (backward compatible behavior)
	if storageType == storage.StorageTypeYAML {
		if !tracking.FileExists(effectivePath) {
			return nil, fmt.Errorf("tracking file not found: %s", effectivePath)
		}
	}

	return storage.NewStorage(ctx, effectivePath)
}

// FileOrDBExists checks if the storage file exists.
// For YAML files, uses os.Stat.
// For SQLite files, uses os.Stat.
func FileOrDBExists(filePath string) bool {
	effectivePath := GetDBPath(filePath)
	if effectivePath == "" {
		return false
	}
	_, err := os.Stat(effectivePath)
	return err == nil
}

// LoadEntriesFromStorage loads all entries from storage.
// This is a convenience function for commands that need to load all entries.
func LoadEntriesFromStorage(ctx context.Context, filePath string) ([]tracking.Entry, error) {
	store, err := GetStorageOrError(ctx, filePath)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	return store.List(ctx, storage.ListFilter{})
}
