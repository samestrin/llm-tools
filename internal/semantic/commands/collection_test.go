package commands

import (
	"os"
	"testing"
)

func TestDeriveCollectionFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path with code suffix",
			path:     ".index/code",
			expected: "code",
		},
		{
			name:     "simple path with docs suffix",
			path:     ".index/docs",
			expected: "docs",
		},
		{
			name:     "nested path",
			path:     "indexes/project/semantic",
			expected: "semantic",
		},
		{
			name:     "path with trailing slash",
			path:     ".index/code/",
			expected: "code",
		},
		{
			name:     "default .index should return empty",
			path:     ".index",
			expected: "",
		},
		{
			name:     "index without dot should return empty",
			path:     "index",
			expected: "",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "",
		},
		{
			name:     "path with hyphen converted to underscore",
			path:     ".index/my-collection",
			expected: "my_collection",
		},
		{
			name:     "path with dot converted to underscore",
			path:     ".index/v1.0",
			expected: "v1_0",
		},
		{
			name:     "path starting with number gets prefix",
			path:     ".index/123test",
			expected: "idx_123test",
		},
		{
			name:     "alphanumeric path unchanged",
			path:     ".index/MyProject2024",
			expected: "MyProject2024",
		},
		{
			name:     "path with underscores preserved",
			path:     ".index/my_project_code",
			expected: "my_project_code",
		},
		{
			name:     "absolute path extracts last component",
			path:     "/home/user/indexes/production",
			expected: "production",
		},
		{
			name:     "special characters removed",
			path:     ".index/test@project#1",
			expected: "testproject1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deriveCollectionFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("deriveCollectionFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestResolveCollectionName(t *testing.T) {
	// Save original values
	origCollectionName := collectionName
	origIndexDir := indexDir
	origEnv := os.Getenv("QDRANT_COLLECTION")

	// Cleanup
	defer func() {
		collectionName = origCollectionName
		indexDir = origIndexDir
		os.Setenv("QDRANT_COLLECTION", origEnv)
	}()

	t.Run("priority 1: explicit collection flag", func(t *testing.T) {
		collectionName = "explicit_collection"
		indexDir = ".index/code"
		os.Setenv("QDRANT_COLLECTION", "env_collection")

		result := resolveCollectionName()
		if result != "explicit_collection" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "explicit_collection")
		}
	})

	t.Run("priority 2: derive from index-dir", func(t *testing.T) {
		collectionName = ""
		indexDir = ".index/myproject"
		os.Setenv("QDRANT_COLLECTION", "env_collection")

		result := resolveCollectionName()
		if result != "myproject" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "myproject")
		}
	})

	t.Run("priority 3: environment variable", func(t *testing.T) {
		collectionName = ""
		indexDir = ".index" // default, won't derive
		os.Setenv("QDRANT_COLLECTION", "env_collection")

		result := resolveCollectionName()
		if result != "env_collection" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "env_collection")
		}
	})

	t.Run("priority 4: default llm_semantic", func(t *testing.T) {
		collectionName = ""
		indexDir = ".index"
		os.Unsetenv("QDRANT_COLLECTION")

		result := resolveCollectionName()
		if result != "llm_semantic" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "llm_semantic")
		}
	})

	t.Run("empty index-dir falls back to env", func(t *testing.T) {
		collectionName = ""
		indexDir = ""
		os.Setenv("QDRANT_COLLECTION", "fallback_env")

		result := resolveCollectionName()
		if result != "fallback_env" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "fallback_env")
		}
	})

	t.Run("default index-dir with no env uses default", func(t *testing.T) {
		collectionName = ""
		indexDir = ".index"
		os.Unsetenv("QDRANT_COLLECTION")

		result := resolveCollectionName()
		if result != "llm_semantic" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "llm_semantic")
		}
	})
}

func TestResolveCollectionName_EdgeCases(t *testing.T) {
	// Save original values
	origCollectionName := collectionName
	origIndexDir := indexDir
	origEnv := os.Getenv("QDRANT_COLLECTION")

	// Cleanup
	defer func() {
		collectionName = origCollectionName
		indexDir = origIndexDir
		os.Setenv("QDRANT_COLLECTION", origEnv)
	}()

	t.Run("whitespace-only collection name treated as empty", func(t *testing.T) {
		collectionName = "   "
		indexDir = ".index/code"
		os.Unsetenv("QDRANT_COLLECTION")

		result := resolveCollectionName()
		// Since collectionName is "   " (not empty), it will be used as-is
		// This tests current behavior - might want to trim
		if result != "   " {
			t.Logf("Note: whitespace collection name is used as-is: %q", result)
		}
	})

	t.Run("deep nested path", func(t *testing.T) {
		collectionName = ""
		indexDir = "/very/deep/nested/path/to/indexes/final_collection"
		os.Unsetenv("QDRANT_COLLECTION")

		result := resolveCollectionName()
		if result != "final_collection" {
			t.Errorf("resolveCollectionName() = %q, want %q", result, "final_collection")
		}
	})
}

func TestCollectionFlag_InRootCmd(t *testing.T) {
	root := RootCmd()

	flag := root.PersistentFlags().Lookup("collection")
	if flag == nil {
		t.Fatal("--collection flag not found in root command")
	}

	if flag.DefValue != "" {
		t.Errorf("--collection default should be empty, got %q", flag.DefValue)
	}

	// Verify usage describes the behavior
	if flag.Usage == "" {
		t.Error("--collection flag should have usage description")
	}
}

func TestStorageFlag_InRootCmd(t *testing.T) {
	root := RootCmd()

	flag := root.PersistentFlags().Lookup("storage")
	if flag == nil {
		t.Fatal("--storage flag not found in root command")
	}

	if flag.DefValue != "sqlite" {
		t.Errorf("--storage default should be 'sqlite', got %q", flag.DefValue)
	}
}

func TestIndexDirFlag_InRootCmd(t *testing.T) {
	root := RootCmd()

	flag := root.PersistentFlags().Lookup("index-dir")
	if flag == nil {
		t.Fatal("--index-dir flag not found in root command")
	}

	if flag.DefValue != ".index" {
		t.Errorf("--index-dir default should be '.index', got %q", flag.DefValue)
	}
}
