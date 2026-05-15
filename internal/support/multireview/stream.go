package multireview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// tdLinePattern matches pipe-delimited TD lines of the form
// `SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY` where SEVERITY is one of the
// known severity tokens. The match anchors at line start and requires the
// pipe immediately after the severity, so prose mentions of "HIGH severity"
// or similar are not picked up.
var tdLinePattern = regexp.MustCompile(`(?m)^(CRITICAL|HIGH|MEDIUM|LOW)\|.+$`)

// padTo7Columns ensures the input line has exactly 7 pipe-separated fields
// by appending empty fields until the count reaches 7. The caller appends
// the REVIEWER as field 8.
//
// Inbound openclaw lines are typically 5 columns; this pads them to 7. If a
// line already has 7 or more columns, it is returned unchanged (the caller's
// reviewer-append still works correctly — the result will simply have more
// than 8 total columns, which reconcile's compat shim handles).
func padTo7Columns(line string) string {
	fields := strings.Split(line, "|")
	for len(fields) < 7 {
		fields = append(fields, "")
	}
	return strings.Join(fields, "|")
}

// ExtractTDLines returns all pipe-delimited TD findings from a review body.
// Order is preserved. Trailing whitespace is trimmed.
func ExtractTDLines(reviewProse string) []string {
	if reviewProse == "" {
		return nil
	}
	matches := tdLinePattern.FindAllString(reviewProse, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, strings.TrimRight(m, " \t\r"))
	}
	return out
}

// ReviewerOutputPaths records where one reviewer's artifacts were written.
type ReviewerOutputPaths struct {
	Dir          string
	ReviewMD     string
	TDStream     string
	StatusJSON   string
	ResponseJSON string
}

// WriteReviewerOutput writes one reviewer's full output to a per-agent dir:
//
//	<root>/<agent>/review.md       — extracted prose, human-readable
//	<root>/<agent>/td-stream.txt   — pipe-delimited TD lines in unified 8-col format
//	<root>/<agent>/status.json     — small metadata blob (model, duration, status)
//	<root>/<agent>/response.json   — raw openclaw envelope, untouched
//
// td-stream.txt rows use the unified 8-column format:
//
//	SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER
//
// Inbound openclaw reviewer prose typically contains 5-column lines
// (SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY). We pad with empty EST_MINUTES
// and EVIDENCE before appending the agent name as REVIEWER, so all writers
// in the system produce identically-shaped per-source streams. If a future
// reviewer emits more columns, they pass through unchanged up to and
// including position 7 (EVIDENCE), with REVIEWER always being the final
// column we append here.
func WriteReviewerOutput(rootDir string, res InvokeReviewerResult) (ReviewerOutputPaths, error) {
	if rootDir == "" {
		return ReviewerOutputPaths{}, fmt.Errorf("write: root dir required")
	}
	if res.AgentName == "" {
		return ReviewerOutputPaths{}, fmt.Errorf("write: agent name required")
	}

	dir := filepath.Join(rootDir, res.AgentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ReviewerOutputPaths{}, fmt.Errorf("write: mkdir %s: %w", dir, err)
	}

	paths := ReviewerOutputPaths{
		Dir:          dir,
		ReviewMD:     filepath.Join(dir, "review.md"),
		TDStream:     filepath.Join(dir, "td-stream.txt"),
		StatusJSON:   filepath.Join(dir, "status.json"),
		ResponseJSON: filepath.Join(dir, "response.json"),
	}

	if err := os.WriteFile(paths.ReviewMD, []byte(res.ReviewProse), 0o644); err != nil {
		return paths, fmt.Errorf("write review.md: %w", err)
	}

	// Build per-reviewer td-stream in the unified 8-col format. Inbound
	// lines from reviewer prose are typically 5-col; pad with empty fields
	// up to position 7 (EVIDENCE), then append the agent name as REVIEWER.
	var tdBuf strings.Builder
	for _, line := range ExtractTDLines(res.ReviewProse) {
		tdBuf.WriteString(padTo7Columns(line))
		tdBuf.WriteString("|")
		tdBuf.WriteString(res.AgentName)
		tdBuf.WriteString("\n")
	}
	if err := os.WriteFile(paths.TDStream, []byte(tdBuf.String()), 0o644); err != nil {
		return paths, fmt.Errorf("write td-stream.txt: %w", err)
	}

	status := map[string]interface{}{
		"agent":       res.AgentName,
		"model":       res.Model,
		"status":      res.Status,
		"durationMs":  res.DurationMS,
		"aborted":     res.Aborted,
		"tdLineCount": len(ExtractTDLines(res.ReviewProse)),
	}
	statusJSON, _ := json.MarshalIndent(status, "", "  ")
	if err := os.WriteFile(paths.StatusJSON, statusJSON, 0o644); err != nil {
		return paths, fmt.Errorf("write status.json: %w", err)
	}

	if res.RawJSON != "" {
		if err := os.WriteFile(paths.ResponseJSON, []byte(res.RawJSON), 0o644); err != nil {
			return paths, fmt.Errorf("write response.json: %w", err)
		}
	} else {
		// Always create the file so callers can rely on it existing.
		if err := os.WriteFile(paths.ResponseJSON, []byte("{}"), 0o644); err != nil {
			return paths, fmt.Errorf("write response.json: %w", err)
		}
	}

	return paths, nil
}

// MergeStreams concatenates per-reviewer td-stream.txt files into a single
// td-stream-all.txt at rootDir. Missing reviewer directories are silently
// skipped (a reviewer might have failed to invoke; we proceed with what we
// have). Returns the merged file path and the total line count.
func MergeStreams(rootDir string, reviewers []string) (string, int, error) {
	if rootDir == "" {
		return "", 0, fmt.Errorf("merge: root dir required")
	}

	mergedPath := filepath.Join(rootDir, "td-stream-all.txt")
	var out strings.Builder
	out.WriteString("# TD_STREAM - merged from " + fmt.Sprintf("%d", len(reviewers)) + " reviewer(s)\n")
	out.WriteString("# Format: SEVERITY|FILE:LINE|PROBLEM|FIX|CATEGORY|EST_MINUTES|EVIDENCE|REVIEWER\n")

	count := 0
	for _, agent := range reviewers {
		streamPath := filepath.Join(rootDir, agent, "td-stream.txt")
		data, err := os.ReadFile(streamPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", 0, fmt.Errorf("merge: read %s: %w", streamPath, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			out.WriteString(line)
			out.WriteString("\n")
			count++
		}
	}

	if err := os.WriteFile(mergedPath, []byte(out.String()), 0o644); err != nil {
		return "", 0, fmt.Errorf("merge: write %s: %w", mergedPath, err)
	}
	return mergedPath, count, nil
}
