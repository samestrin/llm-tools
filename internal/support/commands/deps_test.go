package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDepsPackageJSON(t *testing.T) {
	// Create temp package.json
	tmpDir := t.TempDir()
	pkgJSON := filepath.Join(tmpDir, "package.json")
	content := `{
  "name": "test-app",
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "~4.17.21"
  },
  "devDependencies": {
    "jest": "^29.0.0",
    "eslint": "^8.0.0"
  },
  "optionalDependencies": {
    "fsevents": "^2.3.0"
  }
}`
	if err := os.WriteFile(pkgJSON, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		typeFlag string
		wantDeps int
	}{
		{"all dependencies", "all", 5},
		{"prod only", "prod", 2},
		{"dev only", "dev", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newDepsCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{pkgJSON, "--type", tt.typeFlag})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			if !strings.Contains(output, "DEPENDENCIES:") {
				t.Errorf("expected DEPENDENCIES in output, got: %s", output)
			}
		})
	}
}

func TestDepsPackageJSONWithJSON(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := filepath.Join(tmpDir, "package.json")
	content := `{
  "name": "test-app",
  "dependencies": {
    "express": "^4.18.0"
  }
}`
	if err := os.WriteFile(pkgJSON, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{pkgJSON, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ManifestType != "package.json" {
		t.Errorf("expected manifest_type 'package.json', got: %s", result.ManifestType)
	}
	if len(result.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got: %d", len(result.Dependencies))
	}
}

func TestDepsGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	content := `module example.com/app

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/stretchr/testify v1.8.4
)

require github.com/single/dep v1.0.0
`
	if err := os.WriteFile(goMod, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{goMod, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ManifestType != "go.mod" {
		t.Errorf("expected manifest_type 'go.mod', got: %s", result.ManifestType)
	}
	if len(result.Dependencies) != 3 {
		t.Errorf("expected 3 dependencies, got: %d", len(result.Dependencies))
	}
}

func TestDepsRequirementsTxt(t *testing.T) {
	tmpDir := t.TempDir()
	reqTxt := filepath.Join(tmpDir, "requirements.txt")
	content := `# Core dependencies
flask==2.3.0
requests>=2.28.0,<3.0.0
# Testing
pytest==7.4.0
-e ./local/package
`
	if err := os.WriteFile(reqTxt, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{reqTxt, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ManifestType != "requirements.txt" {
		t.Errorf("expected manifest_type 'requirements.txt', got: %s", result.ManifestType)
	}
	if len(result.Dependencies) != 4 {
		t.Errorf("expected 4 dependencies, got: %d", len(result.Dependencies))
	}
}

func TestDepsCargoToml(t *testing.T) {
	tmpDir := t.TempDir()
	cargoToml := filepath.Join(tmpDir, "Cargo.toml")
	content := `[package]
name = "test-app"
version = "0.1.0"

[dependencies]
serde = "1.0"
tokio = { version = "1.0", features = ["full"] }

[dev-dependencies]
criterion = "0.5"
`
	if err := os.WriteFile(cargoToml, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{cargoToml, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ManifestType != "Cargo.toml" {
		t.Errorf("expected manifest_type 'Cargo.toml', got: %s", result.ManifestType)
	}
	if len(result.Dependencies) != 3 {
		t.Errorf("expected 3 dependencies, got: %d", len(result.Dependencies))
	}
}

func TestDepsGemfile(t *testing.T) {
	tmpDir := t.TempDir()
	gemfile := filepath.Join(tmpDir, "Gemfile")
	content := `source 'https://rubygems.org'

gem 'rails', '~> 7.0'
gem 'pg', '>= 1.0'

group :development do
  gem 'pry'
end
`
	if err := os.WriteFile(gemfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{gemfile, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ManifestType != "Gemfile" {
		t.Errorf("expected manifest_type 'Gemfile', got: %s", result.ManifestType)
	}
	if len(result.Dependencies) != 3 {
		t.Errorf("expected 3 dependencies, got: %d", len(result.Dependencies))
	}
}

func TestDepsPyprojectToml(t *testing.T) {
	tmpDir := t.TempDir()
	pyproject := filepath.Join(tmpDir, "pyproject.toml")
	content := `[project]
name = "test-app"
dependencies = [
    "flask>=2.0",
    "requests",
]

[project.optional-dependencies]
dev = [
    "pytest",
]
`
	if err := os.WriteFile(pyproject, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{pyproject, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if result.ManifestType != "pyproject.toml" {
		t.Errorf("expected manifest_type 'pyproject.toml', got: %s", result.ManifestType)
	}
	if len(result.Dependencies) != 3 {
		t.Errorf("expected 3 dependencies, got: %d", len(result.Dependencies))
	}
}

func TestDepsNonExistentFile(t *testing.T) {
	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"/nonexistent/package.json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "manifest file not found") {
		t.Errorf("expected 'manifest file not found' error, got: %v", err)
	}
}

func TestDepsUnsupportedManifest(t *testing.T) {
	tmpDir := t.TempDir()
	unknown := filepath.Join(tmpDir, "unknown.txt")
	if err := os.WriteFile(unknown, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{unknown})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported manifest")
	}
	if !strings.Contains(err.Error(), "unsupported manifest type") {
		t.Errorf("expected 'unsupported manifest type' error, got: %v", err)
	}
}

func TestDepsInvalidTypeFlag(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgJSON, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{pkgJSON, "--type", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid type flag")
	}
	if !strings.Contains(err.Error(), "type must be: all, prod, or dev") {
		t.Errorf("expected type validation error, got: %v", err)
	}
}

func TestDepsEmptyPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgJSON, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{pkgJSON, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result DepsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(result.Dependencies) != 0 {
		t.Errorf("expected 0 dependencies for empty package.json, got: %d", len(result.Dependencies))
	}
}

// TestDepsMinimalOutput tests minimal output mode
func TestDepsMinimalOutput(t *testing.T) {
	tmpDir := t.TempDir()
	pkgJSON := filepath.Join(tmpDir, "package.json")
	content := `{"dependencies": {"express": "^4.18.0"}}`
	os.WriteFile(pkgJSON, []byte(content), 0644)

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{pkgJSON, "--min"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Minimal output should contain dependencies
	if !strings.Contains(output, "express") {
		t.Errorf("minimal output should contain 'express', got: %s", output)
	}
}

// TestDepsPyprojectTomlWithPoetry tests pyproject.toml with Poetry format
func TestDepsPyprojectTomlWithPoetry(t *testing.T) {
	tmpDir := t.TempDir()
	pyproject := filepath.Join(tmpDir, "pyproject.toml")
	content := `[tool.poetry]
name = "test-app"

[tool.poetry.dependencies]
python = "^3.10"
requests = "^2.28.0"

[tool.poetry.dev-dependencies]
pytest = "^7.0"
`
	os.WriteFile(pyproject, []byte(content), 0644)

	cmd := newDepsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{pyproject, "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should parse Poetry format
	if !strings.Contains(output, "pyproject.toml") {
		t.Errorf("output should contain pyproject.toml, got: %s", output)
	}
}
