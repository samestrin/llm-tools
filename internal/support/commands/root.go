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

	// GlobalAXIOutput selects TOON output. It reaches every command through
	// output.SetDefaultAXI rather than through a per-command flag: each command
	// builds its own formatter with output.New(json, min, w), so a package
	// default is the one place that covers all of them without editing sixty
	// files and inventing sixty chances to miss one.
	GlobalAXIOutput bool
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
		if f := cmd.Flag("axi"); f != nil && f.Changed {
			GlobalAXIOutput = true
		}
		// Published once, here, because this is the single point where flags
		// are known to be parsed. Every output.New call in every command reads
		// it from there.
		output.SetDefaultAXI(GlobalAXIOutput)
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
	RootCmd.PersistentFlags().BoolVar(&GlobalAXIOutput, "axi", false, "Output as TOON (AXI token-dense format)")
}
