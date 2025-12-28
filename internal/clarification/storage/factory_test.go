package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDetectStorageType(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    StorageType
		wantErr bool
		errType string
	}{
		{"yaml extension", "/path/to/file.yaml", StorageTypeYAML, false, ""},
		{"yml extension", "/path/to/file.yml", StorageTypeYAML, false, ""},
		{"YAML uppercase", "/path/to/file.YAML", StorageTypeYAML, false, ""},
		{"db extension", "/path/to/file.db", StorageTypeSQLite, false, ""},
		{"sqlite extension", "/path/to/file.sqlite", StorageTypeSQLite, false, ""},
		{"sqlite3 extension", "/path/to/file.sqlite3", StorageTypeSQLite, false, ""},
		{"DB uppercase", "/path/to/file.DB", StorageTypeSQLite, false, ""},
		{"txt extension", "/path/to/file.txt", "", true, "UnsupportedBackendError"},
		{"json extension", "/path/to/file.json", "", true, "UnsupportedBackendError"},
		{"no extension", "/path/to/file", "", true, "UnsupportedBackendError"},
		{"empty path", "", "", true, "ErrInvalidPath"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectStorageType(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if tt.errType == "ErrInvalidPath" && err != ErrInvalidPath {
					t.Errorf("expected ErrInvalidPath, got %T", err)
				}
				if tt.errType == "UnsupportedBackendError" && !isUnsupportedBackendError(err) {
					t.Errorf("expected UnsupportedBackendError, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewStorage_YAML(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	t.Run("yaml extension creates YAMLStorage", func(t *testing.T) {
		path := filepath.Join(dir, "test.yaml")
		storage, err := NewStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewStorage failed: %v", err)
		}
		defer storage.Close()

		_, ok := storage.(*YAMLStorage)
		if !ok {
			t.Errorf("expected *YAMLStorage, got %T", storage)
		}
	})

	t.Run("yml extension creates YAMLStorage", func(t *testing.T) {
		path := filepath.Join(dir, "test.yml")
		storage, err := NewStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewStorage failed: %v", err)
		}
		defer storage.Close()

		_, ok := storage.(*YAMLStorage)
		if !ok {
			t.Errorf("expected *YAMLStorage, got %T", storage)
		}
	})
}

func TestNewStorage_SQLite(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	t.Run("db extension creates SQLiteStorage", func(t *testing.T) {
		path := filepath.Join(dir, "test.db")
		storage, err := NewStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewStorage failed: %v", err)
		}
		defer storage.Close()

		_, ok := storage.(*SQLiteStorage)
		if !ok {
			t.Errorf("expected *SQLiteStorage, got %T", storage)
		}
	})

	t.Run("sqlite extension creates SQLiteStorage", func(t *testing.T) {
		path := filepath.Join(dir, "test.sqlite")
		storage, err := NewStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewStorage failed: %v", err)
		}
		defer storage.Close()

		_, ok := storage.(*SQLiteStorage)
		if !ok {
			t.Errorf("expected *SQLiteStorage, got %T", storage)
		}
	})

	t.Run("sqlite3 extension creates SQLiteStorage", func(t *testing.T) {
		path := filepath.Join(dir, "test.sqlite3")
		storage, err := NewStorage(ctx, path)
		if err != nil {
			t.Fatalf("NewStorage failed: %v", err)
		}
		defer storage.Close()

		_, ok := storage.(*SQLiteStorage)
		if !ok {
			t.Errorf("expected *SQLiteStorage, got %T", storage)
		}
	})
}

func TestNewStorage_InvalidExtension(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	path := filepath.Join(dir, "test.txt")
	_, err := NewStorage(ctx, path)
	if err == nil {
		t.Error("expected error for invalid extension")
	}
	if !isUnsupportedBackendError(err) {
		t.Errorf("expected UnsupportedBackendError, got %T: %v", err, err)
	}
}

func TestNewStorage_EmptyPath(t *testing.T) {
	ctx := context.Background()
	_, err := NewStorage(ctx, "")
	if err != ErrInvalidPath {
		t.Errorf("expected ErrInvalidPath, got %v", err)
	}
}

func TestNewStorage_StorageInterfaceCompliance(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Test that both storage types work with the same interface
	paths := []string{
		filepath.Join(dir, "test.yaml"),
		filepath.Join(dir, "test.db"),
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			storage, err := NewStorage(ctx, path)
			if err != nil {
				t.Fatalf("NewStorage failed: %v", err)
			}
			defer storage.Close()

			// Test basic CRUD via interface
			entry := createTestEntry("factory-test")
			if err := storage.Create(ctx, entry); err != nil {
				t.Errorf("Create failed: %v", err)
			}

			result, err := storage.Read(ctx, "factory-test")
			if err != nil {
				t.Errorf("Read failed: %v", err)
			}
			if result.ID != "factory-test" {
				t.Errorf("ID mismatch: got %q", result.ID)
			}

			if err := storage.Delete(ctx, "factory-test"); err != nil {
				t.Errorf("Delete failed: %v", err)
			}
		})
	}
}

func TestMustNewStorage_Success(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	path := filepath.Join(dir, "must.yaml")

	// Should not panic
	storage := MustNewStorage(ctx, path)
	defer storage.Close()

	_, ok := storage.(*YAMLStorage)
	if !ok {
		t.Errorf("expected *YAMLStorage, got %T", storage)
	}
}

func TestMustNewStorage_Panic(t *testing.T) {
	ctx := context.Background()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid path")
		}
	}()

	MustNewStorage(ctx, "")
}

func TestStorageType_Constants(t *testing.T) {
	if StorageTypeYAML != "yaml" {
		t.Errorf("StorageTypeYAML should be 'yaml', got %q", StorageTypeYAML)
	}
	if StorageTypeSQLite != "sqlite" {
		t.Errorf("StorageTypeSQLite should be 'sqlite', got %q", StorageTypeSQLite)
	}
}
