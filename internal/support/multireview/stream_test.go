package multireview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleReview = `# Review: example#1

**Author:** Sam
**Verdict:** ship with fixes

## Findings

### Significant

- Some prose finding about install.sh

## TD_STREAM

HIGH|install.sh:42|missing validation|add zod check|security
MEDIUM|src/foo.go:80|no nil guard|wrap in if|robustness
LOW|README.md:5|typo|fix it|docs
`

func TestExtractTDLines_HappyPath(t *testing.T) {
	lines := ExtractTDLines(sampleReview)
	if len(lines) != 3 {
		t.Fatalf("got %d lines want 3: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "HIGH|") {
		t.Errorf("line 0 should start HIGH: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "MEDIUM|") {
		t.Errorf("line 1 should start MEDIUM: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "LOW|") {
		t.Errorf("line 2 should start LOW: %q", lines[2])
	}
}

func TestExtractTDLines_IgnoresProse(t *testing.T) {
	// Lines that mention "HIGH" but don't match the strict severity-prefix
	// format (uppercase severity followed by pipe) should not be picked up.
	body := `Some prose mentions HIGH severity here.
And MEDIUM is referenced inline.
LOW|src/x.go:1|real one|fix|cat
But "HIGH " (with trailing space and no pipe) should be ignored.
`
	lines := ExtractTDLines(body)
	if len(lines) != 1 {
		t.Errorf("got %d lines want 1: %v", len(lines), lines)
	}
}

func TestExtractTDLines_HandlesCriticalToo(t *testing.T) {
	body := `CRITICAL|src/auth.go:1|SQL injection|parameterize|security
HIGH|src/foo.go:1|something|fix|cat
`
	lines := ExtractTDLines(body)
	if len(lines) != 2 {
		t.Fatalf("got %d lines want 2", len(lines))
	}
}

func TestExtractTDLines_EmptyBody(t *testing.T) {
	if got := ExtractTDLines(""); len(got) != 0 {
		t.Errorf("empty input should give empty output, got %v", got)
	}
}

func TestWriteReviewerStream_WritesAndAnnotates(t *testing.T) {
	dir := t.TempDir()
	res := InvokeReviewerResult{
		AgentName:   "bruce",
		Model:       "qwen-3.6-plus",
		Status:      "ok",
		DurationMS:  239000,
		ReviewProse: sampleReview,
		RawJSON:     `{"runId":"x"}`,
	}

	paths, err := WriteReviewerOutput(dir, res)
	if err != nil {
		t.Fatalf("WriteReviewerOutput: %v", err)
	}

	// Expected files: review.md, td-stream.txt, status.json, response.json
	for _, p := range []string{paths.ReviewMD, paths.TDStream, paths.StatusJSON, paths.ResponseJSON} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}

	// td-stream.txt should have 3 lines in the unified 8-col format with
	// REVIEWER annotation appended. Inbound openclaw lines are 5-col
	// (SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY); we pad with empty EST_MINUTES
	// and EVIDENCE, then append the reviewer:
	//   SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY||<empty EVIDENCE>|<agent>
	// (Reviewers don't currently emit EST_MINUTES/EVIDENCE; leaving them
	// empty preserves a consistent column count.)
	tdData, err := os.ReadFile(paths.TDStream)
	if err != nil {
		t.Fatal(err)
	}
	tdLines := strings.Split(strings.TrimSpace(string(tdData)), "\n")
	if len(tdLines) != 3 {
		t.Fatalf("td-stream lines=%d want 3", len(tdLines))
	}
	for i, line := range tdLines {
		if !strings.HasSuffix(line, "|bruce") {
			t.Errorf("line %d should end with |bruce: %q", i, line)
		}
		// Verify 8 columns: count pipes (8 columns = 7 separators).
		fields := strings.Split(line, "|")
		if len(fields) != 8 {
			t.Errorf("line %d should have 8 columns (7 pipes), got %d fields: %q", i, len(fields), line)
			continue
		}
		// Fields 6 (EST_MINUTES) and 7 (EVIDENCE) should be empty for openclaw output.
		if fields[5] != "" {
			t.Errorf("line %d field 6 (EST_MINUTES) should be empty for openclaw reviewer, got %q", i, fields[5])
		}
		if fields[6] != "" {
			t.Errorf("line %d field 7 (EVIDENCE) should be empty for openclaw reviewer, got %q", i, fields[6])
		}
	}
}

func TestPadTo7Columns(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  string
		nCols int // expected pipe-separated field count after padding
	}{
		{
			name:  "5-col openclaw line padded to 7",
			in:    "HIGH|src/x.go:1|problem|fix|cat",
			want:  "HIGH|src/x.go:1|problem|fix|cat||",
			nCols: 7,
		},
		{
			name:  "already 7 cols pass-through",
			in:    "HIGH|src/x.go:1|p|f|c|10|src/x.go:1-5",
			want:  "HIGH|src/x.go:1|p|f|c|10|src/x.go:1-5",
			nCols: 7,
		},
		{
			name:  "more than 7 cols pass-through (no truncation)",
			in:    "HIGH|src/x.go:1|p|f|c|10|src/x.go:1-5|extra",
			want:  "HIGH|src/x.go:1|p|f|c|10|src/x.go:1-5|extra",
			nCols: 8,
		},
		{
			name:  "2-col pathological case padded to 7",
			in:    "HIGH|only",
			want:  "HIGH|only|||||",
			nCols: 7,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := padTo7Columns(c.in)
			if got != c.want {
				t.Errorf("padTo7Columns(%q) = %q, want %q", c.in, got, c.want)
			}
			cols := strings.Count(got, "|") + 1
			if cols != c.nCols {
				t.Errorf("padded result has %d cols, want %d", cols, c.nCols)
			}
		})
	}
}

func TestWriteReviewerOutput_HandlesEmptyTD(t *testing.T) {
	// A reviewer that produces a review but no TD lines (e.g. "no issues")
	// should still write all artifacts; td-stream.txt is just empty.
	dir := t.TempDir()
	res := InvokeReviewerResult{
		AgentName:   "otto",
		Model:       "gemma-4-31b",
		ReviewProse: "no issues found",
	}
	paths, err := WriteReviewerOutput(dir, res)
	if err != nil {
		t.Fatalf("WriteReviewerOutput: %v", err)
	}
	td, err := os.ReadFile(paths.TDStream)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(td)) != "" {
		t.Errorf("td-stream should be empty, got %q", td)
	}
}

func TestMergeStreams_ConcatenatesWithHeader(t *testing.T) {
	// Per-reviewer td-stream.txt files are now in the unified 8-col format:
	//   SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER
	// MergeStreams concatenates them verbatim (it doesn't re-append the
	// reviewer — WriteReviewerOutput did that already).
	dir := t.TempDir()
	bruceDir := filepath.Join(dir, "bruce")
	gretaDir := filepath.Join(dir, "greta")
	otto := filepath.Join(dir, "otto")
	for _, d := range []string{bruceDir, gretaDir, otto} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bruceDir, "td-stream.txt"),
		[]byte("HIGH|f:1|p|x|c|||bruce\nMEDIUM|f:2|p|x|c|||bruce\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gretaDir, "td-stream.txt"),
		[]byte("LOW|f:3|p|x|c|||greta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// otto produced empty
	if err := os.WriteFile(filepath.Join(otto, "td-stream.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	mergedPath, count, err := MergeStreams(dir, []string{"bruce", "greta", "otto"})
	if err != nil {
		t.Fatalf("MergeStreams: %v", err)
	}
	if count != 3 {
		t.Errorf("merged count=%d want 3", count)
	}
	data, err := os.ReadFile(mergedPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "HIGH|f:1|p|x|c|||bruce") {
		t.Error("missing bruce line 1 in 8-col form")
	}
	if !strings.Contains(content, "LOW|f:3|p|x|c|||greta") {
		t.Error("missing greta line in 8-col form")
	}
	if !strings.Contains(content, "# TD_STREAM - merged") {
		t.Error("merged file missing header")
	}
	// Header should document the new 8-col format.
	if !strings.Contains(content, "SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER") {
		t.Error("merged file header should document the 8-col format")
	}
}

func TestMergeStreams_ToleratesMissingFiles(t *testing.T) {
	// If a reviewer's dir doesn't exist (e.g. it failed to invoke), merge
	// should skip it without error.
	dir := t.TempDir()
	bruceDir := filepath.Join(dir, "bruce")
	if err := os.MkdirAll(bruceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bruceDir, "td-stream.txt"),
		[]byte("HIGH|f:1|p|x|c|||bruce\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, count, err := MergeStreams(dir, []string{"bruce", "ghost-reviewer-that-failed"})
	if err != nil {
		t.Fatalf("MergeStreams should tolerate missing: %v", err)
	}
	if count != 1 {
		t.Errorf("merged count=%d want 1", count)
	}
}
