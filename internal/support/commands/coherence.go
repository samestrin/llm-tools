package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
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

// --- signal 1: bi-encoder cosine (RED stubs; implemented in GREEN) ---

// embedderClient produces embedding vectors for a batch of texts. The cosine
// signal and the qdrant grounder depend on this seam so tests can inject a fake.
type embedderClient interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// httpEmbedder is a self-contained OpenAI-compatible /v1/embeddings client.
type httpEmbedder struct {
	apiURL string
	model  string
	apiKey string
	client *http.Client
}

// newHTTPEmbedderFromEnv builds an embedder from LLM_SEMANTIC_API_URL /
// LLM_SEMANTIC_MODEL, with the key from LLM_SEMANTIC_API_KEY or OPENAI_API_KEY.
func newHTTPEmbedderFromEnv() *httpEmbedder {
	return &httpEmbedder{
		apiURL: strings.TrimRight(os.Getenv("LLM_SEMANTIC_API_URL"), "/"),
		model:  os.Getenv("LLM_SEMANTIC_MODEL"),
		apiKey: firstNonEmpty(os.Getenv("LLM_SEMANTIC_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// available reports whether an endpoint is configured.
func (e *httpEmbedder) available() bool { return e.apiURL != "" }

// EmbedBatch returns one vector per input text, in input order.
func (e *httpEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{
		"model":           e.model,
		"input":           texts,
		"encoding_format": "float",
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	client := e.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embeddings endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var parsed struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings: got %d vectors for %d inputs", len(parsed.Data), len(texts))
	}
	out := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, fmt.Errorf("embeddings: index %d out of range for %d inputs", d.Index, len(texts))
		}
		out[d.Index] = d.Embedding
	}
	for i := range out {
		if out[i] == nil {
			return nil, fmt.Errorf("embeddings: missing vector at index %d", i)
		}
	}
	return out, nil
}

// cosineSignal scores each row's PROBLEM↔FIX pair as 1-cosine (higher = more
// suspect). It degrades to available=false if embedding fails.
func cosineSignal(ctx context.Context, e embedderClient, rows []coherenceRow) signalResult {
	res := signalResult{name: "cosine"}
	if len(rows) == 0 {
		res.available = true
		res.scores = []float64{}
		return res
	}
	texts := make([]string, 0, len(rows)*2)
	for _, r := range rows {
		texts = append(texts,
			"Problem: "+stripCoherenceBoilerplate(r.problem),
			"Fix: "+stripCoherenceBoilerplate(r.fix))
	}
	vecs, err := e.EmbedBatch(ctx, texts)
	if err != nil || len(vecs) != len(texts) {
		return res // available=false
	}
	scores := make([]float64, len(rows))
	for i := range rows {
		scores[i] = clamp01(1 - cosine(vecs[2*i], vecs[2*i+1]))
	}
	res.available = true
	res.scores = scores
	return res
}

// firstNonEmpty returns a if non-empty, else b.
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// clamp01 constrains x to [0,1].
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// --- signal 2: cross-encoder reranker (RED stubs; implemented in GREEN) ---

// rerankerClient scores a query against documents, returning one relevance
// score per document in document order.
type rerankerClient interface {
	Rerank(ctx context.Context, query string, documents []string) ([]float64, error)
}

// httpReranker is a self-contained cohere-format /v1/rerank client.
type httpReranker struct {
	apiURL string
	model  string
	apiKey string
	client *http.Client
}

// newHTTPRerankerFromEnv builds a reranker from LLM_SEMANTIC_RERANKER_API_URL /
// LLM_SEMANTIC_RERANKER_MODEL, key from LLM_SEMANTIC_RERANKER_API_KEY or
// LLM_SEMANTIC_API_KEY.
func newHTTPRerankerFromEnv() *httpReranker { return &httpReranker{} }

// available reports whether a reranker endpoint is configured.
func (r *httpReranker) available() bool { return false }

// Rerank returns one relevance score per document, in document order.
func (r *httpReranker) Rerank(ctx context.Context, query string, documents []string) ([]float64, error) {
	return nil, nil
}

// rerankSignal scores each row's PROBLEM↔FIX pair as 1-relevance (higher = more
// suspect). It degrades to available=false if any row's rerank call fails.
func rerankSignal(ctx context.Context, r rerankerClient, rows []coherenceRow) signalResult {
	return signalResult{name: "rerank"}
}
