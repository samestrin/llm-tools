// Package commands implements CLI commands for llm-clarification.
package commands

import (
	"github.com/spf13/cobra"
)

// Version is set at build time using ldflags
var Version = "dev"

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

func init() {
	// Global flags can be added here
}
