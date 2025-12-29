package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/samestrin/llm-tools/internal/semantic"
)

// Config holds configuration for the MCP handlers
type Config struct {
	APIURL   string
	Model    string
	APIKey   string
	IndexDir string
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		APIURL:   getEnvOrDefault("LLM_SEMANTIC_API_URL", "http://localhost:11434"),
		Model:    getEnvOrDefault("LLM_SEMANTIC_MODEL", "mxbai-embed-large"),
		APIKey:   os.Getenv("LLM_SEMANTIC_API_KEY"),
		IndexDir: getEnvOrDefault("LLM_SEMANTIC_INDEX_DIR", ".llm-index"),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// Handler processes MCP tool calls
type Handler struct {
	config Config
}

// NewHandler creates a new MCP handler
func NewHandler(cfg Config) *Handler {
	return &Handler{config: cfg}
}

// HandleSearch handles the llm_semantic_search tool
func (h *Handler) HandleSearch(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Parse options
	topK := 10
	if v, ok := args["top_k"].(float64); ok {
		topK = int(v)
	}

	var threshold float32
	if v, ok := args["threshold"].(float64); ok {
		threshold = float32(v)
	}

	typeFilter, _ := args["type"].(string)
	pathFilter, _ := args["path"].(string)
	minOutput, _ := args["min"].(bool)

	// Find and open index
	indexPath, err := h.findIndexPath()
	if err != nil {
		return "", err
	}

	storage, err := semantic.NewSQLiteStorage(indexPath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open index: %w", err)
	}
	defer storage.Close()

	// Create embedder
	embedder, err := semantic.NewEmbedder(semantic.EmbedderConfig{
		APIURL: h.config.APIURL,
		Model:  h.config.Model,
		APIKey: h.config.APIKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create embedder: %w", err)
	}

	// Perform search
	searcher := semantic.NewSearcher(storage, embedder)
	results, err := searcher.Search(ctx, query, semantic.SearchOptions{
		TopK:       topK,
		Threshold:  threshold,
		Type:       typeFilter,
		PathFilter: pathFilter,
	})
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	// Format output
	if minOutput {
		return h.formatMinimalResults(results)
	}
	return h.formatJSONResults(results)
}

// HandleStatus handles the llm_semantic_status tool
func (h *Handler) HandleStatus(ctx context.Context, args map[string]interface{}) (string, error) {
	indexPath, err := h.findIndexPath()
	if err != nil {
		return h.formatJSON(map[string]interface{}{
			"indexed": false,
			"error":   err.Error(),
		})
	}

	storage, err := semantic.NewSQLiteStorage(indexPath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open index: %w", err)
	}
	defer storage.Close()

	stats, err := storage.Stats(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get stats: %w", err)
	}

	return h.formatJSON(map[string]interface{}{
		"indexed":       true,
		"path":          indexPath,
		"files_indexed": stats.FilesIndexed,
		"chunks_total":  stats.ChunksTotal,
		"last_updated":  stats.LastUpdated,
	})
}

// HandleIndex handles the llm_semantic_index tool
func (h *Handler) HandleIndex(ctx context.Context, args map[string]interface{}) (string, error) {
	path := "."
	if v, ok := args["path"].(string); ok && v != "" {
		path = v
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Parse options
	var includes, excludes []string
	if v, ok := args["include"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				includes = append(includes, s)
			}
		}
	}
	if v, ok := args["exclude"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				excludes = append(excludes, s)
			}
		}
	}
	if len(excludes) == 0 {
		excludes = []string{"vendor", "node_modules", ".git"}
	}
	force, _ := args["force"].(bool)

	// Create index path
	indexPath := h.resolveIndexPath(absPath)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create index directory: %w", err)
	}

	// Create components
	storage, err := semantic.NewSQLiteStorage(indexPath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	embedder, err := semantic.NewEmbedder(semantic.EmbedderConfig{
		APIURL: h.config.APIURL,
		Model:  h.config.Model,
		APIKey: h.config.APIKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create embedder: %w", err)
	}

	factory := semantic.NewChunkerFactory()
	factory.Register("go", semantic.NewGoChunker())

	// Register JS/TS chunker
	jsChunker := semantic.NewJSChunker()
	for _, ext := range jsChunker.SupportedExtensions() {
		factory.Register(ext, jsChunker)
	}

	// Register Python chunker
	pyChunker := semantic.NewPythonChunker()
	for _, ext := range pyChunker.SupportedExtensions() {
		factory.Register(ext, pyChunker)
	}

	// Register PHP chunker
	phpChunker := semantic.NewPHPChunker()
	for _, ext := range phpChunker.SupportedExtensions() {
		factory.Register(ext, phpChunker)
	}

	// Register generic chunker for other file types
	generic := semantic.NewGenericChunker(2000)
	for _, ext := range generic.SupportedExtensions() {
		factory.Register(ext, generic)
	}

	mgr := semantic.NewIndexManager(storage, embedder, factory)

	result, err := mgr.Index(ctx, absPath, semantic.IndexOptions{
		Includes: includes,
		Excludes: excludes,
		Force:    force,
	})
	if err != nil {
		return "", fmt.Errorf("indexing failed: %w", err)
	}

	return h.formatJSON(result)
}

// HandleUpdate handles the llm_semantic_update tool
func (h *Handler) HandleUpdate(ctx context.Context, args map[string]interface{}) (string, error) {
	path := "."
	if v, ok := args["path"].(string); ok && v != "" {
		path = v
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	indexPath, err := h.findIndexPath()
	if err != nil {
		return "", err
	}

	// Parse options
	var includes, excludes []string
	if v, ok := args["include"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				includes = append(includes, s)
			}
		}
	}
	if v, ok := args["exclude"].([]interface{}); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				excludes = append(excludes, s)
			}
		}
	}
	if len(excludes) == 0 {
		excludes = []string{"vendor", "node_modules", ".git"}
	}

	// Create components
	storage, err := semantic.NewSQLiteStorage(indexPath, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open storage: %w", err)
	}
	defer storage.Close()

	embedder, err := semantic.NewEmbedder(semantic.EmbedderConfig{
		APIURL: h.config.APIURL,
		Model:  h.config.Model,
		APIKey: h.config.APIKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create embedder: %w", err)
	}

	factory := semantic.NewChunkerFactory()
	factory.Register("go", semantic.NewGoChunker())

	// Register JS/TS chunker
	jsChunker := semantic.NewJSChunker()
	for _, ext := range jsChunker.SupportedExtensions() {
		factory.Register(ext, jsChunker)
	}

	// Register Python chunker
	pyChunker := semantic.NewPythonChunker()
	for _, ext := range pyChunker.SupportedExtensions() {
		factory.Register(ext, pyChunker)
	}

	// Register PHP chunker
	phpChunker := semantic.NewPHPChunker()
	for _, ext := range phpChunker.SupportedExtensions() {
		factory.Register(ext, phpChunker)
	}

	// Register generic chunker for other file types
	generic := semantic.NewGenericChunker(2000)
	for _, ext := range generic.SupportedExtensions() {
		factory.Register(ext, generic)
	}

	mgr := semantic.NewIndexManager(storage, embedder, factory)

	result, err := mgr.Update(ctx, absPath, semantic.UpdateOptions{
		Includes: includes,
		Excludes: excludes,
	})
	if err != nil {
		return "", fmt.Errorf("update failed: %w", err)
	}

	return h.formatJSON(result)
}

// Helper methods

func (h *Handler) findIndexPath() (string, error) {
	// Try specified index directory
	if h.config.IndexDir != "" {
		path := filepath.Join(h.config.IndexDir, "semantic.db")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try .llm-index in current directory
	path := filepath.Join(".llm-index", "semantic.db")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	// Try git root
	if gitRoot, err := findGitRoot(); err == nil {
		path := filepath.Join(gitRoot, ".llm-index", "semantic.db")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("semantic index not found. Run llm_semantic_index first")
}

func (h *Handler) resolveIndexPath(rootPath string) string {
	if h.config.IndexDir != "" && h.config.IndexDir != ".llm-index" {
		return filepath.Join(h.config.IndexDir, "semantic.db")
	}

	if gitRoot, err := findGitRootFrom(rootPath); err == nil {
		return filepath.Join(gitRoot, ".llm-index", "semantic.db")
	}

	return filepath.Join(rootPath, ".llm-index", "semantic.db")
}

func findGitRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findGitRootFrom(dir)
}

func findGitRootFrom(startPath string) (string, error) {
	dir := startPath
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not in a git repository")
		}
		dir = parent
	}
}

func (h *Handler) formatMinimalResults(results []semantic.SearchResult) (string, error) {
	minResults := make([]map[string]interface{}, len(results))
	for i, r := range results {
		minResults[i] = map[string]interface{}{
			"file":  r.Chunk.FilePath,
			"name":  r.Chunk.Name,
			"line":  r.Chunk.StartLine,
			"score": r.Score,
		}
	}
	return h.formatJSON(minResults)
}

func (h *Handler) formatJSONResults(results []semantic.SearchResult) (string, error) {
	return h.formatJSON(results)
}

func (h *Handler) formatJSON(v interface{}) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
