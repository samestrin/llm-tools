package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	discoverTestsJSON bool
	discoverTestsPath string
)

// newDiscoverTestsCmd creates the discover-tests command
func newDiscoverTestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover-tests",
		Short: "Discover test patterns and infrastructure",
		Long: `Discover test patterns, runners, and infrastructure in a project.

Output fields:
  PATTERN: SEPARATED | COLOCATED | UNKNOWN
  FRAMEWORK: wasp | nextjs | nuxt | angular | vue | remix
  TEST_RUNNER: vitest | jest | mocha | pytest
  CONFIG_FILE: path to test config file
  SOURCE_DIR: source directory
  TEST_DIR: test directory
  E2E_DIR: e2e test directory
  UNIT_TEST_COUNT: number of unit test files
  E2E_TEST_COUNT: number of e2e test files`,
		Args: cobra.NoArgs,
		RunE: runDiscoverTests,
	}

	cmd.Flags().StringVar(&discoverTestsPath, "path", ".", "Project path to analyze")
	cmd.Flags().BoolVar(&discoverTestsJSON, "json", false, "Output as JSON")

	return cmd
}

func runDiscoverTests(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(discoverTestsPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	result := map[string]interface{}{
		"pattern":          "UNKNOWN",
		"framework":        "",
		"test_runner":      "",
		"config_file":      "",
		"source_dir":       "",
		"test_dir":         "",
		"e2e_dir":          "",
		"unit_test_count":  0,
		"e2e_test_count":   0,
		"total_test_count": 0,
	}

	// Detect framework
	frameworkMarkers := map[string]string{
		"main.wasp":       "wasp",
		"next.config.js":  "nextjs",
		"next.config.mjs": "nextjs",
		"next.config.ts":  "nextjs",
		"nuxt.config.js":  "nuxt",
		"nuxt.config.ts":  "nuxt",
		"angular.json":    "angular",
		"vue.config.js":   "vue",
		"remix.config.js": "remix",
	}

	for marker, framework := range frameworkMarkers {
		if fileExists(filepath.Join(path, marker)) {
			result["framework"] = framework
			break
		}
	}

	// Detect test runner and config
	testConfigs := []struct {
		file   string
		runner string
	}{
		{"vitest.config.ts", "vitest"},
		{"vitest.config.js", "vitest"},
		{"vitest.config.mts", "vitest"},
		{"jest.config.js", "jest"},
		{"jest.config.ts", "jest"},
		{"jest.config.json", "jest"},
		{".mocharc.js", "mocha"},
		{".mocharc.json", "mocha"},
		{"pytest.ini", "pytest"},
	}

	for _, cfg := range testConfigs {
		if fileExists(filepath.Join(path, cfg.file)) {
			result["test_runner"] = cfg.runner
			result["config_file"] = cfg.file
			break
		}
	}

	// Check package.json for test runner hints
	if result["test_runner"] == "" {
		packageJSON := filepath.Join(path, "package.json")
		if fileExists(packageJSON) {
			content, _ := os.ReadFile(packageJSON)
			contentStr := string(content)
			if strings.Contains(contentStr, "vitest") {
				result["test_runner"] = "vitest"
			} else if strings.Contains(contentStr, "jest") {
				result["test_runner"] = "jest"
			} else if strings.Contains(contentStr, "mocha") {
				result["test_runner"] = "mocha"
			}
		}
	}

	// Detect source directory
	sourceDirs := []string{"src", "lib", "app", "source", "packages"}
	for _, srcDir := range sourceDirs {
		if dirExists(filepath.Join(path, srcDir)) {
			result["source_dir"] = srcDir + "/"
			break
		}
	}

	// Detect test directories
	testDirs := []string{"tests", "test", "__tests__", "spec", "specs"}
	for _, testDir := range testDirs {
		testPath := filepath.Join(path, testDir)
		if dirExists(testPath) {
			result["test_dir"] = testDir + "/"
			result["unit_test_count"] = countTestFiles(testPath)
			break
		}
	}

	// Detect e2e directory
	e2eDirs := []string{"e2e", "e2e-tests", "cypress", "playwright"}
	for _, e2eDir := range e2eDirs {
		e2ePath := filepath.Join(path, e2eDir)
		if dirExists(e2ePath) {
			result["e2e_dir"] = e2eDir + "/"
			result["e2e_test_count"] = countTestFiles(e2ePath)
			break
		}
	}

	// Count colocated tests
	colocatedCount := 0
	if srcDir, ok := result["source_dir"].(string); ok && srcDir != "" {
		srcPath := filepath.Join(path, strings.TrimSuffix(srcDir, "/"))
		colocatedCount = countColocatedTests(srcPath)
	}

	// Determine pattern
	hasSeparateTests := result["test_dir"] != "" && result["unit_test_count"].(int) > 0
	hasColocatedTests := colocatedCount > 0

	if hasSeparateTests && !hasColocatedTests {
		result["pattern"] = "SEPARATED"
	} else if hasColocatedTests && !hasSeparateTests {
		result["pattern"] = "COLOCATED"
		result["unit_test_count"] = colocatedCount
	} else if hasSeparateTests && hasColocatedTests {
		result["pattern"] = "SEPARATED" // Prefer separated
	}

	// Calculate total
	result["total_test_count"] = result["unit_test_count"].(int) + result["e2e_test_count"].(int)

	// Output
	if discoverTestsJSON {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "PATTERN: %s\n", result["pattern"])
		fmt.Fprintf(cmd.OutOrStdout(), "FRAMEWORK: %s\n", result["framework"])
		fmt.Fprintf(cmd.OutOrStdout(), "TEST_RUNNER: %s\n", result["test_runner"])
		fmt.Fprintf(cmd.OutOrStdout(), "CONFIG_FILE: %s\n", result["config_file"])
		fmt.Fprintf(cmd.OutOrStdout(), "SOURCE_DIR: %s\n", result["source_dir"])
		fmt.Fprintf(cmd.OutOrStdout(), "TEST_DIR: %s\n", result["test_dir"])
		fmt.Fprintf(cmd.OutOrStdout(), "E2E_DIR: %s\n", result["e2e_dir"])
		fmt.Fprintf(cmd.OutOrStdout(), "UNIT_TEST_COUNT: %d\n", result["unit_test_count"])
		fmt.Fprintf(cmd.OutOrStdout(), "E2E_TEST_COUNT: %d\n", result["e2e_test_count"])
		fmt.Fprintf(cmd.OutOrStdout(), "TOTAL_TEST_COUNT: %d\n", result["total_test_count"])
	}

	return nil
}

func countTestFiles(dir string) int {
	count := 0
	testPatterns := []string{".test.", ".spec.", "_test.", "_spec."}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		for _, pattern := range testPatterns {
			if strings.Contains(name, pattern) {
				count++
				break
			}
		}
		return nil
	})

	return count
}

func countColocatedTests(dir string) int {
	count := 0
	testPatterns := []string{".test.", ".spec.", "_test.", "_spec."}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		for _, pattern := range testPatterns {
			if strings.Contains(name, pattern) {
				count++
				break
			}
		}
		return nil
	})

	return count
}

func init() {
	RootCmd.AddCommand(newDiscoverTestsCmd())
}
