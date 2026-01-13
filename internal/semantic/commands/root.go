package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/internal/semantic"
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

// RootCmd returns the root command for llm-semantic
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "llm-semantic",
		Short: "Semantic code search with local embeddings",
		Long: `llm-semantic provides semantic code search using local embedding models.
Supports any OpenAI-compatible embedding API (Ollama, vLLM, OpenAI, Azure, etc.)`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Sync local command flags to global vars for error handling
			if f := cmd.Flag("json"); f != nil && f.Changed {
				GlobalJSONOutput = true
			}
			if f := cmd.Flag("min"); f != nil && f.Changed {
				GlobalMinOutput = true
			}
		},
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", getDefaultAPIURL(), "Embedding API URL (OpenAI-compatible)")
	rootCmd.PersistentFlags().StringVar(&model, "model", getDefaultModel(), "Embedding model name (or set LLM_SEMANTIC_MODEL env var)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (or set LLM_SEMANTIC_API_KEY env var)")
	rootCmd.PersistentFlags().StringVar(&indexDir, "index-dir", ".llm-index", "Directory for semantic index")
	rootCmd.PersistentFlags().StringVar(&storageType, "storage", "sqlite", "Storage backend: sqlite (default) or qdrant")
	rootCmd.PersistentFlags().StringVar(&collectionName, "collection", "", "Qdrant collection name (default: QDRANT_COLLECTION env or 'llm_semantic')")
	rootCmd.PersistentFlags().StringVar(&embedderType, "embedder", "openai", "Embedding provider: openai (default), cohere, huggingface, openrouter")
	rootCmd.PersistentFlags().BoolVar(&GlobalJSONOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&GlobalMinOutput, "min", false, "Minimal/token-optimized output")

	// Add subcommands
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(indexStatusCmd())
	rootCmd.AddCommand(indexUpdateCmd())
	rootCmd.AddCommand(memoryCmd())

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
// 2. Derived from --index-dir (e.g., ".llm-index/code" → "code", ".llm-index/docs" → "docs")
// 3. QDRANT_COLLECTION environment variable
// 4. Default: "llm_semantic"
func resolveCollectionName() string {
	// Priority 1: explicit --collection flag
	if collectionName != "" {
		return collectionName
	}

	// Priority 2: derive from index-dir if non-default
	if indexDir != "" && indexDir != ".llm-index" {
		// Extract the last path component as collection name
		// e.g., ".llm-index/code" → "code", "indexes/docs" → "docs"
		derived := deriveCollectionFromPath(indexDir)
		if derived != "" {
			return derived
		}
	}

	// Priority 3: environment variable
	if envCollection := os.Getenv("QDRANT_COLLECTION"); envCollection != "" {
		return envCollection
	}

	// Priority 4: default
	return "llm_semantic"
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

	// Skip if it's just ".llm-index" or similar default
	if name == ".llm-index" || name == "llm-index" || name == "" {
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
