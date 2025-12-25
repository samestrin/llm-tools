package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportBasic(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--title", "Test Report", "--status", "success"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# Test Report") {
		t.Error("expected title in output")
	}
	if !strings.Contains(output, "✅ SUCCESS") {
		t.Error("expected success status with emoji")
	}
	if !strings.Contains(output, "Generated:") {
		t.Error("expected timestamp in output")
	}
}

func TestReportWithStats(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--title", "Build Report",
		"--status", "partial",
		"--stat", "tests=50",
		"--stat", "passed=48",
		"--stat", "failed=2",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "⚠️ PARTIAL") {
		t.Error("expected partial status with emoji")
	}
	if !strings.Contains(output, "## Statistics") {
		t.Error("expected Statistics section")
	}
	if !strings.Contains(output, "| Metric | Value |") {
		t.Error("expected table headers")
	}
	if !strings.Contains(output, "tests") && !strings.Contains(output, "50") {
		t.Error("expected tests stat in output")
	}
}

func TestReportFailed(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--title", "Failed Build", "--status", "failed"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "❌ FAILED") {
		t.Error("expected failed status with emoji")
	}
}

func TestReportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "report.md")

	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--title", "File Report",
		"--status", "success",
		"-o", outputFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check confirmation message
	if !strings.Contains(buf.String(), "Report written to:") {
		t.Error("expected confirmation message")
	}

	// Check file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "# File Report") {
		t.Error("expected title in file")
	}
}

func TestReportMissingTitle(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--status", "success"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing title")
	}
	// Cobra reports required flags, but our validation also catches it
}

func TestReportMissingStatus(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--title", "Test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing status")
	}
}

func TestReportInvalidStatus(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--title", "Test", "--status", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
	if !strings.Contains(err.Error(), "status must be: success, partial, or failed") {
		t.Errorf("expected status validation error, got: %v", err)
	}
}

func TestReportInvalidStatFormat(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--title", "Test", "--status", "success", "--stat", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid stat format")
	}
	if !strings.Contains(err.Error(), "stat must be in KEY=VALUE format") {
		t.Errorf("expected stat format error, got: %v", err)
	}
}

func TestReportEmptyStatKey(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--title", "Test", "--status", "success", "--stat", "=value"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty stat key")
	}
}

func TestReportEmptyStatValue(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--title", "Test", "--status", "success", "--stat", "key="})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty stat value")
	}
}

func TestReportMarkdownEscaping(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{
		"--title", "Test *bold* and _italic_",
		"--status", "success",
		"--stat", "value|with|pipes=100",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Check that special characters are escaped
	if strings.Contains(output, "# Test *bold*") {
		t.Error("expected asterisks to be escaped")
	}
	if strings.Contains(output, "| value|with|pipes |") {
		t.Error("expected pipes in stat key to be escaped")
	}
}

func TestReportNoStats(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--title", "Minimal Report", "--status", "success"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should not contain statistics section
	if strings.Contains(output, "## Statistics") {
		t.Error("expected no Statistics section for report without stats")
	}
}

func TestGetStatusEmoji(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"success", "✅"},
		{"partial", "⚠️"},
		{"failed", "❌"},
		{"unknown", "❓"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := getStatusEmoji(tt.status)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"*bold*", "\\*bold\\*"},
		{"_italic_", "\\_italic\\_"},
		{"|pipe|", "\\|pipe\\|"},
		{"`code`", "\\`code\\`"},
		{"[link]", "\\[link\\]"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestReportCannotWriteFile(t *testing.T) {
	cmd := newReportCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--title", "Test",
		"--status", "success",
		"-o", "/nonexistent/directory/report.md",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-writable path")
	}
	if !strings.Contains(err.Error(), "failed to write report") {
		t.Errorf("expected write error, got: %v", err)
	}
}
