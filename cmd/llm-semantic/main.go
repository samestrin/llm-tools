package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

var rootCmd = &cobra.Command{
	Use:   "llm-semantic",
	Short: "Semantic code search with local embeddings",
	Long: `llm-semantic provides semantic code search using local embedding models.

It indexes your codebase into vector embeddings and enables natural language
queries like "find authentication middleware" instead of exact keyword matching.

Supports any OpenAI-compatible embedding API (Ollama, vLLM, OpenAI, Azure, etc.)`,
	Version: fmt.Sprintf("%s (%s)", version, commit),
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().String("api-url", "http://localhost:11434", "Embedding API URL (OpenAI-compatible)")
	rootCmd.PersistentFlags().String("model", "mxbai-embed-large", "Embedding model name")
	rootCmd.PersistentFlags().String("api-key", "", "API key (or set LLM_SEMANTIC_API_KEY env var)")
	rootCmd.PersistentFlags().String("index-dir", ".llm-index", "Directory for semantic index")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
