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

func TestServerAllowedDirs(t *testing.T) {
	allowedDirs := []string{"/tmp", "/home"}
	server, _ := NewServer(allowedDirs)

	got := server.AllowedDirs()
	if len(got) != len(allowedDirs) {
		t.Errorf("AllowedDirs() length = %v, want %v", len(got), len(allowedDirs))
	}
	for i, dir := range allowedDirs {
		if got[i] != dir {
			t.Errorf("AllowedDirs()[%d] = %v, want %v", i, got[i], dir)
		}
	}
}

func TestServerAllowedDirsEmpty(t *testing.T) {
	server, _ := NewServer(nil)
	got := server.AllowedDirs()
	if got != nil && len(got) != 0 {
		t.Errorf("AllowedDirs() for nil input should be empty, got %v", got)
	}
}

func TestServerMultipleAllowedDirs(t *testing.T) {
	allowedDirs := []string{"/tmp", "/home", "/var"}
	server, _ := NewServer(allowedDirs)

	if server.ToolCount() != 27 {
		t.Errorf("ToolCount() = %d, want 27", server.ToolCount())
	}

	if server.Name() != "llm-filesystem" {
		t.Errorf("Name() = %q, want %q", server.Name(), "llm-filesystem")
	}
}
