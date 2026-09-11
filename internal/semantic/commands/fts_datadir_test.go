package commands

import (
	"path/filepath"
	"testing"
)

func TestResolveFTSDataDir(t *testing.T) {
	origIndexDir := indexDir
	defer func() { indexDir = origIndexDir }()

	t.Run("uses the index file's own directory", func(t *testing.T) {
		indexDir = ".index"
		want := filepath.Join("some", "place")
		if got := resolveFTSDataDir(filepath.Join(want, "semantic.db")); got != want {
			t.Errorf("resolveFTSDataDir = %q, want %q", got, want)
		}
	})

	t.Run("honours a custom index dir", func(t *testing.T) {
		indexDir = "custom-index"
		if got := resolveFTSDataDir(""); got != "custom-index" {
			t.Errorf("resolveFTSDataDir = %q, want %q", got, "custom-index")
		}
	})

	// Qdrant passes an empty index path. Returning the working directory there
	// is what put the database in the repository root.
	t.Run("never resolves to the working directory", func(t *testing.T) {
		indexDir = ".index"
		got := resolveFTSDataDir("")
		if got == "." || got == "" {
			t.Fatalf("resolveFTSDataDir = %q; the database would land in the repository root", got)
		}
		if filepath.Base(got) != ".index" {
			t.Errorf("resolveFTSDataDir = %q, want a path ending in .index", got)
		}
	})
}
