package gitignore

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewParser verifies parser creation
func TestNewParser(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	parser, err := NewParser(tmpDir)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}

	if parser == nil {
		t.Fatal("Parser should not be nil")
	}
}

// TestNewParserWithGitignore verifies parser loads .gitignore
func TestNewParserWithGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore
	gitignore := filepath.Join(tmpDir, ".gitignore")
	err := os.WriteFile(gitignore, []byte("*.log\nnode_modules/\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, err := NewParser(tmpDir)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}

	// Test .log file is ignored
	logFile := filepath.Join(tmpDir, "debug.log")
	if !parser.IsIgnored(logFile) {
		t.Error("*.log pattern should match debug.log")
	}
}

// TestIsIgnoredExtensionPattern tests *.ext patterns
func TestIsIgnoredExtensionPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore with extension pattern
	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.log\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"matches .log", "debug.log", true},
		{"matches nested .log", "logs/error.log", true},
		{"does not match .txt", "readme.txt", false},
		{"does not match .logger", "app.logger", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, tt.path)
			result := parser.IsIgnored(fullPath)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestIsIgnoredDirectoryPattern tests dir/ patterns
func TestIsIgnoredDirectoryPattern(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules/\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"matches directory", "node_modules", true},
		{"matches file in directory", "node_modules/package.json", true},
		{"does not match similar name", "my_node_modules", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, tt.path)
			result := parser.IsIgnored(fullPath)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestIsIgnoredNoGitignore tests behavior when no .gitignore exists
func TestIsIgnoredNoGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	parser, _ := NewParser(tmpDir)

	// Without .gitignore, nothing should be ignored
	testFile := filepath.Join(tmpDir, "test.log")
	if parser.IsIgnored(testFile) {
		t.Error("With no .gitignore, nothing should be ignored")
	}
}

// TestIsIgnoredCommentLines tests that comments are ignored
func TestIsIgnoredCommentLines(t *testing.T) {
	tmpDir := t.TempDir()

	// .gitignore with comments
	content := `# This is a comment
*.log
# Another comment
*.tmp
`
	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	// .log and .tmp should be ignored, but comment text should not affect matching
	if !parser.IsIgnored(filepath.Join(tmpDir, "test.log")) {
		t.Error("*.log should be matched")
	}
	if !parser.IsIgnored(filepath.Join(tmpDir, "test.tmp")) {
		t.Error("*.tmp should be matched")
	}
}

// TestIsIgnoredOutsideRoot tests paths outside root directory
func TestIsIgnoredOutsideRoot(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.log\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	// Path outside root should not be ignored
	outsidePath := "/some/other/path/debug.log"
	if parser.IsIgnored(outsidePath) {
		t.Error("Paths outside root should not be ignored")
	}
}

// TestParserAbsolutePathResolution tests that parser handles relative and absolute paths
func TestParserAbsolutePathResolution(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("*.log\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	// Absolute path should work
	absPath := filepath.Join(tmpDir, "error.log")
	if !parser.IsIgnored(absPath) {
		t.Error("Absolute path with matching pattern should be ignored")
	}
}

// TestRootPath tests that RootPath returns the correct path
func TestRootPath(t *testing.T) {
	tmpDir := t.TempDir()

	parser, err := NewParser(tmpDir)
	if err != nil {
		t.Fatalf("NewParser failed: %v", err)
	}

	if parser.RootPath() != tmpDir {
		t.Errorf("RootPath() = %q, want %q", parser.RootPath(), tmpDir)
	}
}

// TestIsIgnoredNegationPattern tests negation patterns (!)
func TestIsIgnoredNegationPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .gitignore with negation pattern
	content := `*.log
!important.log
`
	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"matches .log", "debug.log", true},
		{"negation should not ignore", "important.log", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, tt.path)
			result := parser.IsIgnored(fullPath)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestIsIgnoredDoubleStarPattern tests **/ patterns
func TestIsIgnoredDoubleStarPattern(t *testing.T) {
	tmpDir := t.TempDir()

	content := `**/secret/**
`
	err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	parser, _ := NewParser(tmpDir)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"matches nested secret", "a/b/secret/file.txt", true},
		{"matches secret at root", "secret/file.txt", true},
		{"does not match partial", "secrets/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullPath := filepath.Join(tmpDir, tt.path)
			result := parser.IsIgnored(fullPath)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
