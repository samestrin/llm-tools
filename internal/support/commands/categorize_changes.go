package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	categorizeChangesFile              string
	categorizeChangesContent           string
	categorizeChangesSensitivePatterns string
	categorizeChangesJSON              bool
	categorizeChangesMinimal           bool

	// Test file patterns (test files take priority over source)
	categorizeTestPatterns = []*regexp.Regexp{
		regexp.MustCompile(`_test\.go$`),
		regexp.MustCompile(`\.test\.(ts|tsx|js|jsx)$`),
		regexp.MustCompile(`\.spec\.(ts|tsx|js|jsx)$`),
		regexp.MustCompile(`^test_.*\.py$`),
		regexp.MustCompile(`_test\.py$`),
		regexp.MustCompile(`Test\.java$`),
		regexp.MustCompile(`_test\.rs$`),
	}

	// Generated file patterns (path-based)
	categorizeGeneratedDirs = []string{
		"dist/", "build/", "node_modules/", "vendor/", "__pycache__/",
		".cache/", "coverage/", "out/", "target/",
	}
	categorizeGeneratedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\.gen\.go$`),
		regexp.MustCompile(`\.generated\.(ts|js)$`),
		regexp.MustCompile(`\.min\.(js|css)$`),
	}

	// Source file extensions
	categorizeSourceExts = map[string]bool{
		".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".cs": true, ".swift": true, ".kt": true,
		".rb": true, ".php": true, ".scala": true, ".ex": true, ".exs": true,
	}

	// Config file extensions
	categorizeConfigExts = map[string]bool{
		".json": true, ".yaml": true, ".yml": true, ".toml": true,
		".ini": true, ".cfg": true, ".conf": true, ".xml": true,
	}

	// Config file names (no extension)
	categorizeConfigNames = map[string]bool{
		"Makefile": true, "Dockerfile": true, "Vagrantfile": true,
		".gitignore": true, ".gitattributes": true, ".editorconfig": true,
		".prettierrc": true, ".eslintrc": true, ".babelrc": true,
		"Gemfile": true, "Rakefile": true, "Procfile": true,
		"tsconfig.json": true, "package.json": true, "go.mod": true, "go.sum": true,
		"Cargo.toml": true, "Cargo.lock": true, "requirements.txt": true,
		"pyproject.toml": true, "setup.py": true, "setup.cfg": true,
	}

	// Docs file extensions and names
	categorizeDocsExts = map[string]bool{
		".md": true, ".rst": true, ".txt": true, ".adoc": true,
	}
	categorizeDocsPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^README`),
		regexp.MustCompile(`(?i)^CHANGELOG`),
		regexp.MustCompile(`(?i)^LICENSE`),
		regexp.MustCompile(`(?i)^CONTRIBUTING`),
		regexp.MustCompile(`(?i)^AUTHORS`),
		regexp.MustCompile(`(?i)^HISTORY`),
	}

	// Default sensitive file patterns
	defaultSensitivePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\.env($|\.)`),            // .env, .env.local, .env.production
		regexp.MustCompile(`(?i)^credentials\..+$`),      // credentials.json, credentials.yaml
		regexp.MustCompile(`(?i)^secrets?\..+$`),         // secret.json, secrets.yaml
		regexp.MustCompile(`(?i)^\.secrets?$`),           // .secret, .secrets
		regexp.MustCompile(`(?i)\.(key|pem|crt|p12)$`),   // *.key, *.pem, *.crt
		regexp.MustCompile(`(?i)_rsa$`),                  // id_rsa, github_rsa
		regexp.MustCompile(`(?i)password`),               // passwords.txt, etc
		regexp.MustCompile(`(?i)auth.*token`),            // auth-token.json
		regexp.MustCompile(`(?i)^\.htpasswd$`),           // .htpasswd
		regexp.MustCompile(`(?i)^\.netrc$`),              // .netrc
		regexp.MustCompile(`(?i)aws_access_key`),         // AWS credentials
		regexp.MustCompile(`(?i)gcp.*credentials`),       // GCP credentials
		regexp.MustCompile(`(?i)service.*account.*json`), // Service account keys
	}
)

// FileCategory represents a file change category
type FileCategory string

const (
	CategorySource    FileCategory = "source"
	CategoryTest      FileCategory = "test"
	CategoryConfig    FileCategory = "config"
	CategoryDocs      FileCategory = "docs"
	CategoryGenerated FileCategory = "generated"
	CategoryOther     FileCategory = "other"
)

// CategorizedFile represents a file with its category and status
type CategorizedFile struct {
	Path     string       `json:"path"`
	Status   string       `json:"status"` // M, A, D, R, ?
	Category FileCategory `json:"category"`
}

// CategoryCounts holds the count for each category
type CategoryCounts struct {
	Source    int `json:"source"`
	Test      int `json:"test"`
	Config    int `json:"config"`
	Docs      int `json:"docs"`
	Generated int `json:"generated"`
	Other     int `json:"other"`
}

// Categories holds files grouped by category
type Categories struct {
	Source    []string `json:"source"`
	Test      []string `json:"test"`
	Config    []string `json:"config"`
	Docs      []string `json:"docs"`
	Generated []string `json:"generated"`
	Other     []string `json:"other"`
}

// CategorizeChangesResult holds the complete categorization result
type CategorizeChangesResult struct {
	Categories     Categories     `json:"categories"`
	Counts         CategoryCounts `json:"counts"`
	SensitiveFiles []string       `json:"sensitive_files"`
	Total          int            `json:"total"`
}

// newCategorizeChangesCmd creates the categorize-changes command
func newCategorizeChangesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categorize-changes",
		Short: "Categorize git status output by file type",
		Long: `Parse git status output and categorize files by type.

Categories:
  source:    Code files (.go, .ts, .tsx, .py, .js, etc.)
  test:      Test files (*_test.go, *.test.ts, *.spec.js, etc.)
  config:    Configuration files (.json, .yaml, Dockerfile, etc.)
  docs:      Documentation files (.md, README, LICENSE, etc.)
  generated: Generated/build files (dist/*, node_modules/*, *.gen.go)
  other:     Files that don't match any category

Also detects sensitive files (*.env, credentials.*, *.key, etc.) that
should not be committed.

Input: Git status --porcelain output via --content, --file, or stdin.

Examples:
  git status --porcelain | llm-support categorize-changes --json
  llm-support categorize-changes --file changes.txt --json
  llm-support categorize-changes --content "M  src/main.go" --json`,
		RunE: runCategorizeChanges,
	}

	cmd.Flags().StringVar(&categorizeChangesFile, "file", "", "Path to file containing git status output")
	cmd.Flags().StringVar(&categorizeChangesContent, "content", "", "Git status porcelain content")
	cmd.Flags().StringVar(&categorizeChangesSensitivePatterns, "sensitive-patterns", "", "Additional sensitive file patterns (comma-separated globs)")
	cmd.Flags().BoolVar(&categorizeChangesJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&categorizeChangesMinimal, "min", false, "Minimal output format")

	return cmd
}

func runCategorizeChanges(cmd *cobra.Command, args []string) error {
	var input string

	if categorizeChangesContent != "" {
		input = categorizeChangesContent
	} else if categorizeChangesFile != "" {
		data, err := os.ReadFile(categorizeChangesFile)
		if err != nil {
			return fmt.Errorf("cannot read input file: %s: %w", categorizeChangesFile, err)
		}
		input = string(data)
	} else {
		// Try to read from stdin if available
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("cannot read stdin: %w", err)
			}
			input = string(data)
		}
	}

	if input == "" {
		// Return empty result for empty input
		result := CategorizeChangesResult{
			Categories:     Categories{},
			Counts:         CategoryCounts{},
			SensitiveFiles: []string{},
			Total:          0,
		}
		return outputResult(cmd, result)
	}

	// Parse git status
	files := parseGitStatusPorcelain(input)

	// Categorize files
	result := categorizeFiles(files)

	return outputResult(cmd, result)
}

func outputResult(cmd *cobra.Command, result CategorizeChangesResult) error {
	// Ensure arrays are non-null for JSON
	if result.Categories.Source == nil {
		result.Categories.Source = []string{}
	}
	if result.Categories.Test == nil {
		result.Categories.Test = []string{}
	}
	if result.Categories.Config == nil {
		result.Categories.Config = []string{}
	}
	if result.Categories.Docs == nil {
		result.Categories.Docs = []string{}
	}
	if result.Categories.Generated == nil {
		result.Categories.Generated = []string{}
	}
	if result.Categories.Other == nil {
		result.Categories.Other = []string{}
	}
	if result.SensitiveFiles == nil {
		result.SensitiveFiles = []string{}
	}

	formatter := output.New(categorizeChangesJSON, categorizeChangesMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(CategorizeChangesResult)
		fmt.Fprintf(w, "CATEGORIZE_CHANGES:\n")
		fmt.Fprintf(w, "  Total: %d\n", r.Total)
		fmt.Fprintf(w, "  Counts:\n")
		fmt.Fprintf(w, "    Source: %d\n", r.Counts.Source)
		fmt.Fprintf(w, "    Test: %d\n", r.Counts.Test)
		fmt.Fprintf(w, "    Config: %d\n", r.Counts.Config)
		fmt.Fprintf(w, "    Docs: %d\n", r.Counts.Docs)
		fmt.Fprintf(w, "    Generated: %d\n", r.Counts.Generated)
		fmt.Fprintf(w, "    Other: %d\n", r.Counts.Other)
		if len(r.SensitiveFiles) > 0 {
			fmt.Fprintf(w, "  SENSITIVE FILES DETECTED: %d\n", len(r.SensitiveFiles))
			for _, f := range r.SensitiveFiles {
				fmt.Fprintf(w, "    - %s\n", f)
			}
		}
	})
}

// GitStatusEntry represents a parsed git status line
type GitStatusEntry struct {
	Status string // M, A, D, R, ?, etc.
	Path   string
}

// parseGitStatusPorcelain parses git status --porcelain output
func parseGitStatusPorcelain(input string) []GitStatusEntry {
	var entries []GitStatusEntry

	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Git status porcelain format: XY PATH
		// where X is index status, Y is worktree status
		// Minimum format: "XY file" where XY is 2 chars
		if len(line) < 4 { // "M  f" minimum
			continue
		}

		status := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])

		// Handle renamed files: "R  old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			if len(parts) == 2 {
				path = strings.TrimSpace(parts[1])
			}
		}

		if path == "" {
			continue
		}

		entries = append(entries, GitStatusEntry{
			Status: status,
			Path:   path,
		})
	}

	return entries
}

// categorizeFiles categorizes git status entries
func categorizeFiles(entries []GitStatusEntry) CategorizeChangesResult {
	result := CategorizeChangesResult{
		Categories:     Categories{},
		Counts:         CategoryCounts{},
		SensitiveFiles: []string{},
		Total:          len(entries),
	}

	for _, entry := range entries {
		category := determineCategory(entry.Path)

		switch category {
		case CategorySource:
			result.Categories.Source = append(result.Categories.Source, entry.Path)
			result.Counts.Source++
		case CategoryTest:
			result.Categories.Test = append(result.Categories.Test, entry.Path)
			result.Counts.Test++
		case CategoryConfig:
			result.Categories.Config = append(result.Categories.Config, entry.Path)
			result.Counts.Config++
		case CategoryDocs:
			result.Categories.Docs = append(result.Categories.Docs, entry.Path)
			result.Counts.Docs++
		case CategoryGenerated:
			result.Categories.Generated = append(result.Categories.Generated, entry.Path)
			result.Counts.Generated++
		case CategoryOther:
			result.Categories.Other = append(result.Categories.Other, entry.Path)
			result.Counts.Other++
		}

		// Check for sensitive files
		if isSensitiveFile(entry.Path) {
			result.SensitiveFiles = append(result.SensitiveFiles, entry.Path)
		}
	}

	// Sort all arrays for consistent output
	sort.Strings(result.Categories.Source)
	sort.Strings(result.Categories.Test)
	sort.Strings(result.Categories.Config)
	sort.Strings(result.Categories.Docs)
	sort.Strings(result.Categories.Generated)
	sort.Strings(result.Categories.Other)
	sort.Strings(result.SensitiveFiles)

	return result
}

// determineCategory determines the category for a file path
func determineCategory(path string) FileCategory {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	// Check for test files first (they may have source extensions)
	for _, pattern := range categorizeTestPatterns {
		if pattern.MatchString(base) {
			return CategoryTest
		}
	}

	// Check for generated directories
	for _, dir := range categorizeGeneratedDirs {
		if strings.Contains(path, dir) {
			return CategoryGenerated
		}
	}

	// Check for generated file patterns
	for _, pattern := range categorizeGeneratedPatterns {
		if pattern.MatchString(base) {
			return CategoryGenerated
		}
	}

	// Check for docs patterns (README, LICENSE, etc.)
	for _, pattern := range categorizeDocsPatterns {
		if pattern.MatchString(base) {
			return CategoryDocs
		}
	}

	// Check for docs extensions
	if categorizeDocsExts[ext] {
		return CategoryDocs
	}

	// Check for config file names
	if categorizeConfigNames[base] {
		return CategoryConfig
	}

	// Check for config extensions
	if categorizeConfigExts[ext] {
		return CategoryConfig
	}

	// Check for source extensions
	if categorizeSourceExts[ext] {
		return CategorySource
	}

	return CategoryOther
}

// isSensitiveFile checks if a file matches sensitive file patterns
func isSensitiveFile(path string) bool {
	base := filepath.Base(path)

	for _, pattern := range defaultSensitivePatterns {
		if pattern.MatchString(base) {
			return true
		}
	}

	return false
}

func init() {
	RootCmd.AddCommand(newCategorizeChangesCmd())
}
