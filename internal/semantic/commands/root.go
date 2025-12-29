package commands

import (
	"github.com/spf13/cobra"
)

var (
	// Global flags
	apiURL   string
	model    string
	apiKey   string
	indexDir string
)

// RootCmd returns the root command for llm-semantic
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "llm-semantic",
		Short: "Semantic code search with local embeddings",
		Long: `llm-semantic provides semantic code search using local embedding models.
Supports any OpenAI-compatible embedding API (Ollama, vLLM, OpenAI, Azure, etc.)`,
	}

	// Persistent flags
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:11434", "Embedding API URL (OpenAI-compatible)")
	rootCmd.PersistentFlags().StringVar(&model, "model", "mxbai-embed-large", "Embedding model name")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "API key (or set LLM_SEMANTIC_API_KEY env var)")
	rootCmd.PersistentFlags().StringVar(&indexDir, "index-dir", ".llm-index", "Directory for semantic index")

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
