package storage

import (
	"context"
	"path/filepath"
	"strings"
)

// StorageType represents the type of storage backend.
type StorageType string

const (
	// StorageTypeYAML represents YAML file storage.
	StorageTypeYAML StorageType = "yaml"
	// StorageTypeSQLite represents SQLite database storage.
	StorageTypeSQLite StorageType = "sqlite"
)

// NewStorage creates a Storage instance based on the file extension.
// Supported extensions:
//   - .yaml, .yml -> YAMLStorage
//   - .db, .sqlite, .sqlite3 -> SQLiteStorage
func NewStorage(ctx context.Context, path string) (Storage, error) {
	storageType, err := DetectStorageType(path)
	if err != nil {
		return nil, err
	}

	switch storageType {
	case StorageTypeYAML:
		return NewYAMLStorage(ctx, path)
	case StorageTypeSQLite:
		return NewSQLiteStorage(ctx, path)
	default:
		return nil, &UnsupportedBackendError{Extension: filepath.Ext(path)}
	}
}

// DetectStorageType determines the storage type from the file path extension.
func DetectStorageType(path string) (StorageType, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return StorageTypeYAML, nil
	case ".db", ".sqlite", ".sqlite3":
		return StorageTypeSQLite, nil
	default:
		return "", &UnsupportedBackendError{Extension: ext}
	}
}

// MustNewStorage is like NewStorage but panics on error.
// Use only in initialization code where panicking is acceptable.
func MustNewStorage(ctx context.Context, path string) Storage {
	storage, err := NewStorage(ctx, path)
	if err != nil {
		panic("storage: " + err.Error())
	}
	return storage
}
