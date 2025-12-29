package semantic

import (
	"path/filepath"
	"strings"
	"sync"
)

// Chunker is the interface for language-specific code chunking
type Chunker interface {
	// Chunk breaks source code into semantic chunks
	Chunk(path string, content []byte) ([]Chunk, error)

	// SupportedExtensions returns the file extensions this chunker handles
	SupportedExtensions() []string
}

// ChunkerFactory manages language-specific chunkers
type ChunkerFactory struct {
	mu       sync.RWMutex
	chunkers map[string]Chunker
}

// NewChunkerFactory creates a new chunker factory
func NewChunkerFactory() *ChunkerFactory {
	return &ChunkerFactory{
		chunkers: make(map[string]Chunker),
	}
}

// Register adds a chunker for a specific language/extension
func (f *ChunkerFactory) Register(ext string, chunker Chunker) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunkers[ext] = chunker
}

// GetChunker returns a chunker for the specified language/extension
func (f *ChunkerFactory) GetChunker(ext string) (Chunker, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	chunker, ok := f.chunkers[ext]
	return chunker, ok
}

// GetByExtension returns the appropriate chunker based on filename extension
func (f *ChunkerFactory) GetByExtension(filename string) (Chunker, bool) {
	ext := LanguageFromExtension(filename)
	if ext == "" {
		return nil, false
	}
	return f.GetChunker(ext)
}

// SupportedExtensions returns all registered extensions
func (f *ChunkerFactory) SupportedExtensions() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	exts := make([]string, 0, len(f.chunkers))
	for ext := range f.chunkers {
		exts = append(exts, ext)
	}
	return exts
}

// LanguageFromExtension extracts the language identifier from a filename
func LanguageFromExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return ""
	}
	// Remove leading dot
	return strings.TrimPrefix(ext, ".")
}
