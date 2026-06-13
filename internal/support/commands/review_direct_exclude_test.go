package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/pkg/llmapi"
)

// excludeFixtureRepo builds a repo whose HEAD commit changes a code file, a
// nested .planning artifact, and CHANGELOG.md. Reviewing --merge-commit HEAD
// (HEAD^..HEAD) touches all three. Returns the repo dir.
func excludeFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) { gitInDir(t, dir, args...) }
	writeN := func(name, content string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	git("config", "commit.gpgsign", "false")
	writeN("code.go", "package p\n\nfunc A() int { return 1 }\n")
	writeN(".planning/technical-debt/README.md", "# TD\n\nold\n")
	writeN("CHANGELOG.md", "# Changelog\n\nold\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	writeN("code.go", "package p\n\nfunc A() int { return 2 }\n")
	writeN(".planning/technical-debt/README.md", "# TD\n\nnew\n")
	writeN("CHANGELOG.md", "# Changelog\n\nnew\n")
	git("add", "-A")
	git("commit", "-q", "-m", "head")
	return dir
}

// runReviewDirectSelfServe runs review_direct in self-serve mode against repoDir
// reviewing HEAD (merge-commit), with a mock provider so the fan-out succeeds.
// Returns stdout and the generated diff.txt content.
func runReviewDirectSelfServe(t *testing.T, repoDir string, extraArgs ...string) (string, string, error) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := llmapi.ChatResponse{Choices: []llmapi.Choice{{Message: llmapi.Message{Content: "LGTM"}}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	registryDir := t.TempDir()
	os.WriteFile(filepath.Join(registryDir, "registry.yaml"), []byte(`
providers:
  test:
    api_key_env: TEST_API_KEY
    base_url: `+server.URL+`
agents:
  alice:
    provider: test
    model: test-model
    timeout_secs: 60
`), 0644)
	os.WriteFile(filepath.Join(registryDir, "alice.md"), []byte("reviewer"), 0644)
	os.Setenv("TEST_API_KEY", "k")
	defer os.Unsetenv("TEST_API_KEY")

	head := gitInDir(t, repoDir, "rev-parse", "HEAD")
	outputDir := t.TempDir()
	args := []string{
		"--reviewers", "alice",
		"--repo", repoDir,
		"--merge-commit", head,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
		"--timeout-seconds", "60",
	}
	args = append(args, extraArgs...)

	cmd := newReviewDirectCmd()
	cmd.SetArgs(args)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()

	diffTxt, _ := os.ReadFile(filepath.Join(outputDir, "diff.txt"))
	return stdout.String(), string(diffTxt), err
}

func TestReviewDirect_DefaultExcludes(t *testing.T) {
	dir := excludeFixtureRepo(t)
	stdout, diff, err := runReviewDirectSelfServe(t, dir)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(diff, "code.go") {
		t.Errorf("default excludes dropped code.go")
	}
	if strings.Contains(diff, ".planning/") {
		t.Errorf("default excludes kept .planning/:\n%s", diff)
	}
	if strings.Contains(diff, "CHANGELOG.md") {
		t.Errorf("default excludes kept CHANGELOG.md")
	}
	if !strings.Contains(stdout, "excluded 2 file") {
		t.Errorf("report line should note 2 excluded files; got: %s", stdout)
	}
}

func TestReviewDirect_DisabledWithEmptyFlag(t *testing.T) {
	dir := excludeFixtureRepo(t)
	stdout, diff, err := runReviewDirectSelfServe(t, dir, "--exclude", "")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, want := range []string{"code.go", ".planning/technical-debt/README.md", "CHANGELOG.md"} {
		if !strings.Contains(diff, want) {
			t.Errorf("--exclude='' should keep everything; missing %q", want)
		}
	}
	if strings.Contains(stdout, "excluded") {
		t.Errorf("disabled excludes should not print an excluded clause; got: %s", stdout)
	}
}

func TestReviewDirect_FlagReplacesDefault(t *testing.T) {
	dir := excludeFixtureRepo(t)
	// Only CHANGELOG.md excluded; .planning must survive (flag replaces default).
	_, diff, err := runReviewDirectSelfServe(t, dir, "--exclude", "CHANGELOG.md")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(diff, ".planning/technical-debt/README.md") {
		t.Errorf("flag should replace default; .planning must remain:\n%s", diff)
	}
	if strings.Contains(diff, "CHANGELOG.md") {
		t.Errorf("CHANGELOG.md should be excluded by flag")
	}
}

func TestReviewDirect_ConfigExcludes(t *testing.T) {
	dir := excludeFixtureRepo(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfg, []byte(`
review:
  direct:
    exclude_globs:
      - "CHANGELOG.md"
`), 0644)
	_, diff, err := runReviewDirectSelfServe(t, dir, "--config", cfg)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(diff, ".planning/technical-debt/README.md") {
		t.Errorf("config should replace default; .planning must remain")
	}
	if strings.Contains(diff, "CHANGELOG.md") {
		t.Errorf("CHANGELOG.md should be excluded by config")
	}
}

func TestReviewDirect_FlagBeatsConfig(t *testing.T) {
	dir := excludeFixtureRepo(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfg, []byte(`
review:
  direct:
    exclude_globs:
      - "CHANGELOG.md"
`), 0644)
	// Flag excludes .planning only → CHANGELOG must survive (flag wins over config).
	_, diff, err := runReviewDirectSelfServe(t, dir, "--config", cfg, "--exclude", ".planning/**")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(diff, ".planning/") {
		t.Errorf("flag should exclude .planning")
	}
	if !strings.Contains(diff, "CHANGELOG.md") {
		t.Errorf("flag wins over config; CHANGELOG must remain")
	}
}

// Adversarial: an empty list in config disables exclusion (everything reviewed).
func TestReviewDirect_ConfigEmptyListDisables(t *testing.T) {
	dir := excludeFixtureRepo(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfg, []byte("review:\n  direct:\n    exclude_globs: []\n"), 0644)
	stdout, diff, err := runReviewDirectSelfServe(t, dir, "--config", cfg)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, want := range []string{".planning/technical-debt/README.md", "CHANGELOG.md"} {
		if !strings.Contains(diff, want) {
			t.Errorf("empty config list should disable excludes; missing %q", want)
		}
	}
	if strings.Contains(stdout, "excluded") {
		t.Errorf("disabled excludes should print no excluded clause; got: %s", stdout)
	}
}

// Adversarial: globs active but matching nothing → diff is full, report says 0.
func TestReviewDirect_NoMatchReportsZero(t *testing.T) {
	dir := excludeFixtureRepo(t)
	stdout, diff, err := runReviewDirectSelfServe(t, dir, "--exclude", "nomatch/**")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(diff, ".planning/technical-debt/README.md") {
		t.Errorf("non-matching glob should keep all files")
	}
	if !strings.Contains(stdout, "excluded 0 file(s) via [nomatch/**]") {
		t.Errorf("report should state 0 excluded with the active globs; got: %s", stdout)
	}
}

// Adversarial: when every changed file is excluded, the empty-diff error names
// the exclusion as the cause rather than blaming empty commits.
func TestReviewDirect_AllExcludedNamesCause(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) { gitInDir(t, dir, args...) }
	writeN := func(name, content string) {
		full := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}
	git("init", "-q", "-b", "main")
	git("config", "user.email", "t@e.com")
	git("config", "user.name", "T")
	git("config", "commit.gpgsign", "false")
	writeN(".planning/x.md", "old\n")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	writeN(".planning/x.md", "new\n")
	git("add", "-A")
	git("commit", "-q", "-m", "head")

	_, _, err := runReviewDirectSelfServe(t, dir)
	if err == nil {
		t.Fatal("expected error when every file is excluded")
	}
	if !strings.Contains(err.Error(), "every changed file was excluded") {
		t.Errorf("error should name exclusion as cause; got: %v", err)
	}
}

// Adversarial: in pre-computed --diff-file mode, excludes are inert (the diff is
// used verbatim — exclusion only applies to self-serve git diff generation).
func TestReviewDirect_DiffFileIgnoresExcludes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(llmapi.ChatResponse{Choices: []llmapi.Choice{{Message: llmapi.Message{Content: "ok"}}}})
	}))
	defer server.Close()
	registryDir := t.TempDir()
	os.WriteFile(filepath.Join(registryDir, "registry.yaml"), []byte("providers:\n  test:\n    api_key_env: TEST_API_KEY\n    base_url: "+server.URL+"\nagents:\n  alice:\n    provider: test\n    model: m\n    timeout_secs: 60\n"), 0644)
	os.WriteFile(filepath.Join(registryDir, "alice.md"), []byte("r"), 0644)
	os.Setenv("TEST_API_KEY", "k")
	defer os.Unsetenv("TEST_API_KEY")

	diffFile := filepath.Join(t.TempDir(), "pre.diff")
	os.WriteFile(diffFile, []byte("diff --git a/.planning/x.md b/.planning/x.md\n+changed\n"), 0644)
	outputDir := t.TempDir()

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--reviewers", "alice", "--diff-file", diffFile, "--output-dir", outputDir,
		"--registry-dir", registryDir, "--timeout-seconds", "60", "--exclude", ".planning/**",
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "excluded") {
		t.Errorf("pre-computed diff mode must not apply excludes; got: %s", stdout.String())
	}
}

// Adversarial: a non-scalar exclude_globs entry fails fast with a clear message.
func TestReviewDirect_MalformedConfigExcludes(t *testing.T) {
	dir := excludeFixtureRepo(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(cfg, []byte("review:\n  direct:\n    exclude_globs:\n      - nested:\n          bad: map\n"), 0644)
	_, _, err := runReviewDirectSelfServe(t, dir, "--config", cfg)
	if err == nil {
		t.Fatal("expected error for non-scalar exclude_globs entry")
	}
	if !strings.Contains(err.Error(), "exclude_globs") {
		t.Errorf("error should name the offending key; got: %v", err)
	}
}
