package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeFixtureReadme(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(fixtureTDReadme), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runTDFilterCmd(t *testing.T, args ...string) (TDFilterResult, string, error) {
	t.Helper()
	cmd := newTDFilterCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var res TDFilterResult
	if err == nil {
		if jerr := json.Unmarshal(out.Bytes(), &res); jerr != nil {
			t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out.String())
		}
	}
	return res, errb.String(), err
}

func TestTDFilterCmd_JSON(t *testing.T) {
	path := writeFixtureReadme(t)
	res, stderr, err := runTDFilterCmd(t, "--path", path, "--mode", "all", "--severity", "high", "--json")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}
	if len(res.Items) != 1 || res.Items[0].FileLine != "a.go:10" {
		t.Errorf("items = %+v, want single a.go:10", res.Items)
	}
	if res.Summary.ExcludedBySeverity != 3 {
		t.Errorf("excluded_by_severity = %d, want 3", res.Summary.ExcludedBySeverity)
	}
}

func TestTDFilterCmd_GroupScope(t *testing.T) {
	path := writeFixtureReadme(t)
	res, stderr, err := runTDFilterCmd(t, "--path", path, "--mode", "all", "--group", "1")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}
	if len(res.Summary.GroupScope) != 2 {
		t.Errorf("group_scope = %v, want 2 files", res.Summary.GroupScope)
	}
}

func TestTDFilterCmd_MissingPath(t *testing.T) {
	cmd := newTDFilterCmd()
	cmd.SetArgs([]string{"--mode", "all"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing --path must error")
	}
}

func TestTDFilterCmd_BadMode(t *testing.T) {
	path := writeFixtureReadme(t)
	cmd := newTDFilterCmd()
	cmd.SetArgs([]string{"--path", path, "--mode", "bogus"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("bad --mode must error")
	}
}

func TestTDFilterCmd_UnreadableFile(t *testing.T) {
	cmd := newTDFilterCmd()
	cmd.SetArgs([]string{"--path", "/no/such/file/README.md"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("unreadable --path must error")
	}
}
