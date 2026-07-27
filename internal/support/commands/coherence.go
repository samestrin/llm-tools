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
	"path"
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

// coherenceRow is one TD row's text pair (and cited file) fed to the check.
type coherenceRow struct {
	problem string
	fix     string
	file    string // cited file path (for grounding); "" when unknown
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

// --- signal 3: qdrant grounding (RED stubs; implemented in GREEN) ---

// qdrantGrounder embeds a FIX and asks a qdrant collection for the nearest code
// chunk, flagging rows whose FIX grounds to a file other than the cited one.
type qdrantGrounder struct {
	apiURL     string
	collection string
	apiKey     string
	embedder   embedderClient
	client     *http.Client
}

// newQdrantGrounderFromEnv builds a grounder from QDRANT_API_URL/QDRANT_URL and
// QDRANT_API_KEY, for the given collection, reusing the provided embedder.
func newQdrantGrounderFromEnv(collection string, e embedderClient) *qdrantGrounder {
	return &qdrantGrounder{
		apiURL:     strings.TrimRight(firstNonEmpty(os.Getenv("QDRANT_API_URL"), os.Getenv("QDRANT_URL")), "/"),
		collection: collection,
		apiKey:     os.Getenv("QDRANT_API_KEY"),
		embedder:   e,
		client:     &http.Client{Timeout: 60 * time.Second},
	}
}

// available reports whether grounding can run (endpoint + collection + embedder).
func (g *qdrantGrounder) available() bool {
	return g.apiURL != "" && g.collection != "" && g.embedder != nil
}

// nearestFile returns the file-path payload of the top hit for a query vector,
// or "" when there is no hit / no file payload.
func (g *qdrantGrounder) nearestFile(ctx context.Context, vec []float32) (string, error) {
	body, err := json.Marshal(map[string]any{"vector": vec, "limit": 1, "with_payload": true})
	if err != nil {
		return "", err
	}
	url := g.apiURL + "/collections/" + g.collection + "/points/search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("api-key", g.apiKey)
	}
	client := g.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("qdrant search returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var parsed struct {
		Result []struct {
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Result) == 0 {
		return "", nil
	}
	return fileFromPayload(parsed.Result[0].Payload), nil
}

// fileFromPayload extracts a file path from a qdrant point payload, tolerating
// the common key spellings used by the semantic indexer.
func fileFromPayload(p map[string]any) string {
	for _, k := range []string{"file", "file_path", "path", "source"} {
		if v, ok := p[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// groundSignal scores a row 1.0 when its FIX grounds to a file other than the
// cited one, else 0.0. Degrades to available=false on any embed/search failure.
func groundSignal(ctx context.Context, g *qdrantGrounder, rows []coherenceRow) signalResult {
	res := signalResult{name: "ground"}
	if len(rows) == 0 {
		res.available = true
		res.scores = []float64{}
		return res
	}
	texts := make([]string, len(rows))
	for i, r := range rows {
		texts[i] = "Fix: " + stripCoherenceBoilerplate(r.fix)
	}
	vecs, err := g.embedder.EmbedBatch(ctx, texts)
	if err != nil || len(vecs) != len(rows) {
		return res
	}
	scores := make([]float64, len(rows))
	for i, r := range rows {
		nearest, err := g.nearestFile(ctx, vecs[i])
		if err != nil {
			return res
		}
		if r.file != "" && nearest != "" && !filePathsMatch(nearest, r.file) {
			scores[i] = 1
		}
	}
	res.available = true
	res.scores = scores
	return res
}

// applyCoherence runs the enabled coherence signals over eligible items and
// annotates them in place. Fail-soft: any missing endpoint drops that signal,
// and if nothing runs it sets summary.CoherenceSkipped and warns. (RED stub;
// implemented in GREEN.)
func applyCoherence(ctx context.Context, items []TDValidateItem, rows []TDFilterRow, summary *TDValidateSummary, pct int, collection string, warn io.Writer) {
}

// filePathsMatch reports whether two repo-relative paths plausibly refer to the
// same file (equal, suffix, or same base name). Permissive by design: grounding
// is a noisy signal, so it only fires on a clear file difference.
func filePathsMatch(nearest, cited string) bool {
	n := strings.Trim(strings.ReplaceAll(nearest, "\\", "/"), "/")
	c := strings.Trim(strings.ReplaceAll(cited, "\\", "/"), "/")
	if n == "" || c == "" {
		return true
	}
	if n == c || strings.HasSuffix(n, "/"+c) || strings.HasSuffix(c, "/"+n) {
		return true
	}
	return path.Base(n) == path.Base(c)
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
func newHTTPRerankerFromEnv() *httpReranker {
	return &httpReranker{
		apiURL: strings.TrimRight(os.Getenv("LLM_SEMANTIC_RERANKER_API_URL"), "/"),
		model:  firstNonEmpty(os.Getenv("LLM_SEMANTIC_RERANKER_MODEL"), "Qwen/Qwen3-Reranker-0.6B"),
		apiKey: firstNonEmpty(os.Getenv("LLM_SEMANTIC_RERANKER_API_KEY"), os.Getenv("LLM_SEMANTIC_API_KEY")),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// available reports whether a reranker endpoint is configured.
func (r *httpReranker) available() bool { return r.apiURL != "" }

// Rerank returns one relevance score per document, in document order.
func (r *httpReranker) Rerank(ctx context.Context, query string, documents []string) ([]float64, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{
		"model":     r.model,
		"query":     query,
		"documents": documents,
		"top_n":     len(documents),
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.apiURL+"/v1/rerank", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.apiKey)
	}
	client := r.client
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
		return nil, fmt.Errorf("rerank endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	var parsed struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Results) != len(documents) {
		return nil, fmt.Errorf("rerank: got %d scores for %d documents", len(parsed.Results), len(documents))
	}
	out := make([]float64, len(documents))
	seen := make([]bool, len(documents))
	for _, res := range parsed.Results {
		if res.Index < 0 || res.Index >= len(out) {
			return nil, fmt.Errorf("rerank: index %d out of range for %d documents", res.Index, len(documents))
		}
		out[res.Index] = res.RelevanceScore
		seen[res.Index] = true
	}
	for i, ok := range seen {
		if !ok {
			return nil, fmt.Errorf("rerank: missing score at index %d", i)
		}
	}
	return out, nil
}

// rerankSignal scores each row's PROBLEM↔FIX pair as 1-relevance (higher = more
// suspect). It degrades to available=false if any row's rerank call fails.
func rerankSignal(ctx context.Context, r rerankerClient, rows []coherenceRow) signalResult {
	res := signalResult{name: "rerank"}
	if len(rows) == 0 {
		res.available = true
		res.scores = []float64{}
		return res
	}
	scores := make([]float64, len(rows))
	for i, row := range rows {
		rel, err := r.Rerank(ctx,
			"Problem: "+stripCoherenceBoilerplate(row.problem),
			[]string{"Fix: " + stripCoherenceBoilerplate(row.fix)})
		if err != nil || len(rel) != 1 {
			return res // available=false
		}
		scores[i] = clamp01(1 - rel[0])
	}
	res.available = true
	res.scores = scores
	return res
}
