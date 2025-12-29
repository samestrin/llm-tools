package filesystem

import (
	"testing"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name        string
		allowedDirs []string
		wantErr     bool
	}{
		{
			name:        "create server with allowed dirs",
			allowedDirs: []string{"/tmp", "/home"},
			wantErr:     false,
		},
		{
			name:        "create server with empty allowed dirs",
			allowedDirs: []string{},
			wantErr:     false,
		},
		{
			name:        "create server with nil allowed dirs",
			allowedDirs: nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.allowedDirs)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && server == nil {
				t.Error("NewServer() returned nil server")
			}
		})
	}
}

func TestServerToolCount(t *testing.T) {
	server, err := NewServer([]string{"/tmp"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Should have 27 tools registered (25 v3.4.0 + 2 v3.5.1)
	count := server.ToolCount()
	if count != 27 {
		t.Errorf("ToolCount() = %d, want 27", count)
	}
}

func TestServerName(t *testing.T) {
	server, err := NewServer([]string{"/tmp"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if server.Name() != "llm-filesystem" {
		t.Errorf("Name() = %q, want %q", server.Name(), "llm-filesystem")
	}
}

func TestServerVersion(t *testing.T) {
	server, err := NewServer([]string{"/tmp"})
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if server.Version() == "" {
		t.Error("Version() returned empty string")
	}
}
