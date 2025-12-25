package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initTempPreserve bool

// newInitTempCmd creates the init-temp command
func newInitTempCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-temp <name>",
		Short: "Initialize temp directory",
		Long: `Initialize and manage temp directories with consistent patterns.

Creates .planning/.temp/{name}/ directory for command-specific temp files.

Modes:
  --clean (default): Remove existing files before creating
  --preserve: Keep existing files if directory exists

Output:
  TEMP_DIR: path to temp directory
  STATUS: CREATED | EXISTS
  CLEANED: N files removed (with --clean)
  EXISTING_FILES: N (with --preserve when dir exists)`,
		Args: cobra.ExactArgs(1),
		RunE: runInitTemp,
	}

	cmd.Flags().BoolVar(&initTempPreserve, "preserve", false, "Keep existing files")

	return cmd
}

func runInitTemp(cmd *cobra.Command, args []string) error {
	name := args[0]
	baseTemp := filepath.Join(".planning", ".temp")
	tempDir := filepath.Join(baseTemp, name)

	cleanedCount := 0
	existingFiles := 0
	status := "CREATED"

	// Handle existing directory
	if info, err := os.Stat(tempDir); err == nil && info.IsDir() {
		if initTempPreserve {
			// Count existing files
			existingFiles = countFilesRecursive(tempDir)
			status = "EXISTS"
		} else {
			// Clean mode - remove existing files
			cleanedCount = cleanDirectory(tempDir)
			status = "CREATED"
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Output
	fmt.Fprintf(cmd.OutOrStdout(), "TEMP_DIR: %s\n", tempDir)
	fmt.Fprintf(cmd.OutOrStdout(), "STATUS: %s\n", status)
	if initTempPreserve && status == "EXISTS" {
		fmt.Fprintf(cmd.OutOrStdout(), "EXISTING_FILES: %d\n", existingFiles)
	} else if !initTempPreserve {
		fmt.Fprintf(cmd.OutOrStdout(), "CLEANED: %d files removed\n", cleanedCount)
	}

	return nil
}

func countFilesRecursive(dir string) int {
	count := 0
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func cleanDirectory(dir string) int {
	count := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			count += countFilesRecursive(path)
			os.RemoveAll(path)
		} else {
			os.Remove(path)
			count++
		}
	}
	return count
}

func init() {
	RootCmd.AddCommand(newInitTempCmd())
}
