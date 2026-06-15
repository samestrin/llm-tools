package commands

import (
	"testing"
)

func dsSmell(res *SmellResult, typ string) *Smell {
	for i := range res.Smells {
		if res.Smells[i].Type == typ {
			return &res.Smells[i]
		}
	}
	return nil
}

// A "fix" that touches only a test file is a reward-hack — HARD.
func TestAnalyzeDiff_TestOnly(t *testing.T) {
	diff := `diff --git a/foo_test.go b/foo_test.go
--- a/foo_test.go
+++ b/foo_test.go
@@ -1,2 +1,3 @@
 package main
+// loosen expectations
`
	res := analyzeDiff(diff)
	if res.Summary.Verdict != "hard" {
		t.Fatalf("verdict = %q, want hard", res.Summary.Verdict)
	}
	if s := dsSmell(res, "test_only"); s == nil || s.Severity != "hard" {
		t.Errorf("expected hard test_only smell, got %+v", res.Smells)
	}
	if res.Summary.TestFiles != 1 || res.Summary.ImplFiles != 0 {
		t.Errorf("file classification: test=%d impl=%d, want 1/0", res.Summary.TestFiles, res.Summary.ImplFiles)
	}
}

// Removing an assertion from a test (while touching impl, so test_only does NOT
// fire) is a weakened assertion — HARD.
func TestAnalyzeDiff_WeakenedAssertion(t *testing.T) {
	diff := `diff --git a/auth.ts b/auth.ts
--- a/auth.ts
+++ b/auth.ts
@@ -1 +1 @@
-export const x = 1
+export const x = 2
diff --git a/auth.test.ts b/auth.test.ts
--- a/auth.test.ts
+++ b/auth.test.ts
@@ -5,4 +5,3 @@
-  expect(result).toBe(42)
   doStuff()
`
	res := analyzeDiff(diff)
	if res.Summary.Verdict != "hard" {
		t.Fatalf("verdict = %q, want hard", res.Summary.Verdict)
	}
	if s := dsSmell(res, "weakened_assertion"); s == nil || s.Severity != "hard" {
		t.Errorf("expected hard weakened_assertion, got %+v", res.Smells)
	}
	// Impl was touched, so test_only must NOT fire.
	if dsSmell(res, "test_only") != nil {
		t.Errorf("test_only should not fire when impl is also changed")
	}
	if res.Summary.ImplFiles != 1 || res.Summary.TestFiles != 1 {
		t.Errorf("file classification: test=%d impl=%d, want 1/1", res.Summary.TestFiles, res.Summary.ImplFiles)
	}
}

// A suppression directive in an impl file is SOFT (verdict soft_only).
func TestAnalyzeDiff_Suppression(t *testing.T) {
	diff := `diff --git a/svc.ts b/svc.ts
--- a/svc.ts
+++ b/svc.ts
@@ -1 +1,2 @@
 const a = compute()
+// @ts-ignore
`
	res := analyzeDiff(diff)
	if res.Summary.Verdict != "soft_only" {
		t.Fatalf("verdict = %q, want soft_only", res.Summary.Verdict)
	}
	if s := dsSmell(res, "suppression"); s == nil || s.Severity != "soft" {
		t.Errorf("expected soft suppression, got %+v", res.Smells)
	}
}

func TestAnalyzeDiff_SuppressionVariants(t *testing.T) {
	for _, line := range []string{
		"+# type: ignore",
		"+    # noqa: E501",
		"+/* eslint-disable no-unused-vars */",
		"+//nolint:errcheck",
		"+# nosec",
		"+# pylint: disable=broad-except",
	} {
		diff := "diff --git a/m.x b/m.x\n--- a/m.x\n+++ b/m.x\n@@ -1 +1,2 @@\n keep\n" + line + "\n"
		res := analyzeDiff(diff)
		if dsSmell(res, "suppression") == nil {
			t.Errorf("suppression not detected for %q", line)
		}
	}
}

// An empty / swallowing catch is SOFT.
func TestAnalyzeDiff_EmptyCatch(t *testing.T) {
	diff := `diff --git a/h.py b/h.py
--- a/h.py
+++ b/h.py
@@ -1,2 +1,4 @@
 def f():
+    try:
+        risky()
+    except Exception: pass
`
	res := analyzeDiff(diff)
	if dsSmell(res, "empty_catch") == nil {
		t.Errorf("expected empty_catch, got %+v", res.Smells)
	}

	jsDiff := "diff --git a/h.js b/h.js\n--- a/h.js\n+++ b/h.js\n@@ -1 +1,2 @@\n run()\n+try { risky() } catch (e) {}\n"
	if dsSmell(analyzeDiff(jsDiff), "empty_catch") == nil {
		t.Errorf("expected empty_catch for js catch {}")
	}
}

// A stub / not-implemented body is SOFT.
func TestAnalyzeDiff_StubBody(t *testing.T) {
	for _, body := range []string{
		"+	panic(\"todo\")",
		"+    raise NotImplementedError",
		"+  throw new Error(\"not implemented\")",
		"+	return nil // TODO",
	} {
		diff := "diff --git a/svc.go b/svc.go\n--- a/svc.go\n+++ b/svc.go\n@@ -1 +1,2 @@\n keep\n" + body + "\n"
		res := analyzeDiff(diff)
		if dsSmell(res, "stub_body") == nil {
			t.Errorf("stub_body not detected for %q; got %+v", body, res.Smells)
		}
	}
}

// A genuine impl fix has no smells — clean.
func TestAnalyzeDiff_CleanFix(t *testing.T) {
	diff := `diff --git a/calc.go b/calc.go
--- a/calc.go
+++ b/calc.go
@@ -1,3 +1,3 @@
 func Add(a, b int) int {
-	return a - b
+	return a + b
 }
`
	res := analyzeDiff(diff)
	if res.Summary.Verdict != "clean" {
		t.Errorf("verdict = %q, want clean; smells=%+v", res.Summary.Verdict, res.Smells)
	}
	if len(res.Smells) != 0 {
		t.Errorf("clean fix produced smells: %+v", res.Smells)
	}
}

// --- File classification ---

func TestIsTestPath(t *testing.T) {
	tests := map[string]bool{
		"foo_test.go":              true,
		"pkg/auth/auth_test.go":    true,
		"src/auth.test.ts":         true,
		"src/auth.spec.ts":         true,
		"src/__tests__/auth.js":    true,
		"tests/test_handler.py":    true,
		"test_handler.py":          true,
		"spec/models/user_spec.rb": true,
		"src/auth.ts":              false,
		"internal/calc.go":         false,
		"latest_test_results.go":   false, // contains "test" but is not a test file
		"contest.go":               false,
		"src/attestation.ts":       false,
	}
	for path, want := range tests {
		if got := isTestPath(path); got != want {
			t.Errorf("isTestPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// --- Adversarial ---

// Diff header lines (+++/---) must not be counted as added/removed content.
func TestAnalyzeDiff_HeaderLinesIgnored(t *testing.T) {
	// Only a +++ header line "added" — no real content. Should be clean.
	diff := `diff --git a/calc.go b/calc.go
--- a/calc.go
+++ b/calc.go
@@ -1 +1 @@
 unchanged
`
	res := analyzeDiff(diff)
	if res.Summary.Verdict != "clean" {
		t.Errorf("verdict = %q, want clean (only headers/context)", res.Summary.Verdict)
	}
}

// A suppression token appearing on a REMOVED line (the author deleted a
// suppression — a good thing) must NOT be flagged.
func TestAnalyzeDiff_SuppressionOnRemovedLineNotFlagged(t *testing.T) {
	diff := `diff --git a/svc.ts b/svc.ts
--- a/svc.ts
+++ b/svc.ts
@@ -1,2 +1 @@
-// @ts-ignore
 const a = compute()
`
	res := analyzeDiff(diff)
	if dsSmell(res, "suppression") != nil {
		t.Errorf("removing a suppression must not be flagged: %+v", res.Smells)
	}
}

// Empty diff is clean, not a panic.
func TestAnalyzeDiff_Empty(t *testing.T) {
	res := analyzeDiff("")
	if res.Summary.Verdict != "clean" || len(res.Smells) != 0 {
		t.Errorf("empty diff should be clean, got %+v", res)
	}
}

// A renamed/added file with no impl change still classifies correctly.
func TestAnalyzeDiff_NoImplOnlyTestRename(t *testing.T) {
	diff := `diff --git a/x_test.go b/x_test.go
--- a/x_test.go
+++ b/x_test.go
@@ -1 +1,2 @@
 package x
+var _ = 1
`
	res := analyzeDiff(diff)
	if dsSmell(res, "test_only") == nil {
		t.Errorf("expected test_only; got %+v", res.Smells)
	}
}
