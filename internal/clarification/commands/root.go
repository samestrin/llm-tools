// Package commands implements CLI commands for llm-clarification.
package commands

import (
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time using ldflags
var Version = "1.2.0"

// globalDBPath is the path set by the --db flag, overrides per-command --file flags
var globalDBPath string

// Global output flags accessible to all commands
var (
	GlobalJSONOutput bool
	GlobalMinOutput  bool
)

var rootCmd = &cobra.Command{
	Use:           "llm-clarification",
	Short:         "LLM Clarification Learning System",
	Long:          `A CLI tool for tracking and managing clarifications gathered during LLM-assisted development.`,
	Version:       Version,
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
	// Global output flags
	rootCmd.PersistentFlags().BoolVar(&GlobalJSONOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&GlobalMinOutput, "min", false, "Minimal/token-optimized output")
}
