package commands

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestTDDComplianceTestFirstPattern tests detection of test-first commits
func TestTDDComplianceTestFirstPattern(t *testing.T) {
	// Simulate git log output with test-first pattern
	gitLog := `abc1234|John Doe|2026-01-15|test: add auth tests|auth_test.go
def5678|John Doe|2026-01-15|feat: implement auth|auth.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, buf.String())
	}

	if result.Breakdown.TestFirst != 1 {
		t.Errorf("test_first = %d, want 1", result.Breakdown.TestFirst)
	}
}

// TestTDDComplianceTestWithPattern tests detection of test-with commits
func TestTDDComplianceTestWithPattern(t *testing.T) {
	// Commit with both test and implementation files
	gitLog := `abc1234|John Doe|2026-01-15|feat: add feature with tests|feature.go,feature_test.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Breakdown.TestWith != 1 {
		t.Errorf("test_with = %d, want 1", result.Breakdown.TestWith)
	}
}

// TestTDDComplianceTestAfterPattern tests detection of test-after commits
func TestTDDComplianceTestAfterPattern(t *testing.T) {
	// Implementation first, then test
	gitLog := `abc1234|John Doe|2026-01-15|feat: implement feature|feature.go
def5678|John Doe|2026-01-16|test: add feature tests|feature_test.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Breakdown.TestAfter != 1 {
		t.Errorf("test_after = %d, want 1", result.Breakdown.TestAfter)
	}
}

// TestTDDComplianceNoTestPattern tests detection of commits without tests
func TestTDDComplianceNoTestPattern(t *testing.T) {
	// Implementation without any tests
	gitLog := `abc1234|John Doe|2026-01-15|feat: add feature without tests|feature.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Breakdown.NoTest != 1 {
		t.Errorf("no_test = %d, want 1", result.Breakdown.NoTest)
	}
}

// TestTDDComplianceNonCodeCommits tests exclusion of non-code commits
func TestTDDComplianceNonCodeCommits(t *testing.T) {
	// Documentation commits should be excluded
	gitLog := `abc1234|John Doe|2026-01-15|docs: update readme|README.md,docs/guide.md`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Breakdown.NonCode != 1 {
		t.Errorf("non_code = %d, want 1", result.Breakdown.NonCode)
	}

	// Non-code shouldn't affect compliance score
	if result.TotalCodeCommits != 0 {
		t.Errorf("total_code_commits = %d, want 0", result.TotalCodeCommits)
	}
}

// TestTDDComplianceScorePerfect tests 100% compliance score
func TestTDDComplianceScorePerfect(t *testing.T) {
	// All test-first commits
	gitLog := `abc1234|John Doe|2026-01-15|test: add tests|auth_test.go
def5678|John Doe|2026-01-15|feat: implement auth|auth.go
ghi9012|John Doe|2026-01-16|test: add user tests|user_test.go
jkl3456|John Doe|2026-01-16|feat: implement user|user.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.ComplianceScore != 100.0 {
		t.Errorf("compliance_score = %.1f, want 100.0", result.ComplianceScore)
	}

	if result.ComplianceGrade != "A" {
		t.Errorf("compliance_grade = %s, want A", result.ComplianceGrade)
	}
}

// TestTDDComplianceScoreZero tests 0% compliance score
func TestTDDComplianceScoreZero(t *testing.T) {
	// All no-test commits
	gitLog := `abc1234|John Doe|2026-01-15|feat: add feature1|feature1.go
def5678|John Doe|2026-01-16|feat: add feature2|feature2.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.ComplianceScore != 0.0 {
		t.Errorf("compliance_score = %.1f, want 0.0", result.ComplianceScore)
	}

	if result.ComplianceGrade != "F" {
		t.Errorf("compliance_grade = %s, want F", result.ComplianceGrade)
	}
}

// TestTDDComplianceScoreMixed tests mixed compliance score
func TestTDDComplianceScoreMixed(t *testing.T) {
	// Mixed: 1 test-first (100), 1 test-with (75), 1 test-after (25), 1 no-test (0)
	// Score = (100 + 75 + 25 + 0) / 4 = 50
	gitLog := `abc1234|John Doe|2026-01-15|test: add tests|auth_test.go
def5678|John Doe|2026-01-15|feat: implement auth|auth.go
ghi9012|John Doe|2026-01-16|feat: add with test|user.go,user_test.go
jkl3456|John Doe|2026-01-17|feat: impl first|data.go
mno7890|John Doe|2026-01-18|test: test after|data_test.go
pqr1234|John Doe|2026-01-19|feat: no test|other.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Score should be between 0 and 100
	if result.ComplianceScore < 0 || result.ComplianceScore > 100 {
		t.Errorf("compliance_score = %.1f, want 0-100", result.ComplianceScore)
	}
}

// TestTDDComplianceViolations tests violation reporting
func TestTDDComplianceViolations(t *testing.T) {
	// Commits with violations (no-test and test-after)
	gitLog := `abc1234|John Doe|2026-01-15|feat: no test feature|feature.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.ViolationsCount == 0 {
		t.Error("expected at least 1 violation")
	}

	if len(result.Violations) == 0 {
		t.Error("violations array should not be empty")
	}

	// Check violation structure
	v := result.Violations[0]
	if v.CommitHash == "" {
		t.Error("violation should have commit_hash")
	}
	if v.Severity == "" {
		t.Error("violation should have severity")
	}
}

// TestTDDComplianceNoViolations tests clean repository
func TestTDDComplianceNoViolations(t *testing.T) {
	// All test-first commits = no violations
	gitLog := `abc1234|John Doe|2026-01-15|test: add tests|auth_test.go
def5678|John Doe|2026-01-15|feat: implement auth|auth.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.ViolationsCount != 0 {
		t.Errorf("violations_count = %d, want 0", result.ViolationsCount)
	}
}

// TestTDDComplianceEmptyInput tests empty git log
func TestTDDComplianceEmptyInput(t *testing.T) {
	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", "", "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.TotalCommits != 0 {
		t.Errorf("total_commits = %d, want 0", result.TotalCommits)
	}
}

// TestTDDComplianceGradeThresholds tests grade assignment thresholds
func TestTDDComplianceGradeThresholds(t *testing.T) {
	tests := []struct {
		name          string
		score         float64
		expectedGrade string
	}{
		{"A grade at 90", 90.0, "A"},
		{"A grade at 100", 100.0, "A"},
		{"B grade at 75", 75.0, "B"},
		{"B grade at 89", 89.0, "B"},
		{"C grade at 60", 60.0, "C"},
		{"C grade at 74", 74.0, "C"},
		{"D grade at 40", 40.0, "D"},
		{"D grade at 59", 59.0, "D"},
		{"F grade at 0", 0.0, "F"},
		{"F grade at 39", 39.0, "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grade := calculateGrade(tt.score)
			if grade != tt.expectedGrade {
				t.Errorf("grade for %.0f = %s, want %s", tt.score, grade, tt.expectedGrade)
			}
		})
	}
}

// TestTDDComplianceMultiLanguageTestFiles tests recognition of various test file conventions
func TestTDDComplianceMultiLanguageTestFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		isTest   bool
	}{
		{"Go test file", "auth_test.go", true},
		{"TypeScript test file", "auth.test.ts", true},
		{"TypeScript spec file", "auth.spec.ts", true},
		{"JavaScript test file", "auth.test.js", true},
		{"JavaScript spec file", "auth.spec.js", true},
		{"Python test prefix", "test_auth.py", true},
		{"Python test suffix", "auth_test.py", true},
		{"Regular Go file", "auth.go", false},
		{"Regular TypeScript file", "auth.ts", false},
		{"Regular Python file", "auth.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTestFile(tt.filename)
			if result != tt.isTest {
				t.Errorf("isTestFile(%s) = %v, want %v", tt.filename, result, tt.isTest)
			}
		})
	}
}

// TestTDDComplianceViolationSeverity tests severity assignment
func TestTDDComplianceViolationSeverity(t *testing.T) {
	// no-test should be "error", test-after should be "warning"
	gitLog := `abc1234|John Doe|2026-01-15|feat: no test|feature.go
def5678|John Doe|2026-01-16|feat: impl first|data.go
ghi9012|John Doe|2026-01-17|test: test after|data_test.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Should have at least one error severity (no-test)
	hasError := false
	for _, v := range result.Violations {
		if v.Severity == "error" {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("expected at least one violation with severity 'error'")
	}
}

// TestTDDComplianceRemediation tests remediation suggestions
func TestTDDComplianceRemediation(t *testing.T) {
	gitLog := `abc1234|John Doe|2026-01-15|feat: no test|auth/login.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if len(result.Violations) == 0 {
		t.Fatal("expected at least 1 violation")
	}

	v := result.Violations[0]
	if v.Remediation == "" {
		t.Error("violation should have remediation suggestion")
	}
}

// TestTDDComplianceLongContent tests handling of multi-line git log content
func TestTDDComplianceLongContent(t *testing.T) {
	content := `abc1234|John Doe|2026-01-15|test: add tests|auth_test.go
def5678|John Doe|2026-01-15|feat: implement auth|auth.go
ghi9012|Jane Doe|2026-01-16|test: add more tests|user_test.go
jkl3456|Jane Doe|2026-01-16|feat: implement user|user.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", content, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.TotalCommits != 4 {
		t.Errorf("total_commits = %d, want 4", result.TotalCommits)
	}
}

// TestTDDComplianceMinimalOutput tests minimal output mode
func TestTDDComplianceMinimalOutput(t *testing.T) {
	gitLog := `abc1234|John Doe|2026-01-15|test: add tests|auth_test.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected some output in minimal mode")
	}
}

// TestTDDComplianceNoInput tests error when no input provided
func TestTDDComplianceNoInput(t *testing.T) {
	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--json"})

	err := cmd.Execute()
	// Should run but with empty results (no commits analyzed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result TDDComplianceResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.TotalCommits != 0 {
		t.Errorf("total_commits = %d, want 0", result.TotalCommits)
	}
}

// TestTDDComplianceIsCodeFile tests code file detection
func TestTDDComplianceIsCodeFile(t *testing.T) {
	tests := []struct {
		name   string
		file   string
		isCode bool
	}{
		{"Go source", "main.go", true},
		{"TypeScript", "app.ts", true},
		{"Python", "script.py", true},
		{"README", "README.md", false},
		{"Config", "config.json", false},
		{"Makefile", "Makefile", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCodeFile(tt.file)
			if result != tt.isCode {
				t.Errorf("isCodeFile(%s) = %v, want %v", tt.file, result, tt.isCode)
			}
		})
	}
}

// TestTDDComplianceIsNonCodeFile tests non-code file detection
func TestTDDComplianceIsNonCodeFile(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		isNonCode bool
	}{
		{"README.md", "README.md", true},
		{"Markdown doc", "docs/guide.md", true},
		{"LICENSE file", "LICENSE", true},
		{"CHANGELOG", "CHANGELOG.md", true},
		{"gitignore", ".gitignore", true},
		{"Go source", "main.go", false},
		{"Go test", "main_test.go", false},
		{"Config JSON (treated as code)", "config.json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNonCodeFile(tt.file)
			if result != tt.isNonCode {
				t.Errorf("isNonCodeFile(%s) = %v, want %v", tt.file, result, tt.isNonCode)
			}
		})
	}
}

// TestGetImplementationFile tests test-to-implementation file mapping
func TestGetImplementationFile(t *testing.T) {
	tests := []struct {
		name         string
		testFile     string
		expectedImpl string
	}{
		{"Go test file", "auth_test.go", "auth.go"},
		{"Go test with dir", "pkg/auth_test.go", "pkg/auth.go"},
		{"TypeScript .test.ts", "auth.test.ts", "auth.ts"},
		{"TypeScript .spec.ts", "auth.spec.ts", "auth.ts"},
		{"TypeScript .test.tsx", "component.test.tsx", "component.tsx"},
		{"TypeScript .spec.tsx", "component.spec.tsx", "component.tsx"},
		{"JavaScript .test.js", "utils.test.js", "utils.js"},
		{"JavaScript .spec.js", "utils.spec.js", "utils.js"},
		{"JavaScript .test.jsx", "App.test.jsx", "App.jsx"},
		{"JavaScript .spec.jsx", "App.spec.jsx", "App.jsx"},
		{"Python test_ prefix", "test_auth.py", "auth.py"},
		{"Python _test suffix", "auth_test.py", "auth.py"},
		{"Unknown format", "unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getImplementationFile(tt.testFile)
			if result != tt.expectedImpl {
				t.Errorf("getImplementationFile(%s) = %q, want %q", tt.testFile, result, tt.expectedImpl)
			}
		})
	}
}

// TestSuggestTestFile tests implementation-to-test file suggestion
func TestSuggestTestFile(t *testing.T) {
	tests := []struct {
		name         string
		implFile     string
		expectedTest string
	}{
		{"Go file", "auth.go", "auth_test.go"},
		{"Go with dir", "pkg/auth.go", "pkg/auth_test.go"},
		{"TypeScript .ts", "auth.ts", "auth.test.ts"},
		{"TypeScript .tsx", "Component.tsx", "Component.test.tsx"},
		{"JavaScript .js", "utils.js", "utils.test.js"},
		{"JavaScript .jsx", "App.jsx", "App.test.jsx"},
		{"Python", "auth.py", "test_auth.py"},
		{"Unknown extension", "file.rb", "file_test.rb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := suggestTestFile(tt.implFile)
			if result != tt.expectedTest {
				t.Errorf("suggestTestFile(%s) = %q, want %q", tt.implFile, result, tt.expectedTest)
			}
		})
	}
}

// TestTDDComplianceHumanReadableOutput tests human-readable output mode
func TestTDDComplianceHumanReadableOutput(t *testing.T) {
	gitLog := `abc1234|John Doe|2026-01-15|test: add tests|auth_test.go
def5678|John Doe|2026-01-15|feat: implement auth|auth.go`

	cmd := newTddComplianceCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", gitLog})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("expected output in human-readable mode")
	}
}
