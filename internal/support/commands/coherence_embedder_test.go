package commands

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// embedResponse builds an OpenAI-shaped /v1/embeddings response.
func embedResponse(vecs map[int][]float32) map[string]any {
	data := make([]map[string]any, 0, len(vecs))
	for idx, v := range vecs {
		data = append(data, map[string]any{"index": idx, "object": "embedding", "embedding": v})
	}
	return map[string]any{"object": "list", "data": data}
}

func TestHTTPEmbedder_EmbedBatch(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(embedResponse(map[int][]float32{
			0: {1, 0}, 1: {0, 1},
		}))
	}))
	defer srv.Close()

	e := &httpEmbedder{apiURL: srv.URL, model: "test-model", apiKey: "secret", client: srv.Client()}
	vecs, err := e.EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("EmbedBatch error: %v", err)
	}
	if gotPath != "/v1/embeddings" {
		t.Fatalf("path = %q, want /v1/embeddings", gotPath)
	}
	if gotAuth != "Bearer secret" {
		t.Fatalf("auth = %q, want Bearer secret", gotAuth)
	}
	if gotBody.Model != "test-model" || len(gotBody.Input) != 2 {
		t.Fatalf("body = %+v", gotBody)
	}
	if len(vecs) != 2 || vecs[0][0] != 1 || vecs[1][1] != 1 {
		t.Fatalf("vecs = %v", vecs)
	}
}

func TestHTTPEmbedder_ReordersByIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// deliberately emit index 1 before index 0
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"index": 1, "embedding": []float32{9, 9}},
			{"index": 0, "embedding": []float32{1, 1}},
		}})
	}))
	defer srv.Close()
	e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
	vecs, err := e.EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if vecs[0][0] != 1 || vecs[1][0] != 9 {
		t.Fatalf("reorder failed: %v", vecs)
	}
}

func TestHTTPEmbedder_NoAuthWhenNoKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(embedResponse(map[int][]float32{0: {1}}))
	}))
	defer srv.Close()
	e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
	if _, err := e.EmbedBatch(context.Background(), []string{"a"}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("auth = %q, want empty", gotAuth)
	}
}

func TestHTTPEmbedder_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
	if _, err := e.EmbedBatch(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestHTTPEmbedder_CountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(embedResponse(map[int][]float32{0: {1}})) // 1 vec for 2 inputs
	}))
	defer srv.Close()
	e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
	if _, err := e.EmbedBatch(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("expected error on count mismatch")
	}
}

func TestHTTPEmbedder_BadURL(t *testing.T) {
	e := &httpEmbedder{apiURL: "http://127.0.0.1:0", client: http.DefaultClient}
	if _, err := e.EmbedBatch(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestHTTPEmbedder_ContextCancel(t *testing.T) {
	e := &httpEmbedder{apiURL: "http://example.invalid", client: http.DefaultClient}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.EmbedBatch(ctx, []string{"a"}); err == nil {
		t.Fatal("expected error with canceled context")
	}
}

func TestHTTPEmbedder_Available(t *testing.T) {
	if (&httpEmbedder{apiURL: "http://x"}).available() != true {
		t.Fatal("available should be true when apiURL set")
	}
	if (&httpEmbedder{}).available() != false {
		t.Fatal("available should be false when apiURL empty")
	}
}

func TestHTTPEmbedder_EmptyInput(t *testing.T) {
	e := &httpEmbedder{apiURL: "http://should-not-be-called"}
	vecs, err := e.EmbedBatch(context.Background(), nil)
	if err != nil || len(vecs) != 0 {
		t.Fatalf("empty input: vecs=%v err=%v", vecs, err)
	}
}

func TestNewHTTPEmbedderFromEnv(t *testing.T) {
	t.Run("trailing slash + openai key fallback", func(t *testing.T) {
		t.Setenv("LLM_SEMANTIC_API_URL", "http://box:8081/")
		t.Setenv("LLM_SEMANTIC_MODEL", "M")
		t.Setenv("LLM_SEMANTIC_API_KEY", "")
		t.Setenv("OPENAI_API_KEY", "openai-key")
		e := newHTTPEmbedderFromEnv()
		if e.apiURL != "http://box:8081" {
			t.Fatalf("apiURL = %q, want trimmed", e.apiURL)
		}
		if e.model != "M" || e.apiKey != "openai-key" {
			t.Fatalf("model=%q key=%q", e.model, e.apiKey)
		}
	})
	t.Run("semantic key precedence", func(t *testing.T) {
		t.Setenv("LLM_SEMANTIC_API_URL", "http://box")
		t.Setenv("LLM_SEMANTIC_API_KEY", "sem-key")
		t.Setenv("OPENAI_API_KEY", "openai-key")
		if e := newHTTPEmbedderFromEnv(); e.apiKey != "sem-key" {
			t.Fatalf("apiKey = %q, want sem-key", e.apiKey)
		}
	})
}

// fakeEmbedder returns preset vectors (or an error) regardless of input.
type fakeEmbedder struct {
	out      [][]float32
	err      error
	gotTexts []string
}

func (f *fakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	f.gotTexts = texts
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestCosineSignal(t *testing.T) {
	rows := []coherenceRow{
		{problem: "p0", fix: "f0"},
		{problem: "p1", fix: "f1"},
	}
	t.Run("scores 1-cosine per row", func(t *testing.T) {
		fake := &fakeEmbedder{out: [][]float32{
			{1, 0}, {1, 0}, // row0: identical -> cos 1 -> score 0
			{1, 0}, {0, 1}, // row1: orthogonal -> cos 0 -> score 1
		}}
		res := cosineSignal(context.Background(), fake, rows)
		if !res.available {
			t.Fatal("expected available")
		}
		if math.Abs(res.scores[0]-0) > 1e-6 || math.Abs(res.scores[1]-1) > 1e-6 {
			t.Fatalf("scores = %v, want [0 1]", res.scores)
		}
		if len(fake.gotTexts) != 4 || fake.gotTexts[0] != "Problem: p0" || fake.gotTexts[1] != "Fix: f0" {
			t.Fatalf("gotTexts = %v", fake.gotTexts)
		}
	})
	t.Run("embed error degrades", func(t *testing.T) {
		fake := &fakeEmbedder{err: context.DeadlineExceeded}
		res := cosineSignal(context.Background(), fake, rows)
		if res.available {
			t.Fatal("expected unavailable on embed error")
		}
	})
	t.Run("empty rows available with no scores", func(t *testing.T) {
		fake := &fakeEmbedder{}
		res := cosineSignal(context.Background(), fake, nil)
		if !res.available || len(res.scores) != 0 {
			t.Fatalf("empty: available=%v scores=%v", res.available, res.scores)
		}
	})
}

func TestHTTPEmbedder_Adversarial(t *testing.T) {
	t.Run("malformed json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{not json"))
		}))
		defer srv.Close()
		e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
		if _, err := e.EmbedBatch(context.Background(), []string{"a"}); err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("duplicate index leaves a gap", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// two entries, both index 0 -> index 1 never filled
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"index": 0, "embedding": []float32{1}},
				{"index": 0, "embedding": []float32{2}},
			}})
		}))
		defer srv.Close()
		e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
		if _, err := e.EmbedBatch(context.Background(), []string{"a", "b"}); err == nil {
			t.Fatal("expected missing-vector error")
		}
	})

	t.Run("out-of-range index", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"index": 5, "embedding": []float32{1}},
			}})
		}))
		defer srv.Close()
		e := &httpEmbedder{apiURL: srv.URL, client: srv.Client()}
		if _, err := e.EmbedBatch(context.Background(), []string{"a"}); err == nil {
			t.Fatal("expected out-of-range error")
		}
	})
}

func TestCosineSignal_ShortCountDegrades(t *testing.T) {
	// embedder returns fewer vectors than 2*rows -> degrade, no panic
	fake := &fakeEmbedder{out: [][]float32{{1, 0}}} // 1 vec for 2 texts
	res := cosineSignal(context.Background(), fake, []coherenceRow{{problem: "p", fix: "f"}})
	if res.available {
		t.Fatal("expected unavailable on short vector count")
	}
}

func TestClamp01AndFirstNonEmpty(t *testing.T) {
	if clamp01(-0.5) != 0 || clamp01(1.5) != 1 || clamp01(0.4) != 0.4 {
		t.Fatal("clamp01 wrong")
	}
	if firstNonEmpty("", "b") != "b" || firstNonEmpty("a", "b") != "a" || firstNonEmpty("", "") != "" {
		t.Fatal("firstNonEmpty wrong")
	}
}
