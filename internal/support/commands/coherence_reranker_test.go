package commands

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func rerankResponse(scores map[int]float64) map[string]any {
	results := make([]map[string]any, 0, len(scores))
	for idx, s := range scores {
		results = append(results, map[string]any{"index": idx, "relevance_score": s})
	}
	return map[string]any{"results": results}
}

func TestHTTPReranker_Rerank(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody struct {
		Model     string   `json:"model"`
		Query     string   `json:"query"`
		Documents []string `json:"documents"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(rerankResponse(map[int]float64{0: 0.8, 1: 0.2}))
	}))
	defer srv.Close()

	r := &httpReranker{apiURL: srv.URL, model: "rr", apiKey: "k", client: srv.Client()}
	scores, err := r.Rerank(context.Background(), "q", []string{"d0", "d1"})
	if err != nil {
		t.Fatalf("Rerank error: %v", err)
	}
	if gotPath != "/v1/rerank" {
		t.Fatalf("path = %q, want /v1/rerank", gotPath)
	}
	if gotAuth != "Bearer k" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if gotBody.Model != "rr" || gotBody.Query != "q" || len(gotBody.Documents) != 2 {
		t.Fatalf("body = %+v", gotBody)
	}
	if math.Abs(scores[0]-0.8) > 1e-6 || math.Abs(scores[1]-0.2) > 1e-6 {
		t.Fatalf("scores = %v, want [0.8 0.2]", scores)
	}
}

func TestHTTPReranker_ReordersByIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{
			{"index": 1, "relevance_score": 0.1},
			{"index": 0, "relevance_score": 0.9},
		}})
	}))
	defer srv.Close()
	r := &httpReranker{apiURL: srv.URL, client: srv.Client()}
	scores, err := r.Rerank(context.Background(), "q", []string{"d0", "d1"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if math.Abs(scores[0]-0.9) > 1e-6 || math.Abs(scores[1]-0.1) > 1e-6 {
		t.Fatalf("reorder failed: %v", scores)
	}
}

func TestHTTPReranker_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusBadGateway)
	}))
	defer srv.Close()
	r := &httpReranker{apiURL: srv.URL, client: srv.Client()}
	if _, err := r.Rerank(context.Background(), "q", []string{"d"}); err == nil {
		t.Fatal("expected error on 502")
	}
}

func TestHTTPReranker_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(rerankResponse(map[int]float64{0: 0.5})) // 1 for 2 docs
	}))
	defer srv.Close()
	r := &httpReranker{apiURL: srv.URL, client: srv.Client()}
	if _, err := r.Rerank(context.Background(), "q", []string{"d0", "d1"}); err == nil {
		t.Fatal("expected count-mismatch error")
	}
}

func TestHTTPReranker_BadURL(t *testing.T) {
	r := &httpReranker{apiURL: "http://127.0.0.1:0", client: http.DefaultClient}
	if _, err := r.Rerank(context.Background(), "q", []string{"d"}); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestHTTPReranker_Available(t *testing.T) {
	if !(&httpReranker{apiURL: "http://x"}).available() {
		t.Fatal("want available when url set")
	}
	if (&httpReranker{}).available() {
		t.Fatal("want unavailable when url empty")
	}
}

func TestNewHTTPRerankerFromEnv(t *testing.T) {
	t.Run("defaults + key fallback", func(t *testing.T) {
		t.Setenv("LLM_SEMANTIC_RERANKER_API_URL", "http://rr:8002/")
		t.Setenv("LLM_SEMANTIC_RERANKER_MODEL", "")
		t.Setenv("LLM_SEMANTIC_RERANKER_API_KEY", "")
		t.Setenv("LLM_SEMANTIC_API_KEY", "shared-key")
		r := newHTTPRerankerFromEnv()
		if r.apiURL != "http://rr:8002" {
			t.Fatalf("apiURL = %q", r.apiURL)
		}
		if r.model == "" {
			t.Fatal("model should default when unset")
		}
		if r.apiKey != "shared-key" {
			t.Fatalf("apiKey = %q, want shared-key", r.apiKey)
		}
	})
}

// fakeReranker returns preset relevance per document (or an error).
type fakeReranker struct {
	byDoc      map[string]float64
	err        error
	failOnCall int
	call       int
}

func (f *fakeReranker) Rerank(ctx context.Context, query string, documents []string) ([]float64, error) {
	f.call++
	if f.err != nil {
		return nil, f.err
	}
	if f.failOnCall == f.call {
		return nil, errors.New("boom")
	}
	out := make([]float64, len(documents))
	for i, d := range documents {
		out[i] = f.byDoc[d]
	}
	return out, nil
}

func TestHTTPReranker_Adversarial(t *testing.T) {
	t.Run("malformed json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("]["))
		}))
		defer srv.Close()
		r := &httpReranker{apiURL: srv.URL, client: srv.Client()}
		if _, err := r.Rerank(context.Background(), "q", []string{"d"}); err == nil {
			t.Fatal("expected decode error")
		}
	})
	t.Run("duplicate index leaves a gap", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{
				{"index": 0, "relevance_score": 0.5},
				{"index": 0, "relevance_score": 0.6},
			}})
		}))
		defer srv.Close()
		r := &httpReranker{apiURL: srv.URL, client: srv.Client()}
		if _, err := r.Rerank(context.Background(), "q", []string{"d0", "d1"}); err == nil {
			t.Fatal("expected missing-score error")
		}
	})
	t.Run("empty documents", func(t *testing.T) {
		r := &httpReranker{apiURL: "http://should-not-call"}
		out, err := r.Rerank(context.Background(), "q", nil)
		if err != nil || len(out) != 0 {
			t.Fatalf("empty docs: out=%v err=%v", out, err)
		}
	})
}

func TestRerankSignal(t *testing.T) {
	rows := []coherenceRow{{problem: "p0", fix: "f0"}, {problem: "p1", fix: "f1"}}
	t.Run("scores 1-relevance per row", func(t *testing.T) {
		fake := &fakeReranker{byDoc: map[string]float64{
			"Fix: f0": 1.0, // relevant -> suspicion 0
			"Fix: f1": 0.0, // irrelevant -> suspicion 1
		}}
		res := rerankSignal(context.Background(), fake, rows)
		if !res.available {
			t.Fatal("expected available")
		}
		if math.Abs(res.scores[0]-0) > 1e-6 || math.Abs(res.scores[1]-1) > 1e-6 {
			t.Fatalf("scores = %v, want [0 1]", res.scores)
		}
	})
	t.Run("degrades if any row fails", func(t *testing.T) {
		fake := &fakeReranker{byDoc: map[string]float64{"Fix: f0": 1}, failOnCall: 2}
		res := rerankSignal(context.Background(), fake, rows)
		if res.available {
			t.Fatal("expected unavailable when a row's rerank fails")
		}
	})
	t.Run("empty rows available", func(t *testing.T) {
		res := rerankSignal(context.Background(), &fakeReranker{}, nil)
		if !res.available || len(res.scores) != 0 {
			t.Fatalf("empty: available=%v scores=%v", res.available, res.scores)
		}
	})
}
