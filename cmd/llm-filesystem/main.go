package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/samestrin/llm-tools/internal/filesystem"
	"github.com/spf13/cobra"
)

var (
	allowedDirs []string
	version     = "1.0.0"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "llm-filesystem",
		Short: "High-performance MCP filesystem server",
		Long: `llm-filesystem is a Go MCP server providing fast file operations for Claude Code.

It is a drop-in replacement for fast-filesystem-mcp with 10-30x faster cold start
and 3-5x lower memory usage.`,
		Version: version,
		RunE:    runServer,
	}

	rootCmd.Flags().StringSliceVar(&allowedDirs, "allowed-dirs", nil,
		"Directories the server is allowed to access (comma-separated or multiple flags)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Expand home directory in allowed dirs
	expandedDirs := make([]string, 0, len(allowedDirs))
	for _, dir := range allowedDirs {
		if strings.HasPrefix(dir, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to expand home directory: %w", err)
			}
			dir = strings.Replace(dir, "~", home, 1)
		}
		expandedDirs = append(expandedDirs, dir)
	}

	server, err := filesystem.NewServer(expandedDirs)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	return server.Run(context.Background())
}
