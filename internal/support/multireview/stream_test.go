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

	// td-stream.txt should have 3 lines, REVIEWER annotation appended
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
		[]byte("HIGH|f:1|p|x|c|bruce\nMEDIUM|f:2|p|x|c|bruce\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gretaDir, "td-stream.txt"),
		[]byte("LOW|f:3|p|x|c|greta\n"), 0o644); err != nil {
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
	if !strings.Contains(content, "HIGH|f:1|") {
		t.Error("missing bruce line 1")
	}
	if !strings.Contains(content, "LOW|f:3|") {
		t.Error("missing greta line")
	}
	if !strings.Contains(content, "# TD_STREAM - merged") {
		t.Error("merged file missing header")
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
		[]byte("HIGH|f:1|p|x|c|bruce\n"), 0o644); err != nil {
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
