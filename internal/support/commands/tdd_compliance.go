package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tddCompliancePath    string
	tddComplianceContent string
	tddComplianceSince   string
	tddComplianceUntil   string
	tddComplianceCount   int
	tddComplianceJSON    bool
	tddComplianceMinimal bool

	// Test file patterns for various languages
	testFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`_test\.go$`),               // Go
		regexp.MustCompile(`\.test\.(ts|tsx|js|jsx)$`), // TypeScript/JavaScript .test
		regexp.MustCompile(`\.spec\.(ts|tsx|js|jsx)$`), // TypeScript/JavaScript .spec
		regexp.MustCompile(`^test_.*\.py$`),            // Python test_ prefix
		regexp.MustCompile(`_test\.py$`),               // Python _test suffix
		regexp.MustCompile(`Test\.java$`),              // Java
		regexp.MustCompile(`_test\.rs$`),               // Rust
		regexp.MustCompile(`_test\.rb$`),               // Ruby
		regexp.MustCompile(`\.test\.rb$`),              // Ruby .test
		regexp.MustCompile(`_spec\.rb$`),               // Ruby rspec
	}

	// Non-code file patterns
	nonCodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\.md$`),
		regexp.MustCompile(`\.txt$`),
		regexp.MustCompile(`\.rst$`),
		regexp.MustCompile(`\.adoc$`),
		regexp.MustCompile(`LICENSE`),
		regexp.MustCompile(`CHANGELOG`),
		regexp.MustCompile(`\.gitignore$`),
		regexp.MustCompile(`\.editorconfig$`),
	}

	// Code file patterns
	codeFilePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\.(go|ts|tsx|js|jsx|py|java|rs|rb|c|cpp|h|hpp|cs|swift|kt)$`),
	}
)

// TDDClassification represents how a commit relates to TDD practices
type TDDClassification string

const (
	ClassTestFirst TDDClassification = "test-first"
	ClassTestWith  TDDClassification = "test-with"
	ClassTestAfter TDDClassification = "test-after"
	ClassTestOnly  TDDClassification = "test-only"
	ClassNoTest    TDDClassification = "no-test"
	ClassNonCode   TDDClassification = "non-code"
)

// CommitInfo holds parsed commit information
type CommitInfo struct {
	Hash    string
	Author  string
	Date    string
	Message string
	Files   []string
}

// ClassifiedCommit holds a commit with its TDD classification
type ClassifiedCommit struct {
	CommitInfo
	Classification TDDClassification
	HasTestFiles   bool
	HasCodeFiles   bool
}

// TDDViolation represents a TDD compliance violation
type TDDViolation struct {
	CommitHash     string   `json:"commit_hash"`
	Author         string   `json:"author"`
	Date           string   `json:"date"`
	Message        string   `json:"message"`
	Files          []string `json:"files"`
	Classification string   `json:"classification"`
	Severity       string   `json:"severity"` // "error" for no-test, "warning" for test-after
	Remediation    string   `json:"remediation"`
}

// TDDBreakdown holds counts for each classification
type TDDBreakdown struct {
	TestFirst int `json:"test_first"`
	TestWith  int `json:"test_with"`
	TestAfter int `json:"test_after"`
	TestOnly  int `json:"test_only"`
	NoTest    int `json:"no_test"`
	NonCode   int `json:"non_code"`
}

// TDDComplianceResult holds the complete TDD analysis result
type TDDComplianceResult struct {
	TotalCommits     int            `json:"total_commits"`
	TotalCodeCommits int            `json:"total_code_commits"`
	Breakdown        TDDBreakdown   `json:"breakdown"`
	ComplianceScore  float64        `json:"compliance_score"`
	ComplianceGrade  string         `json:"compliance_grade"`
	Violations       []TDDViolation `json:"violations"`
	ViolationsCount  int            `json:"violations_count"`
	Message          string         `json:"message,omitempty"`
}

// newTddComplianceCmd creates the tdd-compliance command
func newTddComplianceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tdd-compliance",
		Short: "Analyze git history for TDD compliance",
		Long: `Analyze git commit history and classify commits by TDD patterns.

Classifications:
  test-first: Test files committed before implementation
  test-with:  Test and implementation files in same commit
  test-after: Implementation committed before tests
  test-only:  Only test files modified
  no-test:    Implementation without any associated tests
  non-code:   Documentation or config changes (excluded from scoring)

Compliance Score Formula:
  (test-first × 100 + test-with × 75 + test-after × 25 + no-test × 0) / total_code_commits

Grade Thresholds:
  A: 90-100, B: 75-89, C: 60-74, D: 40-59, F: 0-39

Examples:
  llm-support tdd-compliance --path ./repo --since "2026-01-01"
  llm-support tdd-compliance --content "hash|author|date|msg|files" --json
  llm-support tdd-compliance --path ./repo --count 50`,
		RunE: runTddCompliance,
	}

	cmd.Flags().StringVar(&tddCompliancePath, "path", "", "Path to git repository")
	cmd.Flags().StringVar(&tddComplianceContent, "content", "", "Git log content (pipe-delimited: hash|author|date|message|files)")
	cmd.Flags().StringVar(&tddComplianceSince, "since", "", "Analyze commits since date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&tddComplianceUntil, "until", "", "Analyze commits until date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&tddComplianceCount, "count", 0, "Maximum number of commits to analyze")
	cmd.Flags().BoolVar(&tddComplianceJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&tddComplianceMinimal, "min", false, "Minimal output format")

	return cmd
}

func runTddCompliance(cmd *cobra.Command, args []string) error {
	var commits []CommitInfo

	if tddComplianceContent != "" {
		// Parse from content flag
		commits = parseGitLogContent(tddComplianceContent)
	} else if tddCompliancePath != "" {
		// TODO: Execute git log command on path
		// For now, return empty result
		commits = []CommitInfo{}
	} else {
		// Default to current directory
		commits = []CommitInfo{}
	}

	// Classify commits
	classified := classifyCommits(commits)

	// Calculate metrics
	result := calculateTDDMetrics(classified)

	formatter := output.New(tddComplianceJSON, tddComplianceMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(TDDComplianceResult)
		fmt.Fprintf(w, "TDD_COMPLIANCE:\n")
		fmt.Fprintf(w, "  Score: %.1f%% (Grade: %s)\n", r.ComplianceScore, r.ComplianceGrade)
		fmt.Fprintf(w, "  Total Commits: %d (Code: %d)\n", r.TotalCommits, r.TotalCodeCommits)
		fmt.Fprintf(w, "  Breakdown:\n")
		fmt.Fprintf(w, "    Test-First: %d\n", r.Breakdown.TestFirst)
		fmt.Fprintf(w, "    Test-With: %d\n", r.Breakdown.TestWith)
		fmt.Fprintf(w, "    Test-After: %d\n", r.Breakdown.TestAfter)
		fmt.Fprintf(w, "    No-Test: %d\n", r.Breakdown.NoTest)
		fmt.Fprintf(w, "    Non-Code: %d\n", r.Breakdown.NonCode)
		if r.ViolationsCount > 0 {
			fmt.Fprintf(w, "  Violations: %d\n", r.ViolationsCount)
		}
	})
}

// parseGitLogContent parses pipe-delimited git log content
// Format: hash|author|date|message|files (files comma-separated)
func parseGitLogContent(content string) []CommitInfo {
	var commits []CommitInfo

	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		files := strings.Split(parts[4], ",")
		for i := range files {
			files[i] = strings.TrimSpace(files[i])
		}

		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
			Files:   files,
		})
	}

	return commits
}

// isTestFile checks if a filename matches test file patterns
func isTestFile(filename string) bool {
	base := filepath.Base(filename)
	for _, pattern := range testFilePatterns {
		if pattern.MatchString(base) {
			return true
		}
	}
	return false
}

// isCodeFile checks if a filename is a code file
func isCodeFile(filename string) bool {
	base := filepath.Base(filename)
	for _, pattern := range codeFilePatterns {
		if pattern.MatchString(base) {
			return true
		}
	}
	return false
}

// isNonCodeFile checks if a filename is documentation/config
func isNonCodeFile(filename string) bool {
	base := filepath.Base(filename)
	for _, pattern := range nonCodePatterns {
		if pattern.MatchString(base) {
			return true
		}
	}
	return false
}

// classifyCommits classifies each commit by TDD pattern
func classifyCommits(commits []CommitInfo) []ClassifiedCommit {
	var classified []ClassifiedCommit

	// First pass: classify each commit individually
	for _, commit := range commits {
		hasTest := false
		hasCode := false
		hasNonCode := false

		for _, file := range commit.Files {
			if isTestFile(file) {
				hasTest = true
			} else if isCodeFile(file) {
				hasCode = true
			} else if isNonCodeFile(file) {
				hasNonCode = true
			}
		}

		var classification TDDClassification
		if hasTest && hasCode {
			classification = ClassTestWith
		} else if hasTest && !hasCode {
			classification = ClassTestOnly
		} else if hasCode && !hasTest {
			// Need to determine if test-first, test-after, or no-test
			// For now, mark as potential no-test (will refine in second pass)
			classification = ClassNoTest
		} else if hasNonCode && !hasCode && !hasTest {
			classification = ClassNonCode
		} else {
			classification = ClassNonCode
		}

		classified = append(classified, ClassifiedCommit{
			CommitInfo:     commit,
			Classification: classification,
			HasTestFiles:   hasTest,
			HasCodeFiles:   hasCode,
		})
	}

	// Second pass: detect test-first and test-after patterns
	// Look at temporal ordering of commits
	classified = detectTemporalPatterns(classified)

	return classified
}

// detectTemporalPatterns analyzes commit order to detect test-first/test-after
func detectTemporalPatterns(commits []ClassifiedCommit) []ClassifiedCommit {
	if len(commits) < 2 {
		return commits
	}

	// Build map of files to their test files
	// e.g., auth.go -> auth_test.go, feature.ts -> feature.test.ts
	fileToTest := make(map[string]string)
	testToFile := make(map[string]string)

	for _, commit := range commits {
		for _, file := range commit.Files {
			if isTestFile(file) {
				// Find corresponding implementation file
				implFile := getImplementationFile(file)
				if implFile != "" {
					testToFile[file] = implFile
					fileToTest[implFile] = file
				}
			}
		}
	}

	// Track when we first see each implementation file and its test
	implFirstSeen := make(map[string]int)
	testFirstSeen := make(map[string]int)

	for i, commit := range commits {
		for _, file := range commit.Files {
			if isTestFile(file) {
				if _, exists := testFirstSeen[file]; !exists {
					testFirstSeen[file] = i
				}
			} else if isCodeFile(file) {
				if _, exists := implFirstSeen[file]; !exists {
					implFirstSeen[file] = i
				}
			}
		}
	}

	// Re-classify based on temporal patterns
	for i := range commits {
		if commits[i].Classification == ClassNoTest {
			// Check if any of the implementation files have tests that came before
			hasTestFirst := false
			hasTestAfter := false

			for _, file := range commits[i].Files {
				if !isCodeFile(file) || isTestFile(file) {
					continue
				}

				testFile := fileToTest[file]
				if testFile == "" {
					continue
				}

				testIdx, testExists := testFirstSeen[testFile]
				if testExists {
					if testIdx < i {
						hasTestFirst = true
					} else if testIdx > i {
						hasTestAfter = true
					}
				}
			}

			if hasTestFirst {
				commits[i].Classification = ClassTestFirst
			} else if hasTestAfter {
				commits[i].Classification = ClassTestAfter
			}
			// Otherwise remains ClassNoTest
		}
	}

	return commits
}

// getImplementationFile returns the implementation file for a test file
func getImplementationFile(testFile string) string {
	base := filepath.Base(testFile)
	dir := filepath.Dir(testFile)

	// Go: _test.go -> .go
	if strings.HasSuffix(base, "_test.go") {
		return filepath.Join(dir, strings.TrimSuffix(base, "_test.go")+".go")
	}

	// TypeScript/JavaScript: .test.ts -> .ts, .spec.ts -> .ts
	for _, suffix := range []string{".test.ts", ".spec.ts", ".test.tsx", ".spec.tsx", ".test.js", ".spec.js", ".test.jsx", ".spec.jsx"} {
		if strings.HasSuffix(base, suffix) {
			ext := strings.TrimPrefix(suffix, ".test")
			ext = strings.TrimPrefix(ext, ".spec")
			return filepath.Join(dir, strings.TrimSuffix(base, suffix)+ext)
		}
	}

	// Python: test_*.py -> *.py, *_test.py -> *.py
	if strings.HasPrefix(base, "test_") && strings.HasSuffix(base, ".py") {
		return filepath.Join(dir, strings.TrimPrefix(base, "test_"))
	}
	if strings.HasSuffix(base, "_test.py") {
		return filepath.Join(dir, strings.TrimSuffix(base, "_test.py")+".py")
	}

	return ""
}

// calculateTDDMetrics computes the compliance score and violations
func calculateTDDMetrics(commits []ClassifiedCommit) TDDComplianceResult {
	var breakdown TDDBreakdown
	var violations []TDDViolation

	for _, commit := range commits {
		switch commit.Classification {
		case ClassTestFirst:
			breakdown.TestFirst++
		case ClassTestWith:
			breakdown.TestWith++
		case ClassTestAfter:
			breakdown.TestAfter++
			// Test-after is a warning-level violation
			violations = append(violations, createViolation(commit, "warning"))
		case ClassTestOnly:
			breakdown.TestOnly++
		case ClassNoTest:
			breakdown.NoTest++
			// No-test is an error-level violation
			violations = append(violations, createViolation(commit, "error"))
		case ClassNonCode:
			breakdown.NonCode++
		}
	}

	totalCommits := len(commits)
	totalCodeCommits := breakdown.TestFirst + breakdown.TestWith + breakdown.TestAfter + breakdown.NoTest

	// Calculate score
	var score float64
	if totalCodeCommits > 0 {
		// Weighted scoring: test-first=100, test-with=75, test-after=25, no-test=0
		points := float64(breakdown.TestFirst)*100 +
			float64(breakdown.TestWith)*75 +
			float64(breakdown.TestAfter)*25 +
			float64(breakdown.NoTest)*0
		score = points / float64(totalCodeCommits)
	}

	grade := calculateGrade(score)

	// Sort violations by severity (errors first) then by date
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Severity != violations[j].Severity {
			return violations[i].Severity == "error"
		}
		return violations[i].Date > violations[j].Date
	})

	result := TDDComplianceResult{
		TotalCommits:     totalCommits,
		TotalCodeCommits: totalCodeCommits,
		Breakdown:        breakdown,
		ComplianceScore:  score,
		ComplianceGrade:  grade,
		Violations:       violations,
		ViolationsCount:  len(violations),
	}

	if totalCommits == 0 {
		result.Message = "no commits found in range"
	} else if totalCodeCommits == 0 {
		result.Message = "no code commits to analyze"
	}

	return result
}

// createViolation creates a TDDViolation from a classified commit
func createViolation(commit ClassifiedCommit, severity string) TDDViolation {
	// Truncate message if too long
	message := commit.Message
	if len(message) > 200 {
		message = message[:197] + "..."
	}

	// Generate remediation suggestion
	remediation := generateRemediation(commit)

	return TDDViolation{
		CommitHash:     commit.Hash,
		Author:         commit.Author,
		Date:           commit.Date,
		Message:        message,
		Files:          commit.Files,
		Classification: string(commit.Classification),
		Severity:       severity,
		Remediation:    remediation,
	}
}

// generateRemediation creates a remediation suggestion for a violation
func generateRemediation(commit ClassifiedCommit) string {
	var codeFiles []string
	for _, file := range commit.Files {
		if isCodeFile(file) && !isTestFile(file) {
			codeFiles = append(codeFiles, file)
		}
	}

	if len(codeFiles) == 0 {
		return "Review commit for missing tests"
	}

	if len(codeFiles) == 1 {
		testFile := suggestTestFile(codeFiles[0])
		return fmt.Sprintf("Add tests for: %s (suggested: %s)", codeFiles[0], testFile)
	}

	return fmt.Sprintf("Add tests for: %s", strings.Join(codeFiles, ", "))
}

// suggestTestFile suggests a test file name for an implementation file
func suggestTestFile(implFile string) string {
	ext := filepath.Ext(implFile)
	base := strings.TrimSuffix(filepath.Base(implFile), ext)
	dir := filepath.Dir(implFile)

	switch ext {
	case ".go":
		return filepath.Join(dir, base+"_test.go")
	case ".ts", ".tsx":
		return filepath.Join(dir, base+".test"+ext)
	case ".js", ".jsx":
		return filepath.Join(dir, base+".test"+ext)
	case ".py":
		return filepath.Join(dir, "test_"+base+".py")
	default:
		return filepath.Join(dir, base+"_test"+ext)
	}
}

// calculateGrade returns the letter grade for a score
func calculateGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func init() {
	RootCmd.AddCommand(newTddComplianceCmd())
}
