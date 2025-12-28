// Package commands implements CLI commands for llm-clarification.
package commands

import (
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time using ldflags
var Version = "1.0.0"

// globalDBPath is the path set by the --db flag, overrides per-command --file flags
var globalDBPath string

var rootCmd = &cobra.Command{
	Use:     "llm-clarification",
	Short:   "LLM Clarification Learning System",
	Long:    `A CLI tool for tracking and managing clarifications gathered during LLM-assisted development.`,
	Version: Version,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// GetDBPath returns the effective database path.
// Priority: --db flag > CLARIFY_DB_PATH env var > per-command --file flag
func GetDBPath(cmdFilePath string) string {
	if globalDBPath != "" {
		return globalDBPath
	}
	if envPath := os.Getenv("CLARIFY_DB_PATH"); envPath != "" {
		return envPath
	}
	return cmdFilePath
}

func init() {
	// Global persistent flag for database path
	rootCmd.PersistentFlags().StringVar(&globalDBPath, "db", "", "Storage file path (.yaml, .yml, .db, .sqlite) - overrides per-command --file flags")
}
