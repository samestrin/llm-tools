package commands

import (
	"reflect"
	"sort"
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
