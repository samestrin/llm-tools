package mcpserver

import (
	"reflect"
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "file_path to path",
			args: map[string]interface{}{
				"file_path": "/some/path",
			},
			want: map[string]interface{}{
				"path": "/some/path",
			},
		},
		{
			name: "filepath to path",
			args: map[string]interface{}{
				"filepath": "/some/path",
			},
			want: map[string]interface{}{
				"path": "/some/path",
			},
		},
		{
			name: "file_paths to paths",
			args: map[string]interface{}{
				"file_paths": []string{"/a", "/b"},
			},
			want: map[string]interface{}{
				"paths": []string{"/a", "/b"},
			},
		},
		{
			name: "canonical path preserved",
			args: map[string]interface{}{
				"path": "/canonical/path",
			},
			want: map[string]interface{}{
				"path": "/canonical/path",
			},
		},
		{
			name: "canonical takes precedence over alias",
			args: map[string]interface{}{
				"path":      "/canonical/path",
				"file_path": "/aliased/path",
			},
			want: map[string]interface{}{
				"path":      "/canonical/path",
				"file_path": "/aliased/path",
			},
		},
		{
			name: "source alias src",
			args: map[string]interface{}{
				"src": "/source/path",
			},
			want: map[string]interface{}{
				"source": "/source/path",
			},
		},
		{
			name: "nil args returns nil",
			args: nil,
			want: nil,
		},
		{
			name: "empty args returns empty",
			args: map[string]interface{}{},
			want: map[string]interface{}{},
		},
		{
			name: "non-aliased params unchanged",
			args: map[string]interface{}{
				"content":   "some content",
				"recursive": true,
			},
			want: map[string]interface{}{
				"content":   "some content",
				"recursive": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildExtractLinesArgs_WithAlias(t *testing.T) {
	// Test that file_path alias works for extract_lines
	args := map[string]interface{}{
		"file_path": "/test/file.txt",
		"start":     float64(1),
		"end":       float64(10),
	}

	// Normalize first (as buildArgs does)
	normalized := normalizeArgs(args)
	cmdArgs := buildExtractLinesArgs(normalized)

	// Should contain --path /test/file.txt
	found := false
	for i, arg := range cmdArgs {
		if arg == "--path" && i+1 < len(cmdArgs) && cmdArgs[i+1] == "/test/file.txt" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected --path /test/file.txt in args, got %v", cmdArgs)
	}
}
