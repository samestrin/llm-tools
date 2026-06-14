package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// gitAt runs a git command in dir with a fixed author/committer date.
func gitAt(t *testing.T, dir, date string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFileLines(t *testing.T, path string, n int) {
	t.Helper()
	var b []byte
	for i := 0; i < n; i++ {
		b = append(b, []byte("line\n")...)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupDriftRepo builds a repo where fileA.go is added on 2026-01-01 and
// modified on 2026-06-01, so an entry "created" 2026-03-01 sees exactly one
// post-capture change.
func setupDriftRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitAt(t, dir, "2026-01-01T00:00:00", "init", "-q")
	writeFileLines(t, filepath.Join(dir, "fileA.go"), 10)
	gitAt(t, dir, "2026-01-01T12:00:00", "add", "fileA.go")
	gitAt(t, dir, "2026-01-01T12:00:00", "commit", "-q", "-m", "add fileA")
	writeFileLines(t, filepath.Join(dir, "fileA.go"), 20)
	gitAt(t, dir, "2026-06-01T12:00:00", "add", "fileA.go")
	gitAt(t, dir, "2026-06-01T12:00:00", "commit", "-q", "-m", "modify fileA")
	return dir
}

func TestGitFirstAddDate(t *testing.T) {
	dir := setupDriftRepo(t)
	if got := gitFirstAddDate(dir, "fileA.go"); got != "2026-01-01" {
		t.Errorf("first-add date = %q, want 2026-01-01", got)
	}
	if got := gitFirstAddDate(dir, "nope.go"); got != "" {
		t.Errorf("untracked file should yield empty date, got %q", got)
	}
}

func TestGitCommitsSince(t *testing.T) {
	dir := setupDriftRepo(t)
	// Only the 2026-06-01 change is after a 2026-03-01 capture.
	if n := gitCommitsSince(dir, "fileA.go", "2026-03-01"); n != 1 {
		t.Errorf("commits since 2026-03-01 = %d, want 1", n)
	}
	// Everything is after 2025-01-01.
	if n := gitCommitsSince(dir, "fileA.go", "2025-01-01"); n != 2 {
		t.Errorf("commits since 2025-01-01 = %d, want 2", n)
	}
}

func TestAnalyzeDrift_FlagsCodeChangeAndDangling(t *testing.T) {
	dir := setupDriftRepo(t)
	src := `---
id: mem-2026-03-01-aa11bb
question: "Q?"
created: 2026-03-01
files:
- fileA.go
- deleted.go
status: active
type: knowledge
---

# Title

## Decision
Old decision text. Rationale: - [from context]
`
	e := parseEntry("entry.md", []byte(src))
	now := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	d := analyzeDrift(dir, "entry.md", e, now)

	if d.Created != "2026-03-01" {
		t.Errorf("created = %q, want frontmatter 2026-03-01", d.Created)
	}
	if d.AgeDays != 105 {
		t.Errorf("age_days = %d, want 105 (2026-03-01..2026-06-14)", d.AgeDays)
	}
	byPath := map[string]FileDrift{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	if !byPath["fileA.go"].Exists || byPath["fileA.go"].CommitsSince != 1 {
		t.Errorf("fileA.go drift wrong: %+v", byPath["fileA.go"])
	}
	if byPath["deleted.go"].Exists {
		t.Errorf("deleted.go should not exist")
	}
	if !hasFlag(d.Flags, "dangling_ref") {
		t.Errorf("expected dangling_ref flag, got %v", d.Flags)
	}
	if !hasFlag(d.Flags, "code_changed_after_capture") {
		t.Errorf("expected code_changed_after_capture flag, got %v", d.Flags)
	}
	if !d.Placeholders || !hasFlag(d.Flags, "incomplete") {
		t.Errorf("expected placeholders+incomplete, got placeholders=%v flags=%v", d.Placeholders, d.Flags)
	}
}

// When frontmatter lacks `created`, it is derived from the entry file's git
// first-add date.
func TestAnalyzeDrift_CreatedFromGitWhenAbsent(t *testing.T) {
	dir := setupDriftRepo(t)
	// Commit the entry file itself on 2026-02-02.
	entry := `---
id: mem-x
type: knowledge
---

# T
`
	if err := os.WriteFile(filepath.Join(dir, "entry.md"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "2026-02-02T12:00:00", "add", "entry.md")
	gitAt(t, dir, "2026-02-02T12:00:00", "commit", "-q", "-m", "add entry")

	e := parseEntry("entry.md", []byte(entry))
	now := time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)
	d := analyzeDrift(dir, "entry.md", e, now)
	if d.Created != "2026-02-02" {
		t.Errorf("created = %q, want git-derived 2026-02-02", d.Created)
	}
	if d.AgeDays != 10 {
		t.Errorf("age_days = %d, want 10", d.AgeDays)
	}
}

// --- Adversarial ---

// A non-git directory must not crash; git-derived fields are simply empty.
func TestAnalyzeDrift_NonGitDirSafe(t *testing.T) {
	dir := t.TempDir()
	src := "---\ncreated: 2026-01-01\nfiles:\n- a/b.go\nstatus: active\ntype: knowledge\n---\n\n# T\n"
	e := parseEntry("e.md", []byte(src))
	now := time.Date(2026, 1, 11, 0, 0, 0, 0, time.UTC)
	d := analyzeDrift(dir, "e.md", e, now) // must not panic
	if len(d.Files) != 1 || d.Files[0].Exists {
		t.Errorf("non-git missing file: %+v", d.Files)
	}
}

// An entry with no files list yields an empty file-drift slice, no crash.
func TestAnalyzeDrift_NoFiles(t *testing.T) {
	dir := setupDriftRepo(t)
	src := "---\ncreated: 2026-03-01\nstatus: active\ntype: knowledge\n---\n\n# T\n"
	e := parseEntry("e.md", []byte(src))
	now := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	d := analyzeDrift(dir, "e.md", e, now)
	if len(d.Files) != 0 {
		t.Errorf("expected no files, got %+v", d.Files)
	}
	if hasFlag(d.Flags, "dangling_ref") || hasFlag(d.Flags, "code_changed_after_capture") {
		t.Errorf("no-files entry should not flag code drift: %v", d.Flags)
	}
}

// A future created date yields a non-negative age (clamped), not a negative.
func TestAnalyzeDrift_FutureCreatedClampsAge(t *testing.T) {
	dir := t.TempDir()
	src := "---\ncreated: 2099-01-01\nstatus: active\ntype: knowledge\n---\n\n# T\n"
	e := parseEntry("e.md", []byte(src))
	now := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	d := analyzeDrift(dir, "e.md", e, now)
	if d.AgeDays < 0 {
		t.Errorf("age_days = %d, want clamped to >= 0", d.AgeDays)
	}
}

func hasFlag(flags []string, f string) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}
