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

func TestCombineSignals(t *testing.T) {
	t.Run("mean of available signals", func(t *testing.T) {
		results := []signalResult{
			{name: "cosine", scores: []float64{0.9, 0.1, 0.5}, available: true},
			{name: "rerank", scores: []float64{0.7, 0.3, 0.5}, available: true},
			{name: "ground", scores: []float64{1, 1, 1}, available: false}, // excluded
		}
		combined, active := combineSignals(results, 3)
		want := []float64{0.8, 0.2, 0.5}
		for i := range want {
			if math.Abs(combined[i]-want[i]) > 1e-9 {
				t.Fatalf("combined[%d] = %v, want %v", i, combined[i], want[i])
			}
		}
		if len(active) != 2 || active[0] != "cosine" || active[1] != "rerank" {
			t.Fatalf("active = %v, want [cosine rerank]", active)
		}
	})

	t.Run("length-mismatched signal excluded", func(t *testing.T) {
		results := []signalResult{
			{name: "cosine", scores: []float64{0.4, 0.6}, available: true},
			{name: "bad", scores: []float64{0.1}, available: true}, // wrong length
		}
		combined, active := combineSignals(results, 2)
		if len(active) != 1 || active[0] != "cosine" {
			t.Fatalf("active = %v, want [cosine]", active)
		}
		if math.Abs(combined[0]-0.4) > 1e-9 || math.Abs(combined[1]-0.6) > 1e-9 {
			t.Fatalf("combined = %v, want [0.4 0.6]", combined)
		}
	})

	t.Run("no available signals", func(t *testing.T) {
		results := []signalResult{{name: "x", scores: []float64{1, 1}, available: false}}
		combined, active := combineSignals(results, 2)
		if len(active) != 0 {
			t.Fatalf("active = %v, want empty", active)
		}
		for i, c := range combined {
			if c != 0 {
				t.Fatalf("combined[%d] = %v, want 0", i, c)
			}
		}
	})
}

func TestFlagCoherence(t *testing.T) {
	rows := []coherenceRow{
		{fix: "add io.LimitReader guard"},                      // 0
		{fix: "A backend decision owned by the atcr.dev team"}, // 1 (punt)
		{fix: "thread per-chunk membership through runEngine"}, // 2
		{fix: "just do the thing"},                             // 3 (no code id)
		{fix: "call normalizeScope before the lookup"},         // 4
	}

	t.Run("top pct flagged, ranked by suspicion", func(t *testing.T) {
		combined := []float64{0.1, 0.9, 0.2, 0.3, 0.05}
		v := flagCoherence(rows, combined, 20) // ceil(5*0.2)=1 -> only the max
		if !v[1].suspect {
			t.Fatalf("row 1 (max 0.9) should be suspect")
		}
		for _, i := range []int{0, 2, 3, 4} {
			if v[i].suspect {
				t.Fatalf("row %d should not be suspect", i)
			}
		}
		if v[1].tier != "low" { // punt -> low
			t.Fatalf("row 1 tier = %q, want low", v[1].tier)
		}
	})

	t.Run("larger cut flags more, tiers by fix", func(t *testing.T) {
		combined := []float64{0.1, 0.85, 0.2, 0.9, 0.05}
		v := flagCoherence(rows, combined, 40) // ceil(5*0.4)=2 -> rows 3 and 1
		if !v[3].suspect || !v[1].suspect {
			t.Fatalf("rows 3 and 1 should be suspect; got %+v", v)
		}
		if v[3].tier != "low" { // "just do the thing" has no code identifier
			t.Fatalf("row 3 tier = %q, want low", v[3].tier)
		}
	})

	t.Run("high tier for technical fix", func(t *testing.T) {
		combined := []float64{0.99, 0.1, 0.1, 0.1, 0.1}
		v := flagCoherence(rows, combined, 20)
		if !v[0].suspect || v[0].tier != "high" { // has io.LimitReader, not a punt
			t.Fatalf("row 0 should be suspect/high; got suspect=%v tier=%q", v[0].suspect, v[0].tier)
		}
	})

	t.Run("deterministic tie-break by index", func(t *testing.T) {
		combined := []float64{0.5, 0.5, 0.5, 0.5, 0.5}
		v := flagCoherence(rows, combined, 20) // 1 flagged -> lowest index wins
		if !v[0].suspect {
			t.Fatalf("tie should flag lowest index (0); got %+v", v)
		}
	})

	t.Run("pct 0 flags none", func(t *testing.T) {
		combined := []float64{0.9, 0.9, 0.9, 0.9, 0.9}
		v := flagCoherence(rows, combined, 0)
		for i := range v {
			if v[i].suspect {
				t.Fatalf("row %d should not be suspect at pct=0", i)
			}
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		if v := flagCoherence(nil, nil, 10); len(v) != 0 {
			t.Fatalf("empty input should yield empty verdicts, got %v", v)
		}
	})
}

func TestFlagCoherence_Adversarial(t *testing.T) {
	rows := []coherenceRow{{fix: "a"}, {fix: "b"}, {fix: "c"}}

	t.Run("pct over 100 caps at all rows", func(t *testing.T) {
		v := flagCoherence(rows, []float64{0.1, 0.2, 0.3}, 1000)
		for i := range v {
			if !v[i].suspect {
				t.Fatalf("row %d should be suspect when pct>100", i)
			}
		}
	})

	t.Run("single row flagged", func(t *testing.T) {
		v := flagCoherence([]coherenceRow{{fix: "x"}}, []float64{0.5}, 10)
		if !v[0].suspect {
			t.Fatalf("the only row should be flagged")
		}
	})

	t.Run("combined shorter than rows does not panic", func(t *testing.T) {
		v := flagCoherence(rows, []float64{0.9}, 40) // combined len 1, rows len 3
		if len(v) != 3 {
			t.Fatalf("want 3 verdicts, got %d", len(v))
		}
		if v[1].score != 0 || v[2].score != 0 {
			t.Fatalf("missing scores should default to 0; got %+v", v)
		}
	})

	t.Run("combined longer than rows ignored", func(t *testing.T) {
		v := flagCoherence(rows, []float64{0.1, 0.2, 0.3, 0.4, 0.5}, 40)
		if len(v) != 3 {
			t.Fatalf("want 3 verdicts, got %d", len(v))
		}
	})
}

func TestCombineSignals_Adversarial(t *testing.T) {
	t.Run("all signals length-mismatched", func(t *testing.T) {
		results := []signalResult{
			{name: "a", scores: []float64{1}, available: true},
			{name: "b", scores: nil, available: true},
		}
		combined, active := combineSignals(results, 3)
		if len(active) != 0 {
			t.Fatalf("no signal should qualify; active=%v", active)
		}
		for i, c := range combined {
			if c != 0 {
				t.Fatalf("combined[%d]=%v, want 0", i, c)
			}
		}
	})

	t.Run("zero rows", func(t *testing.T) {
		combined, active := combineSignals([]signalResult{{name: "a", scores: nil, available: true}}, 0)
		if len(combined) != 0 || len(active) != 0 {
			t.Fatalf("zero rows should yield empty combined and no active; got %v %v", combined, active)
		}
	})
}

func TestCoherenceTier(t *testing.T) {
	cases := []struct {
		name, fix, want string
	}{
		{"technical", "add io.LimitReader mirroring readCapped", "high"},
		{"punt", "A backend decision owned by the team", "low"},
		{"no code id", "just do the thing carefully", "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coherenceTier(tc.fix); got != tc.want {
				t.Fatalf("coherenceTier(%q) = %q, want %q", tc.fix, got, tc.want)
			}
		})
	}
}
