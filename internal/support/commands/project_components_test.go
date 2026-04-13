package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// createTempConfig creates a temporary config.yaml with the given content and returns its path.
func createTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

type projectComponentsResponse struct {
	Components     []componentEntry `json:"components"`
	IsMonorepo     bool             `json:"is_monorepo"`
	ComponentCount int              `json:"component_count"`
}

type componentEntry struct {
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	Framework       string                 `json:"framework"`
	PackageManager  string                 `json:"package_manager"`
	SourceDirectory string                 `json:"source_directory"`
	Testing         map[string]interface{} `json:"testing"`
	Commands        map[string]interface{} `json:"commands"`
}

func TestProjectComponents_FlatConfig(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  type: go
  framework: ""
  package_manager: go
  source_directory: internal/
testing:
  cmd: go test ./...
  coverage_cmd: go test -cover ./...
commands:
  lint: go vet ./...
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "default") {
		t.Errorf("expected 'default' component name in output: %s", output)
	}
	if !strings.Contains(output, "is_monorepo: false") && !strings.Contains(output, "IS_MONOREPO: false") {
		t.Errorf("expected is_monorepo false in output: %s", output)
	}
	if !strings.Contains(output, "go") {
		t.Errorf("expected 'go' type in output: %s", output)
	}
}

func TestProjectComponents_NestedConfig(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  backend:
    type: python
    framework: fastapi
    package_manager: pip
    source_directory: backend/app/
  frontend:
    type: node
    framework: sveltekit
    package_manager: npm
    source_directory: frontend/src/
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "backend") {
		t.Errorf("expected 'backend' component in output: %s", output)
	}
	if !strings.Contains(output, "frontend") {
		t.Errorf("expected 'frontend' component in output: %s", output)
	}
	if !strings.Contains(output, "is_monorepo: true") && !strings.Contains(output, "IS_MONOREPO: true") {
		t.Errorf("expected is_monorepo true in output: %s", output)
	}
	if !strings.Contains(output, "python") {
		t.Errorf("expected 'python' type in output: %s", output)
	}
	if !strings.Contains(output, "node") {
		t.Errorf("expected 'node' type in output: %s", output)
	}
}

func TestProjectComponents_NestedWithPerComponentTesting(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  backend:
    type: python
    framework: fastapi
    package_manager: pip
    source_directory: backend/app/
testing:
  backend:
    cmd: cd backend && pytest
    coverage_cmd: cd backend && pytest --cov
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "cd backend && pytest") {
		t.Errorf("expected per-component testing cmd resolved in output: %s", output)
	}
}

func TestProjectComponents_NestedWithFlatTestingFallback(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  backend:
    type: python
    framework: fastapi
    package_manager: pip
    source_directory: backend/app/
  frontend:
    type: node
    framework: sveltekit
    package_manager: npm
    source_directory: frontend/src/
testing:
  cmd: make test
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Both components should fall back to the flat testing.cmd
	count := strings.Count(output, "make test")
	if count < 2 {
		t.Errorf("expected 'make test' to appear for both components (at least 2 times), got %d in output: %s", count, output)
	}
}

func TestProjectComponents_EmptyProject(t *testing.T) {
	configPath := createTempConfig(t, `
project: {}
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "0") {
		t.Errorf("expected 0 components in output: %s", output)
	}
}

func TestProjectComponents_JSONOutput(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  backend:
    type: python
    framework: fastapi
    package_manager: pip
    source_directory: backend/app/
  frontend:
    type: node
    framework: sveltekit
    package_manager: npm
    source_directory: frontend/src/
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	var resp projectComponentsResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	if !resp.IsMonorepo {
		t.Errorf("expected is_monorepo=true, got false")
	}
	if resp.ComponentCount != 2 {
		t.Errorf("expected component_count=2, got %d", resp.ComponentCount)
	}
	if len(resp.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(resp.Components))
	}

	// Verify component details
	names := map[string]bool{}
	for _, c := range resp.Components {
		names[c.Name] = true
	}
	if !names["backend"] {
		t.Errorf("expected 'backend' component in JSON response")
	}
	if !names["frontend"] {
		t.Errorf("expected 'frontend' component in JSON response")
	}
}

func TestProjectComponents_MinOutput(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  type: go
  framework: ""
  package_manager: go
  source_directory: internal/
testing:
  cmd: go test ./...
  coverage_cmd: go test -cover ./...
commands:
  lint: go vet ./...
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Min output should still contain essential info but be shorter
	if !strings.Contains(output, "default") {
		t.Errorf("expected 'default' component in min output: %s", output)
	}
}

func TestProjectComponents_MissingFile(t *testing.T) {
	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--file", "/nonexistent/path/config.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
	}
}

func TestProjectComponents_FlatWithNestedTesting(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  type: go
  source_directory: src/
testing:
  backend:
    cmd: pytest
  cmd: go test ./...
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Flat project should produce single "default" component using testing.cmd (flat key)
	if !strings.Contains(output, "default") {
		t.Errorf("expected 'default' component for flat project: %s", output)
	}
	if !strings.Contains(output, "go test ./...") {
		t.Errorf("expected flat testing.cmd 'go test ./...' in output: %s", output)
	}
}

func TestProjectComponents_NestedWithCommands(t *testing.T) {
	configPath := createTempConfig(t, `
project:
  backend:
    type: python
    source_directory: backend/
  frontend:
    type: node
    source_directory: frontend/
commands:
  backend:
    lint: ruff check .
    build: docker compose build
  frontend:
    lint: npm run lint
    build: npm run build
`)

	cmd := newProjectComponentsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", configPath})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "ruff check .") {
		t.Errorf("expected backend lint command 'ruff check .' in output: %s", output)
	}
	if !strings.Contains(output, "npm run lint") {
		t.Errorf("expected frontend lint command 'npm run lint' in output: %s", output)
	}
	if !strings.Contains(output, "docker compose build") {
		t.Errorf("expected backend build command 'docker compose build' in output: %s", output)
	}
	if !strings.Contains(output, "npm run build") {
		t.Errorf("expected frontend build command 'npm run build' in output: %s", output)
	}
}
