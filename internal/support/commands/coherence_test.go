package commands

import (
	"math"
	"testing"
)

func TestCosine(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 2, 3}, []float32{1, 2, 3}, 1.0},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 0.0},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, -1.0},
		{"length mismatch", []float32{1, 0}, []float32{1, 0, 0}, 0.0},
		{"zero vector", []float32{0, 0}, []float32{1, 1}, 0.0},
		{"empty", []float32{}, []float32{}, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cosine(tc.a, tc.b)
			if math.Abs(got-tc.want) > 1e-6 {
				t.Fatalf("cosine(%v,%v) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestStripCoherenceBoilerplate(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"deferred paren", "text (Deferred: Epic 43) more", "text more"},
		{"intent_note", "problem (intent_note: deferred per plan) x", "problem x"},
		{"clarified bracket", "fix [Clarified: copy-pasted from row 77] y", "fix y"},
		{"planned paren", "do it (Planned: 47.0 T5) now", "do it now"},
		{"no boilerplate", "  just a normal sentence  ", "just a normal sentence"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripCoherenceBoilerplate(tc.in); got != tc.want {
				t.Fatalf("strip(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExtractCodeIdentifiers(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // must all be present
		none bool     // if true, expect empty set
	}{
		{"backtick + camel", "`aliasTable` and the resolver", []string{"aliastable"}, false},
		{"dotted paths", "os.ReadFile with io.LimitReader", []string{"os.readfile", "io.limitreader"}, false},
		{"snake_case", "call run_func in the helper", []string{"run_func"}, false},
		{"pascal", "invoke CommitBaselineIndex now", []string{"commitbaselineindex"}, false},
		{"pure prose", "add a test and verify the change", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCodeIdentifiers(tc.in)
			if tc.none {
				if len(got) != 0 {
					t.Fatalf("extract(%q) = %v, want empty", tc.in, keysOf(got))
				}
				return
			}
			for _, w := range tc.want {
				if _, ok := got[w]; !ok {
					t.Fatalf("extract(%q) missing %q; got %v", tc.in, w, keysOf(got))
				}
			}
		})
	}
}

func TestIsDeferralPunt(t *testing.T) {
	cases := []struct {
		name, in string
		want     bool
	}{
		{"owned by team", "A backend architecture decision owned by the atcr.dev team", true},
		{"blocked on", "Blocked on a provisioned backend secret", true},
		{"deferred", "Deferred per AC 04-01: stays hardcoded", true},
		{"technical remedy", "add a stat-size guard + io.LimitReader (mirroring ingest.go)", false},
		{"branch the message", "Branch the message on which flag was set and add a test", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDeferralPunt(tc.in); got != tc.want {
				t.Fatalf("isDeferralPunt(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func keysOf(m map[string]struct{}) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	return k
}
