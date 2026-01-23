package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/samestrin/llm-tools/internal/semantic/config"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	apiURL         string
	model          string
	apiKey         string
	indexDir       string
	storageType    string // "sqlite" (default) or "qdrant"
	collectionName string // Qdrant collection name (default: from env or "llm_semantic")
	embedderType   string // "openai" (default), "cohere", "huggingface", "openrouter"

	// Reranker configuration
	rerankerAPIURL string // Reranker API URL (optional, enables reranking when set)
	rerankerModel  string // Reranker model name

	// Config file support
	configPath string // Path to YAML config file
	profile    string // Profile name: code (default), docs, memory, sprints

	// Loaded config (cached after first load)
	loadedConfig *config.SemanticConfig

	// Global output flags accessible to all commands
	GlobalJSONOutput bool
	GlobalMinOutput  bool
)

// getDefaultAPIURL returns the API URL from environment or default
func getDefaultAPIURL() string {
	if url := os.Getenv("LLM_SEMANTIC_API_URL"); url != "" {
		return url
	}
	return "http://localhost:11434"
}

// getDefaultModel returns the model from environment or empty (embedder-specific default)
func getDefaultModel() string {
	if model := os.Getenv("LLM_SEMANTIC_MODEL"); model != "" {
		return model
	}
	return "" // Let embedder choose default
}

// getDefaultRerankerAPIURL returns the reranker API URL from environment
func getDefaultRerankerAPIURL() string {
	return os.Getenv("LLM_SEMANTIC_RERANKER_API_URL")
}

// getDefaultRerankerModel returns the reranker model from environment or default
func getDefaultRerankerModel() string {
	if model := os.Getenv("LLM_SEMANTIC_RERANKER_MODEL"); model != "" {
		return model
	}
	return "Qwen/Qwen3-Reranker-0.6B"
}

// RootCmd returns the root command for llm-semantic
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "llm-semantic",
		Short: "Semantic code search with local embeddings",
		Long: `llm-semantic provides semantic code search using local embedding models.
Supports any OpenAI-compatible embedding API (Ollama, vLLM, OpenAI, Azure, etc.)`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Sync local command flags to global vars for error handling
			if f := cmd.Flag("json"); f != nil && f.Changed {
				GlobalJSONOutput = true
			}
			if f := cmd.Flag("min"); f != nil && f.Changed {
				GlobalMinOutput = true
			}

			// Validate config path early (reject whitespace-only paths)
			// If user explicitly set --config but with only whitespace, report error
			if configPath != "" && strings.TrimSpace(configPath) == "" {
				return config.ErrConfigPathEmpty()
			}

			// Load config file if specified
			if configPath != "" {
				cfg, err := config.LoadConfig(configPath)
				if err != nil {
					// Error is already a SemanticError with hint, just return it
					return err
				}
				loadedConfig = cfg

				// Validate profile if specified
				if profile != "" && !config.IsValidProfile(profile) {
					return config.ErrProfileNotFound(profile, config.ValidProfiles())
				}
			}

			return nil
		},
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", getDefaultAPIURL(), "Embedding API URL (OpenAI-compatible)")
	rootCmd.PersistentFlags().StringVar(&model, "model", getDefaultModel(), "Embedding model name (or set LLM_SEMANTIC_MODEL env var)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (or set LLM_SEMANTIC_API_KEY env var)")
	rootCmd.PersistentFlags().StringVar(&indexDir, "index-dir", ".index", "Directory for semantic index")
	rootCmd.PersistentFlags().StringVar(&storageType, "storage", "sqlite", "Storage backend: sqlite (default) or qdrant")
	rootCmd.PersistentFlags().StringVar(&collectionName, "collection", "", "Qdrant collection name (default: QDRANT_COLLECTION env or 'llm_semantic')")
	rootCmd.PersistentFlags().StringVar(&embedderType, "embedder", "openai", "Embedding provider: openai (default), cohere, huggingface, openrouter")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to YAML config file (reads 'semantic:' section)")
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "Profile name: code (default), docs, memory, sprints")
	rootCmd.PersistentFlags().BoolVar(&GlobalJSONOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&GlobalMinOutput, "min", false, "Minimal/token-optimized output")

	// Add subcommands
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(searchAliasCmd("code-search", "code"))
	rootCmd.AddCommand(searchAliasCmd("docs-search", "docs"))
	rootCmd.AddCommand(searchAliasCmd("memory-search", "memory"))
	rootCmd.AddCommand(multisearchCmd())
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(indexStatusCmd())
	rootCmd.AddCommand(indexUpdateCmd())
	rootCmd.AddCommand(memoryCmd())
	rootCmd.AddCommand(collectionCmd())

	return rootCmd
}

// getAPIKey returns the API key from flag or environment
func getAPIKey() string {
	if apiKey != "" {
		return apiKey
	}
	// Environment variable fallback handled by caller
	return ""
}

// resolveCollectionName returns the Qdrant collection name using this priority:
// 1. --collection flag if specified
// 2. Profile-specific config value (e.g., code_collection)
// 3. Derived from --index-dir (e.g., ".index/code" → "code", ".index/docs" → "docs")
// 4. QDRANT_COLLECTION environment variable
// 5. Default: "llm_semantic"
func resolveCollectionName() string {
	// Priority 1: explicit --collection flag
	if collectionName != "" {
		return collectionName
	}

	// Priority 2: config value (profile-specific)
	if loadedConfig != nil {
		profileCfg := loadedConfig.GetProfileConfig(profile)
		if profileCfg.Collection != "" {
			return profileCfg.Collection
		}
	}

	// Priority 3: derive from index-dir if non-default
	if indexDir != "" && indexDir != ".index" {
		// Extract the last path component as collection name
		// e.g., ".index/code" → "code", "indexes/docs" → "docs"
		derived := deriveCollectionFromPath(indexDir)
		if derived != "" {
			return derived
		}
	}

	// Priority 4: environment variable
	if envCollection := os.Getenv("QDRANT_COLLECTION"); envCollection != "" {
		return envCollection
	}

	// Priority 5: default
	return "llm_semantic"
}

// resolveStorageType returns the storage type using this priority:
// 1. --storage flag if explicitly set
// 2. Profile-specific config value (e.g., code_storage)
// 3. Default: "sqlite"
func resolveStorageType(cmd *cobra.Command) string {
	// Priority 1: explicit --storage flag
	if f := cmd.Flag("storage"); f != nil && f.Changed {
		return storageType
	}

	// Priority 2: config value (profile-specific)
	if loadedConfig != nil {
		profileCfg := loadedConfig.GetProfileConfig(profile)
		if profileCfg.Storage != "" {
			return profileCfg.Storage
		}
	}

	// Priority 3: default (already set by flag default)
	return storageType
}

// deriveCollectionFromPath extracts a collection name from a path
// Returns the last non-empty path component, sanitized for use as a collection name
func deriveCollectionFromPath(path string) string {
	// Clean the path and get the last component
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}

	// Get the last path component
	lastSlash := strings.LastIndex(path, "/")
	var name string
	if lastSlash >= 0 {
		name = path[lastSlash+1:]
	} else {
		name = path
	}

	// Skip if it's just ".index" or similar default
	if name == ".index" || name == "index" || name == "" {
		return ""
	}

	// Sanitize: replace invalid characters with underscores
	// Qdrant collection names should be alphanumeric with underscores
	var sanitized strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sanitized.WriteRune(r)
		} else if r == '-' || r == '.' {
			sanitized.WriteRune('_')
		}
	}

	result := sanitized.String()
	// Ensure it doesn't start with a number
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "idx_" + result
	}

	return result
}

// createReranker creates a reranker if configured (via env var or flag)
// Returns nil if reranking is not configured (which is valid - reranking is optional)
func createReranker() (semantic.RerankerInterface, error) {
	// Use flag value if set, otherwise fall back to env var
	apiURL := rerankerAPIURL
	if apiURL == "" {
		apiURL = getDefaultRerankerAPIURL()
	}

	// No reranker configured - this is not an error
	if apiURL == "" {
		return nil, nil
	}

	modelName := rerankerModel
	if modelName == "" {
		modelName = getDefaultRerankerModel()
	}

	cfg := semantic.RerankerConfig{
		APIURL: apiURL,
		Model:  modelName,
	}

	return semantic.NewReranker(cfg)
}

// createEmbedder creates an embedder based on the --embedder flag
func createEmbedder() (semantic.EmbedderInterface, error) {
	switch embedderType {
	case "cohere":
		key := apiKey
		if key == "" {
			key = os.Getenv("COHERE_API_KEY")
		}
		cfg := semantic.CohereConfig{
			APIKey: key,
			Model:  model,
		}
		return semantic.NewCohereEmbedder(cfg)

	case "huggingface":
		key := apiKey
		if key == "" {
			key = os.Getenv("HUGGING_FACE_API_KEY")
			if key == "" {
				key = os.Getenv("HUGGINGFACE_API_KEY")
			}
			if key == "" {
				key = os.Getenv("HF_TOKEN")
			}
		}
		cfg := semantic.HuggingFaceConfig{
			APIKey:       key,
			Model:        model,
			WaitForModel: true,
		}
		return semantic.NewHuggingFaceEmbedder(cfg)

	case "openrouter":
		key := apiKey
		if key == "" {
			key = os.Getenv("OPENROUTER_API_KEY")
		}
		cfg := semantic.OpenRouterConfig{
			APIKey: key,
			Model:  model,
		}
		return semantic.NewOpenRouterEmbedder(cfg)

	case "openai", "":
		// Default: OpenAI-compatible (includes Ollama, vLLM, OpenAI, Azure)
		key := apiKey
		if key == "" {
			key = os.Getenv("LLM_SEMANTIC_API_KEY")
			if key == "" {
				key = os.Getenv("OPENAI_API_KEY")
			}
		}
		modelName := model
		if modelName == "" {
			modelName = "nomic-embed-text" // Default for Ollama - 8K context, fast, good for code
		}
		cfg := semantic.EmbedderConfig{
			APIURL: apiURL,
			Model:  modelName,
			APIKey: key,
		}
		return semantic.NewEmbedder(cfg)

	default:
		return nil, fmt.Errorf("unknown embedder type: %s (use 'openai', 'cohere', 'huggingface', or 'openrouter')", embedderType)
	}
}

// TestHelpers provides functions for tests to override global configuration.
// These should only be used in test code.

// SetStorageTypeForTesting sets the storage type and returns a cleanup function
// that restores the original value. Use with defer in tests:
//
//	cleanup := commands.SetStorageTypeForTesting("qdrant")
//	defer cleanup()
func SetStorageTypeForTesting(newType string) func() {
	old := storageType
	storageType = newType
	return func() { storageType = old }
}

// SetEmbedderTypeForTesting sets the embedder type and returns a cleanup function.
func SetEmbedderTypeForTesting(newType string) func() {
	old := embedderType
	embedderType = newType
	return func() { embedderType = old }
}

// SetAPIURLForTesting sets the API URL and returns a cleanup function.
func SetAPIURLForTesting(newURL string) func() {
	old := apiURL
	apiURL = newURL
	return func() { apiURL = old }
}

// SetCollectionNameForTesting sets the collection name and returns a cleanup function.
func SetCollectionNameForTesting(newName string) func() {
	old := collectionName
	collectionName = newName
	return func() { collectionName = old }
}

// SetConfigForTesting sets the loaded config and returns a cleanup function.
func SetConfigForTesting(cfg *config.SemanticConfig) func() {
	old := loadedConfig
	loadedConfig = cfg
	return func() { loadedConfig = old }
}

// ResetGlobalsForTesting resets all global variables to their default values.
// Useful in TestMain or test teardown.
func ResetGlobalsForTesting() {
	apiURL = getDefaultAPIURL()
	model = getDefaultModel()
	apiKey = ""
	indexDir = ".index"
	storageType = "sqlite"
	collectionName = ""
	embedderType = "openai"
	configPath = ""
	profile = ""
	loadedConfig = nil
	GlobalJSONOutput = false
	GlobalMinOutput = false
}

// searchComponents holds initialized components needed for search operations.
// Use initSearchComponents to create and cleanup to release resources.
type searchComponents struct {
	Storage  semantic.Storage
	Embedder semantic.EmbedderInterface
}

// initSearchComponents initializes the common components needed for search operations.
// Returns components and a cleanup function. Always call cleanup when done.
//
// This consolidates the duplicated initialization code from runSearch and runMultisearch:
// - Finding index path (for sqlite)
// - Creating embedder
// - Probing embedder dimensions (for qdrant)
// - Opening storage
func initSearchComponents(ctx context.Context, createStorageFn func(indexPath string, embeddingDim int) (semantic.Storage, error)) (*searchComponents, func(), error) {
	// Find index path (only needed for sqlite)
	indexPath := ""
	if storageType == "" || storageType == "sqlite" {
		indexPath = findIndexPath()
		if indexPath == "" {
			return nil, nil, fmt.Errorf("semantic index not found. Run 'llm-semantic index' first")
		}
	}

	// Create embedder
	embedder, err := createEmbedder()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// For Qdrant, we need to probe the embedder to get dimensions
	embeddingDim := 0
	if storageType == "qdrant" {
		testEmbed, err := embedder.Embed(ctx, "test")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to probe embedder for dimensions: %w", err)
		}
		embeddingDim = len(testEmbed)
	}

	// Open storage
	storage, err := createStorageFn(indexPath, embeddingDim)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open index: %w", err)
	}

	cleanup := func() {
		storage.Close()
	}

	return &searchComponents{
		Storage:  storage,
		Embedder: embedder,
	}, cleanup, nil
}
