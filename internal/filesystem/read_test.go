package filesystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testContent := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string // Check for multiple substrings
		wantErr      bool
	}{
		{
			name: "read entire file",
			args: map[string]interface{}{
				"path": testFile,
			},
			wantContains: []string{"line 1", "line 2", "line 5"},
			wantErr:      false,
		},
		{
			name: "read with line range",
			args: map[string]interface{}{
				"path":       testFile,
				"line_start": float64(2),
				"line_count": float64(2),
			},
			wantContains: []string{"line 2", "line 3"},
			wantErr:      false,
		},
		{
			name: "read empty file",
			args: map[string]interface{}{
				"path": emptyFile,
			},
			wantContains: []string{`"content":""`},
			wantErr:      false,
		},
		{
			name: "read missing file",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantErr: true,
		},
		{
			name: "read with byte offset",
			args: map[string]interface{}{
				"path":         testFile,
				"start_offset": float64(7), // Start at "line 2"
				"max_size":     float64(6), // Read "line 2"
			},
			wantContains: []string{"line 2"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleReadFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for _, want := range tt.wantContains {
					if !strings.Contains(result, want) {
						t.Errorf("handleReadFile() = %v, want to contain %v", result, want)
					}
				}
			}
		})
	}
}

func TestReadMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("content 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content 2"), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name: "read multiple files",
			args: map[string]interface{}{
				"paths": []interface{}{file1, file2},
			},
			wantContains: []string{"content 1", "content 2"},
			wantErr:      false,
		},
		{
			name: "read with one missing file",
			args: map[string]interface{}{
				"paths": []interface{}{file1, filepath.Join(tmpDir, "missing.txt")},
			},
			// Should still return content for file1, with error for missing
			wantContains: []string{"content 1"},
			wantErr:      false, // Partial success is not an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleReadMultipleFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadMultipleFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleReadMultipleFiles() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestExtractLines(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file with numbered lines
	testFile := filepath.Join(tmpDir, "extract_test.txt")
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nTODO: fix this\nline 7\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains []string
		wantErr      bool
	}{
		{
			name: "extract by line numbers",
			args: map[string]interface{}{
				"path":         testFile,
				"line_numbers": []interface{}{float64(1), float64(3)},
			},
			wantContains: []string{"line 1", "line 3"},
			wantErr:      false,
		},
		{
			name: "extract by range",
			args: map[string]interface{}{
				"path":       testFile,
				"start_line": float64(2),
				"end_line":   float64(4),
			},
			wantContains: []string{"line 2", "line 3", "line 4"},
			wantErr:      false,
		},
		{
			name: "extract by pattern",
			args: map[string]interface{}{
				"path":    testFile,
				"pattern": "TODO",
			},
			wantContains: []string{"TODO"},
			wantErr:      false,
		},
		{
			name: "extract with context",
			args: map[string]interface{}{
				"path":          testFile,
				"pattern":       "line 3",
				"context_lines": float64(1),
			},
			wantContains: []string{"line 2", "line 3", "line 4"},
			wantErr:      false,
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleExtractLines(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleExtractLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("handleExtractLines() = %v, want to contain %v", result, want)
				}
			}
		})
	}
}

func TestGetIntSlice(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		want int // length of result
	}{
		{
			name: "get int slice from float64",
			args: map[string]interface{}{"nums": []interface{}{float64(1), float64(2), float64(3)}},
			key:  "nums",
			want: 3,
		},
		{
			name: "get int slice from int",
			args: map[string]interface{}{"nums": []interface{}{1, 2}},
			key:  "nums",
			want: 2,
		},
		{
			name: "missing key returns nil",
			args: map[string]interface{}{},
			key:  "nums",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetIntSlice(tt.args, tt.key)
			if len(got) != tt.want {
				t.Errorf("GetIntSlice() length = %v, want %v", len(got), tt.want)
			}
		})
	}
}

func TestReadFilePathSecurity(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file in allowed dir
	allowedFile := filepath.Join(tmpDir, "allowed.txt")
	if err := os.WriteFile(allowedFile, []byte("allowed content"), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "allowed path",
			args: map[string]interface{}{
				"path": allowedFile,
			},
			wantErr: false,
		},
		{
			name: "path outside allowed dirs",
			args: map[string]interface{}{
				"path": "/etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "path traversal attempt",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "..", "etc", "passwd"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleReadFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadFile() security error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadFileMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	_, err := server.handleReadFile(map[string]interface{}{})
	if err == nil {
		t.Error("handleReadFile() should error when path is missing")
	}
}

func TestReadFileByteRange(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a larger test file
	content := strings.Repeat("abcdefghij", 100) // 1000 bytes
	testFile := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name         string
		args         map[string]interface{}
		wantContains string
		wantErr      bool
	}{
		{
			name: "read from start with max_size",
			args: map[string]interface{}{
				"path":     testFile,
				"max_size": float64(10),
			},
			wantContains: "abcdefghij",
			wantErr:      false,
		},
		{
			name: "read from offset",
			args: map[string]interface{}{
				"path":         testFile,
				"start_offset": float64(10),
				"max_size":     float64(10),
			},
			wantContains: "abcdefghij",
			wantErr:      false,
		},
		{
			name: "read with end_offset",
			args: map[string]interface{}{
				"path":         testFile,
				"start_offset": float64(0),
				"end_offset":   float64(5),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.handleReadFile(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.wantContains != "" && !strings.Contains(result, tt.wantContains) {
				t.Errorf("handleReadFile() = %v, want to contain %v", result, tt.wantContains)
			}
		})
	}
}

func TestExtractLinesErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "nonexistent file",
			args: map[string]interface{}{
				"path":       filepath.Join(tmpDir, "nonexistent.txt"),
				"start_line": float64(1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleExtractLines(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleExtractLines() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadMultipleFilesErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	server, _ := NewServer([]string{tmpDir})

	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name: "empty paths",
			args: map[string]interface{}{
				"paths": []interface{}{},
			},
			wantErr: true, // Empty paths is considered an error
		},
		{
			name:    "missing paths key",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.handleReadMultipleFiles(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleReadMultipleFiles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadFileAutoChunking(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file larger than 1MB chunk size for testing
	// Use a smaller chunk for testing - we'll use max_size to simulate
	content := strings.Repeat("a", 500) // 500 bytes
	testFile := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	server, _ := NewServer([]string{tmpDir})

	t.Run("auto_chunk with max_size returns chunk metadata", func(t *testing.T) {
		result, err := server.handleReadFile(map[string]interface{}{
			"path":       testFile,
			"max_size":   float64(100),
			"auto_chunk": true,
		})
		if err != nil {
			t.Fatalf("handleReadFile() error = %v", err)
		}

		var resp ReadFileResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if !resp.AutoChunked {
			t.Error("expected auto_chunked=true")
		}
		if resp.ChunkIndex != 0 {
			t.Errorf("expected chunk_index=0, got %d", resp.ChunkIndex)
		}
		if resp.TotalChunks != 5 {
			t.Errorf("expected total_chunks=5 (500/100), got %d", resp.TotalChunks)
		}
		if !resp.HasMore {
			t.Error("expected has_more=true")
		}
		if resp.ContinuationToken == "" {
			t.Error("expected continuation_token when has_more=true")
		}
	})

	t.Run("continuation token reads next chunk", func(t *testing.T) {
		// Get first chunk
		result1, err := server.handleReadFile(map[string]interface{}{
			"path":       testFile,
			"max_size":   float64(100),
			"auto_chunk": true,
		})
		if err != nil {
			t.Fatalf("handleReadFile() error = %v", err)
		}

		var resp1 ReadFileResult
		if err := json.Unmarshal([]byte(result1), &resp1); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		// Use token to get second chunk
		result2, err := server.handleReadFile(map[string]interface{}{
			"path":               testFile,
			"max_size":           float64(100),
			"auto_chunk":         true,
			"continuation_token": resp1.ContinuationToken,
		})
		if err != nil {
			t.Fatalf("handleReadFile() with token error = %v", err)
		}

		var resp2 ReadFileResult
		if err := json.Unmarshal([]byte(result2), &resp2); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp2.ChunkIndex != 1 {
			t.Errorf("expected chunk_index=1 for second chunk, got %d", resp2.ChunkIndex)
		}
	})

	t.Run("last chunk has no continuation token", func(t *testing.T) {
		// Read from near end
		result, err := server.handleReadFile(map[string]interface{}{
			"path":         testFile,
			"start_offset": float64(400),
			"max_size":     float64(100),
			"auto_chunk":   true,
		})
		if err != nil {
			t.Fatalf("handleReadFile() error = %v", err)
		}

		var resp ReadFileResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.HasMore {
			t.Error("expected has_more=false for last chunk")
		}
		if resp.ContinuationToken != "" {
			t.Error("expected no continuation_token on last chunk")
		}
	})

	t.Run("auto_chunk=false returns full content", func(t *testing.T) {
		smallFile := filepath.Join(tmpDir, "small.txt")
		if err := os.WriteFile(smallFile, []byte("small content"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := server.handleReadFile(map[string]interface{}{
			"path":       smallFile,
			"auto_chunk": false,
		})
		if err != nil {
			t.Fatalf("handleReadFile() error = %v", err)
		}

		var resp ReadFileResult
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if resp.AutoChunked {
			t.Error("expected auto_chunked=false")
		}
		if resp.Content != "small content" {
			t.Errorf("expected full content, got %q", resp.Content)
		}
	})
}
