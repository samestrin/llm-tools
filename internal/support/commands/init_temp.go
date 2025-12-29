package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	initTempPreserve bool
	initTempName     string
	initTempClean    bool
	initTempJSON     bool
	initTempMinimal  bool
)

// InitTempResult holds the init-temp command result
type InitTempResult struct {
	TempDir       string `json:"temp_dir,omitempty"`
	TD            string `json:"td,omitempty"`
	Status        string `json:"status,omitempty"`
	S             string `json:"s,omitempty"`
	Cleaned       int    `json:"cleaned,omitempty"`
	Cl            *int   `json:"cl,omitempty"`
	ExistingFiles int    `json:"existing_files,omitempty"`
	EF            *int   `json:"ef,omitempty"`
}

// newInitTempCmd creates the init-temp command
func newInitTempCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-temp",
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
		RunE: runInitTemp,
	}

	cmd.Flags().StringVar(&initTempName, "name", "", "Name for temp directory (required)")
	cmd.Flags().BoolVar(&initTempPreserve, "preserve", false, "Keep existing files")
	cmd.Flags().BoolVar(&initTempClean, "clean", true, "Remove existing files (default)")
	cmd.Flags().BoolVar(&initTempJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&initTempMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("name")

	return cmd
}

func runInitTemp(cmd *cobra.Command, args []string) error {
	name := initTempName
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

	// Build result
	var result InitTempResult
	if initTempMinimal {
		result = InitTempResult{
			TD: tempDir,
			S:  status,
		}
		if initTempPreserve && status == "EXISTS" {
			result.EF = &existingFiles
		} else if !initTempPreserve {
			result.Cl = &cleanedCount
		}
	} else {
		result = InitTempResult{
			TempDir: tempDir,
			Status:  status,
		}
		if initTempPreserve && status == "EXISTS" {
			result.ExistingFiles = existingFiles
		} else if !initTempPreserve {
			result.Cleaned = cleanedCount
		}
	}

	// Output
	formatter := output.New(initTempJSON, initTempMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "TEMP_DIR: %s\n", tempDir)
		fmt.Fprintf(w, "STATUS: %s\n", status)
		if initTempPreserve && status == "EXISTS" {
			fmt.Fprintf(w, "EXISTING_FILES: %d\n", existingFiles)
		} else if !initTempPreserve {
			fmt.Fprintf(w, "CLEANED: %d files removed\n", cleanedCount)
		}
	})
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
