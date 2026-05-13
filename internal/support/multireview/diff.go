package multireview

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PreComputeDiffParams configures pre-computing a diff inside the remote workdir.
//
// We do this so reviewer agents only need to `cat` a known file, not navigate
// the cloned repo and run git themselves. In real use we observed weaker
// reviewers hallucinate "clone missing" / "ref not found" failures rather
// than persisting through the diff fetch. Pre-computing removes that surface.
type PreComputeDiffParams struct {
	// Host is the SSH target (e.g. "user@nucleus.lan").
	Host string
	// RemoteRepoPath is the cloned repo on the remote (the `git -C` target).
	RemoteRepoPath string
	// RemoteWorkdir is the parent directory; diff.txt lands here so it's
	// peers with the clone — keeps a single defer-clean dir.
	RemoteWorkdir string
	// BaseRef is required — no working-tree fallback (that mode produced
	// the very failures this fix exists to prevent).
	BaseRef string
	// HeadRef defaults to "HEAD" when empty.
	HeadRef string
	// Timeout for the whole SSH round-trip.
	Timeout time.Duration
}

// PreComputeDiffResult reports the diff that was written.
type PreComputeDiffResult struct {
	// DiffPath is the absolute path of the diff file inside the remote workdir.
	DiffPath string
	// SizeBytes is the byte count of the diff file (from wc -c).
	SizeBytes int64
	// LineCount is the line count of the diff file (from wc -l).
	LineCount int
	// Empty is true when the diff has zero bytes (BaseRef == HeadRef or no
	// changes between them). Callers can decide whether to proceed.
	Empty bool
}

// PreComputeDiff runs `git diff <base>..<head>` on the remote, redirects the
// output to <workdir>/diff.txt, and returns the path plus size/line counts.
//
// Failure semantics:
//   - Missing required inputs → validation error, no SSH call
//   - Non-zero git exit (bad ref, etc.) → wraps stderr in the error
//   - Context deadline → propagated
//
// Zero-byte diff is NOT an error; the result has Empty=true and the caller
// chooses whether to proceed.
func PreComputeDiff(parent context.Context, p PreComputeDiffParams) (PreComputeDiffResult, error) {
	if p.Host == "" {
		return PreComputeDiffResult{}, fmt.Errorf("diff: host required")
	}
	if p.RemoteRepoPath == "" {
		return PreComputeDiffResult{}, fmt.Errorf("diff: remote repo path required")
	}
	if p.RemoteWorkdir == "" {
		return PreComputeDiffResult{}, fmt.Errorf("diff: remote workdir required")
	}
	if p.BaseRef == "" {
		return PreComputeDiffResult{}, fmt.Errorf("diff: base ref required (working-tree mode is not supported — pass --base)")
	}
	if p.HeadRef == "" {
		p.HeadRef = "HEAD"
	}
	if p.Timeout <= 0 {
		p.Timeout = 5 * time.Minute
	}

	diffPath := filepath.Join(p.RemoteWorkdir, "diff.txt")

	// Single composite remote command: write the diff, then emit wc results.
	// `git diff` exits non-zero on bad refs which short-circuits the &&
	// chain and surfaces as a non-zero SSH exit + stderr we wrap in the
	// returned error.
	remoteCmd := fmt.Sprintf(
		"git -C %s diff %s..%s > %s && wc -c %s && wc -l %s",
		shellQuote(p.RemoteRepoPath),
		shellQuote(p.BaseRef),
		shellQuote(p.HeadRef),
		shellQuote(diffPath),
		shellQuote(diffPath),
		shellQuote(diffPath),
	)

	res, err := SSHRun(parent, SSHParams{
		Host:    p.Host,
		Command: remoteCmd,
		Timeout: p.Timeout,
	})
	if err != nil {
		return PreComputeDiffResult{}, fmt.Errorf("diff: ssh: %w", err)
	}
	if res.ExitCode != 0 {
		return PreComputeDiffResult{}, fmt.Errorf("diff: git exit %d, stderr: %s", res.ExitCode, strings.TrimSpace(res.Stderr))
	}

	size, lines, err := parseWcOutput(res.Stdout)
	if err != nil {
		return PreComputeDiffResult{}, fmt.Errorf("diff: parse wc output: %w (stdout: %q)", err, res.Stdout)
	}

	return PreComputeDiffResult{
		DiffPath:  diffPath,
		SizeBytes: size,
		LineCount: lines,
		Empty:     size == 0,
	}, nil
}

// parseWcOutput extracts the byte and line counts from two consecutive `wc`
// outputs. Handles both BSD format ("  1234 path") and GNU format
// ("1234 path") by taking the first numeric token on each non-empty line.
func parseWcOutput(stdout string) (sizeBytes int64, lineCount int, err error) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("expected 2 lines (wc -c, wc -l), got %d", len(lines))
	}
	size, err := firstNumericToken(lines[0])
	if err != nil {
		return 0, 0, fmt.Errorf("wc -c line: %w", err)
	}
	count, err := firstNumericToken(lines[1])
	if err != nil {
		return 0, 0, fmt.Errorf("wc -l line: %w", err)
	}
	return size, int(count), nil
}

// firstNumericToken returns the first whitespace-delimited integer in line.
// Tolerates leading whitespace (BSD wc) and ignores anything after the number
// (path).
func firstNumericToken(line string) (int64, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty line")
	}
	n, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("not a number: %q", fields[0])
	}
	return n, nil
}
