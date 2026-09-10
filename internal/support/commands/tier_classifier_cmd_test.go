package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTierClassifierCmd_JSON(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "package-tiers.yaml")
	os.WriteFile(cfg, []byte(`packages:
  react: critical
patterns:
  - pattern: "eslint-*"
    tier: utility
categories:
  - keyword: "test"
    tier: pattern
`), 0o644)

	cmd := newTierClassifierCmd()
	cmd.SetArgs([]string{"--packages", "react,eslint-plugin-x,supertest,unknownpkg", "--config", cfg, "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, errb.String())
	}
	var res TierResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if res.Assigned["react"].Pass != 1 || res.Assigned["react"].Tier != "critical" {
		t.Errorf("react = %+v, want pass1/critical", res.Assigned["react"])
	}
	if res.Assigned["eslint-plugin-x"].Pass != 2 {
		t.Errorf("eslint-plugin-x should match pattern (pass2); got %+v", res.Assigned["eslint-plugin-x"])
	}
	if res.Assigned["supertest"].Pass != 3 {
		t.Errorf("supertest should match category 'test' (pass3); got %+v", res.Assigned["supertest"])
	}
	if len(res.Unassigned) != 1 || res.Unassigned[0] != "unknownpkg" {
		t.Errorf("unassigned = %v, want [unknownpkg]", res.Unassigned)
	}
}

// Config where patterns/categories are maps (not ordered lists) — loader must
// still classify deterministically (most-specific match first).
func TestTierClassifierCmd_MapFormConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "package-tiers.yaml")
	os.WriteFile(cfg, []byte(`packages:
  react: critical
patterns:
  "eslint-*": utility
categories:
  http: important
`), 0o644)
	cmd := newTierClassifierCmd()
	cmd.SetArgs([]string{"--packages", "react,eslint-plugin-x,fast-http,zzz", "--config", cfg, "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, errb.String())
	}
	var res TierResult
	json.Unmarshal(out.Bytes(), &res)
	if res.Assigned["react"].Tier != "critical" || res.Assigned["eslint-plugin-x"].Tier != "utility" || res.Assigned["fast-http"].Tier != "important" {
		t.Errorf("map-form config misclassified: %+v", res.Assigned)
	}
	if len(res.Unassigned) != 1 || res.Unassigned[0] != "zzz" {
		t.Errorf("unassigned = %v, want [zzz]", res.Unassigned)
	}
}

// Nested schema (critical:/important:/patterns:/utility:, packages buried
// under category headers) must classify via Pass 1, same as the flat schema's
// packages map — this is the shape produced by hand-authored package-tiers.yaml
// files, distinct from the flat glob-rule "patterns" list of the same name.
func TestTierClassifierCmd_NestedSchemaConfig(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "package-tiers.yaml")
	os.WriteFile(cfg, []byte(`version: "1.0"
critical:
  frameworks:
    frontend: [react, vue]
important:
  validation: [zod, yup]
patterns:
  component-libraries: [shadcn/ui, radix-ui]
utility:
  description: "Packages not explicitly listed are treated as utilities"
framework_overrides:
  nextjs:
    promote_context: [react]
`), 0o644)
	cmd := newTierClassifierCmd()
	cmd.SetArgs([]string{"--packages", "react,zod,radix-ui,unknownpkg", "--config", cfg, "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\n%s", err, errb.String())
	}
	var res TierResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	want := map[string]string{"react": "critical", "zod": "important", "radix-ui": "pattern"}
	for pkg, tier := range want {
		got := res.Assigned[pkg]
		if got.Tier != tier || got.Pass != 1 {
			t.Errorf("%s = %+v, want pass1/%s", pkg, got, tier)
		}
	}
	if len(res.Unassigned) != 1 || res.Unassigned[0] != "unknownpkg" {
		t.Errorf("unassigned = %v, want [unknownpkg]", res.Unassigned)
	}
}

func TestTierClassifierCmd_MissingPackages(t *testing.T) {
	cmd := newTierClassifierCmd()
	cmd.SetArgs([]string{"--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing --packages must error")
	}
}

// No --config → all packages unassigned (no error; deterministic passes simply
// match nothing).
func TestTierClassifierCmd_NoConfig(t *testing.T) {
	cmd := newTierClassifierCmd()
	cmd.SetArgs([]string{"--packages", "a,b", "--json"})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("no config should not error: %v", err)
	}
	var res TierResult
	json.Unmarshal(out.Bytes(), &res)
	if len(res.Unassigned) != 2 {
		t.Errorf("no config → all unassigned; got %v", res.Unassigned)
	}
}
