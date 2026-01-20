package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGroupTDBasicPathGrouping(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Issue 1", "EST_MINUTES": 30},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "Issue 2", "EST_MINUTES": 45},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Issue 3", "EST_MINUTES": 60},
		{"FILE_LINE": "src/api/users.ts:40", "PROBLEM": "Issue 4", "EST_MINUTES": 20},
		{"FILE_LINE": "src/api/orders.ts:50", "PROBLEM": "Issue 5", "EST_MINUTES": 25}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "2"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nOutput: %s", err, buf.String())
	}

	// Should have 2 groups: src-auth and src-api
	if result.Summary.GroupCount != 2 {
		t.Errorf("expected 2 groups, got %d", result.Summary.GroupCount)
	}

	// All items should be grouped
	if result.Summary.UngroupedCount != 0 {
		t.Errorf("expected 0 ungrouped, got %d", result.Summary.UngroupedCount)
	}

	// Check total minutes
	authGroup := findGroupByTheme(result.Groups, "src-auth")
	if authGroup == nil {
		t.Fatal("expected src-auth group")
	}
	if authGroup.TotalMinutes != 135 { // 30 + 45 + 60
		t.Errorf("expected 135 total minutes for src-auth, got %d", authGroup.TotalMinutes)
	}
}

func TestGroupTDMinGroupSize(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Issue 1"},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "Issue 2"},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Issue 3"},
		{"FILE_LINE": "src/auth/session.ts:40", "PROBLEM": "Issue 4"},
		{"FILE_LINE": "src/api/users.ts:50", "PROBLEM": "Issue 5"},
		{"FILE_LINE": "src/api/orders.ts:60", "PROBLEM": "Issue 6"},
		{"FILE_LINE": "src/utils/helper.ts:70", "PROBLEM": "Issue 7"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// Only src-auth should qualify as a group (4 items)
	// src-api (2 items) and src-utils (1 item) should be ungrouped
	if result.Summary.GroupCount != 1 {
		t.Errorf("expected 1 group, got %d", result.Summary.GroupCount)
	}

	if result.Summary.GroupedCount != 4 {
		t.Errorf("expected 4 grouped items, got %d", result.Summary.GroupedCount)
	}

	if result.Summary.UngroupedCount != 3 {
		t.Errorf("expected 3 ungrouped items, got %d", result.Summary.UngroupedCount)
	}
}

func TestGroupTDPathDepth(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/handlers/login.ts:10", "PROBLEM": "Issue 1"},
		{"FILE_LINE": "src/auth/handlers/logout.ts:20", "PROBLEM": "Issue 2"},
		{"FILE_LINE": "src/auth/handlers/validate.ts:30", "PROBLEM": "Issue 3"},
		{"FILE_LINE": "src/auth/middleware/session.ts:40", "PROBLEM": "Issue 4"},
		{"FILE_LINE": "src/auth/middleware/token.ts:50", "PROBLEM": "Issue 5"},
		{"FILE_LINE": "src/auth/middleware/refresh.ts:60", "PROBLEM": "Issue 6"}
	]`

	tests := []struct {
		name         string
		depth        int
		expectGroups int
		expectThemes []string
	}{
		{
			name:         "depth 2 - all in src-auth",
			depth:        2,
			expectGroups: 1,
			expectThemes: []string{"src-auth"},
		},
		{
			name:         "depth 3 - split by handlers/middleware",
			depth:        3,
			expectGroups: 2,
			expectThemes: []string{"src-auth-handlers", "src-auth-middleware"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newGroupTDCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--content", input, "--json", "--path-depth", string(rune('0' + tt.depth)), "--min-group-size", "3"})

			// Need to reset and re-parse
			groupTDPathDepth = tt.depth
			groupTDMinGroupSize = 3

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result GroupTDResult
			json.Unmarshal(buf.Bytes(), &result)

			if result.Summary.GroupCount != tt.expectGroups {
				t.Errorf("expected %d groups, got %d", tt.expectGroups, result.Summary.GroupCount)
			}

			for _, theme := range tt.expectThemes {
				if findGroupByTheme(result.Groups, theme) == nil {
					t.Errorf("expected theme %s to exist", theme)
				}
			}
		})
	}
}

func TestGroupTDCriticalOverride(t *testing.T) {
	input := `[
		{"SEVERITY": "CRITICAL", "FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Security hole"},
		{"SEVERITY": "CRITICAL", "FILE_LINE": "src/api/users.ts:20", "PROBLEM": "Data leak"},
		{"SEVERITY": "HIGH", "FILE_LINE": "src/auth/logout.ts:30", "PROBLEM": "Bug"},
		{"SEVERITY": "HIGH", "FILE_LINE": "src/auth/validate.ts:40", "PROBLEM": "Bug 2"},
		{"SEVERITY": "HIGH", "FILE_LINE": "src/auth/session.ts:50", "PROBLEM": "Bug 3"}
	]`

	t.Run("with critical override", func(t *testing.T) {
		cmd := newGroupTDCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--content", input, "--json", "--critical-override=true", "--min-group-size", "3"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result GroupTDResult
		json.Unmarshal(buf.Bytes(), &result)

		// Should have critical group + src-auth group
		criticalGroup := findGroupByTheme(result.Groups, "critical")
		if criticalGroup == nil {
			t.Fatal("expected critical group")
		}
		if criticalGroup.Count != 2 {
			t.Errorf("expected 2 critical items, got %d", criticalGroup.Count)
		}

		// Critical should be first
		if result.Groups[0].Theme != "critical" {
			t.Error("critical group should be first")
		}
	})

	t.Run("without critical override", func(t *testing.T) {
		cmd := newGroupTDCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"--content", input, "--json", "--critical-override=false", "--min-group-size", "2"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var result GroupTDResult
		json.Unmarshal(buf.Bytes(), &result)

		// Should not have separate critical group
		criticalGroup := findGroupByTheme(result.Groups, "critical")
		if criticalGroup != nil {
			t.Error("should not have critical group when override disabled")
		}
	})
}

func TestGroupTDGroupByCategory(t *testing.T) {
	input := `[
		{"CATEGORY": "security", "PROBLEM": "Issue 1"},
		{"CATEGORY": "security", "PROBLEM": "Issue 2"},
		{"CATEGORY": "security", "PROBLEM": "Issue 3"},
		{"CATEGORY": "performance", "PROBLEM": "Issue 4"},
		{"CATEGORY": "performance", "PROBLEM": "Issue 5"},
		{"CATEGORY": "performance", "PROBLEM": "Issue 6"},
		{"CATEGORY": "documentation", "PROBLEM": "Issue 7"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--group-by", "category", "--min-group-size", "3"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// Should have security and performance groups
	if result.Summary.GroupCount != 2 {
		t.Errorf("expected 2 groups, got %d", result.Summary.GroupCount)
	}

	securityGroup := findGroupByTheme(result.Groups, "security")
	if securityGroup == nil || securityGroup.Count != 3 {
		t.Error("expected security group with 3 items")
	}

	// documentation should be ungrouped
	if result.Summary.UngroupedCount != 1 {
		t.Errorf("expected 1 ungrouped, got %d", result.Summary.UngroupedCount)
	}
}

func TestGroupTDGroupByFile(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth.ts:10", "PROBLEM": "Issue 1"},
		{"FILE_LINE": "src/auth.ts:20", "PROBLEM": "Issue 2"},
		{"FILE_LINE": "src/auth.ts:30", "PROBLEM": "Issue 3"},
		{"FILE_LINE": "src/api.ts:40", "PROBLEM": "Issue 4"},
		{"FILE_LINE": "src/api.ts:50", "PROBLEM": "Issue 5"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--group-by", "file", "--min-group-size", "2"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// Should have 2 groups by exact file
	if result.Summary.GroupCount != 2 {
		t.Errorf("expected 2 groups, got %d", result.Summary.GroupCount)
	}

	// Theme should be full file path with hyphens
	authGroup := findGroupByTheme(result.Groups, "src-auth.ts")
	if authGroup == nil {
		t.Error("expected src-auth.ts group")
	}
}

func TestGroupTDMissingFileLine(t *testing.T) {
	input := `[
		{"PROBLEM": "Issue without file", "CATEGORY": "security"},
		{"PROBLEM": "Another without file", "CATEGORY": "security"},
		{"PROBLEM": "Third without file", "CATEGORY": "security"},
		{"FILE_LINE": "src/auth.ts:10", "PROBLEM": "With file", "CATEGORY": "other"},
		{"FILE_LINE": "src/auth.ts:20", "PROBLEM": "With file 2", "CATEGORY": "other"},
		{"FILE_LINE": "src/auth.ts:30", "PROBLEM": "With file 3", "CATEGORY": "other"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--group-by", "path", "--min-group-size", "3"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// Items without FILE_LINE should fall back to CATEGORY
	securityGroup := findGroupByTheme(result.Groups, "security")
	if securityGroup == nil {
		t.Error("expected security group (fallback from missing FILE_LINE)")
	}

	// src/auth.ts items should form src group
	srcGroup := findGroupByTheme(result.Groups, "src")
	if srcGroup == nil {
		t.Error("expected src group")
	}
}

func TestGroupTDRootTheme(t *testing.T) {
	input := `[
		{"FILE_LINE": "config.ts:10", "PROBLEM": "Root file 1"},
		{"FILE_LINE": "main.ts:20", "PROBLEM": "Root file 2"},
		{"FILE_LINE": "index.ts:30", "PROBLEM": "Root file 3"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--root-theme", "root-level", "--min-group-size", "3"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// All root files should be in root-level group
	rootGroup := findGroupByTheme(result.Groups, "root-level")
	if rootGroup == nil {
		t.Error("expected root-level group")
	}
	if rootGroup.Count != 3 {
		t.Errorf("expected 3 items in root-level, got %d", rootGroup.Count)
	}
}

func TestGroupTDDataIntegrity(t *testing.T) {
	// Critical test: ensure no data loss
	input := `[
		{"FILE_LINE": "src/a.ts:1", "PROBLEM": "1"},
		{"FILE_LINE": "src/b.ts:2", "PROBLEM": "2"},
		{"FILE_LINE": "src/c.ts:3", "PROBLEM": "3"},
		{"FILE_LINE": "src/d.ts:4", "PROBLEM": "4"},
		{"FILE_LINE": "src/e.ts:5", "PROBLEM": "5"},
		{"FILE_LINE": "lib/a.ts:6", "PROBLEM": "6"},
		{"FILE_LINE": "lib/b.ts:7", "PROBLEM": "7"},
		{"CATEGORY": "misc", "PROBLEM": "8"},
		{"CATEGORY": "misc", "PROBLEM": "9"},
		{"SEVERITY": "CRITICAL", "PROBLEM": "10"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// Must have exactly 10 items total
	totalOutput := result.Summary.GroupedCount + result.Summary.UngroupedCount
	if totalOutput != 10 {
		t.Errorf("DATA LOSS: expected 10 items, got %d (grouped: %d, ungrouped: %d)",
			totalOutput, result.Summary.GroupedCount, result.Summary.UngroupedCount)
	}

	if result.Summary.TotalItems != 10 {
		t.Errorf("TotalItems should be 10, got %d", result.Summary.TotalItems)
	}
}

func TestGroupTDInputFormats(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectItems int
	}{
		{
			name:        "items wrapper",
			input:       `{"items": [{"PROBLEM": "1"}, {"PROBLEM": "2"}]}`,
			expectItems: 2,
		},
		{
			name:        "rows wrapper",
			input:       `{"rows": [{"PROBLEM": "1"}, {"PROBLEM": "2"}, {"PROBLEM": "3"}]}`,
			expectItems: 3,
		},
		{
			name:        "raw array",
			input:       `[{"PROBLEM": "1"}]`,
			expectItems: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newGroupTDCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs([]string{"--content", tt.input, "--json"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var result GroupTDResult
			json.Unmarshal(buf.Bytes(), &result)

			if result.Summary.TotalItems != tt.expectItems {
				t.Errorf("expected %d items, got %d", tt.expectItems, result.Summary.TotalItems)
			}
		})
	}
}

func TestGroupTDFileInput(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "input.json")

	content := `[{"FILE_LINE": "src/auth.ts:1", "PROBLEM": "Test"}]`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--file", tmpFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	if result.Summary.TotalItems != 1 {
		t.Errorf("expected 1 item, got %d", result.Summary.TotalItems)
	}
}

func TestGroupTDTextOutput(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/a.ts:1", "PROBLEM": "Issue 1", "EST_MINUTES": 30},
		{"FILE_LINE": "src/auth/b.ts:2", "PROBLEM": "Issue 2", "EST_MINUTES": 45},
		{"FILE_LINE": "src/auth/c.ts:3", "PROBLEM": "Issue 3", "EST_MINUTES": 60}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Check section headers
	if output == "" {
		t.Error("expected non-empty output")
	}
	// Text output should have group info
	if !bytes.Contains(buf.Bytes(), []byte("GROUPS")) {
		t.Error("expected GROUPS header")
	}
	if !bytes.Contains(buf.Bytes(), []byte("SUMMARY")) {
		t.Error("expected SUMMARY header")
	}
}

func TestGroupTDMinimalOutput(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/a.ts:1", "PROBLEM": "Issue 1"},
		{"FILE_LINE": "src/auth/b.ts:2", "PROBLEM": "Issue 2"},
		{"FILE_LINE": "src/auth/c.ts:3", "PROBLEM": "Issue 3"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Minimal should be concise
	if bytes.Contains(buf.Bytes(), []byte("GROUPS")) {
		t.Error("minimal output should not have GROUPS header")
	}
	// Should have group name and count
	if output == "" {
		t.Error("expected non-empty minimal output")
	}
}

func TestGroupTDInvalidFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr string
	}{
		{
			name:      "invalid group-by",
			args:      []string{"--content", "[]", "--group-by", "invalid"},
			expectErr: "invalid group-by",
		},
		{
			name:      "path-depth too low",
			args:      []string{"--content", "[]", "--path-depth", "0"},
			expectErr: "path-depth must be at least 1",
		},
		{
			name:      "min-group-size too low",
			args:      []string{"--content", "[]", "--min-group-size", "0"},
			expectErr: "min-group-size must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newGroupTDCmd()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.expectErr)) {
				t.Errorf("expected error containing %q, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestGroupTDNoInput(t *testing.T) {
	// Reset flags
	groupTDContent = ""
	groupTDFile = ""

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no input")
	}
}

func TestGroupTDInvalidJSON(t *testing.T) {
	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", "not valid json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractPathTheme(t *testing.T) {
	tests := []struct {
		name      string
		fileLine  string
		depth     int
		rootTheme string
		expected  string
	}{
		{"depth 2", "src/auth/handlers/login.ts:45", 2, "misc", "src-auth"},
		{"depth 3", "src/auth/handlers/login.ts:45", 3, "misc", "src-auth-handlers"},
		{"depth 1", "src/auth/handlers/login.ts:45", 1, "misc", "src"},
		{"depth exceeds", "src/auth.ts:45", 5, "misc", "src"},
		{"root file", "config.ts:10", 2, "misc", "misc"},
		{"empty dir", "file.ts", 2, "root", "root"},
		{"windows path", "src\\auth\\login.ts:10", 2, "misc", "src-auth"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPathTheme(tt.fileLine, tt.depth, tt.rootTheme)
			if result != tt.expected {
				t.Errorf("extractPathTheme(%s, %d, %s) = %s, want %s",
					tt.fileLine, tt.depth, tt.rootTheme, result, tt.expected)
			}
		})
	}
}

func TestExtractEstMinutesInt(t *testing.T) {
	tests := []struct {
		name     string
		item     map[string]interface{}
		expected int
	}{
		{"float64", map[string]interface{}{"EST_MINUTES": float64(30)}, 30},
		{"int", map[string]interface{}{"EST_MINUTES": 45}, 45},
		{"string", map[string]interface{}{"EST_MINUTES": "60"}, 60},
		{"json.Number", map[string]interface{}{"EST_MINUTES": json.Number("90")}, 90},
		{"missing", map[string]interface{}{"OTHER": "value"}, 0},
		{"nil", map[string]interface{}{"EST_MINUTES": nil}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEstMinutesInt(tt.item)
			if result != tt.expected {
				t.Errorf("extractEstMinutesInt() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestExtractFileLine(t *testing.T) {
	tests := []struct {
		name     string
		item     map[string]interface{}
		expected string
	}{
		{"FILE_LINE", map[string]interface{}{"FILE_LINE": "src/a.ts:10"}, "src/a.ts:10"},
		{"FILE", map[string]interface{}{"FILE": "src/b.ts"}, "src/b.ts"},
		{"PATH", map[string]interface{}{"PATH": "src/c.ts"}, "src/c.ts"},
		{"priority", map[string]interface{}{"FILE_LINE": "first", "FILE": "second"}, "first"},
		{"missing", map[string]interface{}{"OTHER": "value"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFileLine(tt.item)
			if result != tt.expected {
				t.Errorf("extractFileLine() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestGroupTDEmptyInput(t *testing.T) {
	input := `[]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	if result.Summary.TotalItems != 0 {
		t.Errorf("expected 0 items, got %d", result.Summary.TotalItems)
	}
	if len(result.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(result.Groups))
	}
	if len(result.Ungrouped) != 0 {
		t.Errorf("expected 0 ungrouped, got %d", len(result.Ungrouped))
	}
}

// Helper function
func findGroupByTheme(groups []TDGroup, theme string) *TDGroup {
	for _, g := range groups {
		if g.Theme == theme {
			return &g
		}
	}
	return nil
}
