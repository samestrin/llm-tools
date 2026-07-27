package commands

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// coherence.go implements the optional `td-validate --coherence` check, which
// flags TD rows whose FIX cell reads as incoherent with its PROBLEM cell (the
// signature of a copy-pasted-from-an-unrelated-finding fix). The check is
// advisory: it ranks rows by an ensemble of graceful-degrading signals and
// never changes the file/symbol validation result or the process exit code.

// --- pure helpers ---

var (
	reCohBacktick = regexp.MustCompile("`([^`]+)`")
	reCohCamel    = regexp.MustCompile(`\b[a-z]+(?:[A-Z][a-zA-Z0-9]+)+\b`)         // aliasTable
	reCohPascal   = regexp.MustCompile(`\b[A-Z][a-z0-9]+(?:[A-Z][a-zA-Z0-9]+)+\b`) // CommitBaselineIndex
	reCohSnake    = regexp.MustCompile(`\b[a-z0-9]+(?:_[a-z0-9]+)+\b`)             // run_func
	reCohDotPath  = regexp.MustCompile(`\b[a-zA-Z0-9_]+(?:[./][a-zA-Z0-9_]+)+\b`)  // os.ReadFile, internal/x

	// Near-identical deferral/clarification annotations across rows.
	reCohBoilerParen   = regexp.MustCompile(`\((?i:Deferred|Planned|intent_note|Clarified|Won't fix|Won’t fix|Wont fix|Accepted|Resolved|NOTE)[^)]*\)`)
	reCohBoilerBracket = regexp.MustCompile(`\[(?i:Clarified|Resolved|Deferred)[^\]]*\]`)
	reCohWS            = regexp.MustCompile(`\s+`)
)

// cohStop drops common prose words so they do not register as code identifiers.
var cohStop = map[string]struct{}{}

func init() {
	for _, w := range strings.Fields(`the and or but for with via into onto not none add adds added
		verify verifies test tests testing cover covers covered file files line lines row rows note
		fix fixes problem this that value values field fields path paths code case cases change
		before after future later phase plan planned deferred resolve resolved against instead
		within across check checks skip skips use uses used one two set sets map maps make call calls
		return returns present absent lowercase word column columns confirm helper resolver`) {
		cohStop[strings.ToLower(w)] = struct{}{}
	}
}

// cosine returns the cosine similarity of two equal-length vectors, or 0 when
// the lengths differ or either vector has zero magnitude.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// stripCoherenceBoilerplate removes the near-identical deferral/clarification
// annotations that appear across many TD rows, so they do not inflate
// cross-row textual similarity, then collapses whitespace.
func stripCoherenceBoilerplate(s string) string {
	s = reCohBoilerParen.ReplaceAllString(s, "")
	s = reCohBoilerBracket.ReplaceAllString(s, "")
	s = reCohWS.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// extractCodeIdentifiers returns the set of code-shaped tokens in s (backtick
// spans, camelCase, PascalCase, snake_case, dotted/path segments), lowercased,
// with common prose words and sub-3-char noise removed.
func extractCodeIdentifiers(s string) map[string]struct{} {
	out := map[string]struct{}{}
	add := func(tok string) {
		tok = strings.Trim(strings.ToLower(tok), " .,;:()[]{}`")
		if len(tok) < 3 {
			return
		}
		if _, bad := cohStop[tok]; bad {
			return
		}
		out[tok] = struct{}{}
	}
	for _, m := range reCohBacktick.FindAllStringSubmatch(s, -1) {
		inner := m[1]
		add(inner)
		for _, part := range strings.FieldsFunc(inner, func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == ','
		}) {
			add(part)
		}
	}
	for _, re := range []*regexp.Regexp{reCohCamel, reCohPascal, reCohSnake, reCohDotPath} {
		for _, m := range re.FindAllString(s, -1) {
			add(m)
		}
	}
	return out
}

// cohPuntPhrases mark a FIX that defers/hands off rather than prescribing a
// technical remedy.
var cohPuntPhrases = []string{
	"owned by", "backend decision", "architecture decision", "blocked on",
	"no production change", "no code change", "won't fix", "wont fix", "won’t fix",
	"deferred", "defer to", "future sprint", "leave permissive",
}

// isDeferralPunt reports whether a FIX cell defers/hands off rather than
// prescribing a technical remedy (the dominant false-positive class in the
// low-similarity tail).
func isDeferralPunt(fix string) bool {
	low := strings.ToLower(fix)
	for _, p := range cohPuntPhrases {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// --- ensemble (RED stubs; implemented in GREEN) ---

// signalResult holds one provider's per-row suspicion scores, aligned to the
// coherence row order. Higher score = more suspect (less coherent). A provider
// that could not run reports available=false and is excluded from the ensemble.
type signalResult struct {
	name      string
	scores    []float64
	available bool
}

// coherenceRow is one TD row's text pair fed to the coherence check.
type coherenceRow struct {
	problem string
	fix     string
}

// coherenceVerdict is the per-row ensemble outcome, in original row order.
type coherenceVerdict struct {
	score   float64 // combined suspicion (higher = more suspect)
	suspect bool
	tier    string // "high", "low", or "" when not suspect
}

// combineSignals averages the available signals' per-row scores into a single
// suspicion score per row, and returns the names of the signals that ran. A
// signal is included only when available and its score length matches nRows.
func combineSignals(results []signalResult, nRows int) ([]float64, []string) {
	if nRows == 0 {
		return []float64{}, nil
	}
	combined := make([]float64, nRows)
	counts := make([]int, nRows)
	var active []string
	for _, r := range results {
		if !r.available || len(r.scores) != nRows {
			continue
		}
		active = append(active, r.name)
		for i := 0; i < nRows; i++ {
			combined[i] += r.scores[i]
			counts[i]++
		}
	}
	for i := 0; i < nRows; i++ {
		if counts[i] > 0 {
			combined[i] /= float64(counts[i])
		}
	}
	return combined, active
}

// flagCoherence ranks rows by combined suspicion (descending, ties broken by
// ascending original index) and flags the top pct% as suspect, assigning each
// flagged row a confidence tier. At least one row is flagged when pct > 0 and
// there is at least one row.
func flagCoherence(rows []coherenceRow, combined []float64, pct int) []coherenceVerdict {
	verdicts := make([]coherenceVerdict, len(rows))
	for i := range rows {
		if i < len(combined) {
			verdicts[i].score = combined[i]
		}
	}
	if len(rows) == 0 || pct <= 0 {
		return verdicts
	}

	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return verdicts[order[a]].score > verdicts[order[b]].score
	})

	// pct>=1 and len(rows)>=1 here, so ceil yields at least 1.
	flagCount := int(math.Ceil(float64(len(rows)) * float64(pct) / 100))
	if flagCount > len(rows) {
		flagCount = len(rows)
	}
	for _, idx := range order[:flagCount] {
		verdicts[idx].suspect = true
		verdicts[idx].tier = coherenceTier(rows[idx].fix)
	}
	return verdicts
}

// coherenceTier labels a flagged row HIGH when its FIX looks like a technical
// remedy (has code identifiers, not a deferral punt) — the paste-error
// signature — and LOW otherwise.
func coherenceTier(fix string) string {
	if len(extractCodeIdentifiers(fix)) >= 1 && !isDeferralPunt(fix) {
		return "high"
	}
	return "low"
}
