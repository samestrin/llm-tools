package commands

import (
	"reflect"
	"testing"
)

func testTierConfig() TierConfig {
	return TierConfig{
		Explicit: map[string]string{
			"react":             "critical",
			"lodash":            "utility",
			"eslint-config-base": "critical", // also matches eslint-* pattern; pass 1 must win
		},
		Patterns: []TierRule{
			{Match: "@types/*", Tier: "skip"},
			{Match: "eslint-*", Tier: "utility"},
			{Match: "*-loader", Tier: "pattern"},
		},
		Categories: []TierRule{
			{Match: "test", Tier: "pattern"},
			{Match: "http", Tier: "important"},
		},
	}
}

func TestClassifyTiers_AllPasses(t *testing.T) {
	pkgs := []string{"react", "lodash", "@types/node", "eslint-plugin-x", "css-loader", "supertest", "fast-http", "unknownpkg", "eslint-config-base"}
	res := classifyTiers(pkgs, testTierConfig())

	want := map[string]TierAssignment{
		"react":              {Tier: "critical", Pass: 1},
		"lodash":             {Tier: "utility", Pass: 1},
		"eslint-config-base": {Tier: "critical", Pass: 1}, // pass 1 beats pattern
		"@types/node":        {Tier: "skip", Pass: 2},
		"eslint-plugin-x":    {Tier: "utility", Pass: 2},
		"css-loader":         {Tier: "pattern", Pass: 2},
		"supertest":          {Tier: "pattern", Pass: 3}, // contains "test"
		"fast-http":          {Tier: "important", Pass: 3}, // contains "http"
	}
	for pkg, exp := range want {
		got, ok := res.Assigned[pkg]
		if !ok {
			t.Errorf("%s not assigned, want %+v", pkg, exp)
			continue
		}
		if got != exp {
			t.Errorf("%s = %+v, want %+v", pkg, got, exp)
		}
	}
	if !reflect.DeepEqual(res.Unassigned, []string{"unknownpkg"}) {
		t.Errorf("unassigned = %v, want [unknownpkg]", res.Unassigned)
	}
	if res.Counts.Pass1 != 3 || res.Counts.Pass2 != 3 || res.Counts.Pass3 != 2 || res.Counts.Unassigned != 1 {
		t.Errorf("counts = %+v, want pass1=3 pass2=3 pass3=2 unassigned=1", res.Counts)
	}
}

func TestClassifyTiers_FirstMatchWinsWithinPass(t *testing.T) {
	cfg := TierConfig{
		Patterns: []TierRule{
			{Match: "css-*", Tier: "important"}, // first
			{Match: "*-loader", Tier: "pattern"},
		},
	}
	res := classifyTiers([]string{"css-loader"}, cfg)
	if res.Assigned["css-loader"].Tier != "important" {
		t.Errorf("first matching pattern should win; got %q", res.Assigned["css-loader"].Tier)
	}
}

func TestClassifyTiers_EmptyConfigAllUnassigned(t *testing.T) {
	res := classifyTiers([]string{"a", "b"}, TierConfig{})
	if len(res.Assigned) != 0 || !reflect.DeepEqual(res.Unassigned, []string{"a", "b"}) {
		t.Errorf("empty config: all unassigned; got assigned=%v unassigned=%v", res.Assigned, res.Unassigned)
	}
}

// A package matching both a pattern (pass 2) and a category keyword (pass 3)
// must be assigned by pass 2 (earlier pass wins).
func TestClassifyTiers_Pass2BeatsPass3(t *testing.T) {
	cfg := TierConfig{
		Patterns:   []TierRule{{Match: "*-loader", Tier: "pattern"}},
		Categories: []TierRule{{Match: "load", Tier: "important"}}, // "css-loader" contains "load"
	}
	res := classifyTiers([]string{"css-loader"}, cfg)
	got := res.Assigned["css-loader"]
	if got.Pass != 2 || got.Tier != "pattern" {
		t.Errorf("css-loader = %+v, want pass2/pattern (pattern beats category)", got)
	}
}

// path.Match: a glob's "*" does not cross "/", so "@types/*" matches a single
// scoped segment but not a deeper path.
func TestClassifyTiers_ScopedGlobSemantics(t *testing.T) {
	cfg := TierConfig{Patterns: []TierRule{{Match: "@types/*", Tier: "skip"}}}
	res := classifyTiers([]string{"@types/node", "@types/babel__core"}, cfg)
	if res.Assigned["@types/node"].Tier != "skip" || res.Assigned["@types/babel__core"].Tier != "skip" {
		t.Errorf("@types/* should match scoped type packages; got %+v", res.Assigned)
	}
}

func TestClassifyTiers_EmptyPackages(t *testing.T) {
	res := classifyTiers(nil, testTierConfig())
	if len(res.Assigned) != 0 || len(res.Unassigned) != 0 {
		t.Errorf("no packages → empty result")
	}
	if res.Unassigned == nil {
		t.Errorf("Unassigned must be non-nil for JSON []")
	}
}
