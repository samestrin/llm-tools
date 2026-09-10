package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// Epic plans are numbered N, N.M, or N.M.P and execute in ascending order, so
// the number is a POSITION, not just an identity. Two consequences drive these
// tests: a number already used by a COMPLETED epic must never be handed out
// again, and a child number must be derivable so urgent work can be slotted
// between existing plans (3.1 runs before a queued 4.0).

func writeEpics(t *testing.T, dir string, names ...string) string {
	t.Helper()
	full := filepath.Join(t.TempDir(), dir)
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(full, n), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	return full
}

// siblingDirs makes active/ and completed/ under one parent so both can be scanned.
func siblingDirs(t *testing.T, active, completed []string) (string, string) {
	t.Helper()
	root := t.TempDir()
	mk := func(sub string, names []string) string {
		full := filepath.Join(root, sub)
		if err := os.MkdirAll(full, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		for _, n := range names {
			if err := os.WriteFile(filepath.Join(full, n), []byte("x"), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
		}
		return full
	}
	return mk("active", active), mk("completed", completed)
}

func TestEpicNumber_NextTopLevel(t *testing.T) {
	dir := writeEpics(t, "active", "1.0_a.md", "2.0_b.md")
	got, err := nextEpicNumber([]string{dir}, "")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "3.0" {
		t.Errorf("next top-level = %q, want 3.0", got)
	}
}

// AC3: the collision case. A number in completed/ is SPENT — reusing it would
// give two epics the same execution position and overwrite history.
func TestEpicNumber_NeverReusesCompletedNumber(t *testing.T) {
	active, completed := siblingDirs(t, []string{"1.0_a.md", "2.0_b.md"}, []string{"3.0_shipped.md"})
	got, err := nextEpicNumber([]string{active, completed}, "")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got == "3.0" {
		t.Fatal("returned 3.0, which is already used by a COMPLETED epic")
	}
	if got != "4.0" {
		t.Errorf("next top-level = %q, want 4.0", got)
	}
}

func TestEpicNumber_NextChildOfParent(t *testing.T) {
	active, completed := siblingDirs(t, []string{"3.0_a.md", "3.1_b.md"}, []string{"3.2_done.md"})
	got, err := nextEpicNumber([]string{active, completed}, "3")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "3.3" {
		t.Errorf("next child of 3 = %q, want 3.3 (3.2 is completed, still spent)", got)
	}
}

// A parent that is itself a child yields the third level.
func TestEpicNumber_ThreeLevel(t *testing.T) {
	dir := writeEpics(t, "active", "3.1_a.md", "3.1.1_b.md", "3.1.2_c.md")
	got, err := nextEpicNumber([]string{dir}, "3.1")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "3.1.3" {
		t.Errorf("next child of 3.1 = %q, want 3.1.3", got)
	}
}

// First child of a parent that has none yet.
func TestEpicNumber_FirstChild(t *testing.T) {
	dir := writeEpics(t, "active", "3.0_a.md", "4.0_b.md")
	got, err := nextEpicNumber([]string{dir}, "3")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "3.1" {
		t.Errorf("first child of 3 = %q, want 3.1", got)
	}
}

// Non-conforming filenames must be ignored, not crash or shift the count.
func TestEpicNumber_IgnoresNonConforming(t *testing.T) {
	dir := writeEpics(t, "active", "1.0_a.md", "README.md", "notes.txt", "draft-ideas.md", ".gitkeep")
	got, err := nextEpicNumber([]string{dir}, "")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "2.0" {
		t.Errorf("next = %q, want 2.0 (non-conforming names ignored)", got)
	}
}

// An empty or missing directory is not an error — it is a fresh repo.
func TestEpicNumber_EmptyAndMissingDirs(t *testing.T) {
	dir := writeEpics(t, "active")
	got, err := nextEpicNumber([]string{dir, filepath.Join(dir, "does-not-exist")}, "")
	if err != nil {
		t.Fatalf("missing dir should be tolerated: %v", err)
	}
	if got != "1.0" {
		t.Errorf("next on empty = %q, want 1.0", got)
	}
}

// Gaps are not filled: 1.0 and 3.0 present means next is 4.0, because the
// numbers are execution positions and 2.0 may have been deliberately retired.
func TestEpicNumber_DoesNotFillGaps(t *testing.T) {
	dir := writeEpics(t, "active", "1.0_a.md", "3.0_c.md")
	got, err := nextEpicNumber([]string{dir}, "")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "4.0" {
		t.Errorf("next = %q, want 4.0 (gaps are not reused)", got)
	}
}

// Epic numbers are not capped at three levels in practice: atcr carries a whole
// 35.16.6.N family. A deeper name that the scan cannot PARSE is worse than a
// rejected --parent flag, because the number stops being visible at all and the
// tool will hand it out again to a second plan.
func TestEpicNumber_FourLevelNamesAreVisible(t *testing.T) {
	dir := writeEpics(t, "active", "35.16.6.4_a.md", "35.16.6.10_b.md")
	nums, err := collectEpicNums([]string{dir})
	if err != nil {
		t.Fatalf("collectEpicNums: %v", err)
	}
	if len(nums) != 2 {
		t.Fatalf("collected %d epic number(s) from two four-level names, want 2 — a name the scan cannot parse is a number it will re-issue", len(nums))
	}
}

// The reason this matters: a four-level child must be derivable, or urgent
// follow-on work lands at the BACK of its family's queue. Observed in atcr:
// residue from 35.16.6.4 was numbered 35.16.12 and scheduled seventh, behind
// 35.16.6.5, .7, .8, .9, .10 and .11.
func TestEpicNumber_NextChildOfFourLevelParent(t *testing.T) {
	active, completed := siblingDirs(t,
		[]string{"35.16.6.4_a.md", "35.16.6.5_b.md", "35.16.7_c.md"},
		[]string{"35.16.6.1_done.md", "35.16.6.2_done.md"})
	got, err := nextEpicNumber([]string{active, completed}, "35.16.6")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "35.16.6.6" {
		t.Errorf("next child of 35.16.6 = %q, want 35.16.6.6", got)
	}
}

// Depth is not capped anywhere, so a five-level parent works the same way.
func TestEpicNumber_ArbitraryDepth(t *testing.T) {
	dir := writeEpics(t, "active", "1.2.3.4.5_a.md", "1.2.3.4.9_b.md")
	got, err := nextEpicNumber([]string{dir}, "1.2.3.4")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "1.2.3.4.10" {
		t.Errorf("next child of 1.2.3.4 = %q, want 1.2.3.4.10", got)
	}
}

// Backward compatibility: a shallower parent must still count deeper
// descendants at ITS level. 35.16.6.4 contributes 6 to the level under 35.16,
// exactly as 35.16.6 would, so a queued 35.16.11 still yields 35.16.12.
func TestEpicNumber_DeepDescendantsCountAtTheParentLevel(t *testing.T) {
	dir := writeEpics(t, "active", "35.16.6.4_a.md", "35.16.11_b.md")
	got, err := nextEpicNumber([]string{dir}, "35.16")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "35.16.12" {
		t.Errorf("next child of 35.16 = %q, want 35.16.12", got)
	}
}

// A parent with no descendant deeper than itself still yields a first child.
func TestEpicNumber_FirstChildOfDeepParent(t *testing.T) {
	dir := writeEpics(t, "active", "35.16.6_a.md", "35.17_b.md")
	got, err := nextEpicNumber([]string{dir}, "35.16.6")
	if err != nil {
		t.Fatalf("nextEpicNumber: %v", err)
	}
	if got != "35.16.6.1" {
		t.Errorf("first child of 35.16.6 = %q, want 35.16.6.1", got)
	}
}

// A non-numeric parent is still an error; removing the depth cap must not
// remove the validation with it.
func TestEpicNumber_RejectsNonNumericParent(t *testing.T) {
	dir := writeEpics(t, "active", "1.0_a.md")
	if _, err := nextEpicNumber([]string{dir}, "3.x"); err == nil {
		t.Fatal("expected an error for a non-numeric parent component")
	}
	if _, err := nextEpicNumber([]string{dir}, ""); err != nil {
		t.Fatalf("empty parent is not an error: %v", err)
	}
}
