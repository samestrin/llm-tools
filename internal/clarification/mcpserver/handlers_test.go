package mcpserver

import (
	"reflect"
	"testing"
)

func TestBuildMatchArgs(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"question":     "What testing framework?",
		"entries_file": "tracking.yaml",
		"timeout":      float64(60),
	}

	got := buildMatchArgs(args)
	want := []string{"match-clarification", "--question", "What testing framework?", "--entries-file", "tracking.yaml", "--timeout", "60", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildMatchArgs() = %v, want %v", got, want)
	}

	// Test with json+min disabled
	argsNoFlags := map[string]interface{}{
		"question": "What testing framework?",
		"json":     false,
		"min":      false,
	}
	gotNoFlags := buildMatchArgs(argsNoFlags)
	wantNoFlags := []string{"match-clarification", "--question", "What testing framework?"}
	if !reflect.DeepEqual(gotNoFlags, wantNoFlags) {
		t.Errorf("buildMatchArgs(disabled) = %v, want %v", gotNoFlags, wantNoFlags)
	}
}

func TestBuildInitArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic init (defaults to json+min)",
			args: map[string]interface{}{"output": "tracking.yaml"},
			want: []string{"init-tracking", "--output", "tracking.yaml", "--json", "--min"},
		},
		{
			name: "with force (defaults to json+min)",
			args: map[string]interface{}{"output": "tracking.yaml", "force": true},
			want: []string{"init-tracking", "--output", "tracking.yaml", "--force", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"output": "tracking.yaml", "json": false, "min": false},
			want: []string{"init-tracking", "--output", "tracking.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInitArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildInitArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildAddArgs(t *testing.T) {
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"question":      "What is the testing approach?",
		"answer":        "Use Jest for unit tests",
		"sprint_id":     "sprint-1",
		"context_tags":  "testing,frontend",
	}

	got := buildAddArgs(args)

	// Check key elements exist
	expected := []string{"add-clarification", "--tracking-file", "tracking.yaml"}
	for i, exp := range expected {
		if got[i] != exp {
			t.Errorf("buildAddArgs()[%d] = %v, want %v", i, got[i], exp)
		}
	}

	// Check question is present
	hasQuestion := false
	for i, arg := range got {
		if arg == "--question" && i+1 < len(got) && got[i+1] == "What is the testing approach?" {
			hasQuestion = true
			break
		}
	}
	if !hasQuestion {
		t.Error("Expected --question flag with correct value")
	}
}

func TestBuildPromoteArgs(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"id":            "clarify-001",
		"target":        "CLAUDE.md",
		"force":         true,
	}

	got := buildPromoteArgs(args)
	want := []string{"promote-clarification", "--tracking-file", "tracking.yaml", "--id", "clarify-001", "--target", "CLAUDE.md", "--force", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildPromoteArgs() = %v, want %v", got, want)
	}
}

func TestBuildListArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic list (defaults to json+min)",
			args: map[string]interface{}{"tracking_file": "tracking.yaml"},
			want: []string{"list-entries", "tracking.yaml", "--json", "--min"},
		},
		{
			name: "with filters (defaults to json+min)",
			args: map[string]interface{}{
				"tracking_file":   "tracking.yaml",
				"status":          "pending",
				"min_occurrences": float64(3),
			},
			want: []string{"list-entries", "tracking.yaml", "--status", "pending", "--min-occurrences", "3", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"tracking_file": "tracking.yaml", "json": false, "min": false},
			want: []string{"list-entries", "tracking.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildListArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildListArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDetectConflictsArgs(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"timeout":       float64(45),
	}

	got := buildDetectConflictsArgs(args)
	want := []string{"detect-conflicts", "tracking.yaml", "--timeout", "45", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildDetectConflictsArgs() = %v, want %v", got, want)
	}
}

func TestBuildValidateArgs(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"context":       "React frontend project",
	}

	got := buildValidateArgs(args)
	want := []string{"validate-clarifications", "tracking.yaml", "--context", "React frontend project", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildValidateArgs() = %v, want %v", got, want)
	}
}

func TestBuildClusterArgs(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"questions_file": "questions.txt",
		"timeout":        float64(30),
	}

	got := buildClusterArgs(args)
	want := []string{"cluster-clarifications", "--questions-file", "questions.txt", "--timeout", "30", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildClusterArgs() = %v, want %v", got, want)
	}
}

func TestBuildClusterArgsWithJSON(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"questions_json": `["q1","q2"]`,
	}

	got := buildClusterArgs(args)
	want := []string{"cluster-clarifications", "--questions-json", `["q1","q2"]`, "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildClusterArgs() = %v, want %v", got, want)
	}
}

func TestBuildMatchArgsWithEntriesJSON(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"question":     "What framework?",
		"entries_json": `[{"q":"test","a":"answer"}]`,
	}

	got := buildMatchArgs(args)
	want := []string{"match-clarification", "--question", "What framework?", "--entries-json", `[{"q":"test","a":"answer"}]`, "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildMatchArgs() = %v, want %v", got, want)
	}
}

func TestBuildValidateArgsWithTimeout(t *testing.T) {
	// Default behavior includes --json --min
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"timeout":       float64(90),
	}

	got := buildValidateArgs(args)
	want := []string{"validate-clarifications", "tracking.yaml", "--timeout", "90", "--json", "--min"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildValidateArgs() = %v, want %v", got, want)
	}
}

func TestBuildAddArgsWithID(t *testing.T) {
	args := map[string]interface{}{
		"tracking_file": "tracking.yaml",
		"question":      "Test?",
		"answer":        "Yes",
		"id":            "custom-id-123",
		"check_match":   true,
	}

	got := buildAddArgs(args)

	// Check for id flag
	hasID := false
	hasCheckMatch := false
	for i, arg := range got {
		if arg == "--id" && i+1 < len(got) && got[i+1] == "custom-id-123" {
			hasID = true
		}
		if arg == "--check-match" {
			hasCheckMatch = true
		}
	}
	if !hasID {
		t.Error("Expected --id flag with correct value")
	}
	if !hasCheckMatch {
		t.Error("Expected --check-match flag")
	}
}

func TestBuildArgsDispatcher(t *testing.T) {
	tests := []struct {
		command   string
		args      map[string]interface{}
		wantLen   int
		wantFirst string
		wantErr   bool
	}{
		{"match_clarification", map[string]interface{}{"question": "q"}, 3, "match-clarification", false},
		{"cluster_clarifications", map[string]interface{}{"questions_file": "f"}, 3, "cluster-clarifications", false},
		{"detect_conflicts", map[string]interface{}{"tracking_file": "f"}, 2, "detect-conflicts", false},
		{"validate_clarifications", map[string]interface{}{"tracking_file": "f"}, 2, "validate-clarifications", false},
		{"init_tracking", map[string]interface{}{"output": "f"}, 3, "init-tracking", false},
		{"add_clarification", map[string]interface{}{"tracking_file": "f"}, 3, "add-clarification", false},
		{"promote_clarification", map[string]interface{}{"tracking_file": "f"}, 3, "promote-clarification", false},
		{"list_entries", map[string]interface{}{"tracking_file": "f"}, 2, "list-entries", false},
		{"unknown_command", map[string]interface{}{}, 0, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got, err := buildArgs(tt.command, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) < tt.wantLen {
					t.Errorf("buildArgs() len = %d, want >= %d", len(got), tt.wantLen)
				}
				if got[0] != tt.wantFirst {
					t.Errorf("buildArgs()[0] = %s, want %s", got[0], tt.wantFirst)
				}
			}
		})
	}
}

func TestGetIntTypes(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		key     string
		wantVal int
		wantOK  bool
	}{
		{
			name:    "float64",
			args:    map[string]interface{}{"val": float64(42)},
			key:     "val",
			wantVal: 42,
			wantOK:  true,
		},
		{
			name:    "int",
			args:    map[string]interface{}{"val": int(42)},
			key:     "val",
			wantVal: 42,
			wantOK:  true,
		},
		{
			name:    "int64",
			args:    map[string]interface{}{"val": int64(42)},
			key:     "val",
			wantVal: 42,
			wantOK:  true,
		},
		{
			name:    "string (not int)",
			args:    map[string]interface{}{"val": "42"},
			key:     "val",
			wantVal: 0,
			wantOK:  false,
		},
		{
			name:    "missing key",
			args:    map[string]interface{}{},
			key:     "val",
			wantVal: 0,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOK := getInt(tt.args, tt.key)
			if gotVal != tt.wantVal || gotOK != tt.wantOK {
				t.Errorf("getInt() = (%d, %v), want (%d, %v)", gotVal, gotOK, tt.wantVal, tt.wantOK)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		want bool
	}{
		{"true value", map[string]interface{}{"flag": true}, "flag", true},
		{"false value", map[string]interface{}{"flag": false}, "flag", false},
		{"missing key", map[string]interface{}{}, "flag", false},
		{"wrong type", map[string]interface{}{"flag": "true"}, "flag", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBool(tt.args, tt.key)
			if got != tt.want {
				t.Errorf("getBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildDeleteArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic delete (defaults to json+min)",
			args: map[string]interface{}{"file": "tracking.db", "id": "entry-001"},
			want: []string{"delete-clarification", "--file", "tracking.db", "--id", "entry-001", "--json", "--min"},
		},
		{
			name: "with force and quiet (defaults to json+min)",
			args: map[string]interface{}{
				"file":  "tracking.db",
				"id":    "entry-002",
				"force": true,
				"quiet": true,
			},
			want: []string{"delete-clarification", "--file", "tracking.db", "--id", "entry-002", "--force", "--quiet", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"file": "tracking.db", "id": "entry-001", "json": false, "min": false},
			want: []string{"delete-clarification", "--file", "tracking.db", "--id", "entry-001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeleteArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDeleteArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildExportArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic export (defaults to json+min)",
			args: map[string]interface{}{"source": "data.db", "output": "export.yaml"},
			want: []string{"export-memory", "--source", "data.db", "--output", "export.yaml", "--json", "--min"},
		},
		{
			name: "with quiet (defaults to json+min)",
			args: map[string]interface{}{
				"source": "data.db",
				"output": "export.yaml",
				"quiet":  true,
			},
			want: []string{"export-memory", "--source", "data.db", "--output", "export.yaml", "--quiet", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"source": "data.db", "output": "export.yaml", "json": false, "min": false},
			want: []string{"export-memory", "--source", "data.db", "--output", "export.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildExportArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildExportArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildImportArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic import (defaults to json+min)",
			args: map[string]interface{}{"source": "data.yaml", "target": "data.db"},
			want: []string{"import-memory", "--source", "data.yaml", "--target", "data.db", "--json", "--min"},
		},
		{
			name: "with mode and quiet (defaults to json+min)",
			args: map[string]interface{}{
				"source": "data.yaml",
				"target": "data.db",
				"mode":   "merge",
				"quiet":  true,
			},
			want: []string{"import-memory", "--source", "data.yaml", "--target", "data.db", "--mode", "merge", "--quiet", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"source": "data.yaml", "target": "data.db", "json": false, "min": false},
			want: []string{"import-memory", "--source", "data.yaml", "--target", "data.db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildImportArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildImportArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildOptimizeArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic optimize (defaults to json+min)",
			args: map[string]interface{}{"file": "data.db"},
			want: []string{"optimize-memory", "--file", "data.db", "--json", "--min"},
		},
		{
			name: "with vacuum and stats (defaults to json+min)",
			args: map[string]interface{}{
				"file":   "data.db",
				"vacuum": true,
				"stats":  true,
			},
			want: []string{"optimize-memory", "--file", "data.db", "--vacuum", "--stats", "--json", "--min"},
		},
		{
			name: "with prune_stale (defaults to json+min)",
			args: map[string]interface{}{
				"file":        "data.db",
				"prune_stale": "30d",
				"quiet":       true,
			},
			want: []string{"optimize-memory", "--file", "data.db", "--prune-stale", "30d", "--quiet", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"file": "data.db", "json": false, "min": false},
			want: []string{"optimize-memory", "--file", "data.db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOptimizeArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildOptimizeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildReconcileArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want []string
	}{
		{
			name: "basic reconcile (defaults to json+min)",
			args: map[string]interface{}{"file": "data.db", "project_root": "/path/to/project"},
			want: []string{"reconcile-memory", "--file", "data.db", "--project-root", "/path/to/project", "--json", "--min"},
		},
		{
			name: "with dry_run and quiet (defaults to json+min)",
			args: map[string]interface{}{
				"file":         "data.db",
				"project_root": "/path/to/project",
				"dry_run":      true,
				"quiet":        true,
			},
			want: []string{"reconcile-memory", "--file", "data.db", "--project-root", "/path/to/project", "--dry-run", "--quiet", "--json", "--min"},
		},
		{
			name: "with json+min disabled",
			args: map[string]interface{}{"file": "data.db", "project_root": "/path/to/project", "json": false, "min": false},
			want: []string{"reconcile-memory", "--file", "data.db", "--project-root", "/path/to/project"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildReconcileArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildReconcileArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildArgsDispatcherNewCommands(t *testing.T) {
	tests := []struct {
		command   string
		args      map[string]interface{}
		wantLen   int
		wantFirst string
		wantErr   bool
	}{
		{"delete_clarification", map[string]interface{}{"file": "f", "id": "i"}, 5, "delete-clarification", false},
		{"export_memory", map[string]interface{}{"source": "s", "output": "o"}, 5, "export-memory", false},
		{"import_memory", map[string]interface{}{"source": "s", "target": "t"}, 5, "import-memory", false},
		{"optimize_memory", map[string]interface{}{"file": "f"}, 3, "optimize-memory", false},
		{"reconcile_memory", map[string]interface{}{"file": "f", "project_root": "p"}, 5, "reconcile-memory", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got, err := buildArgs(tt.command, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) < tt.wantLen {
					t.Errorf("buildArgs() len = %d, want >= %d", len(got), tt.wantLen)
				}
				if got[0] != tt.wantFirst {
					t.Errorf("buildArgs()[0] = %s, want %s", got[0], tt.wantFirst)
				}
			}
		})
	}
}
