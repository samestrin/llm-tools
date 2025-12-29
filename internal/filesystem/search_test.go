package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file structure
	os.MkdirAll(filepath.Join(tmpDir, "src", "components"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "utils.go"), []byte("package src"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "components", "button.tsx"), []byte("export default"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "components", "card.tsx"), []byte("export default"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Readme"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("node_modules"), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantMissing  []string
		wantErr      bool
	}{
		{
			name: "search by extension",
			args: map[string]interface{}{
				"path":    tmpDir,
				"pattern": "*.go",
			},
			wantContains: []string{"main.go", "utils.go"},
			wantMissing:  []string{"button.tsx"},
			wantErr:      false,
		},
		{
			name: "search recursive",
			args: map[string]interface{}{
				"path":      tmpDir,
				"pattern":   "*.tsx",
				"recursive": true,
			},
			wantContains: []string{"button.tsx", "card.tsx"},
			wantErr:      false,
		},
		{
			name: "search with glob pattern",
			args: map[string]interface{}{
				"path":    tmpDir,
				"pattern": "main.*",
			},
			wantContains: []string{"main.go"},
			wantErr:      false,
		},
		{
			name: "search hidden files",
			args: map[string]interface{}{
				"path":        tmpDir,
				"pattern":     ".*",
				"show_hidden": true,
			},
			wantContains: []string{".gitignore"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleSearchFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSearchFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleSearchFiles() = %v, want to contain %v", result, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(result, missing) {
					t.Errorf("handleSearchFiles() = %v, should not contain %v", result, missing)
				}
			}
		})
	}
}

func TestSearchCode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files with searchable content
	os.MkdirAll(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src", "main.go"), []byte(`package main

func main() {
	fmt.Println("Hello, World!")
}

func helper() {
	// TODO: implement
}
`), 0644)
	os.WriteFile(filepath.Join(tmpDir, "src", "utils.go"), []byte(`package main

func formatString(s string) string {
	return strings.TrimSpace(s)
}

// TODO: add more utils
`), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte(`# Project

TODO: add description
`), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantMissing  []string
		wantErr      bool
	}{
		{
			name: "search for TODO",
			args: map[string]interface{}{
				"path":    tmpDir,
				"pattern": "TODO",
			},
			wantContains: []string{"TODO"},
			wantErr:      false,
		},
		{
			name: "search for function",
			args: map[string]interface{}{
				"path":    tmpDir,
				"pattern": "func main",
			},
			wantContains: []string{"func main"},
			wantErr:      false,
		},
		{
			name: "search with file type filter",
			args: map[string]interface{}{
				"path":       tmpDir,
				"pattern":    "TODO",
				"file_types": []interface{}{".go"},
			},
			wantContains: []string{"main.go", "utils.go"},
			wantMissing:  []string{"README.md"},
			wantErr:      false,
		},
		{
			name: "search case insensitive",
			args: map[string]interface{}{
				"path":            tmpDir,
				"pattern":         "todo",
				"case_insensitive": true,
			},
			wantContains: []string{"TODO"},
			wantErr:      false,
		},
		{
			name: "search with context lines",
			args: map[string]interface{}{
				"path":          tmpDir,
				"pattern":       "func main",
				"context_lines": float64(2),
			},
			wantContains: []string{"package main", "fmt.Println"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleSearchCode(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSearchCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleSearchCode() = %v, want to contain %v", result, want)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(result, missing) {
					t.Errorf("handleSearchCode() = %v, should not contain %v", result, missing)
				}
			}
		})
	}
}

func TestSearchCodeRegex(t *testing.T) {
	tmpDir := t.TempDir()

	os.WriteFile(filepath.Join(tmpDir, "code.go"), []byte(`
func getData() string {
	return "data"
}

func getUser() User {
	return User{}
}

func processData() {
	// process
}
`), 0644)

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name: "regex search for get* functions",
			args: map[string]interface{}{
				"path":    tmpDir,
				"pattern": `func get\w+\(`,
				"regex":   true,
			},
			wantContains: []string{"getData", "getUser"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleSearchCode(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleSearchCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleSearchCode() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}
