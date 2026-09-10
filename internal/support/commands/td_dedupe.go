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

	"github.com/samestrin/llm-tools/internal/support/findings"
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
"same issue?" merge — the only step that needs judgment. Each member keeps
its own severity/problem/fix/category/est/file_line so a split emits one
distinct, correctly-cited finding per member.

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
//
// It carries the member's OWN values, not the cluster's pooled ones, because
// adjudication may SPLIT the cluster into one finding per member. A split row
// needs its own FIX, CATEGORY, EST_MINUTES and FILE:LINE; if those are only
// available pooled, the split can do nothing but paste one cluster-wide FIX
// onto every row and cite the first member's line for all of them. FILE:LINE
// especially: a cluster spans ±tolerance lines and the downstream td_validate
// pass grounds on FILE:LINE, so a wrong citation propagates into validation.
//
// Values are verbatim from the member row (CATEGORY normalized, as for the
// modal vote) — never back-filled from the pooled row. Deciding what an empty
// member field should fall back to is the caller's judgment, not this
// command's; inventing a value here would launder a gap into a fact.
type MemberRef struct {
	Reviewer   string  `json:"reviewer"`
	Severity   string  `json:"severity"`
	FileLine   string  `json:"file_line"`
	Problem    string  `json:"problem"`
	Fix        string  `json:"fix"`
	Category   string  `json:"category"`
	EstMinutes float64 `json:"est_minutes"`
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

	// Warnings names each stream that carried no `# atcr-findings/v1` header and
	// was therefore parsed permissively. Silence is how seven incompatible
	// formats accumulated unnoticed, so a legacy stream is accepted but never
	// accepted QUIETLY.
	Warnings []string `json:"warnings,omitempty"`
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
	var warnings []string
	skipped := 0
	for _, s := range streams {
		cols, warn, err := streamColumns(s)
		if err != nil {
			return nil, err
		}
		if warn != "" {
			warnings = append(warnings, warn)
		}
		for _, line := range strings.Split(s.Content, "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.HasPrefix(t, "#") {
				continue
			}
			// By NAME when the stream declares its columns; by width only when it
			// declares nothing. Width cannot tell three different 8-column
			// layouts apart, which is how a BLOCKING flag became a reviewer.
			var (
				r  tdParsedRow
				ok bool
			)
			if cols != nil {
				r, ok = rowByColumns(t, cols, s.Tag)
			} else {
				r, ok = normalizeRow(t, s.Tag)
			}
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
			Warnings:         warnings,
		},
	}, nil
}

// streamColumns decides how one stream's rows should be read.
//
// It returns the declared column NAMES, a warning when the stream carries no
// version header, or nil names to mean "fall back to width-guessing". A stream
// that declares its columns is read by name, which is the whole point: three of
// the layouts in use are eight columns wide with different columns, and width
// cannot tell them apart.
//
// An unknown or malformed version is fatal, per the v1 contract — a consumer
// must never silently parse incompatible data. A MISSING header is not.
func streamColumns(s StreamInput) ([]string, string, error) {
	res, err := findings.Inspect(strings.NewReader(s.Content))
	if err != nil {
		return nil, "", fmt.Errorf("stream %q: %w", s.Tag, err)
	}
	warn := ""
	if res.Legacy {
		warn = fmt.Sprintf("stream %q: %s", s.Tag, res.Warning)
	}

	// An explicit `# Format:` comment wins over the canonical order: it is what
	// the stream actually claims about ITSELF, and every legacy layout carries
	// one.
	for _, line := range strings.Split(s.Content, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "#") {
			break // past the header block
		}
		body := strings.TrimSpace(strings.TrimPrefix(t, "#"))
		if !strings.HasPrefix(strings.ToUpper(body), "FORMAT:") {
			continue
		}
		spec := strings.TrimSpace(body[len("FORMAT:"):])
		cols := strings.Split(spec, "|")
		for i := range cols {
			cols[i] = strings.ToUpper(strings.TrimSpace(cols[i]))
		}
		if len(cols) >= 5 {
			return cols, warn, nil
		}
	}

	if res.HasHeader {
		return findings.PerSourceNames, warn, nil
	}
	return nil, warn, nil
}

// rowByColumns maps a pipe row onto the canonical fields using the stream's own
// declared column names.
//
// A name the canonical shape has no home for — ORIGIN, RISK_CLASS, BLOCKING —
// is DROPPED rather than shifted into the next field. That is the fix: a
// BLOCKING flag used to land in REVIEWER because the parser counted columns
// instead of reading their names.
func rowByColumns(line string, cols []string, tag string) (tdParsedRow, bool) {
	f := strings.Split(line, "|")
	for i := range f {
		f[i] = strings.TrimSpace(f[i])
	}
	if len(f) < 5 {
		return tdParsedRow{}, false
	}
	r := tdParsedRow{Source: tag}
	for i, name := range cols {
		if i >= len(f) {
			break // short row: the remaining columns are simply absent
		}
		switch name {
		case "SEVERITY":
			r.Severity = strings.ToUpper(f[i])
		case "FILE:LINE", "FILE_LINE", "FILE", "PATH":
			r.FileLine = f[i]
		case "PROBLEM", "DESCRIPTION":
			r.Problem = f[i]
		case "FIX", "RECOMMENDED_FIX":
			r.Fix = f[i]
		case "CATEGORY":
			r.Category = f[i]
		case "EST_MINUTES", "EST":
			r.EstMinutes = parseFloatOr(f[i], 0)
		case "EVIDENCE":
			r.Evidence = f[i]
		case "REVIEWER", "REVIEWERS":
			r.Reviewer = f[i]
		case "SOURCE":
			r.Source = f[i]
		}
	}
	if r.Severity == "" || r.FileLine == "" {
		return tdParsedRow{}, false
	}
	if r.Reviewer == "" {
		r.Reviewer = tag
	}
	return r, true
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
	if len(f) == 10 {
		// 10-col legacy claude has a different column order — FILE:LINE is at
		// position 3, after ORIGIN and RISK_CLASS, and there is no reviewer
		// column: SEV|ORIGIN|RISK_CLASS|FILE:LINE|PROBLEM|FIX|CATEGORY|EST|EVIDENCE|SOURCE.
		r.Severity = strings.ToUpper(f[0])
		r.FileLine = f[3]
		r.Problem = f[4]
		r.Fix = f[5]
		r.Category = f[6]
		r.EstMinutes = parseFloatOr(f[7], 0)
		r.Evidence = f[8]
		r.Reviewer = tag // synthesized — legacy claude rows carry no reviewer
		return r, true
	}
	// 5/6/8-col widths share the leading columns: sev|file_line|problem|fix|category.
	r.Severity = strings.ToUpper(f[0])
	r.FileLine = f[1]
	r.Problem = f[2]
	r.Fix = f[3]
	r.Category = f[4]
	switch {
	case len(f) >= 8:
		// 8-col (primary): ...|cat|est|evidence|reviewer
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

// clusterTextSeparator joins distinct member text inside one pooled cell. It must
// never be "|" (the stream field delimiter) and must survive a markdown table cell.
const clusterTextSeparator = " // "

// combineClusterText pools member PROBLEM/FIX values losslessly.
//
// This replaced a longest-wins pick. Longest-wins is safe only when members are
// restatements of one another; when two reviewers propose DIFFERENT remedies for
// the same issue, it silently discarded one. That loss is invisible downstream —
// the merged row still looks complete — and it lands in the TD README, where the
// discarded remedy is the thing someone would have acted on.
//
// Members that add nothing are still collapsed: a value already contained in a
// kept value (or equal to one, ignoring case and surrounding space) is dropped,
// so genuine duplicates do not inflate the cell. Order follows cluster order,
// which is line-sorted, so the result is deterministic.
func combineClusterText(vals []string) string {
	var kept []string
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		redundant := false
		for i, k := range kept {
			switch {
			case strings.Contains(k, v) || strings.EqualFold(k, v):
				redundant = true
			case strings.Contains(v, k):
				// v subsumes an earlier value: keep the longer, drop the shorter.
				kept[i] = v
				redundant = true
			}
			if redundant {
				break
			}
		}
		if !redundant {
			kept = append(kept, v)
		}
	}
	return strings.Join(kept, clusterTextSeparator)
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
	var problems, fixes []string
	for _, r := range c {
		if r.Reviewer != "" && !seenRev[r.Reviewer] {
			seenRev[r.Reviewer] = true
			revs = append(revs, r.Reviewer)
		}
		if !untrusted[r.Source] {
			allUntrusted = false
		}
		// Count the NORMALIZED value: raw counting splits the modal vote between
		// spellings of one concept and lets the first-seen tiebreak below resolve
		// on stream order rather than consensus.
		if cat := normalizeCategory(r.Category); cat != "" {
			if catCount[cat] == 0 {
				catOrder = append(catOrder, cat)
			}
			catCount[cat]++
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
		problems = append(problems, r.Problem)
		fixes = append(fixes, r.Fix)
	}
	out.Reviewers = strings.Join(revs, ",")
	out.Problem = combineClusterText(problems)
	out.Fix = combineClusterText(fixes)

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
			out.Members = append(out.Members, MemberRef{
				Reviewer:   r.Reviewer,
				Severity:   r.Severity,
				FileLine:   r.FileLine,
				Problem:    r.Problem,
				Fix:        r.Fix,
				Category:   normalizeCategory(r.Category),
				EstMinutes: r.EstMinutes,
			})
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

// categoryAliases folds spellings that mean the same thing onto one value. It is
// deliberately SMALL and evidence-driven — only observed typos and unambiguous
// synonyms. Aliasing two genuinely different concepts together would silently
// destroy information, which is worse than the vote-splitting it fixes, so a new
// entry belongs here only when the two spellings cannot mean different things.
var categoryAliases = map[string]string{
	"maintainancy":  "maintainability", // observed typo in the wild
	"maintainence":  "maintainability", // same typo, other common spelling
	"perf":          "performance",
	"documentation": "docs",
	"doc":           "docs",
	"test":          "testing",
	"tests":         "testing",
	// Deliberately NOT aliased: "sec" (seconds or security?), "spec"
	// (specification or spec-test?). An abbreviation with two live readings must
	// stay distinct — guessing wrong silently relabels the finding.
}

// normalizeCategory canonicalizes a reviewer-authored CATEGORY value so that
// spellings of the same concept compare equal.
//
// CATEGORY is free text: each reviewer writes its own value into its stream and
// aggregateCluster takes the modal one. Counting raw strings splits the vote
// between casing variants ("performance" vs "PERFORMANCE") and separator styles
// ("ERROR_HANDLING" vs "error-handling"), and the first-seen tiebreak then lets
// STREAM ORDER pick the winner instead of reviewer consensus — the exact
// judgment-free-merge property this command exists to provide.
//
// Normalization is lossy only in ways that carry no meaning: case, surrounding
// whitespace, and _ vs - as a word separator. An unrecognized category is
// normalized but never dropped or invented away.
func normalizeCategory(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	if alias, ok := categoryAliases[s]; ok {
		return alias
	}
	return s
}
