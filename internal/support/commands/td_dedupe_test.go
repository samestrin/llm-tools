package commands

import (
	"reflect"
	"sort"
	"testing"
)

// Stream A (claude) and B (multi-agent) share an auth.go finding within ±3
// lines (45 vs 47) and each have one distinct finding.
var dedupeStreamA = StreamInput{Tag: "claude", Content: `HIGH|auth.go:45|Missing validation|Add zod schema|security|15|user input unsanitized|bruce
LOW|util.go:10|Unused var|Remove it|maintainability|5|x unused|greta`}

var dedupeStreamB = StreamInput{Tag: "multi-agent", Content: `MEDIUM|auth.go:47|No input validation on userId|Validate userId before query|security|20|userId to query|kai
HIGH|db.go:88|N+1 query in loop|Batch the lookups|performance|30|loop query|mira`}

func mergedByFile(rows []MergedRow) map[string]MergedRow {
	m := map[string]MergedRow{}
	for _, r := range rows {
		m[r.FileLine] = r
	}
	return m
}

func TestDedupeTD_ClustersAndAggregates(t *testing.T) {
	res, err := dedupeTD([]StreamInput{dedupeStreamA, dedupeStreamB}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if res.Summary.InputRows != 4 {
		t.Errorf("input_rows = %d, want 4", res.Summary.InputRows)
	}
	if res.Summary.Clusters != 3 || res.Summary.MergedRows != 3 {
		t.Errorf("clusters/merged = %d/%d, want 3/3", res.Summary.Clusters, res.Summary.MergedRows)
	}
	if res.Summary.NeedsReviewCount != 1 {
		t.Errorf("needs_review_count = %d, want 1", res.Summary.NeedsReviewCount)
	}

	by := mergedByFile(res.Merged)
	// The auth.go cluster (45,47) merged.
	var auth MergedRow
	found := false
	for fl, r := range by {
		if len(fl) >= 7 && fl[:7] == "auth.go" {
			auth = r
			found = true
		}
	}
	if !found {
		t.Fatalf("no merged auth.go row; got %v", by)
	}
	if auth.Severity != "HIGH" {
		t.Errorf("auth severity = %q, want HIGH (max)", auth.Severity)
	}
	revs := splitSorted(auth.Reviewers)
	if !reflect.DeepEqual(revs, []string{"bruce", "kai"}) {
		t.Errorf("auth reviewers = %q, want bruce,kai", auth.Reviewers)
	}
	if auth.Confidence != "HIGH" {
		t.Errorf("auth confidence = %q, want HIGH (2 distinct reviewers)", auth.Confidence)
	}
	if auth.Disagreement == "" {
		t.Errorf("auth should record severity disagreement (MEDIUM vs HIGH)")
	}
	if auth.EstMinutes != 20 {
		t.Errorf("auth est = %v, want 20 (max)", auth.EstMinutes)
	}
	if auth.ClusterSize != 2 || !auth.NeedsReview {
		t.Errorf("auth cluster_size/needs_review = %d/%v, want 2/true", auth.ClusterSize, auth.NeedsReview)
	}
	if len(auth.Members) != 2 {
		t.Errorf("auth members = %d, want 2", len(auth.Members))
	}
}

func TestDedupeTD_SingletonsAreMediumNotFlagged(t *testing.T) {
	res, err := dedupeTD([]StreamInput{dedupeStreamA, dedupeStreamB}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	for _, r := range res.Merged {
		if r.ClusterSize == 1 {
			if r.Confidence != "MEDIUM" {
				t.Errorf("%s singleton confidence = %q, want MEDIUM", r.FileLine, r.Confidence)
			}
			if r.NeedsReview {
				t.Errorf("%s singleton should not need review", r.FileLine)
			}
		}
	}
}

func TestDedupeTD_DuplicateReviewerNotDoubleCounted(t *testing.T) {
	// Same reviewer (bruce) reports the same line twice → 1 distinct reviewer → MEDIUM.
	a := StreamInput{Tag: "claude", Content: `HIGH|auth.go:45|Missing validation|Add zod|security|15|e|bruce`}
	b := StreamInput{Tag: "claude2", Content: `HIGH|auth.go:46|Missing validation here|Add zod schema|security|15|e|bruce`}
	res, err := dedupeTD([]StreamInput{a, b}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if len(res.Merged) != 1 {
		t.Fatalf("want 1 merged cluster, got %d", len(res.Merged))
	}
	r := res.Merged[0]
	if splitSorted(r.Reviewers)[0] != "bruce" || len(splitSorted(r.Reviewers)) != 1 {
		t.Errorf("reviewers = %q, want single bruce", r.Reviewers)
	}
	if r.Confidence != "MEDIUM" {
		t.Errorf("confidence = %q, want MEDIUM (1 distinct reviewer despite 2 rows)", r.Confidence)
	}
	if !r.NeedsReview {
		t.Errorf("2-row cluster should still flag needs_review")
	}
}

func TestDedupeTD_SyntheszeReviewerFromTag(t *testing.T) {
	// 5-col legacy row (no reviewer field) → reviewer synthesized from tag.
	a := StreamInput{Tag: "external", Content: `HIGH|x.go:1|prob|fix|security`}
	res, err := dedupeTD([]StreamInput{a}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if len(res.Merged) != 1 || res.Merged[0].Reviewers != "external" {
		t.Errorf("reviewers = %q, want synthesized 'external'", res.Merged[0].Reviewers)
	}
}

// --- Adversarial ---

// Two findings far apart in the same file (gap > tolerance) must NOT merge.
func TestDedupeTD_FarApartSameFileNotMerged(t *testing.T) {
	a := StreamInput{Tag: "c", Content: `HIGH|big.go:10|issue one|fix|security|5|e|bruce
LOW|big.go:500|issue two|fix|style|5|e|kai`}
	res, err := dedupeTD([]StreamInput{a}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if res.Summary.Clusters != 2 {
		t.Errorf("clusters = %d, want 2 (far-apart lines must not merge)", res.Summary.Clusters)
	}
}

// Untrusted-only cluster → CONFIDENCE LOW.
func TestDedupeTD_UntrustedLow(t *testing.T) {
	a := StreamInput{Tag: "sketchy", Content: `HIGH|x.go:1|p|f|security|5|e|sketchy`}
	res, err := dedupeTD([]StreamInput{a}, DedupeOpts{Tolerance: 3, Untrusted: []string{"sketchy"}})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if res.Merged[0].Confidence != "LOW" {
		t.Errorf("confidence = %q, want LOW (untrusted source)", res.Merged[0].Confidence)
	}
}

// Rows with no parseable line cluster by file only (one cluster), not merged
// with line-bearing rows.
func TestDedupeTD_NoLineRows(t *testing.T) {
	a := StreamInput{Tag: "c", Content: `HIGH|nolinefile|p1|f|security|5|e|bruce
HIGH|nolinefile|p2|f|security|5|e|kai`}
	res, err := dedupeTD([]StreamInput{a}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if res.Summary.Clusters != 1 || res.Merged[0].ClusterSize != 2 {
		t.Errorf("no-line rows for same file should form one cluster; got clusters=%d size=%d",
			res.Summary.Clusters, res.Merged[0].ClusterSize)
	}
}

func TestDedupeTD_EmptyStreams(t *testing.T) {
	res, err := dedupeTD([]StreamInput{{Tag: "a", Content: ""}, {Tag: "b", Content: "# comment only\n"}}, DedupeOpts{Tolerance: 3})
	if err != nil {
		t.Fatalf("dedupeTD: %v", err)
	}
	if res.Summary.InputRows != 0 || len(res.Merged) != 0 {
		t.Errorf("empty streams should yield 0 rows; got %d", res.Summary.InputRows)
	}
	if res.Merged == nil {
		t.Errorf("Merged must be non-nil for JSON []")
	}
}

func splitSorted(csv string) []string {
	out := []string{}
	for _, p := range splitCSV(csv) {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
