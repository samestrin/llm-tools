package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeShortFile creates a file with n lines (no trailing newline on the last)
// and returns nothing — the path is caller-provided.
func writeShortFile(t *testing.T, path string, n int) {
	t.Helper()
	var b bytes.Buffer
	for i := 1; i <= n; i++ {
		if i > 1 {
			b.WriteByte('\n')
		}
		b.WriteString("line")
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestGroupTDValidateLinesQuarantinesPhantomRows(t *testing.T) {
	tmp := t.TempDir()
	writeShortFile(t, filepath.Join(tmp, "short.go"), 5) // 5-line file

	input := `[
		{"FILE_LINE": "short.go:3", "PROBLEM": "valid line", "EST_MINUTES": 5},
		{"FILE_LINE": "short.go:999", "PROBLEM": "phantom beyond EOF", "EST_MINUTES": 5},
		{"FILE_LINE": "missing.go:10", "PROBLEM": "missing file kept for relocation", "EST_MINUTES": 5},
		{"FILE_LINE": "short.go", "PROBLEM": "no line number", "EST_MINUTES": 5}
	]`

	quarantine := filepath.Join(tmp, "quarantine.json")

	cmd := newGroupTDCmd()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{
		"--content", input,
		"--json",
		"--repo-root", tmp,
		"--quarantine-file", quarantine,
		"--min-group-size", "1",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}

	// Only the phantom (short.go:999) is dropped; the other 3 survive.
	if result.Summary.TotalItems != 3 {
		t.Errorf("expected 3 surviving items, got %d", result.Summary.TotalItems)
	}

	// Data-loss invariant still holds over the filtered set.
	if got := result.Summary.GroupedCount + result.Summary.UngroupedCount; got != 3 {
		t.Errorf("grouped+ungrouped = %d, want 3", got)
	}

	// Phantom must not appear anywhere in the output.
	if bytes.Contains(out.Bytes(), []byte("phantom beyond EOF")) {
		t.Error("phantom row leaked into grouped output")
	}

	// stderr warns about the quarantine.
	if !bytes.Contains(errBuf.Bytes(), []byte("quarantined 1 TD row")) {
		t.Errorf("expected quarantine warning on stderr, got: %s", errBuf.String())
	}

	// Quarantine file holds exactly the phantom row, with a reason.
	data, err := os.ReadFile(quarantine)
	if err != nil {
		t.Fatalf("read quarantine file: %v", err)
	}
	var quarantined []map[string]interface{}
	if err := json.Unmarshal(data, &quarantined); err != nil {
		t.Fatalf("parse quarantine file: %v\n%s", err, string(data))
	}
	if len(quarantined) != 1 {
		t.Fatalf("expected 1 quarantined row, got %d", len(quarantined))
	}
	if reason, _ := quarantined[0]["QUARANTINE_REASON"].(string); reason == "" {
		t.Error("quarantined row missing QUARANTINE_REASON")
	}
	if fl, _ := quarantined[0]["FILE_LINE"].(string); fl != "short.go:999" {
		t.Errorf("quarantined wrong row: FILE_LINE=%q", fl)
	}
}

func TestGroupTDValidateLinesDisabledKeepsAll(t *testing.T) {
	tmp := t.TempDir()
	writeShortFile(t, filepath.Join(tmp, "short.go"), 5)

	input := `[
		{"FILE_LINE": "short.go:3", "PROBLEM": "valid", "EST_MINUTES": 5},
		{"FILE_LINE": "short.go:999", "PROBLEM": "phantom", "EST_MINUTES": 5}
	]`

	cmd := newGroupTDCmd()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"--content", input,
		"--json",
		"--repo-root", tmp,
		"--validate-lines=false",
		"--min-group-size", "1",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result GroupTDResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result.Summary.TotalItems != 2 {
		t.Errorf("with guard off expected 2 items, got %d", result.Summary.TotalItems)
	}
}

func TestPhantomLineItemConservative(t *testing.T) {
	tmp := t.TempDir()
	writeShortFile(t, filepath.Join(tmp, "f.go"), 10)

	cases := []struct {
		name     string
		fileLine string
		phantom  bool
	}{
		{"valid mid-file", "f.go:5", false},
		{"valid last line", "f.go:10", false},
		{"one past EOF", "f.go:11", true},
		{"far past EOF", "f.go:3759", true},
		{"missing file", "gone.go:5", false},
		{"no line ref", "f.go", false},
		{"non-numeric line", "f.go:abc", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := map[string]interface{}{"FILE_LINE": tc.fileLine}
			got, reason := phantomLineItem(item, tmp)
			if got != tc.phantom {
				t.Errorf("phantomLineItem(%q) = %v (reason %q), want %v", tc.fileLine, got, reason, tc.phantom)
			}
		})
	}
}
