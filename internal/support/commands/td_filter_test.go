package commands

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// fixtureTDReadme has two dated sections, mixed checkbox states, an 11-column
// (Source/Reviewers/Confidence) section and an 8-column legacy section.
//
// Unchecked rows (the only ones td-filter ever selects):
//
//	a.go:10  HIGH      15   grp 1     conf HIGH    (section alpha)
//	c.go:30  LOW       45   grp 2     conf LOW     (section alpha)
//	e.go:50  CRITICAL  20   grp solo  conf "" legacy (section beta)
//	f.go:60  MEDIUM    90   grp 1     conf "" legacy (section beta)
//
// b.go (checked [x]) and d.go (deferred [/]) are never selected.
const fixtureTDReadme = `# Technical Debt

### [2026-06-01] From Sprint: 1.0_alpha

| Group | | Severity | File | Problem | Fix | Category | Est Minutes | Source | Reviewers | Confidence |
|-------|---|----------|------|---------|-----|----------|-------------|--------|-----------|------------|
| 1 | [ ] | HIGH | a.go:10 | prob a | fix a | security | 15 | code-review | bruce | HIGH |
| 1 | [x] | LOW | b.go:20 | done | fixed | performance | 10 | code-review | greta | MEDIUM |
| 2 | [ ] | LOW | c.go:30 | prob c | fix c | performance | 45 | code-review | kai | LOW |
| 2 | [/] | MEDIUM | d.go:40 | deferred | later | maintainability | 60 | code-review | mira | HIGH |

### [2026-06-02] From Sprint: 2.0_beta

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| solo | [ ] | CRITICAL | e.go:50 | prob e | fix e | security | 20 |
| 1 | [ ] | MEDIUM | f.go:60 | prob f | fix f | maintainability | 90 |
`

func fileLinesOf(items []TDFilterRow) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.FileLine
	}
	return out
}

func runFilter(t *testing.T, opts TDFilterOpts) *TDFilterResult {
	t.Helper()
	res, err := filterTD(fixtureTDReadme, opts)
	if err != nil {
		t.Fatalf("filterTD: %v", err)
	}
	return res
}

func TestFilterTD_QuickWins(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "quick-wins", Max: 10})
	got := fileLinesOf(res.Items)
	want := []string{"a.go:10", "e.go:50"} // est < 30
	if !reflect.DeepEqual(got, want) {
		t.Errorf("quick-wins items = %v, want %v", got, want)
	}
	if res.Summary.TotalUnchecked != 4 {
		t.Errorf("total_unchecked = %d, want 4", res.Summary.TotalUnchecked)
	}
}

func TestFilterTD_Backlog(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "backlog", Max: 10})
	want := []string{"c.go:30", "f.go:60"} // 30 <= est < 2880
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, want) {
		t.Errorf("backlog items = %v, want %v", got, want)
	}
}

func TestFilterTD_All(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Max: 10})
	want := []string{"a.go:10", "c.go:30", "e.go:50", "f.go:60"}
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, want) {
		t.Errorf("all items = %v, want %v", got, want)
	}
}

func TestFilterTD_Severity(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Severity: []string{"HIGH"}, Max: 10})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"a.go:10"}) {
		t.Errorf("severity=HIGH items = %v, want [a.go:10]", got)
	}
	if res.Summary.ExcludedBySeverity != 3 {
		t.Errorf("excluded_by_severity = %d, want 3", res.Summary.ExcludedBySeverity)
	}
}

func TestFilterTD_Confidence_ExcludesEmpty(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Confidence: []string{"HIGH"}, Max: 10})
	// Only a.go has conf HIGH. c.go is LOW (filtered); e/f have empty conf (legacy, excluded).
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"a.go:10"}) {
		t.Errorf("confidence=HIGH items = %v, want [a.go:10]", got)
	}
	if res.Summary.ExcludedByConfidence != 3 {
		t.Errorf("excluded_by_confidence = %d, want 3", res.Summary.ExcludedByConfidence)
	}
}

func TestFilterTD_Group_AndScope(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Group: "1", Max: 10})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"a.go:10", "f.go:60"}) {
		t.Errorf("group=1 items = %v, want [a.go:10 f.go:60]", got)
	}
	if res.Summary.ExcludedByGroup != 2 {
		t.Errorf("excluded_by_group = %d, want 2 (c grp2, e solo)", res.Summary.ExcludedByGroup)
	}
	scope := append([]string{}, res.Summary.GroupScope...)
	sort.Strings(scope)
	if !reflect.DeepEqual(scope, []string{"a.go", "f.go"}) {
		t.Errorf("group_scope = %v, want [a.go f.go]", scope)
	}
}

func TestFilterTD_GroupCaseInsensitive(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "quick-wins", Group: "SOLO", Max: 10})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"e.go:50"}) {
		t.Errorf("group=SOLO items = %v, want [e.go:50]", got)
	}
}

func TestFilterTD_Focus(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Focus: "beta", Max: 10})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"e.go:50", "f.go:60"}) {
		t.Errorf("focus=beta items = %v, want [e.go:50 f.go:60]", got)
	}
	if res.Summary.FocusMatchedSections != 1 {
		t.Errorf("focus_matched_sections = %d, want 1", res.Summary.FocusMatchedSections)
	}
	if res.Summary.TotalUnchecked != 2 {
		t.Errorf("total_unchecked (focus beta) = %d, want 2", res.Summary.TotalUnchecked)
	}
}

func TestFilterTD_Max_FirstNInSourceOrder(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Max: 1})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"a.go:10"}) {
		t.Errorf("max=1 items = %v, want [a.go:10]", got)
	}
	if res.Summary.Matched != 4 {
		t.Errorf("matched should reflect pre-max count 4, got %d", res.Summary.Matched)
	}
}

func TestFilterTD_GroupScope_ComputedPreMax(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Group: "1", Max: 1})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"a.go:10"}) {
		t.Errorf("group=1 max=1 items = %v, want [a.go:10]", got)
	}
	scope := append([]string{}, res.Summary.GroupScope...)
	sort.Strings(scope)
	// group_scope is the union of group-1 files BEFORE --max truncation.
	if !reflect.DeepEqual(scope, []string{"a.go", "f.go"}) {
		t.Errorf("group_scope (pre-max) = %v, want [a.go f.go]", scope)
	}
}

// --- Adversarial ---

func TestFilterTD_SeverityCaseInsensitive(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Severity: []string{"high"}, Max: 10})
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"a.go:10"}) {
		t.Errorf("lowercase severity input should match; got %v", got)
	}
}

func TestFilterTD_FocusNoMatch(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Focus: "nonexistent", Max: 10})
	if len(res.Items) != 0 {
		t.Errorf("focus matching nothing should yield 0 items, got %v", fileLinesOf(res.Items))
	}
	if res.Summary.FocusMatchedSections != 0 || res.Summary.TotalUnchecked != 0 {
		t.Errorf("focus no-match: matched_sections=%d total_unchecked=%d, want 0/0",
			res.Summary.FocusMatchedSections, res.Summary.TotalUnchecked)
	}
}

func TestFilterTD_GroupNoMatch(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Group: "99", Max: 10})
	if len(res.Items) != 0 {
		t.Errorf("group with no match should yield 0 items")
	}
	if res.Summary.ExcludedByGroup != 4 {
		t.Errorf("excluded_by_group = %d, want 4", res.Summary.ExcludedByGroup)
	}
}

func TestFilterTD_EveryRowExcluded(t *testing.T) {
	res := runFilter(t, TDFilterOpts{Mode: "all", Severity: []string{"NONESUCH"}, Max: 10})
	if len(res.Items) != 0 {
		t.Errorf("impossible severity should yield 0 items")
	}
	if res.Summary.Matched != 0 || res.Summary.ExcludedBySeverity != 4 {
		t.Errorf("matched=%d excluded_by_severity=%d, want 0/4", res.Summary.Matched, res.Summary.ExcludedBySeverity)
	}
}

func TestFilterTD_MalformedEstReported(t *testing.T) {
	content := `### [2026-06-01] From Sprint: x

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | HIGH | good.go:1 | p | f | bug | 10 |
| 1 | [ ] | HIGH | bad.go:2 | p | f | bug | not-a-number |
`
	res, err := filterTD(content, TDFilterOpts{Mode: "all", Max: 10})
	if err != nil {
		t.Fatalf("filterTD: %v", err)
	}
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"good.go:1"}) {
		t.Errorf("malformed row must be excluded from items; got %v", got)
	}
	if !reflect.DeepEqual(res.Summary.Malformed, []string{"bad.go:2"}) {
		t.Errorf("malformed row must be reported; got %v", res.Summary.Malformed)
	}
}

func TestFilterTD_PathologicalTables(t *testing.T) {
	for name, content := range map[string]string{
		"empty":          "",
		"no-sections":    "# Technical Debt\n\nNothing here.\n",
		"all-checked":    "### [2026-06-01] From Sprint: x\n\n| Group | | Severity | File | Problem | Fix | Category | Est Minutes |\n|--|--|--|--|--|--|--|--|\n| 1 | [x] | HIGH | a.go:1 | p | f | bug | 10 |\n",
		"header-no-rows": "### [2026-06-01] From Sprint: x\n\n| Group | | Severity | File | Problem | Fix | Category | Est Minutes |\n|--|--|--|--|--|--|--|--|\n",
	} {
		res, err := filterTD(content, TDFilterOpts{Mode: "all", Max: 10})
		if err != nil {
			t.Fatalf("%s: filterTD: %v", name, err)
		}
		if len(res.Items) != 0 || res.Summary.TotalUnchecked != 0 {
			t.Errorf("%s: want 0 items / 0 unchecked, got %d / %d", name, len(res.Items), res.Summary.TotalUnchecked)
		}
		if res.Items == nil {
			t.Errorf("%s: Items must be non-nil for JSON []", name)
		}
	}
}

// A table under a non-"From" ### section must NOT be collected (mis-attribution
// guard) — only From-sections hold TD items.
func TestFilterTD_NonFromSectionTableIgnored(t *testing.T) {
	content := `### Directory Structure

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|--|--|--|--|--|--|--|--|
| 1 | [ ] | HIGH | tree.go:1 | p | f | bug | 5 |

### [2026-06-01] From Sprint: real

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|--|--|--|--|--|--|--|--|
| 1 | [ ] | HIGH | real.go:1 | p | f | bug | 5 |
`
	res, err := filterTD(content, TDFilterOpts{Mode: "all", Max: 10})
	if err != nil {
		t.Fatalf("filterTD: %v", err)
	}
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"real.go:1"}) {
		t.Errorf("only From-section rows collected; got %v", got)
	}
}

// A heading that merely contains the word "From" but lacks the [date] bracket
// is NOT a TD section — its table must be ignored.
func TestFilterTD_NonDatedFromHeadingIgnored(t *testing.T) {
	content := `### Random notes From the field

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|--|--|--|--|--|--|--|--|
| 1 | [ ] | HIGH | noise.go:1 | p | f | bug | 5 |

### [2026-06-01] From Sprint: real

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|--|--|--|--|--|--|--|--|
| 1 | [ ] | HIGH | real.go:1 | p | f | bug | 5 |
`
	res, err := filterTD(content, TDFilterOpts{Mode: "all", Max: 10})
	if err != nil {
		t.Fatalf("filterTD: %v", err)
	}
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, []string{"real.go:1"}) {
		t.Errorf("only dated From-section rows collected; got %v", got)
	}
}

func TestFilterTD_InvalidMode(t *testing.T) {
	if _, err := filterTD(fixtureTDReadme, TDFilterOpts{Mode: "bogus", Max: 10}); err == nil {
		t.Fatal("invalid mode must error")
	}
}

// ===== ATTEMPTS column (index 11) =====
//
// ATTEMPTS records how many times /resolve-td has SELECTED a row and failed to
// fix it. It is deliberately not a count of how often a reviewer re-reported the
// finding: a row filtered out before selection (by tier ceiling, severity, max,
// or mode) has proven nothing, and escalating it would create planning work for
// an item nobody ever attempted.

// fixtureTDAttempts has a 12-column section carrying ATTEMPTS, plus the legacy
// 8-column section from the main fixture so absent-column behaviour is covered.
//
// Unchecked rows:
//
//	g.go:70  HIGH  20  attempts 0
//	h.go:80  HIGH  20  attempts 3   (exhausted — escalation candidate)
//	i.go:90  HIGH  20  attempts 5
//	j.go:99  HIGH  20  attempts ""  (malformed → 0)
//	k.go:11  HIGH  20  (8-column legacy row, no ATTEMPTS column at all → 0)
const fixtureTDAttempts = `# Technical Debt

### [2026-08-01] From Sprint: 3.0_gamma

| Group | | Severity | File | Problem | Fix | Category | Est Minutes | Source | Reviewers | Confidence | Attempts |
|-------|---|----------|------|---------|-----|----------|-------------|--------|-----------|------------|----------|
| 1 | [ ] | HIGH | g.go:70 | prob g | fix g | correctness | 20 | post | claude | HIGH | 0 |
| 1 | [ ] | HIGH | h.go:80 | prob h | fix h | correctness | 20 | post | claude | HIGH | 3 |
| 1 | [ ] | HIGH | i.go:90 | prob i | fix i | correctness | 20 | post | claude | HIGH | 5 |
| 1 | [ ] | HIGH | j.go:99 | prob j | fix j | correctness | 20 | post | claude | HIGH |  |

### [2026-08-02] From Sprint: 3.1_delta

| Group | | Severity | File | Problem | Fix | Category | Est Minutes |
|-------|---|----------|------|---------|-----|----------|-------------|
| 1 | [ ] | HIGH | k.go:11 | prob k | fix k | correctness | 20 |
`

func runAttemptsFilter(t *testing.T, opts TDFilterOpts) *TDFilterResult {
	t.Helper()
	res, err := filterTD(fixtureTDAttempts, opts)
	if err != nil {
		t.Fatalf("filterTD: %v", err)
	}
	return res
}

func attemptsOf(items []TDFilterRow) map[string]int {
	out := map[string]int{}
	for _, it := range items {
		out[it.FileLine] = it.Attempts
	}
	return out
}

// AC1: a row with no ATTEMPTS column, or an unparseable one, reads as 0 —
// never as an error and never as a skipped row.
func TestFilterTD_Attempts_AbsentOrMalformedIsZero(t *testing.T) {
	res := runAttemptsFilter(t, TDFilterOpts{Mode: "all", Max: 100})
	got := attemptsOf(res.Items)

	if got["k.go:11"] != 0 {
		t.Errorf("8-column legacy row attempts = %d, want 0", got["k.go:11"])
	}
	if got["j.go:99"] != 0 {
		t.Errorf("blank attempts cell = %d, want 0", got["j.go:99"])
	}
	// The legacy row must still be SELECTED, not dropped for being short.
	if _, ok := got["k.go:11"]; !ok {
		t.Error("legacy 8-column row was dropped; adding a column must not break older rows")
	}
}

func TestFilterTD_Attempts_ParsedFromColumn(t *testing.T) {
	res := runAttemptsFilter(t, TDFilterOpts{Mode: "all", Max: 100})
	got := attemptsOf(res.Items)
	for file, want := range map[string]int{"g.go:70": 0, "h.go:80": 3, "i.go:90": 5} {
		if got[file] != want {
			t.Errorf("%s attempts = %d, want %d", file, got[file], want)
		}
	}
}

// AC2: --min-attempts selects exhausted rows (the escalation candidates).
func TestFilterTD_MinAttempts(t *testing.T) {
	res := runAttemptsFilter(t, TDFilterOpts{Mode: "all", Max: 100, MinAttempts: 3})
	want := []string{"h.go:80", "i.go:90"}
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, want) {
		t.Errorf("min-attempts=3 items = %v, want %v", got, want)
	}
	if res.Summary.ExcludedByAttempts != 3 {
		t.Errorf("excluded_by_attempts = %d, want 3", res.Summary.ExcludedByAttempts)
	}
}

// AC2: --max-attempts excludes rows that have already burned their attempts,
// so a weak-tier resolve-td run stops re-attempting known-hard items.
func TestFilterTD_MaxAttempts(t *testing.T) {
	res := runAttemptsFilter(t, TDFilterOpts{Mode: "all", Max: 100, MaxAttempts: 2})
	want := []string{"g.go:70", "j.go:99", "k.go:11"}
	got := fileLinesOf(res.Items)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("max-attempts=2 items = %v, want %v", got, want)
	}
}

// AC2: callers passing neither flag must see byte-identical behaviour to before
// the column existed. MaxAttempts=0 must mean "no ceiling", not "only zero".
func TestFilterTD_Attempts_UnsetFlagsChangeNothing(t *testing.T) {
	res := runAttemptsFilter(t, TDFilterOpts{Mode: "all", Max: 100})
	want := []string{"g.go:70", "h.go:80", "i.go:90", "j.go:99", "k.go:11"}
	got := fileLinesOf(res.Items)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no attempt flags = %v, want all %v", got, want)
	}
	if res.Summary.ExcludedByAttempts != 0 {
		t.Errorf("excluded_by_attempts = %d, want 0 when no flag set", res.Summary.ExcludedByAttempts)
	}
}

// Both bounds together select an exact band.
func TestFilterTD_MinAndMaxAttempts(t *testing.T) {
	res := runAttemptsFilter(t, TDFilterOpts{Mode: "all", Max: 100, MinAttempts: 3, MaxAttempts: 3})
	want := []string{"h.go:80"}
	if got := fileLinesOf(res.Items); !reflect.DeepEqual(got, want) {
		t.Errorf("attempts band [3,3] = %v, want %v", got, want)
	}
}

// Inverted bounds are a caller error, not an empty result set. Silently
// returning nothing would read as "no rows need escalation" — the same
// failure shape as an inverted tier config, which is worse than an error
// because it looks like a clean run.
func TestFilterTD_Attempts_InvertedBoundsError(t *testing.T) {
	_, err := filterTD(fixtureTDAttempts, TDFilterOpts{Mode: "all", Max: 100, MinAttempts: 5, MaxAttempts: 2})
	if err == nil {
		t.Fatal("min-attempts > max-attempts returned no error; an impossible band must not look like an empty result")
	}
	if !strings.Contains(err.Error(), "min-attempts") {
		t.Errorf("error should name the offending flags, got: %v", err)
	}
}
