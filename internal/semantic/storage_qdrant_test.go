package semantic

import (
	"os"
	"testing"
)

// TestQdrantStorage runs the standard storage test suite against Qdrant
// Skips if QDRANT_API_KEY and QDRANT_API_URL are not set
func TestQdrantStorage(t *testing.T) {
	apiKey := os.Getenv("QDRANT_API_KEY")
	apiURL := os.Getenv("QDRANT_API_URL")

	if apiKey == "" || apiURL == "" {
		t.Skip("Skipping Qdrant tests: QDRANT_API_KEY and QDRANT_API_URL not set")
	}

	StorageTestSuite(t, func() (Storage, func()) {
		config := QdrantConfig{
			APIKey:         apiKey,
			URL:            apiURL,
			CollectionName: "llm_semantic_test",
			EmbeddingDim:   4, // Small dimension for tests
		}

		storage, err := NewQdrantStorage(config)
		if err != nil {
			t.Fatalf("Failed to create Qdrant storage: %v", err)
		}

		// Cleanup function deletes the test collection
		cleanup := func() {
			storage.DeleteCollection()
			storage.Close()
		}

		return storage, cleanup
	})
}

func TestQdrantStorage_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  QdrantConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: QdrantConfig{
				APIKey:         "test-key",
				URL:            "https://example.qdrant.io:6334",
				CollectionName: "test",
				EmbeddingDim:   1024,
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: QdrantConfig{
				URL:            "https://example.qdrant.io:6334",
				CollectionName: "test",
				EmbeddingDim:   1024,
			},
			wantErr: true,
		},
		{
			name: "missing URL",
			config: QdrantConfig{
				APIKey:         "test-key",
				CollectionName: "test",
				EmbeddingDim:   1024,
			},
			wantErr: true,
		},
		{
			name: "missing embedding dimension",
			config: QdrantConfig{
				APIKey:         "test-key",
				URL:            "https://example.qdrant.io:6334",
				CollectionName: "test",
			},
			wantErr: true,
		},
		{
			name: "default collection name",
			config: QdrantConfig{
				APIKey:       "test-key",
				URL:          "https://example.qdrant.io:6334",
				EmbeddingDim: 1024,
			},
			wantErr: false, // Should use default collection name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("QdrantConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestQdrantStorage_ParseURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
		wantPort int
		wantTLS  bool
		wantErr  bool
	}{
		{
			name:     "https URL with port",
			url:      "https://abc123.us-west-1-0.aws.cloud.qdrant.io:6334",
			wantHost: "abc123.us-west-1-0.aws.cloud.qdrant.io",
			wantPort: 6334,
			wantTLS:  true,
			wantErr:  false,
		},
		{
			name:     "http URL with port",
			url:      "http://localhost:6334",
			wantHost: "localhost",
			wantPort: 6334,
			wantTLS:  false,
			wantErr:  false,
		},
		{
			name:     "https URL without port",
			url:      "https://abc123.qdrant.io",
			wantHost: "abc123.qdrant.io",
			wantPort: 6334, // default port
			wantTLS:  true,
			wantErr:  false,
		},
		{
			name:    "invalid URL",
			url:     "not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, useTLS, err := parseQdrantURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseQdrantURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if host != tt.wantHost {
				t.Errorf("parseQdrantURL() host = %v, want %v", host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("parseQdrantURL() port = %v, want %v", port, tt.wantPort)
			}
			if useTLS != tt.wantTLS {
				t.Errorf("parseQdrantURL() useTLS = %v, want %v", useTLS, tt.wantTLS)
			}
		})
	}
}
