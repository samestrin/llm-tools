package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// qdrantSearchResponse builds a qdrant points/search response with one hit
// carrying the given file payload.
func qdrantSearchResponse(file string) map[string]any {
	hit := map[string]any{"id": 1, "score": 0.9}
	if file != "" {
		hit["payload"] = map[string]any{"file": file}
	}
	return map[string]any{"result": []map[string]any{hit}, "status": "ok"}
}

func TestQdrantGrounder_NearestFile(t *testing.T) {
	var gotPath, gotAPIKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("api-key")
		_ = json.NewEncoder(w).Encode(qdrantSearchResponse("internal/personas/catalog.go"))
	}))
	defer srv.Close()

	g := &qdrantGrounder{apiURL: srv.URL, collection: "atcr-code", apiKey: "qk", client: srv.Client()}
	file, err := g.nearestFile(context.Background(), []float32{0.1, 0.2})
	if err != nil {
		t.Fatalf("nearestFile error: %v", err)
	}
	if gotPath != "/collections/atcr-code/points/search" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAPIKey != "qk" {
		t.Fatalf("api-key = %q, want qk", gotAPIKey)
	}
	if file != "internal/personas/catalog.go" {
		t.Fatalf("file = %q", file)
	}
}

func TestQdrantGrounder_NearestFile_NoHits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{}, "status": "ok"})
	}))
	defer srv.Close()
	g := &qdrantGrounder{apiURL: srv.URL, collection: "c", client: srv.Client()}
	file, err := g.nearestFile(context.Background(), []float32{1})
	if err != nil || file != "" {
		t.Fatalf("no hits: file=%q err=%v", file, err)
	}
}

func TestQdrantGrounder_NearestFile_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	g := &qdrantGrounder{apiURL: srv.URL, collection: "c", client: srv.Client()}
	if _, err := g.nearestFile(context.Background(), []float32{1}); err == nil {
		t.Fatal("expected error on 503")
	}
}

func TestQdrantGrounder_Available(t *testing.T) {
	e := &fakeEmbedder{}
	if !(&qdrantGrounder{apiURL: "http://x", collection: "c", embedder: e}).available() {
		t.Fatal("should be available with url+collection+embedder")
	}
	if (&qdrantGrounder{collection: "c", embedder: e}).available() {
		t.Fatal("no url -> unavailable")
	}
	if (&qdrantGrounder{apiURL: "http://x", embedder: e}).available() {
		t.Fatal("no collection -> unavailable")
	}
	if (&qdrantGrounder{apiURL: "http://x", collection: "c"}).available() {
		t.Fatal("no embedder -> unavailable")
	}
}

func TestNewQdrantGrounderFromEnv(t *testing.T) {
	t.Setenv("QDRANT_API_URL", "http://db.lan:6333/")
	t.Setenv("QDRANT_API_KEY", "secret")
	g := newQdrantGrounderFromEnv("atcr-code", &fakeEmbedder{})
	if g.apiURL != "http://db.lan:6333" || g.collection != "atcr-code" || g.apiKey != "secret" {
		t.Fatalf("grounder = %+v", g)
	}
}

func TestFilePathsMatch(t *testing.T) {
	cases := []struct {
		nearest, cited string
		want           bool
	}{
		{"internal/personas/drift.go", "internal/personas/drift.go", true}, // equal
		{"internal/personas/drift.go", "personas/drift.go", true},          // suffix
		{"drift.go", "internal/personas/drift.go", true},                   // base name
		{"internal/personas/catalog.go", "internal/personas/drift.go", false},
		{"", "internal/x.go", true}, // unknown -> don't penalize
		{"internal/x.go", "", true}, // unknown -> don't penalize
	}
	for _, tc := range cases {
		if got := filePathsMatch(tc.nearest, tc.cited); got != tc.want {
			t.Fatalf("filePathsMatch(%q,%q) = %v, want %v", tc.nearest, tc.cited, got, tc.want)
		}
	}
}

func TestFileFromPayload(t *testing.T) {
	cases := []struct {
		p    map[string]any
		want string
	}{
		{map[string]any{"file": "a.go"}, "a.go"},
		{map[string]any{"file_path": "b.go"}, "b.go"},
		{map[string]any{"path": "c.go"}, "c.go"},
		{map[string]any{"source": "d.go"}, "d.go"},
		{map[string]any{"other": "x"}, ""},
		{map[string]any{"file": 123}, ""}, // non-string ignored
		{nil, ""},
	}
	for _, tc := range cases {
		if got := fileFromPayload(tc.p); got != tc.want {
			t.Fatalf("fileFromPayload(%v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestQdrantGrounder_Adversarial(t *testing.T) {
	t.Run("malformed json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("nope"))
		}))
		defer srv.Close()
		g := &qdrantGrounder{apiURL: srv.URL, collection: "c", client: srv.Client()}
		if _, err := g.nearestFile(context.Background(), []float32{1}); err == nil {
			t.Fatal("expected decode error")
		}
	})

	t.Run("groundSignal degrades on qdrant error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()
		g := &qdrantGrounder{apiURL: srv.URL, collection: "c", embedder: &fakeEmbedder{out: [][]float32{{1}}}, client: srv.Client()}
		res := groundSignal(context.Background(), g, []coherenceRow{{fix: "f", file: "x.go"}})
		if res.available {
			t.Fatal("expected unavailable when qdrant search errors")
		}
	})

	t.Run("no file payload -> not suspect", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(qdrantSearchResponse("")) // hit without payload
		}))
		defer srv.Close()
		g := &qdrantGrounder{apiURL: srv.URL, collection: "c", embedder: &fakeEmbedder{out: [][]float32{{1}}}, client: srv.Client()}
		res := groundSignal(context.Background(), g, []coherenceRow{{fix: "f", file: "x.go"}})
		if !res.available || res.scores[0] != 0 {
			t.Fatalf("no payload should score 0; available=%v scores=%v", res.available, res.scores)
		}
	})
}

func TestGroundSignal(t *testing.T) {
	rows := []coherenceRow{
		{fix: "f0", file: "internal/personas/drift.go"},   // grounds to catalog.go -> mismatch
		{fix: "f1", file: "internal/payload/fullrepo.go"}, // grounds to fullrepo.go -> match
	}
	// embedder returns a distinct vector per fix
	fake := &fakeEmbedder{out: [][]float32{{1, 0}, {0, 1}}}

	// qdrant returns catalog.go for the first vector, fullrepo.go for the second
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Vector []float32 `json:"vector"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		file := "internal/personas/catalog.go"
		if len(body.Vector) == 2 && body.Vector[1] == 1 {
			file = "internal/payload/fullrepo.go"
		}
		_ = json.NewEncoder(w).Encode(qdrantSearchResponse(file))
	}))
	defer srv.Close()

	g := &qdrantGrounder{apiURL: srv.URL, collection: "atcr-code", embedder: fake, client: srv.Client()}

	t.Run("scores mismatch 1, match 0", func(t *testing.T) {
		res := groundSignal(context.Background(), g, rows)
		if !res.available {
			t.Fatal("expected available")
		}
		if res.scores[0] != 1 {
			t.Fatalf("row0 (drift vs catalog) should be 1, got %v", res.scores[0])
		}
		if res.scores[1] != 0 {
			t.Fatalf("row1 (fullrepo match) should be 0, got %v", res.scores[1])
		}
	})

	t.Run("embed failure degrades", func(t *testing.T) {
		badG := &qdrantGrounder{apiURL: srv.URL, collection: "c", embedder: &fakeEmbedder{err: context.Canceled}, client: srv.Client()}
		res := groundSignal(context.Background(), badG, rows)
		if res.available {
			t.Fatal("expected unavailable on embed failure")
		}
	})

	t.Run("empty rows available", func(t *testing.T) {
		res := groundSignal(context.Background(), g, nil)
		if !res.available || len(res.scores) != 0 {
			t.Fatalf("empty: available=%v scores=%v", res.available, res.scores)
		}
	})
}
