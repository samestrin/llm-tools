package gitrange

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// writeNested writes name (which may contain slashes) under dir, creating
// parent directories as needed, then stages it.
func writeNested(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", name)
}

// fixtureExcludeRepo builds a repo whose base..head range changes a code file,
// a nested .planning artifact, and CHANGELOG.md. Returns (dir, base, head).
func fixtureExcludeRepo(t *testing.T) (string, string, string) {
	t.Helper()
	dir := initRepo(t, "main")
	writeNested(t, dir, "code.go", "package p\n\nfunc A() int { return 1 }\n")
	writeNested(t, dir, ".planning/technical-debt/README.md", "# TD\n\nold\n")
	writeNested(t, dir, "CHANGELOG.md", "# Changelog\n\nold\n")
	mustGit(t, dir, "commit", "-q", "-m", "base")
	base := mustGit(t, dir, "rev-parse", "HEAD")

	writeNested(t, dir, "code.go", "package p\n\nfunc A() int { return 2 }\n")
	writeNested(t, dir, ".planning/technical-debt/README.md", "# TD\n\nnew\n")
	writeNested(t, dir, "CHANGELOG.md", "# Changelog\n\nnew\n")
	mustGit(t, dir, "commit", "-q", "-m", "head")
	head := mustGit(t, dir, "rev-parse", "HEAD")
	return dir, base, head
}

func TestExcludePathspec(t *testing.T) {
	if got := excludePathspec(nil); got != nil {
		t.Errorf("excludePathspec(nil) = %v, want nil", got)
	}
	if got := excludePathspec([]string{}); got != nil {
		t.Errorf("excludePathspec([]) = %v, want nil", got)
	}
	got := excludePathspec([]string{"a/**", "b.md"})
	want := []string{"--", ".", ":(exclude)a/**", ":(exclude)b.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("excludePathspec = %v, want %v", got, want)
	}
}

func TestDiffExcluding_DropsMatchedKeepsCode(t *testing.T) {
	dir, base, head := fixtureExcludeRepo(t)

	full, err := Diff(dir, base, head)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, want := range []string{"code.go", ".planning/technical-debt/README.md", "CHANGELOG.md"} {
		if !strings.Contains(full, want) {
			t.Fatalf("plain Diff missing %q; fixture wrong", want)
		}
	}

	got, err := DiffExcluding(dir, base, head, DefaultExcludeGlobs)
	if err != nil {
		t.Fatalf("DiffExcluding: %v", err)
	}
	if !strings.Contains(got, "code.go") {
		t.Errorf("excluded diff dropped code.go: must keep code")
	}
	if strings.Contains(got, ".planning/") {
		t.Errorf("excluded diff still contains .planning/ path:\n%s", got)
	}
	if strings.Contains(got, "CHANGELOG.md") {
		t.Errorf("excluded diff still contains CHANGELOG.md:\n%s", got)
	}
}

func TestDiffExcluding_NilEqualsPlainDiff(t *testing.T) {
	dir, base, head := fixtureExcludeRepo(t)
	plain, err := Diff(dir, base, head)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	got, err := DiffExcluding(dir, base, head, nil)
	if err != nil {
		t.Fatalf("DiffExcluding nil: %v", err)
	}
	if got != plain {
		t.Errorf("DiffExcluding(nil) must be byte-identical to Diff()")
	}
	// Empty slice behaves like nil.
	got2, err := DiffExcluding(dir, base, head, []string{})
	if err != nil {
		t.Fatalf("DiffExcluding []: %v", err)
	}
	if got2 != plain {
		t.Errorf("DiffExcluding([]) must be byte-identical to Diff()")
	}
}

func TestExcludedFileNames(t *testing.T) {
	dir, base, head := fixtureExcludeRepo(t)
	got, err := ExcludedFileNames(dir, base, head, DefaultExcludeGlobs)
	if err != nil {
		t.Fatalf("ExcludedFileNames: %v", err)
	}
	want := []string{".planning/technical-debt/README.md", "CHANGELOG.md"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExcludedFileNames = %v, want %v", got, want)
	}

	// No excludes → nothing excluded.
	none, err := ExcludedFileNames(dir, base, head, nil)
	if err != nil {
		t.Fatalf("ExcludedFileNames nil: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("ExcludedFileNames(nil) = %v, want empty", none)
	}
}

func TestExcludedFileNames_NoMatch(t *testing.T) {
	dir, base, head := fixtureExcludeRepo(t)
	got, err := ExcludedFileNames(dir, base, head, []string{"does/not/exist/**"})
	if err != nil {
		t.Fatalf("ExcludedFileNames: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ExcludedFileNames(no match) = %v, want empty", got)
	}
}
