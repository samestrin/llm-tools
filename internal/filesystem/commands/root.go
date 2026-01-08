package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags
var Version = "1.5.0"

var (
	// Global flags
	jsonOutput  bool
	minOutput   bool
	allowedDirs []string
)

// RootCmd returns the root command for llm-filesystem
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "llm-filesystem",
		Short:   "High-performance filesystem operations CLI",
		Version: Version,
		Long: `llm-filesystem provides fast file operations for Claude Code and CLI usage.

It supports 27 commands for reading, writing, editing, and managing files.
All commands support --json output for machine parsing.`,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVar(&minOutput, "min", false, "Minimal/token-optimized output")
	rootCmd.PersistentFlags().StringSliceVar(&allowedDirs, "allowed-dirs", nil,
		"Directories the tool is allowed to access (comma-separated)")

	// Add all subcommands
	addReadCommands(rootCmd)
	addWriteCommands(rootCmd)
	addEditCommands(rootCmd)
	addDirectoryCommands(rootCmd)
	addSearchCommands(rootCmd)
	addFileOpsCommands(rootCmd)
	addAdvancedCommands(rootCmd)

	return rootCmd
}

// GetAllowedDirs returns the expanded allowed directories
func GetAllowedDirs() []string {
	expanded := make([]string, 0, len(allowedDirs))
	for _, dir := range allowedDirs {
		if strings.HasPrefix(dir, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				dir = strings.Replace(dir, "~", home, 1)
			}
		}
		expanded = append(expanded, dir)
	}
	return expanded
}

// OutputResult outputs the result in JSON or text format
func OutputResult(result interface{}, textFn func() string) {
	if jsonOutput {
		var jsonBytes []byte
		var err error
		if minOutput {
			// --json --min: single-line compact JSON
			jsonBytes, err = json.Marshal(result)
		} else {
			// --json: pretty-printed JSON
			jsonBytes, err = json.MarshalIndent(result, "", "  ")
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Println(textFn())
	}
}

// OutputError outputs an error in JSON or text format
func OutputError(err error) {
	if jsonOutput {
		if minOutput {
			// Minimal JSON: abbreviated keys, single line
			result := map[string]interface{}{
				"err": true,
				"msg": err.Error(),
			}
			jsonBytes, _ := json.Marshal(result)
			fmt.Println(string(jsonBytes))
		} else {
			result := map[string]interface{}{
				"error":   true,
				"message": err.Error(),
			}
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonBytes))
		}
	} else {
		if minOutput {
			// Minimal text: just the error message
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
	os.Exit(1)
}

// Execute runs the root command
func Execute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
