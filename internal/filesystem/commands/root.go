package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	jsonOutput  bool
	allowedDirs []string
)

// RootCmd returns the root command for llm-filesystem
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "llm-filesystem",
		Short: "High-performance filesystem operations CLI",
		Long: `llm-filesystem provides fast file operations for Claude Code and CLI usage.

It supports 27 commands for reading, writing, editing, and managing files.
All commands support --json output for machine parsing.`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
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
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
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
		result := map[string]interface{}{
			"error":   true,
			"message": err.Error(),
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(1)
}

// Execute runs the root command
func Execute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
