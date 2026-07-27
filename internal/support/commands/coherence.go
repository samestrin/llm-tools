package commands

import (
	"math"
	"regexp"
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
