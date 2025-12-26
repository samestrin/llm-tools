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
		"int_val":    42,
		"float_val":  float64(3.14),
		"int64_val":  int64(100),
		"string_val": "42",
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

func TestBuildMarkdownHeadersArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"file": "README.md"},
			want: []string{"markdown", "headers", "README.md"},
		},
		{
			name: "with level",
			args: map[string]interface{}{"file": "README.md", "level": "2,3"},
			want: []string{"markdown", "headers", "README.md", "--level", "2,3"},
		},
		{
			name: "with plain",
			args: map[string]interface{}{"file": "README.md", "plain": true},
			want: []string{"markdown", "headers", "README.md", "--plain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMarkdownHeadersArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMarkdownHeadersArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildTemplateArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"file": "template.txt"},
			want: []string{"template", "template.txt"},
		},
		{
			name: "with syntax",
			args: map[string]interface{}{"file": "template.txt", "syntax": "braces"},
			want: []string{"template", "template.txt", "--syntax", "braces"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTemplateArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildTemplateArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDiscoverTestsArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "empty",
			args: map[string]interface{}{},
			want: []string{"discover-tests"},
		},
		{
			name: "with path",
			args: map[string]interface{}{"path": "/project"},
			want: []string{"discover-tests", "--path", "/project"},
		},
		{
			name: "with json",
			args: map[string]interface{}{"json": true},
			want: []string{"discover-tests", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDiscoverTestsArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDiscoverTestsArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAnalyzeDepsArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"file": "story.md"},
			want: []string{"analyze-deps", "story.md"},
		},
		{
			name: "with json",
			args: map[string]interface{}{"file": "story.md", "json": true},
			want: []string{"analyze-deps", "story.md", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAnalyzeDepsArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildAnalyzeDepsArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDetectArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "empty",
			args: map[string]interface{}{},
			want: []string{"detect"},
		},
		{
			name: "with path",
			args: map[string]interface{}{"path": "/project"},
			want: []string{"detect", "--path", "/project"},
		},
		{
			name: "with json",
			args: map[string]interface{}{"json": true},
			want: []string{"detect", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDetectArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDetectArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSummarizeDirArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"path": "/dir"},
			want: []string{"summarize-dir", "--path", "/dir"},
		},
		{
			name: "with format",
			args: map[string]interface{}{"path": "/dir", "format": "headers"},
			want: []string{"summarize-dir", "--path", "/dir", "--format", "headers"},
		},
		{
			name: "with max_tokens",
			args: map[string]interface{}{"path": "/dir", "max_tokens": float64(1000)},
			want: []string{"summarize-dir", "--path", "/dir", "--max-tokens", "1000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSummarizeDirArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildSummarizeDirArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDepsArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"manifest": "package.json"},
			want: []string{"deps", "package.json"},
		},
		{
			name: "with type",
			args: map[string]interface{}{"manifest": "package.json", "type": "prod"},
			want: []string{"deps", "package.json", "--type", "prod"},
		},
		{
			name: "with json",
			args: map[string]interface{}{"manifest": "package.json", "json": true},
			want: []string{"deps", "package.json", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDepsArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDepsArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildValidatePlanArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"path": "/plan"},
			want: []string{"validate-plan", "--path", "/plan"},
		},
		{
			name: "with json",
			args: map[string]interface{}{"path": "/plan", "json": true},
			want: []string{"validate-plan", "--path", "/plan", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildValidatePlanArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildValidatePlanArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPartitionWorkArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "with stories",
			args: map[string]interface{}{"stories": "/stories"},
			want: []string{"partition-work", "--stories", "/stories"},
		},
		{
			name: "with tasks",
			args: map[string]interface{}{"tasks": "/tasks"},
			want: []string{"partition-work", "--tasks", "/tasks"},
		},
		{
			name: "with verbose and json",
			args: map[string]interface{}{"stories": "/stories", "verbose": true, "json": true},
			want: []string{"partition-work", "--stories", "/stories", "--verbose", "--json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPartitionWorkArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPartitionWorkArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRepoRootArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "empty",
			args: map[string]interface{}{},
			want: []string{"repo-root"},
		},
		{
			name: "with path",
			args: map[string]interface{}{"path": "/subdir"},
			want: []string{"repo-root", "--path", "/subdir"},
		},
		{
			name: "with validate",
			args: map[string]interface{}{"validate": true},
			want: []string{"repo-root", "--validate"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRepoRootArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildRepoRootArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildExtractRelevantArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic",
			args: map[string]interface{}{"context": "find auth code"},
			want: []string{"extract-relevant", "--context", "find auth code"},
		},
		{
			name: "with path",
			args: map[string]interface{}{"context": "query", "path": "/src"},
			want: []string{"extract-relevant", "--path", "/src", "--context", "query"},
		},
		{
			name: "with concurrency and timeout",
			args: map[string]interface{}{"context": "query", "concurrency": float64(4), "timeout": float64(30)},
			want: []string{"extract-relevant", "--context", "query", "--concurrency", "4", "--timeout", "30"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExtractRelevantArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildExtractRelevantArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildArgsDispatcher(t *testing.T) {
	tests := []struct {
		command string
		args    map[string]interface{}
		wantLen int // Just verify length since we test individual functions above
	}{
		{"tree", map[string]interface{}{"path": "."}, 3},
		{"grep", map[string]interface{}{"pattern": "x", "paths": []interface{}{"f"}}, 3},
		{"multiexists", map[string]interface{}{"paths": []interface{}{"a"}}, 3},
		{"json_query", map[string]interface{}{"file": "f", "query": "q"}, 4},
		{"markdown_headers", map[string]interface{}{"file": "f"}, 3},
		{"template", map[string]interface{}{"file": "f"}, 2},
		{"discover_tests", map[string]interface{}{}, 1},
		{"multigrep", map[string]interface{}{"keywords": "a,b"}, 3},
		{"analyze_deps", map[string]interface{}{"file": "f"}, 2},
		{"detect", map[string]interface{}{}, 1},
		{"count", map[string]interface{}{"mode": "lines", "target": "f"}, 4},
		{"summarize_dir", map[string]interface{}{"path": "p"}, 2},
		{"deps", map[string]interface{}{"manifest": "m"}, 2},
		{"git_context", map[string]interface{}{}, 1},
		{"validate_plan", map[string]interface{}{"path": "p"}, 2},
		{"partition_work", map[string]interface{}{"stories": "s"}, 3},
		{"repo_root", map[string]interface{}{}, 1},
		{"extract_relevant", map[string]interface{}{"context": "c"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got, err := buildArgs(tt.command, tt.args)
			if err != nil {
				t.Errorf("buildArgs(%s) error = %v", tt.command, err)
				return
			}
			if len(got) < tt.wantLen {
				t.Errorf("buildArgs(%s) returned %d args, want at least %d", tt.command, len(got), tt.wantLen)
			}
		})
	}
}

func TestBuildArgsUnknownCommand(t *testing.T) {
	_, err := buildArgs("unknown_command", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for unknown command")
	}
}
