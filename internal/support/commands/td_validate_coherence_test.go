package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// runTDValidateText runs td-validate capturing stdout/stderr without JSON decoding.
func runTDValidateText(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	cmd := newTDValidateCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err = cmd.Execute()
	return out.String(), errb.String(), err
}

// coherenceREADME has two rows with substantive PROBLEM and FIX cells (>=20
// chars each), so both are coherence candidates; both cite scripts/foo.py.
const coherenceREADME = `# Technical Debt

### [2026-01-01] From Sprint: test_sprint

| Group | | Severity | File | Problem | Fix | Category | Est Minutes | Source |
|-------|---|----------|------|---------|-----|----------|-------------|--------|
| 1 | [ ] | MEDIUM | scripts/foo.py:run_func | The reader slurps the entire file into memory with no size cap, a memory-exhaustion vector on large inputs | add a stat-size guard and an io.LimitReader mirroring the ingest path, skipping over-cap files rather than OOMing | performance | 20 | test |
| 2 | [ ] | LOW | scripts/foo.py:42 | The family JSON field has two shapes under one key so a consumer grouping on family sees inconsistent values | when wiring models refresh against the live catalog confirm the latest ids are present and resolve alias locks | correctness | 15 | test |
`

func embedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		data := make([]map[string]any, len(body.Input))
		for i := range body.Input {
			data[i] = map[string]any{"index": i, "embedding": []float32{1, 0, 0}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
}

func rerankTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Documents []string `json:"documents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		results := make([]map[string]any, len(body.Documents))
		for i := range body.Documents {
			results[i] = map[string]any{"index": i, "relevance_score": 0.5}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
}

func qdrantTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{
			{"payload": map[string]any{"file": "scripts/foo.py"}},
		}})
	}))
}

// clearCoherenceEnv points every coherence endpoint var at nothing so a test
// never reaches the real LAN services unless it opts back in.
func clearCoherenceEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"LLM_SEMANTIC_API_URL", "LLM_SEMANTIC_MODEL", "LLM_SEMANTIC_API_KEY", "OPENAI_API_KEY",
		"LLM_SEMANTIC_RERANKER_API_URL", "LLM_SEMANTIC_RERANKER_API_KEY",
		"QDRANT_API_URL", "QDRANT_URL", "QDRANT_API_KEY",
	} {
		t.Setenv(k, "")
	}
}

func TestTDValidate_Coherence_EmbeddingOnly(t *testing.T) {
	clearCoherenceEnv(t)
	embed := embedTestServer(t)
	defer embed.Close()
	t.Setenv("LLM_SEMANTIC_API_URL", embed.URL)

	readmePath, rootDir := writeTDValidateFiles(t, coherenceREADME)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir,
		"--coherence", "--coherence-percentile", "100", "--json")
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	if len(res.Summary.CoherenceSignals) != 1 || res.Summary.CoherenceSignals[0] != "cosine" {
		t.Fatalf("signals = %v, want [cosine]", res.Summary.CoherenceSignals)
	}
	if res.Summary.CoherenceSkipped {
		t.Fatal("should not be skipped when cosine ran")
	}
	// percentile 100 -> every candidate flagged
	var scored, suspects int
	for _, it := range res.Items {
		if it.CoherenceScore != nil {
			scored++
		}
		if it.CoherenceSuspect {
			suspects++
		}
	}
	if scored != 2 || suspects != 2 {
		t.Fatalf("scored=%d suspects=%d, want 2/2", scored, suspects)
	}
	if res.Summary.CoherenceSuspect != 2 {
		t.Fatalf("summary suspects = %d, want 2", res.Summary.CoherenceSuspect)
	}
}

func TestTDValidate_Coherence_AllThreeSignals(t *testing.T) {
	clearCoherenceEnv(t)
	embed, rerank, qdrant := embedTestServer(t), rerankTestServer(t), qdrantTestServer(t)
	defer embed.Close()
	defer rerank.Close()
	defer qdrant.Close()
	t.Setenv("LLM_SEMANTIC_API_URL", embed.URL)
	t.Setenv("LLM_SEMANTIC_RERANKER_API_URL", rerank.URL)
	t.Setenv("QDRANT_API_URL", qdrant.URL)

	readmePath, rootDir := writeTDValidateFiles(t, coherenceREADME)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir,
		"--coherence", "--coherence-collection", "atcr-code", "--json")
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	got := strings.Join(res.Summary.CoherenceSignals, ",")
	if got != "cosine,rerank,ground" {
		t.Fatalf("signals = %q, want cosine,rerank,ground", got)
	}
}

func TestTDValidate_Coherence_RerankerDropsOut(t *testing.T) {
	clearCoherenceEnv(t)
	embed := embedTestServer(t)
	defer embed.Close()
	t.Setenv("LLM_SEMANTIC_API_URL", embed.URL) // reranker + qdrant left empty

	readmePath, rootDir := writeTDValidateFiles(t, coherenceREADME)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--coherence", "--json")
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	if strings.Join(res.Summary.CoherenceSignals, ",") != "cosine" {
		t.Fatalf("signals = %v, want [cosine] only", res.Summary.CoherenceSignals)
	}
}

func TestTDValidate_Coherence_SkipWhenNoEndpoints(t *testing.T) {
	clearCoherenceEnv(t) // nothing configured

	readmePath, rootDir := writeTDValidateFiles(t, coherenceREADME)
	res, stderr, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--coherence", "--json")
	if err != nil {
		t.Fatalf("cmd should still succeed (exit 0): %v", err)
	}
	if !res.Summary.CoherenceSkipped {
		t.Fatal("expected CoherenceSkipped when no endpoints")
	}
	if len(res.Summary.CoherenceSignals) != 0 {
		t.Fatalf("no signals should run; got %v", res.Summary.CoherenceSignals)
	}
	if !strings.Contains(stderr, "coherence") {
		t.Fatalf("expected a skip warning on stderr; got %q", stderr)
	}
	// file/symbol results are intact
	if res.Summary.Total != 2 || res.Summary.Valid != 2 {
		t.Fatalf("file/symbol summary changed: %+v", res.Summary)
	}
	for _, it := range res.Items {
		if it.CoherenceScore != nil || it.CoherenceSuspect {
			t.Fatalf("no item should be scored when skipped: %+v", it)
		}
	}
}

func TestTDValidate_Coherence_TextOutput(t *testing.T) {
	clearCoherenceEnv(t)
	embed := embedTestServer(t)
	defer embed.Close()
	t.Setenv("LLM_SEMANTIC_API_URL", embed.URL)

	readmePath, rootDir := writeTDValidateFiles(t, coherenceREADME)
	stdout, _, err := runTDValidateText(t, "--path", readmePath, "--root", rootDir,
		"--coherence", "--coherence-percentile", "100")
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	if !strings.Contains(stdout, "coherence:") {
		t.Fatalf("text output missing coherence section:\n%s", stdout)
	}
}

func TestTDValidate_Coherence_TextSkipped(t *testing.T) {
	clearCoherenceEnv(t)
	readmePath, rootDir := writeTDValidateFiles(t, coherenceREADME)
	stdout, _, err := runTDValidateText(t, "--path", readmePath, "--root", rootDir, "--coherence")
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	if !strings.Contains(stdout, "coherence: skipped") {
		t.Fatalf("expected skipped line in text output:\n%s", stdout)
	}
}

func TestTDValidate_Coherence_NoCandidates(t *testing.T) {
	clearCoherenceEnv(t)
	embed := embedTestServer(t)
	defer embed.Close()
	t.Setenv("LLM_SEMANTIC_API_URL", embed.URL)

	// fixtureTDValidateReadme rows all have tiny PROBLEM/FIX cells (<20 chars)
	readmePath, rootDir := writeTDValidateFiles(t, fixtureTDValidateReadme)
	res, _, err := runTDValidateCmd(t, "--path", readmePath, "--root", rootDir, "--coherence", "--json")
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	if !res.Summary.CoherenceSkipped {
		t.Fatal("expected skipped when no substantive candidates")
	}
}
