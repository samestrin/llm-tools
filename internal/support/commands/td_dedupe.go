package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	ddStreams    string
	ddSourceTags string
	ddTolerance  int
	ddUntrusted  string
	ddJSON       bool
	ddMin        bool
)

func newTdDedupeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "td-dedupe",
		Short: "Cluster and merge technical-debt streams from multiple reviewers",
		Long: `Parse N td-stream.txt files, cluster findings by (file, line +/- tolerance),
and merge each cluster deterministically: REVIEWERS union, SEVERITY max,
CATEGORY modal, EST_MINUTES max, CONFIDENCE (HIGH for 2+ distinct reviewers),
and a severity-disagreement annotation. Multi-item clusters are flagged
needs_review with their members so the model can confirm or split the
"same issue?" merge — the only step that needs judgment.

Output is JSON: {merged:[...], summary:{...}}.`,
		RunE: runTdDedupe,
	}
	cmd.Flags().StringVar(&ddStreams, "streams", "", "Comma-separated td-stream.txt paths (required)")
	cmd.Flags().StringVar(&ddSourceTags, "source-tags", "", "Comma-separated source labels parallel to --streams (default: parent dir name)")
	cmd.Flags().IntVar(&ddTolerance, "tolerance", 3, "Line-proximity window for clustering same-file findings")
	cmd.Flags().StringVar(&ddUntrusted, "untrusted", "", "Comma-separated source tags whose findings alone yield CONFIDENCE LOW")
	cmd.Flags().BoolVar(&ddJSON, "json", true, "Output as JSON (default true)")
	cmd.Flags().BoolVar(&ddMin, "min", false, "Minimal output format")
	cmd.MarkFlagRequired("streams")
	return cmd
}

func runTdDedupe(cmd *cobra.Command, _ []string) error {
	paths := splitCSV(ddStreams)
	if len(paths) == 0 {
		return fmt.Errorf("--streams required (comma-separated td-stream paths)")
	}
	tags := splitCSV(ddSourceTags)
	streams := make([]StreamInput, 0, len(paths))
	for i, p := range paths {
		content, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("failed to read stream %s: %w", p, err)
		}
		tag := ""
		if i < len(tags) {
			tag = tags[i]
		}
		if tag == "" {
			tag = filepath.Base(filepath.Dir(p)) // parent dir name
		}
		streams = append(streams, StreamInput{Tag: tag, Content: string(content)})
	}
	result, err := dedupeTD(streams, DedupeOpts{Tolerance: ddTolerance, Untrusted: splitCSV(ddUntrusted)})
	if err != nil {
		return err
	}
	formatter := output.New(ddJSON, ddMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(*DedupeResult)
		fmt.Fprintf(w, "%d finding(s) → %d merged (%d need review)\n", r.Summary.InputRows, r.Summary.MergedRows, r.Summary.NeedsReviewCount)
	})
}

func init() {
	RootCmd.AddCommand(newTdDedupeCmd())
}

// StreamInput is one source's td-stream content plus its tag (source label and
// reviewer fallback).
type StreamInput struct {
	Tag     string
	Content string
}

// tdParsedRow is a normalized finding from a td-stream (any legacy width).
type tdParsedRow struct {
	Severity   string
	FileLine   string
	Problem    string
	Fix        string
	Category   string
	EstMinutes float64
	Evidence   string
	Reviewer   string
	Source     string
}

// MemberRef is one finding inside a multi-item cluster (for model adjudication).
type MemberRef struct {
	Reviewer string `json:"reviewer"`
	Severity string `json:"severity"`
	Problem  string `json:"problem"`
}

// MergedRow is one deduped/aggregated finding.
type MergedRow struct {
	Severity     string      `json:"severity"`
	FileLine     string      `json:"file_line"`
	Problem      string      `json:"problem"`
	Fix          string      `json:"fix"`
	Category     string      `json:"category"`
	EstMinutes   float64     `json:"est_minutes"`
	Source       string      `json:"source"`
	Reviewers    string      `json:"reviewers"`
	Confidence   string      `json:"confidence"`
	Disagreement string      `json:"disagreement,omitempty"`
	ClusterSize  int         `json:"cluster_size"`
	NeedsReview  bool        `json:"needs_review"`
	Members      []MemberRef `json:"members,omitempty"`
}

// DedupeOpts configures clustering.
type DedupeOpts struct {
	Tolerance int      // line-proximity window (default 3)
	Untrusted []string // tags whose findings alone yield CONFIDENCE LOW
}

// DedupeSummary reports counts.
type DedupeSummary struct {
	InputRows        int `json:"input_rows"`
	Sources          int `json:"sources"`
	Clusters         int `json:"clusters"`
	MergedRows       int `json:"merged_rows"`
	NeedsReviewCount int `json:"needs_review_count"`
	Skipped          int `json:"skipped"` // malformed rows (<5 columns) — surfaced, not silently dropped
}

// DedupeResult is the full payload.
type DedupeResult struct {
	Merged  []MergedRow   `json:"merged"`
	Summary DedupeSummary `json:"summary"`
}

var severityRank = map[string]int{"CRITICAL": 4, "HIGH": 3, "MEDIUM": 2, "LOW": 1}

// dedupeTD parses the streams, clusters findings by (file, line ±tolerance),
// and aggregates each cluster. Multi-item clusters are flagged needs_review so
// the model can confirm or split the "same issue?" merge — the only semantic
// step. Everything here is deterministic.
func dedupeTD(streams []StreamInput, opts DedupeOpts) (*DedupeResult, error) {
	tol := opts.Tolerance
	if tol < 0 {
		tol = 0
	}
	untrusted := map[string]bool{}
	for _, t := range opts.Untrusted {
		untrusted[strings.TrimSpace(t)] = true
	}

	var rows []tdParsedRow
	skipped := 0
	for _, s := range streams {
		for _, line := range strings.Split(s.Content, "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			r, ok := normalizeRow(t, s.Tag)
			if !ok {
				skipped++ // malformed (<5 columns): surfaced in summary, not silently dropped
				continue
			}
			rows = append(rows, r)
		}
	}

	// Cluster: bucket by file path; within a bucket, single-link by sorted line
	// gap <= tol. Rows with no parseable line form a per-file "no-line" cluster.
	buckets := map[string][]tdParsedRow{}
	for _, r := range rows {
		buckets[filePathOf(r.FileLine)] = append(buckets[filePathOf(r.FileLine)], r)
	}

	var clusters [][]tdParsedRow
	for _, brows := range buckets {
		withLine := make([]tdParsedRow, 0, len(brows))
		var noLine []tdParsedRow
		for _, r := range brows {
			if lineOf(r.FileLine) < 0 {
				noLine = append(noLine, r)
			} else {
				withLine = append(withLine, r)
			}
		}
		sort.SliceStable(withLine, func(i, j int) bool { return lineOf(withLine[i].FileLine) < lineOf(withLine[j].FileLine) })
		var cur []tdParsedRow
		prev := -1 << 30
		for _, r := range withLine {
			ln := lineOf(r.FileLine)
			if len(cur) > 0 && ln-prev > tol {
				clusters = append(clusters, cur)
				cur = nil
			}
			cur = append(cur, r)
			prev = ln
		}
		if len(cur) > 0 {
			clusters = append(clusters, cur)
		}
		if len(noLine) > 0 {
			clusters = append(clusters, noLine)
		}
	}

	// Deterministic order: by file then line of the first member.
	sort.SliceStable(clusters, func(i, j int) bool {
		fi, fj := filePathOf(clusters[i][0].FileLine), filePathOf(clusters[j][0].FileLine)
		if fi != fj {
			return fi < fj
		}
		return lineOf(clusters[i][0].FileLine) < lineOf(clusters[j][0].FileLine)
	})

	merged := make([]MergedRow, 0, len(clusters))
	needsReview := 0
	totalInCluster := 0
	for _, c := range clusters {
		totalInCluster += len(c)
		merged = append(merged, aggregateCluster(c, untrusted))
		if len(c) >= 2 {
			needsReview++
		}
	}

	if totalInCluster != len(rows) {
		return nil, fmt.Errorf("FATAL: dedupe accounting mismatch: rows=%d clustered=%d", len(rows), totalInCluster)
	}

	return &DedupeResult{
		Merged: merged,
		Summary: DedupeSummary{
			InputRows:        len(rows),
			Sources:          len(streams),
			Clusters:         len(clusters),
			MergedRows:       len(merged),
			NeedsReviewCount: needsReview,
			Skipped:          skipped,
		},
	}, nil
}

// normalizeRow maps a pipe row of any documented width onto the canonical
// fields, synthesizing the reviewer from the source tag when absent.
func normalizeRow(line, tag string) (tdParsedRow, bool) {
	f := strings.Split(line, "|")
	for i := range f {
		f[i] = strings.TrimSpace(f[i])
	}
	if len(f) < 5 {
		return tdParsedRow{}, false
	}
	r := tdParsedRow{Source: tag}
	// Columns 0..4 are stable across all widths.
	r.Severity = strings.ToUpper(f[0])
	r.FileLine = f[1]
	r.Problem = f[2]
	r.Fix = f[3]
	r.Category = f[4]
	switch {
	case len(f) >= 8:
		// 8-col (primary) / 10-col legacy: sev|fl|prob|fix|cat|est|evidence|reviewer[|...]
		r.EstMinutes = parseFloatOr(f[5], 0)
		r.Evidence = f[6]
		r.Reviewer = f[7]
	case len(f) == 6:
		// 6-col legacy: ...|cat|reviewer
		r.Reviewer = f[5]
	}
	if r.Reviewer == "" {
		r.Reviewer = tag
	}
	return r, true
}

// aggregateCluster merges a cluster deterministically.
func aggregateCluster(c []tdParsedRow, untrusted map[string]bool) MergedRow {
	out := MergedRow{ClusterSize: len(c), NeedsReview: len(c) >= 2}

	// Reviewers: dedup union preserving first-seen order; distinct count for confidence.
	seenRev := map[string]bool{}
	var revs []string
	allUntrusted := true
	catCount := map[string]int{}
	var catOrder []string
	maxRank := 0
	minRank := 1 << 30
	for _, r := range c {
		if r.Reviewer != "" && !seenRev[r.Reviewer] {
			seenRev[r.Reviewer] = true
			revs = append(revs, r.Reviewer)
		}
		if !untrusted[r.Source] {
			allUntrusted = false
		}
		if r.Category != "" {
			if catCount[r.Category] == 0 {
				catOrder = append(catOrder, r.Category)
			}
			catCount[r.Category]++
		}
		if rk := severityRank[r.Severity]; rk > maxRank {
			maxRank = rk
		}
		if rk := severityRank[r.Severity]; rk > 0 && rk < minRank {
			minRank = rk
		}
		if r.EstMinutes > out.EstMinutes {
			out.EstMinutes = r.EstMinutes
		}
		if len(r.Problem) > len(out.Problem) {
			out.Problem = r.Problem
		}
		if len(r.Fix) > len(out.Fix) {
			out.Fix = r.Fix
		}
	}
	out.Reviewers = strings.Join(revs, ",")

	// Severity: max; disagreement when min != max.
	out.Severity = rankSeverity(maxRank)
	if minRank != (1<<30) && minRank != maxRank {
		out.Disagreement = fmt.Sprintf("%s vs %s", rankSeverity(minRank), rankSeverity(maxRank))
	}

	// Category: modal, first-seen tiebreak.
	bestCat, bestN := "", 0
	for _, cat := range catOrder {
		if catCount[cat] > bestN {
			bestN = catCount[cat]
			bestCat = cat
		}
	}
	out.Category = bestCat

	// FILE:LINE and Source from the first member (lowest line after sort).
	out.FileLine = c[0].FileLine
	out.Source = c[0].Source

	// Confidence: HIGH if >=2 distinct reviewers; LOW if all sources untrusted; else MEDIUM.
	switch {
	case len(revs) >= 2:
		out.Confidence = "HIGH"
	case allUntrusted && len(c) > 0:
		out.Confidence = "LOW"
	default:
		out.Confidence = "MEDIUM"
	}

	if out.NeedsReview {
		for _, r := range c {
			out.Members = append(out.Members, MemberRef{Reviewer: r.Reviewer, Severity: r.Severity, Problem: r.Problem})
		}
	}
	return out
}

func rankSeverity(rank int) string {
	for s, r := range severityRank {
		if r == rank {
			return s
		}
	}
	return ""
}

// lineOf returns the line number in a FILE:LINE value, or -1 if absent/unparseable.
func lineOf(fileLine string) int {
	i := strings.LastIndexByte(fileLine, ':')
	if i < 0 {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSpace(fileLine[i+1:]))
	if err != nil {
		return -1
	}
	return n
}

func parseFloatOr(s string, def float64) float64 {
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return f
	}
	return def
}
