package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags
var Version = "1.2.0"

// RootCmd is the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "llm-support",
	Short: "LLM-focused codebase analysis and transformation tools",
	Long: `llm-support provides 32+ specialized commands for working with
code, configuration files, and documentation in LLM-assisted workflows.

Designed for fast startup (10-20x faster than Python), single binary
distribution, and integration with Claude, Gemini, and Qwen prompts.`,
	Version: Version,
}

// Execute runs the root command
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags (available to all subcommands)
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	RootCmd.PersistentFlags().String("format", "text", "Output format: text, json")
	RootCmd.PersistentFlags().Bool("no-gitignore", false, "Disable .gitignore filtering")
}
