package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		{"FILE:LINE", map[string]interface{}{"FILE:LINE": "src/a.ts:10"}, "src/a.ts:10"},
		{"FILE_LINE", map[string]interface{}{"FILE_LINE": "src/a.ts:10"}, "src/a.ts:10"},
		{"FILE", map[string]interface{}{"FILE": "src/b.ts"}, "src/b.ts"},
		{"PATH", map[string]interface{}{"PATH": "src/c.ts"}, "src/c.ts"},
		{"FILE:LINE priority over FILE_LINE", map[string]interface{}{"FILE:LINE": "first", "FILE_LINE": "second"}, "first"},
		{"FILE_LINE priority over FILE", map[string]interface{}{"FILE_LINE": "first", "FILE": "second"}, "first"},
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

func TestGroupTDAssignNumbers(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Issue 1", "EST_MINUTES": 30},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "Issue 2", "EST_MINUTES": 45},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Issue 3", "EST_MINUTES": 60},
		{"FILE_LINE": "src/api/users.ts:40", "PROBLEM": "Issue 4", "EST_MINUTES": 20},
		{"FILE_LINE": "src/api/orders.ts:50", "PROBLEM": "Issue 5", "EST_MINUTES": 25},
		{"FILE_LINE": "src/api/items.ts:60", "PROBLEM": "Issue 6", "EST_MINUTES": 15},
		{"FILE_LINE": "lib/helper.ts:70", "PROBLEM": "Issue 7", "EST_MINUTES": 10, "SEVERITY": "LOW"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3", "--assign-numbers"})

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

	// Groups should have Number field set
	for _, g := range result.Groups {
		if g.Number == nil {
			t.Errorf("group %s should have Number set", g.Theme)
		}
	}

	// Items should have GROUP field injected
	for _, g := range result.Groups {
		for _, item := range g.Items {
			if _, ok := item["GROUP"]; !ok {
				t.Errorf("item in group %s should have GROUP field", g.Theme)
			}
		}
	}

	// Ungrouped items should have GROUP="U"
	for _, item := range result.Ungrouped {
		groupLabel, ok := item["GROUP"].(string)
		if !ok || groupLabel != "U" {
			t.Errorf("ungrouped item should have GROUP='U', got %v", item["GROUP"])
		}
	}
}

func TestGroupTDSoloDetection(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Issue 1", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "Issue 2", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Issue 3", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/api/users.ts:40", "PROBLEM": "Issue 4", "SEVERITY": "HIGH"},
		{"FILE_LINE": "lib/helper.ts:50", "PROBLEM": "Issue 5", "SEVERITY": "LOW"},
		{"FILE_LINE": "lib/utils.ts:60", "PROBLEM": "Issue 6", "SEVERITY": "MEDIUM"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3", "--assign-numbers", "--critical-override=false"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// HIGH ungrouped item should go to Solo group
	soloGroup := findGroupByTheme(result.Groups, "solo")
	if soloGroup == nil {
		t.Fatal("expected solo group for HIGH ungrouped item")
	}
	if soloGroup.Count != 1 {
		t.Errorf("expected 1 solo item, got %d", soloGroup.Count)
	}

	// Solo group should have Number=0
	if num, ok := soloGroup.Number.(float64); !ok || int(num) != 0 {
		t.Errorf("solo group should have Number=0, got %v", soloGroup.Number)
	}

	// LOW/MEDIUM ungrouped should stay ungrouped (lib items)
	for _, item := range result.Ungrouped {
		sev, _ := item["SEVERITY"].(string)
		if sev == "HIGH" || sev == "CRITICAL" {
			t.Error("HIGH/CRITICAL items should not be in ungrouped when assign-numbers is true")
		}
	}
}

func TestGroupTDSoloAfterCriticalOverride(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Issue 1", "SEVERITY": "CRITICAL"},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "Issue 2", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Issue 3", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/auth/session.ts:35", "PROBLEM": "Issue 3b", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/api/users.ts:40", "PROBLEM": "Issue 4", "SEVERITY": "HIGH"},
		{"FILE_LINE": "lib/helper.ts:50", "PROBLEM": "Issue 5", "SEVERITY": "LOW"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3", "--assign-numbers", "--critical-override=true"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// CRITICAL item should be in critical group (NOT solo)
	criticalGroup := findGroupByTheme(result.Groups, "critical")
	if criticalGroup == nil {
		t.Fatal("expected critical group")
	}
	if criticalGroup.Count != 1 {
		t.Errorf("expected 1 critical item, got %d", criticalGroup.Count)
	}

	// HIGH ungrouped item should be in solo group
	soloGroup := findGroupByTheme(result.Groups, "solo")
	if soloGroup == nil {
		t.Fatal("expected solo group for HIGH ungrouped item")
	}
	if soloGroup.Count != 1 {
		t.Errorf("expected 1 solo item, got %d", soloGroup.Count)
	}

	// Verify no duplication: total output should equal input
	totalOutput := result.Summary.GroupedCount + result.Summary.UngroupedCount
	if totalOutput != 6 {
		t.Errorf("DATA LOSS: expected 6 items, got %d", totalOutput)
	}
}

func TestGroupTDOutputFile(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Missing auth", "FIX": "Add check", "SEVERITY": "HIGH", "CATEGORY": "security", "EST_MINUTES": 30},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "No cleanup", "FIX": "Clean session", "SEVERITY": "MEDIUM", "CATEGORY": "security", "EST_MINUTES": 45},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Weak check", "FIX": "Use zod", "SEVERITY": "HIGH", "CATEGORY": "security", "EST_MINUTES": 60}
	]`

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3", "--assign-numbers",
		"--output-file", outputFile, "--sprint-label", "1.0_feature", "--date-label", "2026-03-15"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	content := string(data)

	// Should have header
	if !bytes.Contains(data, []byte("# Technical Debt Backlog")) {
		t.Error("expected header in output file")
	}

	// Should have sprint label
	if !bytes.Contains(data, []byte("From Sprint: 1.0_feature")) {
		t.Error("expected sprint label in output")
	}

	// Should have date label
	if !bytes.Contains(data, []byte("[2026-03-15]")) {
		t.Error("expected date label in output")
	}

	// Should have 3 data rows (matching input items)
	tableRows := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		lineStr := string(line)
		if bytes.HasPrefix(line, []byte("|")) && !bytes.Contains(line, []byte("---")) &&
			!bytes.HasPrefix(line, []byte("| Group")) {
			tableRows++
		}
		_ = lineStr
	}
	if tableRows != 3 {
		t.Errorf("expected 3 table rows, got %d\nContent:\n%s", tableRows, content)
	}
}

func TestGroupTDOutputFileAppend(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	// Write initial content
	initialContent := "# Technical Debt Backlog\n\nExisting content.\n"
	os.WriteFile(outputFile, []byte(initialContent), 0644)

	input := `[
		{"FILE_LINE": "src/a.ts:1", "PROBLEM": "P1", "SEVERITY": "LOW", "CATEGORY": "test", "EST_MINUTES": 10}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--assign-numbers",
		"--output-file", outputFile, "--sprint-label", "2.0_bugfix"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Existing content should be preserved
	if !bytes.Contains(data, []byte("Existing content.")) {
		t.Error("existing content was not preserved")
	}

	// New section should be appended
	if !bytes.Contains(data, []byte("From Sprint: 2.0_bugfix")) {
		t.Error("new sprint section not appended")
	}
}

func TestGroupTDOutputFileCheckbox(t *testing.T) {
	input := `[
		{"FILE_LINE": "src/a.ts:1", "PROBLEM": "P1", "SEVERITY": "LOW", "CATEGORY": "test", "EST_MINUTES": 10}
	]`

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--assign-numbers",
		"--output-file", outputFile, "--checkbox"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Should have checkbox column
	if !bytes.Contains(data, []byte("[ ]")) {
		t.Error("expected checkbox column in output")
	}

	// Header should have empty checkbox column
	if !bytes.Contains(data, []byte("| Group | |")) {
		t.Error("expected checkbox column header")
	}
}

func TestGroupTDBackwardsCompat(t *testing.T) {
	// Call without new flags - should produce identical output to before
	input := `[
		{"FILE_LINE": "src/auth/login.ts:10", "PROBLEM": "Issue 1", "EST_MINUTES": 30},
		{"FILE_LINE": "src/auth/logout.ts:20", "PROBLEM": "Issue 2", "EST_MINUTES": 45},
		{"FILE_LINE": "src/auth/validate.ts:30", "PROBLEM": "Issue 3", "EST_MINUTES": 60},
		{"FILE_LINE": "src/api/users.ts:40", "PROBLEM": "Issue 4", "EST_MINUTES": 20, "SEVERITY": "HIGH"},
		{"FILE_LINE": "lib/helper.ts:50", "PROBLEM": "Issue 5", "EST_MINUTES": 10, "SEVERITY": "LOW"}
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

	// Without --assign-numbers: no Number field, no Solo group, no GROUP field
	for _, g := range result.Groups {
		if g.Number != nil {
			t.Errorf("without --assign-numbers, Number should be nil, got %v for group %s", g.Number, g.Theme)
		}
		for _, item := range g.Items {
			if _, ok := item["GROUP"]; ok {
				t.Error("without --assign-numbers, items should not have GROUP field")
			}
		}
	}

	// Should not have solo group without --assign-numbers
	soloGroup := findGroupByTheme(result.Groups, "solo")
	if soloGroup != nil {
		t.Error("should not have solo group without --assign-numbers")
	}

	// Data integrity maintained
	totalOutput := result.Summary.GroupedCount + result.Summary.UngroupedCount
	if totalOutput != 5 {
		t.Errorf("DATA LOSS: expected 5, got %d", totalOutput)
	}
}

func TestGroupTDGroupNumberOrdering(t *testing.T) {
	input := `[
		{"SEVERITY": "CRITICAL", "FILE_LINE": "src/x.ts:1", "PROBLEM": "Crit issue"},
		{"FILE_LINE": "src/auth/a.ts:10", "PROBLEM": "A1", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/auth/b.ts:20", "PROBLEM": "A2", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/auth/c.ts:30", "PROBLEM": "A3", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/api/d.ts:40", "PROBLEM": "B1", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/api/e.ts:50", "PROBLEM": "B2", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "src/api/f.ts:60", "PROBLEM": "B3", "SEVERITY": "MEDIUM"},
		{"FILE_LINE": "lib/x.ts:70", "PROBLEM": "Ungrouped", "SEVERITY": "HIGH"}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--content", input, "--json", "--min-group-size", "3", "--assign-numbers", "--critical-override=true"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	// Expected order: solo(0), critical(1), src-api(2), src-auth(3)
	if len(result.Groups) < 3 {
		t.Fatalf("expected at least 3 groups, got %d", len(result.Groups))
	}

	// Solo should be first with Number=0
	if result.Groups[0].Theme != "solo" {
		t.Errorf("first group should be solo, got %s", result.Groups[0].Theme)
	}
	if num, ok := result.Groups[0].Number.(float64); !ok || int(num) != 0 {
		t.Errorf("solo should have Number=0, got %v", result.Groups[0].Number)
	}

	// Critical should be second with Number=1
	if result.Groups[1].Theme != "critical" {
		t.Errorf("second group should be critical, got %s", result.Groups[1].Theme)
	}
	if num, ok := result.Groups[1].Number.(float64); !ok || int(num) != 1 {
		t.Errorf("critical should have Number=1, got %v", result.Groups[1].Number)
	}

	// Alphabetical groups follow
	for i := 2; i < len(result.Groups); i++ {
		if num, ok := result.Groups[i].Number.(float64); !ok || int(num) != i {
			t.Errorf("group %d (%s) should have Number=%d, got %v", i, result.Groups[i].Theme, i, result.Groups[i].Number)
		}
	}

	// Verify data integrity
	totalOutput := result.Summary.GroupedCount + result.Summary.UngroupedCount
	if totalOutput != 8 {
		t.Errorf("DATA LOSS: expected 8, got %d", totalOutput)
	}
}

func TestGroupTDPipeFormat(t *testing.T) {
	pipeInput := `# TD_STREAM - Technical Debt Items
# Format: SEVERITY|FILE_LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES
HIGH|src/auth/login.ts:10|Missing validation|Add zod schema|security|30
MEDIUM|src/auth/logout.ts:20|No error handling|Add try-catch|reliability|45
LOW|src/auth/validate.ts:30|Magic number|Extract constant|maintainability|15
MEDIUM|src/api/users.ts:40|N+1 query|Use eager loading|performance|60
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES",
		"--json",
		"--min-group-size", "2",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v\nOutput: %s", err, buf.String())
	}

	// Should have 4 items (comments and blank lines skipped)
	if result.Summary.TotalItems != 4 {
		t.Errorf("expected 4 items, got %d", result.Summary.TotalItems)
	}

	// src-auth has 3 items → qualifies as group
	authGroup := findGroupByTheme(result.Groups, "src-auth")
	if authGroup == nil {
		t.Fatal("expected src-auth group")
	}
	if authGroup.Count != 3 {
		t.Errorf("expected 3 items in src-auth, got %d", authGroup.Count)
	}

	// Verify field mapping
	item := authGroup.Items[0]
	if sev, ok := item["SEVERITY"].(string); !ok || sev != "HIGH" {
		t.Errorf("expected SEVERITY=HIGH, got %v", item["SEVERITY"])
	}
	if fl, ok := item["FILE_LINE"].(string); !ok || fl != "src/auth/login.ts:10" {
		t.Errorf("expected FILE_LINE=src/auth/login.ts:10, got %v", item["FILE_LINE"])
	}
}

func TestGroupTDPipeFormatFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	pipeFile := filepath.Join(tmpDir, "td-stream.txt")

	pipeContent := `# TD_STREAM
HIGH|src/auth/a.ts:1|Problem 1|Fix 1|security|30
HIGH|src/auth/b.ts:2|Problem 2|Fix 2|security|45
HIGH|src/auth/c.ts:3|Problem 3|Fix 3|security|60
`
	os.WriteFile(pipeFile, []byte(pipeContent), 0644)

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--file", pipeFile,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES",
		"--json",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	json.Unmarshal(buf.Bytes(), &result)

	if result.Summary.TotalItems != 3 {
		t.Errorf("expected 3 items, got %d", result.Summary.TotalItems)
	}
}

func TestGroupTDPipeFormatNoHeaders(t *testing.T) {
	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", "HIGH|src/a.ts:1|Problem|Fix|cat|30",
		"--format", "pipe",
		"--json",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --headers missing with --format=pipe")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--headers required")) {
		t.Errorf("expected headers required error, got: %v", err)
	}
}

func TestGroupTDPipeFormatOutputFile(t *testing.T) {
	// End-to-end: pipe input → group → write README.md
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `# TD_STREAM
HIGH|src/auth/a.ts:1|Auth issue|Fix auth|security|30
MEDIUM|src/auth/b.ts:2|Auth issue 2|Fix auth 2|security|45
LOW|src/auth/c.ts:3|Auth issue 3|Fix auth 3|security|15
MEDIUM|src/api/x.ts:4|API issue|Fix API|performance|60
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
		"--checkbox",
		"--sprint-label", "1.0_test",
		"--date-label", "2026-03-16",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	content := string(data)

	// Should have header
	if !bytes.Contains(data, []byte("# Technical Debt Backlog")) {
		t.Error("missing file header")
	}

	// Should have sprint section
	if !bytes.Contains(data, []byte("[2026-03-16] From Sprint: 1.0_test")) {
		t.Error("missing sprint section header")
	}

	// Should have checkbox
	if !bytes.Contains(data, []byte("[ ]")) {
		t.Error("missing checkbox column")
	}

	// Count data rows (not headers, not separators)
	lines := strings.Split(content, "\n")
	dataRows := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") &&
			!strings.HasPrefix(line, "| Group") {
			dataRows++
		}
	}
	if dataRows != 4 {
		t.Errorf("expected 4 data rows, got %d", dataRows)
	}
}

func TestGroupTDOutputFileAppendRowCountBug(t *testing.T) {
	// Regression test: row count verification should only count rows
	// from the current write, not rows from previous sprint sections
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	// Write initial content with an existing sprint section (3 data rows)
	initialContent := `# Technical Debt Backlog

Items from code review.

### [2026-03-01] From Sprint: previous_sprint

| Group | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|----------|------|---------|-----|----------|-------------|
| 1 | HIGH | src/old/a.ts:1 | Old issue 1 | Fix 1 | security | 30 |
| 1 | MEDIUM | src/old/b.ts:2 | Old issue 2 | Fix 2 | security | 45 |
| U | LOW | lib/old.ts:3 | Old issue 3 | Fix 3 | maint | 15 |
`
	os.WriteFile(outputFile, []byte(initialContent), 0644)

	// Now append a new sprint section with 2 items
	input := `[
		{"FILE_LINE": "src/new/x.ts:1", "PROBLEM": "New issue 1", "SEVERITY": "HIGH", "FIX": "Fix new 1", "CATEGORY": "perf", "EST_MINUTES": 20},
		{"FILE_LINE": "src/new/y.ts:2", "PROBLEM": "New issue 2", "SEVERITY": "LOW", "FIX": "Fix new 2", "CATEGORY": "perf", "EST_MINUTES": 10}
	]`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", input, "--json", "--assign-numbers",
		"--output-file", outputFile,
		"--sprint-label", "new_sprint",
		"--date-label", "2026-03-16",
		"--min-group-size", "1",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("REGRESSION: row count mismatch when appending to existing README.md: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Both sections should be present
	if !bytes.Contains(data, []byte("previous_sprint")) {
		t.Error("old sprint section missing")
	}
	if !bytes.Contains(data, []byte("new_sprint")) {
		t.Error("new sprint section missing")
	}

	// New section should appear BEFORE old section (newest first)
	newIdx := bytes.Index(data, []byte("new_sprint"))
	oldIdx := bytes.Index(data, []byte("previous_sprint"))
	if newIdx > oldIdx {
		t.Error("new sprint section should appear before old sprint section (newest first)")
	}
}

func TestGroupTDPipeFormatWithSource(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `# TD_STREAM
HIGH|src/auth/a.ts:1|Auth issue|Fix auth|security|30|execute-sprint
MEDIUM|src/auth/b.ts:2|Auth issue 2|Fix auth 2|security|45|code-review
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !bytes.Contains(data, []byte("| Est Minutes | Source |")) {
		t.Errorf("expected 8-column header ending '| Est Minutes | Source |', got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| execute-sprint |")) {
		t.Errorf("expected execute-sprint source cell, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| code-review |")) {
		t.Errorf("expected code-review source cell, got:\n%s", data)
	}
}

func TestGroupTDPipeFormatWithoutSource(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `HIGH|src/a.ts:1|Problem A|Fix A|security|15
MEDIUM|src/b.ts:2|Problem B|Fix B|style|10
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if bytes.Contains(data, []byte("Source")) {
		t.Errorf("did not expect Source column in output without SOURCE header, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| Est Minutes |\n")) {
		t.Errorf("expected 7-column header ending '| Est Minutes |', got:\n%s", data)
	}
}

func TestGroupTDPipeFormatMixedSource(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	// Second row has empty SOURCE field (trailing | before newline)
	pipeInput := `HIGH|src/a.ts:1|Problem A|Fix A|security|15|execute-sprint
MEDIUM|src/b.ts:2|Problem B|Fix B|style|10|
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if !bytes.Contains(data, []byte("| execute-sprint |")) {
		t.Errorf("expected non-empty source cell for first row, got:\n%s", data)
	}
	// Empty SOURCE should render as "|  |" (space + empty + space)
	if !bytes.Contains(data, []byte("| 10 |  |")) {
		t.Errorf("expected empty Source cell preserving column alignment, got:\n%s", data)
	}
}

func TestGroupTDPipeFormatSourceWithCheckbox(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `HIGH|src/a.ts:1|Problem A|Fix A|security|15|execute-sprint
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE",
		"--json",
		"--assign-numbers",
		"--checkbox",
		"--output-file", outputFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	expectedHeader := []byte("| Group | | Severity | File | Problem | Fix | Category | Est Minutes | Source |")
	if !bytes.Contains(data, expectedHeader) {
		t.Errorf("expected checkbox header with Source last, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| [ ] |")) {
		t.Errorf("expected checkbox cell in data row, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| execute-sprint |")) {
		t.Errorf("expected execute-sprint source cell in data row, got:\n%s", data)
	}
}

func TestGroupTDPipeFormatAppendWithSource(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	// Seed file with legacy 7-column table (no Source column)
	existing := `# Technical Debt Backlog

Items from code review.

### [2026-01-01] From Sprint: old_sprint

| Group | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|----------|------|---------|-----|----------|-------------|
| 1 | LOW | old.go:1 | old prob | old fix | misc | 5 |
`
	if err := os.WriteFile(outputFile, []byte(existing), 0644); err != nil {
		t.Fatalf("failed to seed file: %v", err)
	}

	pipeInput := `HIGH|src/a.ts:1|New problem|New fix|security|15|execute-sprint
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
		"--sprint-label", "new_sprint",
		"--date-label", "2026-04-22",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Legacy 7-column row must be preserved byte-for-byte
	if !bytes.Contains(data, []byte("| 1 | LOW | old.go:1 | old prob | old fix | misc | 5 |")) {
		t.Errorf("existing 7-column table not preserved, got:\n%s", data)
	}
	// New section must have 8-column header with Source last
	if !bytes.Contains(data, []byte("| Est Minutes | Source |")) {
		t.Errorf("new section missing Source column, got:\n%s", data)
	}
	// New row must have source value
	if !bytes.Contains(data, []byte("| execute-sprint |")) {
		t.Errorf("new row missing source value, got:\n%s", data)
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

// ---- REVIEWERS + CONFIDENCE column tests ----
//
// These columns are emitted only when at least one input row carries a
// non-empty value, mirroring the SOURCE column feature-flag pattern.

func TestGroupTDPipeFormatWithReviewers(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `HIGH|src/a.ts:1|Auth issue|Fix auth|security|30|code-review|bruce,greta|HIGH
MEDIUM|src/b.ts:2|Auth issue 2|Fix auth 2|security|45|code-review|kai|MEDIUM
`

	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE,REVIEWERS,CONFIDENCE",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("| Reviewers |")) {
		t.Errorf("expected Reviewers column header, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| Confidence |")) {
		t.Errorf("expected Confidence column header, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| bruce, greta |")) {
		t.Errorf("expected reviewer attribution cell with space-after-comma, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| HIGH |")) {
		t.Errorf("expected confidence cell, got:\n%s", data)
	}
}

func TestGroupTDPipeFormatWithoutReviewers(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `HIGH|src/a.ts:1|Problem A|Fix A|security|15|code-review
`
	cmd := newGroupTDCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("Reviewers")) {
		t.Errorf("did not expect Reviewers column without REVIEWERS header, got:\n%s", data)
	}
	if bytes.Contains(data, []byte("Confidence")) {
		t.Errorf("did not expect Confidence column without CONFIDENCE header, got:\n%s", data)
	}
}

func TestGroupTDPipeFormatReviewersWithCheckbox(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `HIGH|src/a.ts:1|P|F|security|15|code-review|bruce,greta,kai|HIGH
`
	cmd := newGroupTDCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE,REVIEWERS,CONFIDENCE",
		"--json",
		"--assign-numbers",
		"--checkbox",
		"--output-file", outputFile,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	// Header should include checkbox column + reviewers + confidence
	if !bytes.Contains(data, []byte("| Reviewers | Confidence |")) {
		t.Errorf("expected Reviewers + Confidence trailing columns, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte("| bruce, greta, kai |")) {
		t.Errorf("expected attribution cell, got:\n%s", data)
	}
}

func TestGroupTDPipeFormatPartialReviewers(t *testing.T) {
	// Some rows have REVIEWERS, others don't. The column should appear and
	// rows without attribution should show empty cells.
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "README.md")

	pipeInput := `HIGH|src/a.ts:1|P1|F1|security|15|code-review|bruce|HIGH
MEDIUM|src/b.ts:2|P2|F2|style|10|code-review||
`
	cmd := newGroupTDCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"--content", pipeInput,
		"--format", "pipe",
		"--headers", "SEVERITY,FILE_LINE,PROBLEM,FIX,CATEGORY,EST_MINUTES,SOURCE,REVIEWERS,CONFIDENCE",
		"--json",
		"--assign-numbers",
		"--output-file", outputFile,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("| bruce |")) {
		t.Errorf("expected bruce cell for first row, got:\n%s", data)
	}
	// Row 2 should have empty Reviewers + Confidence cells
	if !bytes.Contains(data, []byte("|  |  |\n")) {
		t.Errorf("expected empty reviewers + confidence cells, got:\n%s", data)
	}
}
