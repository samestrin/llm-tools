package multireview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleVars returns a PromptVars value suitable for rendering test
// templates. Mirrors the shape runMultiReview will pass in production.
func sampleVars(agent string) PromptVars {
	return PromptVars{
		DiffPath:   "/tmp/multi-review-1/diff.txt",
		DiffBytes:  500_000,
		DiffLines:  9_000,
		DiffMB:     0.5,
		LargeDiff:  false,
		BaseRef:    "v1.0.0",
		HeadRef:    "HEAD",
		RemoteRepo: "/tmp/multi-review-1/myrepo",
		AgentName:  agent,
	}
}

func TestLoadAgentPrompt_AgentFileWins(t *testing.T) {
	// When both <agent>.md and _base.md exist, the agent-specific file wins.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"), "BASE BODY for {{.AgentName}}")
	writeFile(t, filepath.Join(dir, "kai.md"), "KAI BODY for {{.AgentName}}")

	got, err := LoadAgentPrompt(dir, "kai", sampleVars("kai"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	if !strings.Contains(got, "KAI BODY for kai") {
		t.Errorf("expected agent-specific body, got: %q", got)
	}
	if strings.Contains(got, "BASE BODY") {
		t.Errorf("agent file should win over base, got: %q", got)
	}
}

func TestLoadAgentPrompt_FallsBackToBase(t *testing.T) {
	// When <agent>.md is missing but _base.md exists, _base.md is rendered.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"), "BASE BODY for {{.AgentName}}")

	got, err := LoadAgentPrompt(dir, "kai", sampleVars("kai"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	if !strings.Contains(got, "BASE BODY for kai") {
		t.Errorf("expected base body, got: %q", got)
	}
}

func TestLoadAgentPrompt_EmptyWhenNoFiles(t *testing.T) {
	// When neither <agent>.md nor _base.md exists, returns ("", nil).
	// Empty string is the signal to the caller to use its hardcoded default —
	// missing files are NOT errors.
	dir := t.TempDir()

	got, err := LoadAgentPrompt(dir, "kai", sampleVars("kai"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string when no prompts exist, got: %q", got)
	}
}

func TestLoadAgentPrompt_MissingDirReturnsEmpty(t *testing.T) {
	// When the prompts dir itself doesn't exist, returns ("", nil) —
	// fresh installs that haven't run update-prompts.sh fall back to hardcoded.
	got, err := LoadAgentPrompt("/nonexistent/dir/that/does/not/exist", "kai", sampleVars("kai"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string when dir missing, got: %q", got)
	}
}

func TestLoadAgentPrompt_RendersVariables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"),
		"Diff at {{.DiffPath}} ({{.DiffBytes}} bytes, {{.DiffLines}} lines). Range: {{.BaseRef}}..{{.HeadRef}}. Repo: {{.RemoteRepo}}")

	got, err := LoadAgentPrompt(dir, "bruce", sampleVars("bruce"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	want := "Diff at /tmp/multi-review-1/diff.txt (500000 bytes, 9000 lines). Range: v1.0.0..HEAD. Repo: /tmp/multi-review-1/myrepo"
	if got != want {
		t.Errorf("variable rendering wrong:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestLoadAgentPrompt_LargeDiffConditional(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"),
		`Always shown.
{{if .LargeDiff}}LARGE DIFF SECTION ({{printf "%.1f" .DiffMB}} MB){{end}}
End.`)

	// Small diff: section omitted.
	small := sampleVars("bruce")
	small.LargeDiff = false
	got, err := LoadAgentPrompt(dir, "bruce", small)
	if err != nil {
		t.Fatalf("small: %v", err)
	}
	if strings.Contains(got, "LARGE DIFF SECTION") {
		t.Errorf("small diff should not render large-diff section, got: %q", got)
	}

	// Large diff: section present, MB formatted.
	large := sampleVars("bruce")
	large.LargeDiff = true
	large.DiffMB = 2.3
	got2, err := LoadAgentPrompt(dir, "bruce", large)
	if err != nil {
		t.Fatalf("large: %v", err)
	}
	if !strings.Contains(got2, "LARGE DIFF SECTION (2.3 MB)") {
		t.Errorf("large diff should render section with formatted MB, got: %q", got2)
	}
}

func TestLoadAgentPrompt_BackticksPassThrough(t *testing.T) {
	// The production base prompt is full of backtick-quoted shell snippets.
	// text/template treats backticks as literal text — verify they survive.
	dir := t.TempDir()
	body := "Run `cat {{.DiffPath}}` then `ls -la {{.DiffPath}}`."
	writeFile(t, filepath.Join(dir, "_base.md"), body)

	got, err := LoadAgentPrompt(dir, "bruce", sampleVars("bruce"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	want := "Run `cat /tmp/multi-review-1/diff.txt` then `ls -la /tmp/multi-review-1/diff.txt`."
	if got != want {
		t.Errorf("backticks corrupted:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestLoadAgentPrompt_InvalidTemplateFallsBackToBase(t *testing.T) {
	// If <agent>.md has a template syntax error, fall back to _base.md
	// rather than crashing the run. (A bad agent file shouldn't take out
	// the whole pool.)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"), "BASE OK for {{.AgentName}}")
	writeFile(t, filepath.Join(dir, "kai.md"), "KAI BROKEN {{.MissingField | nosuchfunc}}")

	got, err := LoadAgentPrompt(dir, "kai", sampleVars("kai"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt should NOT error on invalid agent template, got: %v", err)
	}
	if !strings.Contains(got, "BASE OK for kai") {
		t.Errorf("expected fallback to base on broken agent template, got: %q", got)
	}
}

func TestLoadAgentPrompt_InvalidBaseReturnsEmpty(t *testing.T) {
	// If _base.md ALSO has a template syntax error, return empty string —
	// caller falls back to hardcoded. Better than crashing.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"), "BROKEN {{.MissingField | nosuchfunc}}")

	got, err := LoadAgentPrompt(dir, "kai", sampleVars("kai"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt should not error on broken base, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string when both files broken, got: %q", got)
	}
}

func TestResolvePromptDir_DefaultsToHome(t *testing.T) {
	// With no env var set, ResolvePromptDir returns ~/.llm-tools/multi-review/prompts.
	t.Setenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS", "")
	got := ResolvePromptDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".llm-tools", "multi-review", "prompts")
	if got != want {
		t.Errorf("ResolvePromptDir default = %q, want %q", got, want)
	}
}

func TestResolvePromptDir_EnvVarOverride(t *testing.T) {
	// LLM_TOOLS_MULTI_REVIEW_PROMPTS env var overrides the default location.
	override := "/tmp/custom-prompts-dir"
	t.Setenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS", override)
	got := ResolvePromptDir()
	if got != override {
		t.Errorf("ResolvePromptDir env override = %q, want %q", got, override)
	}
}

func TestLoadAgentPrompt_AgentNameSanitized(t *testing.T) {
	// An agent name containing path-traversal characters must not be able
	// to read files outside the prompts dir.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_base.md"), "BASE OK")

	// Try to escape with "../" — should NOT find a file outside dir,
	// should fall back to _base.md.
	got, err := LoadAgentPrompt(dir, "../etc/passwd", sampleVars("../etc/passwd"))
	if err != nil {
		t.Fatalf("LoadAgentPrompt: %v", err)
	}
	if !strings.Contains(got, "BASE OK") {
		t.Errorf("path-traversal agent name should fall back to base, got: %q", got)
	}
}

// writeFile helper for tests.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}
