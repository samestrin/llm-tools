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
	detectJSON    bool
	detectPath    string
	detectMinimal bool
)

// DetectResult represents the project detection result
type DetectResult struct {
	Stack          string `json:"stack"`
	Language       string `json:"language"`
	PackageManager string `json:"package_manager"`
	Framework      string `json:"framework"`
	HasTests       bool   `json:"has_tests"`
	PytestAvail    bool   `json:"pytest_available"`
}

// newDetectCmd creates the detect command
func newDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect project type and technology stack",
		Long: `Detect project type, language, package manager, and framework.

Output fields:
  STACK: node | python | go | rust | java | ruby | php | dotnet | unknown
  LANGUAGE: typescript | javascript | python | go | rust | java | ruby | php | csharp | unknown
  PACKAGE_MANAGER: npm | yarn | pnpm | pip | poetry | go | cargo | maven | bundler | composer
  FRAMEWORK: nextjs | remix | express | fastapi | django | flask | gin | actix | spring | rails
  HAS_TESTS: true | false
  PYTEST_AVAILABLE: true | false`,
		Args: cobra.NoArgs,
		RunE: runDetect,
	}

	cmd.Flags().StringVar(&detectPath, "path", ".", "Project path to analyze")
	cmd.Flags().BoolVar(&detectJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&detectMinimal, "min", false, "Output in minimal/token-optimized format")

	return cmd
}

func runDetect(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(detectPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	result := map[string]interface{}{
		"stack":            "unknown",
		"language":         "unknown",
		"package_manager":  "",
		"framework":        "",
		"has_tests":        false,
		"pytest_available": false,
	}

	// Detect by marker files
	markers := map[string]struct{ stack, lang string }{
		"package.json":     {"node", "javascript"},
		"tsconfig.json":    {"node", "typescript"},
		"pyproject.toml":   {"python", "python"},
		"setup.py":         {"python", "python"},
		"requirements.txt": {"python", "python"},
		"Pipfile":          {"python", "python"},
		"go.mod":           {"go", "go"},
		"go.sum":           {"go", "go"},
		"Cargo.toml":       {"rust", "rust"},
		"pom.xml":          {"java", "java"},
		"build.gradle":     {"java", "java"},
		"Gemfile":          {"ruby", "ruby"},
		"composer.json":    {"php", "php"},
	}

	for marker, info := range markers {
		if fileExists(filepath.Join(path, marker)) {
			result["stack"] = info.stack
			result["language"] = info.lang
			break
		}
	}

	// Refine TypeScript detection
	if result["stack"] == "node" && fileExists(filepath.Join(path, "tsconfig.json")) {
		result["language"] = "typescript"
	}

	// Detect package manager
	pmMarkers := map[string]string{
		"yarn.lock":         "yarn",
		"pnpm-lock.yaml":    "pnpm",
		"package-lock.json": "npm",
		"poetry.lock":       "poetry",
		"Pipfile.lock":      "pipenv",
		"requirements.txt":  "pip",
		"go.sum":            "go",
		"Cargo.lock":        "cargo",
		"Gemfile.lock":      "bundler",
		"composer.lock":     "composer",
	}

	for marker, pm := range pmMarkers {
		if fileExists(filepath.Join(path, marker)) {
			result["package_manager"] = pm
			break
		}
	}

	// Detect framework
	frameworkMarkers := map[string]string{
		"next.config.js":   "nextjs",
		"next.config.mjs":  "nextjs",
		"next.config.ts":   "nextjs",
		"remix.config.js":  "remix",
		"nuxt.config.js":   "nuxt",
		"nuxt.config.ts":   "nuxt",
		"angular.json":     "angular",
		"svelte.config.js": "sveltekit",
	}

	for marker, framework := range frameworkMarkers {
		if fileExists(filepath.Join(path, marker)) {
			result["framework"] = framework
			break
		}
	}

	// Detect tests
	testDirs := []string{"tests", "test", "__tests__", "spec", "e2e"}
	for _, testDir := range testDirs {
		if dirExists(filepath.Join(path, testDir)) {
			result["has_tests"] = true
			break
		}
	}

	// Check for pytest
	if result["stack"] == "python" {
		pyproject := filepath.Join(path, "pyproject.toml")
		if fileExists(pyproject) {
			content, _ := os.ReadFile(pyproject)
			if containsString(string(content), "pytest") {
				result["pytest_available"] = true
			}
		}
		requirements := filepath.Join(path, "requirements.txt")
		if fileExists(requirements) {
			content, _ := os.ReadFile(requirements)
			if containsString(string(content), "pytest") {
				result["pytest_available"] = true
			}
		}
	}

	// Build structured result
	detectResult := DetectResult{
		Stack:          result["stack"].(string),
		Language:       result["language"].(string),
		PackageManager: result["package_manager"].(string),
		Framework:      result["framework"].(string),
		HasTests:       result["has_tests"].(bool),
		PytestAvail:    result["pytest_available"].(bool),
	}

	formatter := output.New(detectJSON, detectMinimal, cmd.OutOrStdout())
	return formatter.Print(detectResult, printDetectText)
}

func printDetectText(w io.Writer, data interface{}) {
	r := data.(DetectResult)
	fmt.Fprintf(w, "STACK: %s\n", r.Stack)
	fmt.Fprintf(w, "LANGUAGE: %s\n", r.Language)
	fmt.Fprintf(w, "PACKAGE_MANAGER: %s\n", r.PackageManager)
	fmt.Fprintf(w, "FRAMEWORK: %s\n", r.Framework)
	fmt.Fprintf(w, "HAS_TESTS: %v\n", r.HasTests)
	fmt.Fprintf(w, "PYTEST_AVAILABLE: %v\n", r.PytestAvail)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func init() {
	RootCmd.AddCommand(newDetectCmd())
}
