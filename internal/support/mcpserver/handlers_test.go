package mcpserver

import (
	"reflect"
	"testing"
)

func TestBuildTreeArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "empty args",
			args: map[string]interface{}{},
			want: []string{"tree"},
		},
		{
			name: "with path",
			args: map[string]interface{}{"path": "/tmp"},
			want: []string{"tree", "--path", "/tmp"},
		},
		{
			name: "with depth",
			args: map[string]interface{}{"depth": float64(3)}, // JSON numbers are float64
			want: []string{"tree", "--depth", "3"},
		},
		{
			name: "with all options",
			args: map[string]interface{}{
				"path":         "/home",
				"depth":        float64(5),
				"sizes":        true,
				"no_gitignore": true,
			},
			want: []string{"tree", "--path", "/home", "--depth", "5", "--sizes", "--no-gitignore"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTreeArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildTreeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildGrepArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic search",
			args: map[string]interface{}{
				"pattern": "TODO",
				"paths":   []interface{}{"src"},
			},
			want: []string{"grep", "TODO", "src"},
		},
		{
			name: "with flags",
			args: map[string]interface{}{
				"pattern":      "error",
				"paths":        []interface{}{".", "lib"},
				"ignore_case":  true,
				"line_numbers": true,
			},
			want: []string{"grep", "error", ".", "lib", "-i", "-n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGrepArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildGrepArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildMultiexistsArgs(t *testing.T) {
	args := map[string]interface{}{
		"paths":   []interface{}{"file1.txt", "dir/file2.txt"},
		"verbose": true,
	}

	got := buildMultiexistsArgs(args)
	want := []string{"multiexists", "file1.txt", "dir/file2.txt", "--verbose", "--no-fail"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildMultiexistsArgs() = %v, want %v", got, want)
	}
}

func TestBuildJSONQueryArgs(t *testing.T) {
	args := map[string]interface{}{
		"file":  "config.json",
		"query": ".database.host",
	}

	got := buildJSONQueryArgs(args)
	want := []string{"json", "query", "config.json", ".database.host"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildJSONQueryArgs() = %v, want %v", got, want)
	}
}

func TestBuildCountArgs(t *testing.T) {
	args := map[string]interface{}{
		"mode":      "checkboxes",
		"target":    "README.md",
		"recursive": true,
	}

	got := buildCountArgs(args)
	want := []string{"count", "--mode", "checkboxes", "README.md", "--recursive"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildCountArgs() = %v, want %v", got, want)
	}
}

func TestBuildMultigrepArgs(t *testing.T) {
	args := map[string]interface{}{
		"keywords":         "foo,bar,baz",
		"path":             "src",
		"extensions":       "ts,tsx",
		"max_per_keyword":  float64(10),
		"definitions_only": true,
		"json":             true,
	}

	got := buildMultigrepArgs(args)

	// Check key elements exist
	expected := []string{"multigrep", "--keywords", "foo,bar,baz", "--path", "src"}
	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("buildMultigrepArgs()[%d] = %v, want %v", i, got[i], exp)
		}
	}

	// Check flags
	hasDefinitionsOnly := false
	hasJSON := false
	for _, arg := range got {
		if arg == "--definitions-only" {
			hasDefinitionsOnly = true
		}
		if arg == "--json" {
			hasJSON = true
		}
	}
	if !hasDefinitionsOnly {
		t.Error("Expected --definitions-only flag")
	}
	if !hasJSON {
		t.Error("Expected --json flag")
	}
}

func TestBuildGitContextArgs(t *testing.T) {
	args := map[string]interface{}{
		"path":         "/repo",
		"include_diff": true,
		"max_commits":  float64(5),
		"json":         true,
	}

	got := buildGitContextArgs(args)
	want := []string{"git-context", "--path", "/repo", "--include-diff", "--max-commits", "5", "--json"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildGitContextArgs() = %v, want %v", got, want)
	}
}

func TestGetBool(t *testing.T) {
	args := map[string]interface{}{
		"present_true":  true,
		"present_false": false,
		"string_val":    "true",
	}

	if !getBool(args, "present_true") {
		t.Error("getBool(present_true) should be true")
	}
	if getBool(args, "present_false") {
		t.Error("getBool(present_false) should be false")
	}
	if getBool(args, "missing") {
		t.Error("getBool(missing) should be false")
	}
	if getBool(args, "string_val") {
		t.Error("getBool(string_val) should be false (not a bool)")
	}
}

func TestGetInt(t *testing.T) {
	args := map[string]interface{}{
		"int_val":     42,
		"float_val":   float64(3.14),
		"int64_val":   int64(100),
		"string_val":  "42",
	}

	if v, ok := getInt(args, "int_val"); !ok || v != 42 {
		t.Errorf("getInt(int_val) = %d, ok=%v, want 42", v, ok)
	}
	if v, ok := getInt(args, "float_val"); !ok || v != 3 {
		t.Errorf("getInt(float_val) = %d, ok=%v, want 3", v, ok)
	}
	if v, ok := getInt(args, "int64_val"); !ok || v != 100 {
		t.Errorf("getInt(int64_val) = %d, ok=%v, want 100", v, ok)
	}
	if _, ok := getInt(args, "string_val"); ok {
		t.Error("getInt(string_val) should return ok=false")
	}
	if _, ok := getInt(args, "missing"); ok {
		t.Error("getInt(missing) should return ok=false")
	}
}
