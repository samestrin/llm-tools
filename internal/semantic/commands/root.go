package commands

import (
	"fmt"
	"os"

	"github.com/samestrin/llm-tools/internal/semantic"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	apiURL       string
	model        string
	apiKey       string
	indexDir     string
	storageType  string // "sqlite" (default) or "qdrant"
	embedderType string // "openai" (default), "cohere", "huggingface", "openrouter"

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
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Embedding model name (default varies by embedder)")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (or set LLM_SEMANTIC_API_KEY env var)")
	rootCmd.PersistentFlags().StringVar(&indexDir, "index-dir", ".llm-index", "Directory for semantic index")
	rootCmd.PersistentFlags().StringVar(&storageType, "storage", "sqlite", "Storage backend: sqlite (default) or qdrant")
	rootCmd.PersistentFlags().StringVar(&embedderType, "embedder", "openai", "Embedding provider: openai (default), cohere, huggingface, openrouter")
	rootCmd.PersistentFlags().BoolVar(&GlobalJSONOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&GlobalMinOutput, "min", false, "Minimal/token-optimized output")

	// Add subcommands
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(indexStatusCmd())
	rootCmd.AddCommand(indexUpdateCmd())

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
