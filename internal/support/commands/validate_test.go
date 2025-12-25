package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCommand(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "validate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	validJSON := filepath.Join(tmpDir, "valid.json")
	os.WriteFile(validJSON, []byte(`{"name": "test", "value": 123}`), 0644)

	invalidJSON := filepath.Join(tmpDir, "invalid.json")
	os.WriteFile(invalidJSON, []byte(`{"name": "test",}`), 0644)

	validTOML := filepath.Join(tmpDir, "valid.toml")
	os.WriteFile(validTOML, []byte("[section]\nkey = \"value\""), 0644)

	invalidTOML := filepath.Join(tmpDir, "invalid.toml")
	os.WriteFile(invalidTOML, []byte("[section\nkey = value"), 0644)

	validMD := filepath.Join(tmpDir, "valid.md")
	os.WriteFile(validMD, []byte("# Title\n\nContent here"), 0644)

	emptyMD := filepath.Join(tmpDir, "empty.md")
	os.WriteFile(emptyMD, []byte(""), 0644)

	validCSV := filepath.Join(tmpDir, "valid.csv")
	os.WriteFile(validCSV, []byte("name,value\ntest,123\nfoo,456"), 0644)

	validYAML := filepath.Join(tmpDir, "valid.yaml")
	os.WriteFile(validYAML, []byte("name: test\nvalue: 123"), 0644)

	invalidYAML := filepath.Join(tmpDir, "invalid.yaml")
	os.WriteFile(invalidYAML, []byte("name: test\n  bad indent: here\n    worse: indent"), 0644)

	tests := []struct {
		name       string
		files      []string
		wantValid  bool
		wantOutput []string
	}{
		{
			name:       "valid JSON",
			files:      []string{validJSON},
			wantValid:  true,
			wantOutput: []string{"VALID", "JSON"},
		},
		{
			name:       "invalid JSON",
			files:      []string{invalidJSON},
			wantValid:  false,
			wantOutput: []string{"INVALID", "JSON"},
		},
		{
			name:       "valid TOML",
			files:      []string{validTOML},
			wantValid:  true,
			wantOutput: []string{"VALID", "TOML"},
		},
		{
			name:       "invalid TOML",
			files:      []string{invalidTOML},
			wantValid:  false,
			wantOutput: []string{"INVALID", "TOML"},
		},
		{
			name:       "valid Markdown",
			files:      []string{validMD},
			wantValid:  true,
			wantOutput: []string{"VALID", "MARKDOWN"},
		},
		{
			name:       "empty Markdown",
			files:      []string{emptyMD},
			wantValid:  false,
			wantOutput: []string{"INVALID", "Empty file"},
		},
		{
			name:       "valid CSV",
			files:      []string{validCSV},
			wantValid:  true,
			wantOutput: []string{"VALID", "CSV"},
		},
		{
			name:       "valid YAML",
			files:      []string{validYAML},
			wantValid:  true,
			wantOutput: []string{"VALID", "YAML"},
		},
		{
			name:       "invalid YAML",
			files:      []string{invalidYAML},
			wantValid:  false,
			wantOutput: []string{"INVALID", "YAML"},
		},
		{
			name:       "multiple valid files",
			files:      []string{validJSON, validTOML, validMD},
			wantValid:  true,
			wantOutput: []string{"ALL_VALID: TRUE", "VALID_COUNT: 3"},
		},
		{
			name:       "mixed valid and invalid",
			files:      []string{validJSON, invalidJSON},
			wantValid:  false,
			wantOutput: []string{"ALL_VALID: FALSE", "VALID_COUNT: 1", "INVALID_COUNT: 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newValidateCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs(tt.files)

			err := cmd.Execute()

			combined := stdout.String() + stderr.String()

			if tt.wantValid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Errorf("expected invalid (error), got nil")
			}

			for _, want := range tt.wantOutput {
				if !bytes.Contains([]byte(combined), []byte(want)) {
					t.Errorf("output missing %q, got: %s", want, combined)
				}
			}
		})
	}
}

func TestValidateNonexistentFile(t *testing.T) {
	cmd := newValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"/nonexistent/file.json"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for nonexistent file")
	}

	combined := stdout.String() + stderr.String()
	if !bytes.Contains([]byte(combined), []byte("INVALID")) {
		t.Errorf("expected INVALID in output, got: %s", combined)
	}
}

func TestValidateUnsupportedFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "validate-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	unsupported := filepath.Join(tmpDir, "file.xyz")
	os.WriteFile(unsupported, []byte("content"), 0644)

	cmd := newValidateCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{unsupported})

	err = cmd.Execute()
	if err == nil {
		t.Error("expected error for unsupported format")
	}

	combined := stdout.String() + stderr.String()
	if !bytes.Contains([]byte(combined), []byte("Unsupported format")) {
		t.Errorf("expected 'Unsupported format' in output, got: %s", combined)
	}
}
