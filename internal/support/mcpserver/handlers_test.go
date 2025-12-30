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
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic (defaults to json+min)",
			args: map[string]interface{}{
				"paths":   []interface{}{"file1.txt", "dir/file2.txt"},
				"verbose": true,
			},
			want: []string{"multiexists", "file1.txt", "dir/file2.txt", "--verbose", "--no-fail", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{
				"paths": []interface{}{"file.txt"},
				"json":  false,
				"min":   false,
			},
			want: []string{"multiexists", "file.txt", "--no-fail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMultiexistsArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMultiexistsArgs() = %v, want %v", got, tt.want)
			}
		})
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
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic (defaults to json+min)",
			args: map[string]interface{}{
				"mode":      "checkboxes",
				"path":      "README.md",
				"recursive": true,
			},
			want: []string{"count", "--mode", "checkboxes", "--path", "README.md", "--recursive", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{
				"mode": "lines",
				"path": "file.txt",
				"json": false,
				"min":  false,
			},
			want: []string{"count", "--mode", "lines", "--path", "file.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCountArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildCountArgs() = %v, want %v", got, tt.want)
			}
		})
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
	// With explicit json=true, min=true (the defaults)
	args := map[string]interface{}{
		"path":         "/repo",
		"include_diff": true,
		"max_commits":  float64(5),
		"json":         true,
		"min":          true,
	}

	got := buildGitContextArgs(args)
	want := []string{"git-context", "--path", "/repo", "--include-diff", "--max-commits", "5", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildGitContextArgs() = %v, want %v", got, want)
	}

	// Verify flags can be disabled
	argsNoFlags := map[string]interface{}{
		"path": "/repo",
		"json": false,
		"min":  false,
	}
	gotNoFlags := buildGitContextArgs(argsNoFlags)
	wantNoFlags := []string{"git-context", "--path", "/repo"}
	if !reflect.DeepEqual(gotNoFlags, wantNoFlags) {
		t.Errorf("buildGitContextArgs(disabled) = %v, want %v", gotNoFlags, wantNoFlags)
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
			name: "empty (defaults to json+min)",
			args: map[string]interface{}{},
			want: []string{"discover-tests", "--json", "--min"},
		},
		{
			name: "with path (defaults to json+min)",
			args: map[string]interface{}{"path": "/project"},
			want: []string{"discover-tests", "--path", "/project", "--json", "--min"},
		},
		{
			name: "with json explicitly true",
			args: map[string]interface{}{"json": true, "min": true},
			want: []string{"discover-tests", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"json": false, "min": false},
			want: []string{"discover-tests"},
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
			name: "basic (defaults to json+min)",
			args: map[string]interface{}{"file": "story.md"},
			want: []string{"analyze-deps", "story.md", "--json", "--min"},
		},
		{
			name: "with json explicitly true",
			args: map[string]interface{}{"file": "story.md", "json": true, "min": true},
			want: []string{"analyze-deps", "story.md", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"file": "story.md", "json": false, "min": false},
			want: []string{"analyze-deps", "story.md"},
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
			name: "empty (defaults to json+min)",
			args: map[string]interface{}{},
			want: []string{"detect", "--json", "--min"},
		},
		{
			name: "with path (defaults to json+min)",
			args: map[string]interface{}{"path": "/project"},
			want: []string{"detect", "--path", "/project", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"json": false, "min": false},
			want: []string{"detect"},
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
			name: "basic (defaults to json+min)",
			args: map[string]interface{}{"manifest": "package.json"},
			want: []string{"deps", "package.json", "--json", "--min"},
		},
		{
			name: "with type (defaults to json+min)",
			args: map[string]interface{}{"manifest": "package.json", "type": "prod"},
			want: []string{"deps", "package.json", "--type", "prod", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"manifest": "package.json", "json": false, "min": false},
			want: []string{"deps", "package.json"},
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
			name: "basic (defaults to json+min)",
			args: map[string]interface{}{"path": "/plan"},
			want: []string{"validate-plan", "--path", "/plan", "--json", "--min"},
		},
		{
			name: "with json explicitly true",
			args: map[string]interface{}{"path": "/plan", "json": true, "min": true},
			want: []string{"validate-plan", "--path", "/plan", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"path": "/plan", "json": false, "min": false},
			want: []string{"validate-plan", "--path", "/plan"},
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
			name: "with stories (defaults to json+min)",
			args: map[string]interface{}{"stories": "/stories"},
			want: []string{"partition-work", "--stories", "/stories", "--json", "--min"},
		},
		{
			name: "with tasks (defaults to json+min)",
			args: map[string]interface{}{"tasks": "/tasks"},
			want: []string{"partition-work", "--tasks", "/tasks", "--json", "--min"},
		},
		{
			name: "with verbose and json+min",
			args: map[string]interface{}{"stories": "/stories", "verbose": true, "json": true, "min": true},
			want: []string{"partition-work", "--stories", "/stories", "--verbose", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"stories": "/stories", "json": false, "min": false},
			want: []string{"partition-work", "--stories", "/stories"},
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
			name: "basic (defaults to json+min)",
			args: map[string]interface{}{"context": "find auth code"},
			want: []string{"extract-relevant", "--context", "find auth code", "--json", "--min"},
		},
		{
			name: "with path (defaults to json+min)",
			args: map[string]interface{}{"context": "query", "path": "/src"},
			want: []string{"extract-relevant", "--path", "/src", "--context", "query", "--json", "--min"},
		},
		{
			name: "with concurrency and timeout (defaults to json+min)",
			args: map[string]interface{}{"context": "query", "concurrency": float64(4), "timeout": float64(30)},
			want: []string{"extract-relevant", "--context", "query", "--concurrency", "4", "--timeout", "30", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"context": "query", "json": false, "min": false},
			want: []string{"extract-relevant", "--context", "query"},
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
	// Note: Many commands now default to --json --min, so expected lengths are higher
	tests := []struct {
		command string
		args    map[string]interface{}
		wantLen int // Just verify length since we test individual functions above
	}{
		{"tree", map[string]interface{}{"path": "."}, 3},
		{"grep", map[string]interface{}{"pattern": "x", "paths": []interface{}{"f"}}, 3},
		{"multiexists", map[string]interface{}{"paths": []interface{}{"a"}}, 5}, // now includes --json --min
		{"json_query", map[string]interface{}{"file": "f", "query": "q"}, 4},
		{"markdown_headers", map[string]interface{}{"file": "f"}, 3},
		{"template", map[string]interface{}{"file": "f"}, 2},
		{"discover_tests", map[string]interface{}{}, 3},                    // now includes --json --min
		{"multigrep", map[string]interface{}{"keywords": "a,b"}, 5},        // now includes --json --min
		{"analyze_deps", map[string]interface{}{"file": "f"}, 4},           // now includes --json --min
		{"detect", map[string]interface{}{}, 3},                            // now includes --json --min
		{"count", map[string]interface{}{"mode": "lines", "path": "f"}, 6}, // now includes --json --min
		{"summarize_dir", map[string]interface{}{"path": "p"}, 2},
		{"deps", map[string]interface{}{"manifest": "m"}, 4},          // now includes --json --min
		{"git_context", map[string]interface{}{}, 3},                  // now includes --json --min
		{"validate_plan", map[string]interface{}{"path": "p"}, 4},     // now includes --json --min
		{"partition_work", map[string]interface{}{"stories": "s"}, 5}, // now includes --json --min
		{"repo_root", map[string]interface{}{}, 1},
		{"extract_relevant", map[string]interface{}{"context": "c"}, 5}, // now includes --json --min
		{"plan_type", map[string]interface{}{}, 3},                      // now includes --json --min
		{"git_changes", map[string]interface{}{}, 3},                    // now includes --json --min
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

func TestNormalizeArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "nil args",
			args: nil,
			want: nil,
		},
		{
			name: "empty args",
			args: map[string]interface{}{},
			want: map[string]interface{}{},
		},
		{
			name: "no aliases needed",
			args: map[string]interface{}{"path": "/tmp", "recursive": true},
			want: map[string]interface{}{"path": "/tmp", "recursive": true},
		},
		{
			name: "target -> path alias",
			args: map[string]interface{}{"target": "/tmp", "mode": "files"},
			want: map[string]interface{}{"path": "/tmp", "mode": "files"},
		},
		{
			name: "dir -> path alias",
			args: map[string]interface{}{"dir": "/home", "depth": 3},
			want: map[string]interface{}{"path": "/home", "depth": 3},
		},
		{
			name: "template -> file alias",
			args: map[string]interface{}{"template": "my.tmpl", "vars": map[string]interface{}{"x": 1}},
			want: map[string]interface{}{"file": "my.tmpl", "vars": map[string]interface{}{"x": 1}},
		},
		{
			name: "canonical takes precedence over alias",
			args: map[string]interface{}{"path": "/correct", "target": "/ignored"},
			want: map[string]interface{}{"path": "/correct", "target": "/ignored"},
		},
		{
			name: "package -> manifest alias",
			args: map[string]interface{}{"package": "package.json"},
			want: map[string]interface{}{"manifest": "package.json"},
		},
		{
			name: "regex -> pattern alias",
			args: map[string]interface{}{"regex": "TODO.*"},
			want: map[string]interface{}{"pattern": "TODO.*"},
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

func TestNormalizeArgsIntegration(t *testing.T) {
	// Test that aliases work end-to-end through buildArgs
	tests := []struct {
		name    string
		command string
		args    map[string]interface{}
		wantArg string // Check that this argument appears in output
	}{
		{
			name:    "count with target alias",
			command: "count",
			args:    map[string]interface{}{"mode": "checkboxes", "target": "/readme.md"},
			wantArg: "--path",
		},
		{
			name:    "tree with dir alias",
			command: "tree",
			args:    map[string]interface{}{"dir": "/src"},
			wantArg: "--path",
		},
		{
			name:    "template with template alias for file",
			command: "template",
			args:    map[string]interface{}{"template": "my.tmpl"},
			wantArg: "my.tmpl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildArgs(tt.command, tt.args)
			if err != nil {
				t.Fatalf("buildArgs() error = %v", err)
			}
			found := false
			for _, arg := range got {
				if arg == tt.wantArg {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("buildArgs() = %v, expected to contain %q", got, tt.wantArg)
			}
		})
	}
}

func TestBuildPlanTypeArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "empty (defaults to json+min)",
			args: map[string]interface{}{},
			want: []string{"plan-type", "--json", "--min"},
		},
		{
			name: "with path (defaults to json+min)",
			args: map[string]interface{}{"path": "/plan"},
			want: []string{"plan-type", "--path", "/plan", "--json", "--min"},
		},
		{
			name: "with all flags explicitly",
			args: map[string]interface{}{"path": "/plan", "json": true, "min": true},
			want: []string{"plan-type", "--path", "/plan", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"json": false, "min": false},
			want: []string{"plan-type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPlanTypeArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPlanTypeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildGitChangesArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "empty (defaults to json+min)",
			args: map[string]interface{}{},
			want: []string{"git-changes", "--json", "--min"},
		},
		{
			name: "with path (defaults to json+min)",
			args: map[string]interface{}{"path": ".planning/"},
			want: []string{"git-changes", "--path", ".planning/", "--json", "--min"},
		},
		{
			name: "exclude untracked (defaults to json+min)",
			args: map[string]interface{}{"include_untracked": false},
			want: []string{"git-changes", "--include-untracked=false", "--json", "--min"},
		},
		{
			name: "staged only (defaults to json+min)",
			args: map[string]interface{}{"staged_only": true},
			want: []string{"git-changes", "--staged-only", "--json", "--min"},
		},
		{
			name: "all options",
			args: map[string]interface{}{"path": ".planning/", "include_untracked": false, "staged_only": true, "json": true, "min": true},
			want: []string{"git-changes", "--path", ".planning/", "--include-untracked=false", "--staged-only", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"json": false, "min": false},
			want: []string{"git-changes"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildGitChangesArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildGitChangesArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildContextArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "init operation (defaults to json+min)",
			args: map[string]interface{}{
				"operation": "init",
				"dir":       "/tmp/mycontext",
			},
			want: []string{"context", "init", "--dir", "/tmp/mycontext", "--json", "--min"},
		},
		{
			name: "set operation",
			args: map[string]interface{}{
				"operation": "set",
				"dir":       "/tmp/mycontext",
				"key":       "MY_VAR",
				"value":     "hello world",
			},
			want: []string{"context", "set", "--dir", "/tmp/mycontext", "MY_VAR", "hello world"},
		},
		{
			name: "get operation (defaults to json+min)",
			args: map[string]interface{}{
				"operation": "get",
				"dir":       "/tmp/mycontext",
				"key":       "MY_VAR",
			},
			want: []string{"context", "get", "--dir", "/tmp/mycontext", "MY_VAR", "--json", "--min"},
		},
		{
			name: "get with default (defaults to json+min)",
			args: map[string]interface{}{
				"operation": "get",
				"dir":       "/tmp/mycontext",
				"key":       "MISSING",
				"default":   "fallback",
			},
			want: []string{"context", "get", "--dir", "/tmp/mycontext", "MISSING", "--default", "fallback", "--json", "--min"},
		},
		{
			name: "get with json+min disabled",
			args: map[string]interface{}{
				"operation": "get",
				"dir":       "/tmp/mycontext",
				"key":       "MY_VAR",
				"json":      false,
				"min":       false,
			},
			want: []string{"context", "get", "--dir", "/tmp/mycontext", "MY_VAR"},
		},
		{
			name: "list operation (defaults to json+min)",
			args: map[string]interface{}{
				"operation": "list",
				"dir":       "/tmp/mycontext",
			},
			want: []string{"context", "list", "--dir", "/tmp/mycontext", "--json", "--min"},
		},
		{
			name: "list with json+min disabled",
			args: map[string]interface{}{
				"operation": "list",
				"dir":       "/tmp/mycontext",
				"json":      false,
				"min":       false,
			},
			want: []string{"context", "list", "--dir", "/tmp/mycontext"},
		},
		{
			name: "dump operation",
			args: map[string]interface{}{
				"operation": "dump",
				"dir":       "/tmp/mycontext",
			},
			want: []string{"context", "dump", "--dir", "/tmp/mycontext"},
		},
		{
			name: "clear operation",
			args: map[string]interface{}{
				"operation": "clear",
				"dir":       "/tmp/mycontext",
			},
			want: []string{"context", "clear", "--dir", "/tmp/mycontext"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildContextArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildContextArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildYamlGetArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic get (defaults to json+min)",
			args: map[string]interface{}{"file": "/tmp/config.yaml", "key": "helper.llm"},
			want: []string{"yaml", "get", "--file", "/tmp/config.yaml", "helper.llm", "--json", "--min"},
		},
		{
			name: "with default value",
			args: map[string]interface{}{"file": "/tmp/config.yaml", "key": "missing.key", "default": "fallback"},
			want: []string{"yaml", "get", "--file", "/tmp/config.yaml", "missing.key", "--default", "fallback", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"file": "/tmp/config.yaml", "key": "helper.llm", "json": false, "min": false},
			want: []string{"yaml", "get", "--file", "/tmp/config.yaml", "helper.llm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildYamlGetArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildYamlGetArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildYamlSetArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic set (defaults to json+min)",
			args: map[string]interface{}{"file": "/tmp/config.yaml", "key": "helper.llm", "value": "claude"},
			want: []string{"yaml", "set", "--file", "/tmp/config.yaml", "helper.llm", "claude", "--json", "--min"},
		},
		{
			name: "with create flag",
			args: map[string]interface{}{"file": "/tmp/config.yaml", "key": "new.key", "value": "newvalue", "create": true},
			want: []string{"yaml", "set", "--file", "/tmp/config.yaml", "new.key", "newvalue", "--create", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"file": "/tmp/config.yaml", "key": "helper.llm", "value": "claude", "json": false, "min": false},
			want: []string{"yaml", "set", "--file", "/tmp/config.yaml", "helper.llm", "claude"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildYamlSetArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildYamlSetArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildYamlMultigetArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic multiget (defaults to json+min)",
			args: map[string]interface{}{
				"file": "/tmp/config.yaml",
				"keys": []interface{}{"helper.llm", "project.type"},
			},
			want: []string{"yaml", "multiget", "--file", "/tmp/config.yaml", "helper.llm", "project.type", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{
				"file": "/tmp/config.yaml",
				"keys": []interface{}{"helper.llm"},
				"json": false,
				"min":  false,
			},
			want: []string{"yaml", "multiget", "--file", "/tmp/config.yaml", "helper.llm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildYamlMultigetArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildYamlMultigetArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildYamlMultisetArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           map[string]interface{}
		wantPrefix     []string
		wantContains   []string
		wantSuffix     []string
		skipDeepEqual  bool
	}{
		{
			name: "basic multiset (defaults to json+min)",
			args: map[string]interface{}{
				"file": "/tmp/config.yaml",
				"pairs": map[string]interface{}{
					"helper.llm": "claude",
				},
			},
			wantPrefix:    []string{"yaml", "multiset", "--file", "/tmp/config.yaml"},
			wantContains:  []string{"helper.llm", "claude"},
			wantSuffix:    []string{"--json", "--min"},
			skipDeepEqual: true,
		},
		{
			name: "with create flag",
			args: map[string]interface{}{
				"file": "/tmp/config.yaml",
				"pairs": map[string]interface{}{
					"key": "value",
				},
				"create": true,
			},
			wantPrefix:    []string{"yaml", "multiset", "--file", "/tmp/config.yaml"},
			wantContains:  []string{"key", "value", "--create"},
			wantSuffix:    []string{"--json", "--min"},
			skipDeepEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildYamlMultisetArgs(tt.args)

			// Check prefix
			for i, want := range tt.wantPrefix {
				if i >= len(got) || got[i] != want {
					t.Errorf("buildYamlMultisetArgs() prefix mismatch at %d: got %v, want prefix %v", i, got, tt.wantPrefix)
					return
				}
			}

			// Check that expected elements are present
			for _, want := range tt.wantContains {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildYamlMultisetArgs() = %v, missing expected element %q", got, want)
				}
			}
		})
	}
}
