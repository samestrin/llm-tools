package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samestrin/llm-tools/internal/support/multireview"
)

func writeConfigYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigCSV(t *testing.T) {
	cfg := map[string]interface{}{
		"review": map[string]interface{}{
			"direct": map[string]interface{}{
				"agents_list": []interface{}{"alice", "bob"},
				"agents_csv":  "alice,bob",
				"agents_bad":  map[string]interface{}{"x": 1},
			},
		},
	}

	if v, ok, err := configCSV(cfg, "review.direct.agents_list"); err != nil || !ok || v != "alice,bob" {
		t.Errorf("yaml list: got (%q, %v, %v), want (alice,bob, true, nil)", v, ok, err)
	}
	if v, ok, err := configCSV(cfg, "review.direct.agents_csv"); err != nil || !ok || v != "alice,bob" {
		t.Errorf("csv string: got (%q, %v, %v)", v, ok, err)
	}
	if _, ok, _ := configCSV(cfg, "review.direct.missing"); ok {
		t.Error("missing key should report ok=false")
	}
	if _, _, err := configCSV(cfg, "review.direct.agents_bad"); err == nil {
		t.Error("map-shaped value should error")
	}
}

func TestConfigInt(t *testing.T) {
	cfg := map[string]interface{}{
		"a": map[string]interface{}{
			"int":    900,
			"str":    "120",
			"badstr": "abc",
		},
	}

	if v, ok, err := configInt(cfg, "a.int"); err != nil || !ok || v != 900 {
		t.Errorf("int: got (%d, %v, %v)", v, ok, err)
	}
	if v, ok, err := configInt(cfg, "a.str"); err != nil || !ok || v != 120 {
		t.Errorf("numeric string: got (%d, %v, %v)", v, ok, err)
	}
	if _, ok, _ := configInt(cfg, "a.missing"); ok {
		t.Error("missing key should report ok=false")
	}
	if _, _, err := configInt(cfg, "a.badstr"); err == nil {
		t.Error("non-numeric string should error")
	}
}

func TestMultiReview_ConfigProvidesRosterAndHost(t *testing.T) {
	repo := initRangeFixtureRepo(t)
	outDir := filepath.Join(t.TempDir(), "out")
	cfgPath := writeConfigYAML(t, `
review:
  multi_agent:
    openclaw_host: user@config.lan
    reviewers:
      - bruce
      - greta
    timeout_seconds: "45"
`)

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	var invoked []string
	var mu chan struct{} = make(chan struct{}, 1)
	mu <- struct{}{}
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		<-mu
		invoked = append(invoked, p.AgentName)
		mu <- struct{}{}
		return mockResultFor(p.AgentName, "LOW|x:1|p|f|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"--repo", repo,
		"--output-dir", outDir,
		// no --reviewers, no --openclaw-host, no --base → config + auto-resolve
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(invoked) != 2 {
		t.Errorf("invoked = %v, want bruce+greta from config", invoked)
	}
}

func TestMultiReview_ExplicitFlagBeatsConfig(t *testing.T) {
	repo := initRangeFixtureRepo(t)
	outDir := filepath.Join(t.TempDir(), "out")
	cfgPath := writeConfigYAML(t, `
review:
  multi_agent:
    openclaw_host: user@config.lan
    reviewers: [bruce, greta, otto]
`)

	withMockShipBundle(t)
	withMockPreComputeDiff(t)
	var invoked []string
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	withMockInvoker(t, func(ctx context.Context, p multireview.InvokeReviewerParams) (multireview.InvokeReviewerResult, error) {
		<-gate
		invoked = append(invoked, p.AgentName)
		gate <- struct{}{}
		return mockResultFor(p.AgentName, "LOW|x:1|p|f|c"), nil
	})

	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"--reviewers", "bruce", // explicit beats config's 3-agent roster
		"--repo", repo,
		"--output-dir", outDir,
	})
	cmd.SetOut(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(invoked) != 1 || invoked[0] != "bruce" {
		t.Errorf("invoked = %v, want [bruce] (explicit flag wins)", invoked)
	}
}

func TestMultiReview_ConfigMissingFile(t *testing.T) {
	cmd := newMultiReviewCmd()
	cmd.SetArgs([]string{
		"--config", "/nonexistent/config.yaml",
		"--reviewers", "bruce",
		"--repo", ".",
		"--openclaw-host", "x@y",
		"--output-dir", "/tmp/out",
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --config file")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Errorf("error %q should mention config", err.Error())
	}
}

func TestReviewDirect_ConfigProvidesRosterAndTimeout(t *testing.T) {
	registryDir := newDirectTestEnv(t)
	cfgPath := writeConfigYAML(t, `
review:
  direct:
    agents:
      - alice
    timeout_seconds: 60
`)

	diffFile := filepath.Join(t.TempDir(), "diff.txt")
	if err := os.WriteFile(diffFile, []byte("+func foo() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()

	cmd := newReviewDirectCmd()
	cmd.SetArgs([]string{
		"--config", cfgPath,
		"--diff-file", diffFile,
		"--output-dir", outputDir,
		"--registry-dir", registryDir,
		// no --reviewers → config supplies alice
	})
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "raw", "alice", "review.md")); err != nil {
		t.Errorf("alice (from config) should have been invoked: %v", err)
	}
}
