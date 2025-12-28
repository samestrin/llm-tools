package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlanTypeDetection tests the plan type extraction from files
func TestPlanTypeDetection(t *testing.T) {
	tests := []struct {
		name            string
		metadataContent string
		planContent     string
		expectedType    string
		expectWarning   bool
	}{
		{
			name:            "type found in metadata.md",
			metadataContent: "# Plan\nPlan Type: feature\nOther content",
			planContent:     "",
			expectedType:    "feature",
			expectWarning:   false,
		},
		{
			name:            "type found in plan.md (fallback)",
			metadataContent: "",
			planContent:     "# Plan\nPlan Type: bugfix\n",
			expectedType:    "bugfix",
			expectWarning:   false,
		},
		{
			name:            "metadata.md takes precedence",
			metadataContent: "Plan Type: feature\n",
			planContent:     "Plan Type: bugfix\n",
			expectedType:    "feature",
			expectWarning:   false,
		},
		{
			name:            "case insensitive extraction",
			metadataContent: "PLAN TYPE: FEATURE\n",
			planContent:     "",
			expectedType:    "feature",
			expectWarning:   false,
		},
		{
			name:            "whitespace handling",
			metadataContent: "Plan Type:   tech-debt   \n",
			planContent:     "",
			expectedType:    "tech-debt",
			expectWarning:   false,
		},
		{
			name:            "multiple Plan Type lines uses first",
			metadataContent: "Plan Type: bugfix\nPlan Type: feature\n",
			planContent:     "",
			expectedType:    "bugfix",
			expectWarning:   false,
		},
		{
			name:            "no files exist defaults to feature",
			metadataContent: "",
			planContent:     "",
			expectedType:    "feature",
			expectWarning:   false, // warnings suppressed in --min mode
		},
		{
			name:            "files exist but no Plan Type field",
			metadataContent: "# Some content\nNo plan type here",
			planContent:     "# Also no plan type",
			expectedType:    "feature",
			expectWarning:   false, // warnings suppressed in --min mode
		},
		{
			name:            "test-remediation type",
			metadataContent: "Plan Type: test-remediation\n",
			planContent:     "",
			expectedType:    "test-remediation",
			expectWarning:   false,
		},
		{
			name:            "infrastructure type",
			metadataContent: "Plan Type: infrastructure\n",
			planContent:     "",
			expectedType:    "infrastructure",
			expectWarning:   false,
		},
		{
			name:            "underscore to hyphen normalization",
			metadataContent: "Plan Type: test_remediation\n",
			planContent:     "",
			expectedType:    "test-remediation",
			expectWarning:   false,
		},
		{
			name:            "tech_debt underscore variant",
			metadataContent: "Plan Type: tech_debt\n",
			planContent:     "",
			expectedType:    "tech-debt",
			expectWarning:   false,
		},
		{
			name:            "markdown bold formatting",
			metadataContent: "**Plan Type:** feature\n",
			planContent:     "",
			expectedType:    "feature",
			expectWarning:   false,
		},
		{
			name:            "markdown italic formatting",
			metadataContent: "*Plan Type:* bugfix\n",
			planContent:     "",
			expectedType:    "bugfix",
			expectWarning:   false,
		},
		{
			name:            "backtick formatting",
			metadataContent: "Plan Type: `tech-debt`\n",
			planContent:     "",
			expectedType:    "tech-debt",
			expectWarning:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Create test files if content provided
			if tt.metadataContent != "" {
				writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), tt.metadataContent)
			}
			if tt.planContent != "" {
				writeTestFile(t, filepath.Join(tmpDir, "plan.md"), tt.planContent)
			}

			// Run the command
			var stdout, stderr bytes.Buffer
			cmd := newPlanTypeCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--path", tmpDir, "--min"})

			err := cmd.Execute()

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			gotType := strings.TrimSpace(stdout.String())
			if gotType != tt.expectedType {
				t.Errorf("expected type %q, got %q", tt.expectedType, gotType)
			}

			hasWarning := strings.Contains(stderr.String(), "Warning")
			if hasWarning != tt.expectWarning {
				t.Errorf("expected warning=%v, got warning=%v, stderr: %s", tt.expectWarning, hasWarning, stderr.String())
			}
		})
	}
}

// TestPlanTypeValidation tests invalid plan types
func TestPlanTypeValidation(t *testing.T) {
	invalidTypes := []string{
		"custom",
		"unknown",
		"features",
		"bug-fix",
		"feature-dev",
		"testing",
	}

	for _, invalidType := range invalidTypes {
		t.Run("invalid_"+invalidType, func(t *testing.T) {
			tmpDir := t.TempDir()
			writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), "Plan Type: "+invalidType+"\n")

			var stdout, stderr bytes.Buffer
			cmd := newPlanTypeCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--path", tmpDir})

			err := cmd.Execute()

			if err == nil {
				t.Errorf("expected error for invalid type %q, got none", invalidType)
				return
			}

			if !strings.Contains(err.Error(), "Invalid plan type") {
				t.Errorf("expected 'Invalid plan type' error, got: %v", err)
			}

			// Error should list valid types
			if !strings.Contains(err.Error(), "feature") || !strings.Contains(err.Error(), "bugfix") {
				t.Errorf("error should list valid types, got: %v", err)
			}
		})
	}
}

// TestPlanTypeEnrichment tests enrichment data for all plan types
func TestPlanTypeEnrichment(t *testing.T) {
	tests := []struct {
		planType                 string
		expectedLabel            string
		expectedIcon             string
		expectedRequiresStories  bool
		expectedWorkSource       string
	}{
		{"feature", "Feature Development", "✨", true, "user-stories"},
		{"bugfix", "Bug Fix", "🐛", false, "tasks"},
		{"test-remediation", "Test Remediation", "🧪", false, "tasks"},
		{"tech-debt", "Technical Debt", "🔧", false, "tasks"},
		{"infrastructure", "Infrastructure", "🏗️", false, "tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.planType, func(t *testing.T) {
			tmpDir := t.TempDir()
			writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), "Plan Type: "+tt.planType+"\n")

			var stdout bytes.Buffer
			cmd := newPlanTypeCmd()
			cmd.SetOut(&stdout)
			cmd.SetArgs([]string{"--path", tmpDir, "--json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result PlanTypeResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v", err)
			}

			if result.Type != tt.planType {
				t.Errorf("expected type %q, got %q", tt.planType, result.Type)
			}
			if result.Label != tt.expectedLabel {
				t.Errorf("expected label %q, got %q", tt.expectedLabel, result.Label)
			}
			if result.Icon != tt.expectedIcon {
				t.Errorf("expected icon %q, got %q", tt.expectedIcon, result.Icon)
			}
			if result.RequiresUserStories != tt.expectedRequiresStories {
				t.Errorf("expected requires_user_stories=%v, got %v", tt.expectedRequiresStories, result.RequiresUserStories)
			}
			if result.WorkSource != tt.expectedWorkSource {
				t.Errorf("expected work_source=%q, got %q", tt.expectedWorkSource, result.WorkSource)
			}
		})
	}
}

// TestOutputModes tests all output mode combinations
func TestOutputModes(t *testing.T) {
	tests := []struct {
		name           string
		jsonFlag       bool
		minFlag        bool
		validateOutput func(t *testing.T, output string)
	}{
		{
			name:     "default text output",
			jsonFlag: false,
			minFlag:  false,
			validateOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "Plan Type: feature") {
					t.Errorf("expected 'Plan Type: feature' in output, got: %s", output)
				}
				if !strings.Contains(output, "Label: Feature Development") {
					t.Errorf("expected 'Label:' in output, got: %s", output)
				}
			},
		},
		{
			name:     "json output",
			jsonFlag: true,
			minFlag:  false,
			validateOutput: func(t *testing.T, output string) {
				var result PlanTypeResult
				if err := json.Unmarshal([]byte(output), &result); err != nil {
					t.Errorf("invalid JSON output: %v", err)
				}
				if result.Type != "feature" {
					t.Errorf("expected type 'feature', got %q", result.Type)
				}
				if result.Label == "" {
					t.Error("expected non-empty label in JSON output")
				}
			},
		},
		{
			name:     "minimal output",
			jsonFlag: false,
			minFlag:  true,
			validateOutput: func(t *testing.T, output string) {
				trimmed := strings.TrimSpace(output)
				if trimmed != "feature" {
					t.Errorf("expected 'feature', got %q", trimmed)
				}
			},
		},
		{
			name:     "json minimal output",
			jsonFlag: true,
			minFlag:  true,
			validateOutput: func(t *testing.T, output string) {
				expected := `{"type":"feature"}`
				trimmed := strings.TrimSpace(output)
				if trimmed != expected {
					t.Errorf("expected %q, got %q", expected, trimmed)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), "Plan Type: feature\n")

			var stdout bytes.Buffer
			cmd := newPlanTypeCmd()
			cmd.SetOut(&stdout)

			args := []string{"--path", tmpDir}
			if tt.jsonFlag {
				args = append(args, "--json")
			}
			if tt.minFlag {
				args = append(args, "--min")
			}
			cmd.SetArgs(args)

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validateOutput(t, stdout.String())
		})
	}
}

// TestPathHandling tests path resolution and validation
func TestPathHandling(t *testing.T) {
	t.Run("non-existent directory", func(t *testing.T) {
		var stdout bytes.Buffer
		cmd := newPlanTypeCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"--path", "/non/existent/path/that/does/not/exist"})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for non-existent directory")
			return
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("expected 'does not exist' error, got: %v", err)
		}
	})

	t.Run("path is a file not directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "somefile.txt")
		writeTestFile(t, filePath, "content")

		var stdout bytes.Buffer
		cmd := newPlanTypeCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"--path", filePath})

		err := cmd.Execute()
		if err == nil {
			t.Error("expected error for file path")
			return
		}
		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("expected 'not a directory' error, got: %v", err)
		}
	})

	t.Run("trailing slash handling", func(t *testing.T) {
		tmpDir := t.TempDir()
		writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), "Plan Type: feature\n")

		var stdout bytes.Buffer
		cmd := newPlanTypeCmd()
		cmd.SetOut(&stdout)
		cmd.SetArgs([]string{"--path", tmpDir + "/", "--min"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if strings.TrimSpace(stdout.String()) != "feature" {
			t.Errorf("expected 'feature', got %q", stdout.String())
		}
	})
}

// TestWarningsSuppressedInMinimalMode tests that warnings are suppressed with --min
func TestWarningsSuppressedInMinimalMode(t *testing.T) {
	tmpDir := t.TempDir()
	// No files = warning condition

	var stdout, stderr bytes.Buffer
	cmd := newPlanTypeCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--path", tmpDir, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stderr.Len() > 0 {
		t.Errorf("expected no stderr in minimal mode, got: %s", stderr.String())
	}

	if strings.TrimSpace(stdout.String()) != "feature" {
		t.Errorf("expected 'feature', got %q", stdout.String())
	}
}

// TestWarningsShownInTextMode tests that warnings appear in default text mode
func TestWarningsShownInTextMode(t *testing.T) {
	t.Run("no files exist shows warning", func(t *testing.T) {
		tmpDir := t.TempDir()
		// No files = warning condition

		var stdout, stderr bytes.Buffer
		cmd := newPlanTypeCmd()
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--path", tmpDir}) // No --min flag

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stderr.String(), "Warning") {
			t.Errorf("expected warning in stderr, got: %s", stderr.String())
		}
	})

	t.Run("no Plan Type field shows warning", func(t *testing.T) {
		tmpDir := t.TempDir()
		writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), "# No plan type here\n")

		var stdout, stderr bytes.Buffer
		cmd := newPlanTypeCmd()
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--path", tmpDir}) // No --min flag

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(stderr.String(), "Warning") {
			t.Errorf("expected warning in stderr, got: %s", stderr.String())
		}
	})
}

// TestPlanTypeInfoMap tests the planTypeInfo map contains all expected types
func TestPlanTypeInfoMap(t *testing.T) {
	expectedTypes := []string{"feature", "bugfix", "test-remediation", "tech-debt", "infrastructure"}

	for _, typ := range expectedTypes {
		info, ok := planTypeInfo[typ]
		if !ok {
			t.Errorf("planTypeInfo missing type %q", typ)
			continue
		}
		if info.Type != typ {
			t.Errorf("planTypeInfo[%q].Type = %q, want %q", typ, info.Type, typ)
		}
		if info.Label == "" {
			t.Errorf("planTypeInfo[%q].Label is empty", typ)
		}
		if info.Icon == "" {
			t.Errorf("planTypeInfo[%q].Icon is empty", typ)
		}
		if info.WorkSource == "" {
			t.Errorf("planTypeInfo[%q].WorkSource is empty", typ)
		}
	}

	if len(planTypeInfo) != 5 {
		t.Errorf("planTypeInfo should have exactly 5 entries, got %d", len(planTypeInfo))
	}
}

// TestJSONOutputValid tests that JSON output is valid and can be unmarshaled
func TestJSONOutputValid(t *testing.T) {
	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "metadata.md"), "Plan Type: bugfix\n")

	var stdout bytes.Buffer
	cmd := newPlanTypeCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--path", tmpDir, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.Bytes()
	if !json.Valid(output) {
		t.Errorf("output is not valid JSON: %s", output)
	}

	var result PlanTypeResult
	if err := json.Unmarshal(output, &result); err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}
}

// writeTestFile is a helper to write test fixtures
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
