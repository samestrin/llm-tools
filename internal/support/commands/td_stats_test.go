package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTDStatsBasic(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# Tech Debt

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [x] | HIGH | foo.py:1 | Fixed issue | Was fixed | error-handling | 30 |
| 1 | [ ] | HIGH | bar.py:1 | Open issue | Needs fix | security | 120 |
| 1 | [/] | MEDIUM | baz.py:1 | Deferred | Later | performance | 60 |
| 1 | [ ] | MEDIUM | qux.py:1 | Open medium | Fix it | maintainability | 45 |
| 1 | [x] | CRITICAL | crit.py:1 | Fixed critical | Done | security | 90 |
| 1 | [ ] | LOW | low.py:1 | Low issue | Fix | maintainability | 15 |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Verify markdown table structure
	if !strings.Contains(output, "## Stats") {
		t.Error("output should contain ## Stats header")
	}
	if !strings.Contains(output, "| Severity | Open | Deferred | Resolved |") {
		t.Error("output should contain table header")
	}

	// CRITICAL: 0 open, 0 deferred, 1 resolved
	if !strings.Contains(output, "| CRITICAL | 0 | 0 | 1 |") {
		t.Errorf("CRITICAL row incorrect, got:\n%s", output)
	}
	// HIGH: 1 open, 0 deferred, 1 resolved
	if !strings.Contains(output, "| HIGH | 1 | 0 | 1 |") {
		t.Errorf("HIGH row incorrect, got:\n%s", output)
	}
	// MEDIUM: 1 open, 1 deferred, 0 resolved
	if !strings.Contains(output, "| MEDIUM | 1 | 1 | 0 |") {
		t.Errorf("MEDIUM row incorrect, got:\n%s", output)
	}
	// LOW: 1 open, 0 deferred, 0 resolved
	if !strings.Contains(output, "| LOW | 1 | 0 | 0 |") {
		t.Errorf("LOW row incorrect, got:\n%s", output)
	}
}

func TestTDStatsJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [x] | HIGH | foo.py |
| 1 | [ ] | MEDIUM | bar.py |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"severity"`) {
		t.Errorf("JSON output should contain severity key, got: %s", output)
	}
	if !strings.Contains(output, `"HIGH"`) {
		t.Errorf("JSON output should contain HIGH, got: %s", output)
	}
	if !strings.Contains(output, `"markdown"`) {
		t.Errorf("JSON output should contain markdown key, got: %s", output)
	}
	if !strings.Contains(output, "## Stats") {
		t.Errorf("JSON markdown field should contain rendered table, got: %s", output)
	}
}

func TestTDStatsEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte("# No tables here\n"), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should still produce a valid table with all zeros
	if !strings.Contains(output, "| CRITICAL | 0 | 0 | 0 |") {
		t.Errorf("empty file should produce zero counts, got:\n%s", output)
	}
}

func TestTDStatsMultipleTables(t *testing.T) {
	tmpDir := t.TempDir()
	content := `# Sprint 17.4

| Group | | Severity | File | Problem | Fix | Category | Est |
|-------|---|----------|------|---------|-----|----------|-----|
| 1 | [x] | HIGH | a.py | Done | Fixed | cat | 30 |
| 1 | [ ] | HIGH | b.py | Open | Fix | cat | 60 |

Some text between tables.

# Sprint 17.3

| Group | | Severity | File | Problem | Fix | Category | Est |
|-------|---|----------|------|---------|-----|----------|-----|
| 1 | [/] | MEDIUM | c.py | Deferred | Later | cat | 45 |
| 1 | [ ] | LOW | d.py | Open | Fix | cat | 15 |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should aggregate across both tables
	if !strings.Contains(output, "| HIGH | 1 | 0 | 1 |") {
		t.Errorf("HIGH row incorrect, got:\n%s", output)
	}
	if !strings.Contains(output, "| MEDIUM | 0 | 1 | 0 |") {
		t.Errorf("MEDIUM row incorrect, got:\n%s", output)
	}
	if !strings.Contains(output, "| LOW | 1 | 0 | 0 |") {
		t.Errorf("LOW row incorrect, got:\n%s", output)
	}
}

func TestTDStatsSeverityOrder(t *testing.T) {
	tmpDir := t.TempDir()
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [ ] | LOW | a.py |
| 1 | [ ] | CRITICAL | b.py |
| 1 | [ ] | MEDIUM | c.py |
| 1 | [ ] | HIGH | d.py |
`
	mdFile := filepath.Join(tmpDir, "README.md")
	os.WriteFile(mdFile, []byte(content), 0644)

	cmd := newTDStatsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--path", mdFile})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Verify order: CRITICAL before HIGH before MEDIUM before LOW
	critIdx := strings.Index(output, "CRITICAL")
	highIdx := strings.Index(output, "| HIGH")
	medIdx := strings.Index(output, "| MEDIUM")
	lowIdx := strings.Index(output, "| LOW")

	if critIdx > highIdx || highIdx > medIdx || medIdx > lowIdx {
		t.Errorf("severity order should be CRITICAL > HIGH > MEDIUM > LOW, got:\n%s", output)
	}
}

func TestParseTDStats(t *testing.T) {
	content := `| Group | | Severity | File |
|-------|---|----------|------|
| 1 | [x] | HIGH | foo.py |
| 1 | [ ] | HIGH | bar.py |
| 1 | [/] | HIGH | baz.py |
| 1 | [x] | MEDIUM | qux.py |
`
	result, err := parseTDStats(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	high := result.Severity["HIGH"]
	if high.Open != 1 || high.Deferred != 1 || high.Resolved != 1 {
		t.Errorf("HIGH: expected open=1 deferred=1 resolved=1, got open=%d deferred=%d resolved=%d",
			high.Open, high.Deferred, high.Resolved)
	}

	medium := result.Severity["MEDIUM"]
	if medium.Open != 0 || medium.Deferred != 0 || medium.Resolved != 1 {
		t.Errorf("MEDIUM: expected open=0 deferred=0 resolved=1, got open=%d deferred=%d resolved=%d",
			medium.Open, medium.Deferred, medium.Resolved)
	}
}

func TestSplitTableRow(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"| a | b | c |", 3},
		{"| a | b |", 2},
		{"|a|b|c|d|", 4},
	}

	for _, tt := range tests {
		cells := splitTableRow(tt.input)
		if len(cells) != tt.want {
			t.Errorf("splitTableRow(%q) = %d cells, want %d", tt.input, len(cells), tt.want)
		}
	}
}

func TestIsSeparatorRow(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"---", true},
		{":---:", true},
		{":---", true},
		{"---:", true},
		{"content", false},
		{"[ ]", false},
		{"", true}, // empty cells are ok
	}

	for _, tt := range tests {
		cells := []string{tt.input}
		got := isSeparatorRow(cells)
		if got != tt.want {
			t.Errorf("isSeparatorRow([%q]) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
