package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDiscoveryFixture creates a temp root dir containing a real file at
// "src/real.go" and writes the given codebase-discovery.json content at
// <root>/codebase-discovery.json. Returns (jsonPath, rootDir).
func writeDiscoveryFixture(t *testing.T, content string) (jsonPath, rootDir string) {
	t.Helper()
	rootDir = t.TempDir()
	srcDir := filepath.Join(rootDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "real.go"), []byte("package src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jsonPath = filepath.Join(rootDir, "codebase-discovery.json")
	if err := os.WriteFile(jsonPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

func runDiscoveryValidateCmd(t *testing.T, args ...string) (DiscoveryValidateResult, string, error) {
	t.Helper()
	cmd := newDiscoveryValidateCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	var res DiscoveryValidateResult
	if err == nil {
		if jerr := json.Unmarshal(out.Bytes(), &res); jerr != nil {
			t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out.String())
		}
	}
	return res, errb.String(), err
}

// fixtureAllFields exercises every checked field with one real and one fake
// path (or, for existing_patterns, one real+fake pair inside "files").
const fixtureAllFields = `{
  "generated": "2026-07-10",
  "build_from": {"primary_file": "src/real.go", "reason": "anchor"},
  "files_to_modify": [
    {"path": "src/real.go", "reason": "human reason", "scope": "minor"},
    {"path": "src/fake.go", "reason": "human reason 2", "scope": "minor"}
  ],
  "related_files": [
    {"path": "src/real.go", "relevance": "high", "purpose": "p", "likely_modification": "reference-only"},
    {"path": "src/fake2.go", "relevance": "low", "purpose": "p", "likely_modification": "reference-only"}
  ],
  "semantic_matches": [
    {"file": "src/real.go", "symbol": "s", "type": "function", "line": 1, "score": 0.9, "query": "q", "relevance": "r"},
    {"file": "src/fake3.go", "symbol": "s", "type": "function", "line": 1, "score": 0.9, "query": "q", "relevance": "r"}
  ],
  "reusable_components": [
    {"name": "c1", "path": "src/real.go", "can_extend": true, "usage_example": "u"},
    {"name": "c2", "path": "src/fake4.go", "can_extend": true, "usage_example": "u"}
  ],
  "existing_patterns": [
    {"name": "p1", "description": "d", "files": ["src/real.go"], "follow_for": "f"},
    {"name": "p2", "description": "d", "files": ["src/real.go", "src/fake5.go"], "follow_for": "f"}
  ],
  "integration_points": [
    {"location": "src/real.go:Func", "type": "hook", "description": "d"},
    {"location": "src/fake6.go:Func", "type": "hook", "description": "d"},
    {"location": "malformed-no-colon", "type": "hook", "description": "d"}
  ],
  "files_to_create": [
    {"path": "src/brand_new.go", "purpose": "p", "based_on": null}
  ],
  "test_patterns": {
    "framework": "go test",
    "test_location": "src",
    "example_test": "src/fake_test.go"
  }
}`

func TestDiscoveryValidate_ReportMode_DetectsMissingAcrossFields(t *testing.T) {
	jsonPath, rootDir := writeDiscoveryFixture(t, fixtureAllFields)
	before, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	res, stderr, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}

	// Missing: files_to_modify(1) + related_files(1) + semantic_matches(1) +
	// reusable_components(1) + existing_patterns(1 entry, 1 missing file) +
	// integration_points(1) = 6
	if res.Summary.Missing != 6 {
		t.Errorf("missing = %d, want 6", res.Summary.Missing)
	}
	if res.Summary.AlreadyDeprecated != 0 {
		t.Errorf("already_deprecated = %d, want 0", res.Summary.AlreadyDeprecated)
	}
	// Informational skipped: test_patterns.example_test (build_from.primary_file
	// exists, test_patterns.test_location exists as a real dir "src") plus the
	// malformed (colon-less) integration_points location, which also can't be
	// deterministically checked.
	if res.Summary.InformationalSkipped != 2 {
		t.Errorf("informational_skipped = %d, want 2", res.Summary.InformationalSkipped)
	}
	if res.Written {
		t.Errorf("written should be false in report mode")
	}

	// Report mode must never touch the file on disk.
	after, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("report mode mutated the file:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestDiscoveryValidate_WriteMode_MarksDeprecatedWithoutClobberingReason(t *testing.T) {
	jsonPath, rootDir := writeDiscoveryFixture(t, fixtureAllFields)

	res, stderr, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--write", "--json")
	if err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr)
	}
	if !res.Written {
		t.Errorf("written should be true when stale entries exist and --write is set")
	}
	if res.Summary.Missing != 6 {
		t.Errorf("missing = %d, want 6", res.Summary.Missing)
	}

	updated, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(updated) {
		t.Fatalf("file is not valid JSON after --write:\n%s", updated)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(updated, &doc); err != nil {
		t.Fatal(err)
	}

	ftm := doc["files_to_modify"].([]interface{})
	fake := ftm[1].(map[string]interface{})
	if fake["status"] != "deprecated" {
		t.Errorf("files_to_modify[1].status = %v, want deprecated", fake["status"])
	}
	if fake["reason"] != "human reason 2" {
		t.Errorf("files_to_modify[1].reason was clobbered: got %v, want %q", fake["reason"], "human reason 2")
	}
	if depReason, _ := fake["deprecated_reason"].(string); !strings.Contains(depReason, "src/fake.go") {
		t.Errorf("files_to_modify[1].deprecated_reason = %q, want to mention src/fake.go", depReason)
	}

	real := ftm[0].(map[string]interface{})
	if _, hasStatus := real["status"]; hasStatus {
		t.Errorf("files_to_modify[0] (real path) should not be annotated: %+v", real)
	}

	// files_to_create must never be touched.
	ftc := doc["files_to_create"].([]interface{})
	created := ftc[0].(map[string]interface{})
	if _, hasStatus := created["status"]; hasStatus {
		t.Errorf("files_to_create entries must never be auto-deprecated: %+v", created)
	}

	// test_patterns is a single-object anchor; must never be mutated even with --write.
	tp := doc["test_patterns"].(map[string]interface{})
	if _, hasStatus := tp["status"]; hasStatus {
		t.Errorf("test_patterns must never be auto-mutated: %+v", tp)
	}
}

func TestDiscoveryValidate_Idempotent(t *testing.T) {
	jsonPath, rootDir := writeDiscoveryFixture(t, fixtureAllFields)

	if _, _, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--write", "--json"); err != nil {
		t.Fatal(err)
	}
	firstPass, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}

	res2, _, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--write", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if res2.Summary.Missing != 0 {
		t.Errorf("second pass missing = %d, want 0", res2.Summary.Missing)
	}
	if res2.Summary.AlreadyDeprecated != 6 {
		t.Errorf("second pass already_deprecated = %d, want 6", res2.Summary.AlreadyDeprecated)
	}
	if res2.Written {
		t.Errorf("second pass should not report written=true (nothing changed)")
	}

	secondPass, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstPass) != string(secondPass) {
		t.Errorf("second pass changed the file even though nothing was newly stale:\nfirst=%s\nsecond=%s", firstPass, secondPass)
	}
}

func TestDiscoveryValidate_MalformedIntegrationPointSkippedNotMissing(t *testing.T) {
	jsonPath, rootDir := writeDiscoveryFixture(t, fixtureAllFields)
	res, _, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var sawMalformed bool
	for _, it := range res.Items {
		if it.Field == "integration_points" && it.Path == "malformed-no-colon" {
			sawMalformed = true
			if it.Status != "skipped" {
				t.Errorf("malformed location status = %q, want skipped", it.Status)
			}
		}
	}
	if !sawMalformed {
		t.Errorf("expected a skipped item for the malformed (colon-less) integration_points location")
	}
}

func TestDiscoveryValidate_AlreadyDeprecatedEntrySkipped(t *testing.T) {
	fixture := `{
  "files_to_modify": [
    {"path": "src/fake.go", "reason": "old", "scope": "minor", "status": "deprecated", "deprecated_reason": "already handled"}
  ]
}`
	jsonPath, rootDir := writeDiscoveryFixture(t, fixture)
	res, _, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--write", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.AlreadyDeprecated != 1 {
		t.Errorf("already_deprecated = %d, want 1", res.Summary.AlreadyDeprecated)
	}
	if res.Summary.Missing != 0 {
		t.Errorf("missing = %d, want 0 (already-deprecated entries are not re-checked)", res.Summary.Missing)
	}
	if res.Written {
		t.Errorf("written should be false — nothing new to change")
	}
}

func TestDiscoveryValidate_JSONOutput_ItemsNotNull(t *testing.T) {
	jsonPath, rootDir := writeDiscoveryFixture(t, `{}`)
	cmd := newDiscoveryValidateCmd()
	cmd.SetArgs([]string{"--path", jsonPath, "--root", rootDir, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var res DiscoveryValidateResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if res.Items == nil {
		t.Error("items must not be JSON null (should be [])")
	}
}

func TestDiscoveryValidate_MissingPathFlag(t *testing.T) {
	cmd := newDiscoveryValidateCmd()
	cmd.SetArgs([]string{"--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing --path must return an error")
	}
}

func TestDiscoveryValidate_InvalidJSONFile(t *testing.T) {
	rootDir := t.TempDir()
	jsonPath := filepath.Join(rootDir, "codebase-discovery.json")
	if err := os.WriteFile(jsonPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--json")
	if err == nil {
		t.Fatal("invalid JSON input must return an error")
	}
}

func TestDiscoveryValidate_TextOutput(t *testing.T) {
	jsonPath, rootDir := writeDiscoveryFixture(t, fixtureAllFields)
	cmd := newDiscoveryValidateCmd()
	cmd.SetArgs([]string{"--path", jsonPath, "--root", rootDir})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "discovery_validate:") {
		t.Errorf("text output missing summary line, got:\n%s", text)
	}
	if !strings.Contains(text, "MISSING:") {
		t.Errorf("text output missing MISSING entries, got:\n%s", text)
	}
}

func TestDiscoveryValidate_RootFlagResolvesRelativePaths(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "pkg", "real.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := `{"files_to_modify": [{"path": "pkg/real.go", "reason": "r", "scope": "minor"}]}`
	jsonPath := filepath.Join(rootDir, "codebase-discovery.json")
	if err := os.WriteFile(jsonPath, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _, err := runDiscoveryValidateCmd(t, "--path", jsonPath, "--root", rootDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if res.Summary.Missing != 0 {
		t.Errorf("missing = %d, want 0 (--root should resolve pkg/real.go)", res.Summary.Missing)
	}
	if res.Summary.Checked != 1 {
		t.Errorf("checked = %d, want 1", res.Summary.Checked)
	}
}
