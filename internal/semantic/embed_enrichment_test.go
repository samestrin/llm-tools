package semantic

import (
	"strings"
	"testing"
)

func TestEnrichForEmbedding(t *testing.T) {
	tests := []struct {
		name         string
		chunk        Chunk
		wantPrefix   string
		wantContains string
	}{
		{
			name: "function with signature",
			chunk: Chunk{
				FilePath:  "internal/semantic/search.go",
				Type:      ChunkFunction,
				Name:      "HandleSearch",
				Signature: "func HandleSearch(ctx context.Context, query string) ([]SearchResult, error)",
				Content:   "func HandleSearch(ctx context.Context, query string) ([]SearchResult, error) {\n\treturn nil, nil\n}",
				Language:  "go",
			},
			wantPrefix:   "[go:function] func HandleSearch(ctx context.Context, query string) ([]SearchResult, error) in internal/semantic/search.go\n\n",
			wantContains: "func HandleSearch",
		},
		{
			name: "struct without signature",
			chunk: Chunk{
				FilePath: "internal/semantic/config.go",
				Type:     ChunkStruct,
				Name:     "Config",
				Content:  "type Config struct {\n\tAPIURL string\n}",
				Language: "go",
			},
			wantPrefix:   "[go:struct] Config in internal/semantic/config.go\n\n",
			wantContains: "type Config struct",
		},
		{
			name: "interface",
			chunk: Chunk{
				FilePath:  "internal/semantic/storage.go",
				Type:      ChunkInterface,
				Name:      "Storage",
				Signature: "type Storage interface",
				Content:   "type Storage interface {\n\tCreate(ctx context.Context) error\n}",
				Language:  "go",
			},
			wantPrefix:   "[go:interface] type Storage interface in internal/semantic/storage.go\n\n",
			wantContains: "type Storage interface",
		},
		{
			name: "method with receiver",
			chunk: Chunk{
				FilePath:  "internal/semantic/embedder.go",
				Type:      ChunkMethod,
				Name:      "Embed",
				Signature: "func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error)",
				Content:   "func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {\n\treturn nil, nil\n}",
				Language:  "go",
			},
			wantPrefix:   "[go:method] func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) in internal/semantic/embedder.go\n\n",
			wantContains: "func (e *Embedder) Embed",
		},
		{
			name: "file chunk (markdown)",
			chunk: Chunk{
				FilePath: "README.md",
				Type:     ChunkFile,
				Name:     "README",
				Content:  "# Project Title\n\nSome description here.",
				Language: "md",
			},
			wantPrefix:   "[md:file] README.md\n\n",
			wantContains: "# Project Title",
		},
		{
			name: "python function",
			chunk: Chunk{
				FilePath:  "app/handlers.py",
				Type:      ChunkFunction,
				Name:      "handle_request",
				Signature: "def handle_request(request: Request) -> Response:",
				Content:   "def handle_request(request: Request) -> Response:\n    return Response(200)",
				Language:  "py",
			},
			wantPrefix:   "[py:function] def handle_request(request: Request) -> Response: in app/handlers.py\n\n",
			wantContains: "def handle_request",
		},
		{
			name: "empty language",
			chunk: Chunk{
				FilePath: "unknown_file",
				Type:     ChunkFile,
				Name:     "unknown",
				Content:  "some content",
				Language: "",
			},
			wantPrefix:   "[file] unknown_file\n\n",
			wantContains: "some content",
		},
		{
			name: "empty signature falls back to name",
			chunk: Chunk{
				FilePath: "pkg/util.go",
				Type:     ChunkFunction,
				Name:     "helper",
				Content:  "func helper() {}",
				Language: "go",
			},
			wantPrefix:   "[go:function] helper in pkg/util.go\n\n",
			wantContains: "func helper",
		},
		{
			name: "chunk with EmbedText set uses EmbedText as content",
			chunk: Chunk{
				FilePath:  "pkg/util.go",
				Type:      ChunkFunction,
				Name:      "helper",
				Signature: "func helper()",
				Content:   "func helper() {}",
				EmbedText: "package main\n\nfunc helper() {}",
				Language:  "go",
			},
			wantPrefix:   "[go:function] func helper() in pkg/util.go\n\n",
			wantContains: "package main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnrichForEmbedding(tt.chunk)

			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("EnrichForEmbedding() prefix mismatch\ngot:  %q\nwant: %q", got[:min(len(got), len(tt.wantPrefix)+20)], tt.wantPrefix)
			}

			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("EnrichForEmbedding() should contain %q, got: %q", tt.wantContains, got)
			}

			// Verify content comes after the double newline
			parts := strings.SplitN(got, "\n\n", 2)
			if len(parts) != 2 {
				t.Errorf("EnrichForEmbedding() should have prefix and content separated by \\n\\n")
			}
		})
	}
}

func TestEnrichForEmbedding_EmptyContent(t *testing.T) {
	chunk := Chunk{
		FilePath: "empty.go",
		Type:     ChunkFunction,
		Name:     "empty",
		Content:  "",
		Language: "go",
	}
	got := EnrichForEmbedding(chunk)
	if !strings.HasPrefix(got, "[go:function]") {
		t.Errorf("should still produce prefix for empty content, got: %q", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
