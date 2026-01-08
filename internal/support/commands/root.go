package commands

import (
	"os"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags
var Version = "1.5.0"

// Global output flags accessible to all commands
var (
	GlobalJSONOutput bool
	GlobalMinOutput  bool
)

// RootCmd is the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "llm-support",
	Short: "LLM-focused codebase analysis and transformation tools",
	Long: `llm-support provides 32+ specialized commands for working with
code, configuration files, and documentation in LLM-assisted workflows.

Designed for fast startup (10-20x faster than Python), single binary
distribution, and integration with Claude, Gemini, and Qwen prompts.`,
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

// Execute runs the root command
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		f := output.New(GlobalJSONOutput, GlobalMinOutput, os.Stdout)
		os.Exit(f.PrintError(err))
	}
}

func init() {
	// Global persistent flags (available to all subcommands)
	RootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	RootCmd.PersistentFlags().String("format", "text", "Output format: text, json")
	RootCmd.PersistentFlags().Bool("no-gitignore", false, "Disable .gitignore filtering")
	RootCmd.PersistentFlags().BoolVar(&GlobalJSONOutput, "json", false, "Output as JSON")
	RootCmd.PersistentFlags().BoolVar(&GlobalMinOutput, "min", false, "Minimal/token-optimized output")
}
