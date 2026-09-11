package config

import "testing"

func TestGetProfileConfig_Scope(t *testing.T) {
	cfg := &SemanticConfig{
		CodeInclude:    "*.go",
		CodeExclude:    "vendor,node_modules,dist",
		DocsInclude:    "*.md,*.txt",
		DocsExclude:    "CHANGELOG.md,README.md",
		MemoryInclude:  "*.md",
		SprintsExclude: "archive",
	}

	tests := []struct {
		profile     string
		wantInclude []string
		wantExclude []string
	}{
		{"code", []string{"*.go"}, []string{"vendor", "node_modules", "dist"}},
		{"docs", []string{"*.md", "*.txt"}, []string{"CHANGELOG.md", "README.md"}},
		{"memory", []string{"*.md"}, nil},
		{"sprints", nil, []string{"archive"}},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got := cfg.GetProfileConfig(tt.profile)
			if !equalSlices(got.Include, tt.wantInclude) {
				t.Errorf("Include = %v, want %v", got.Include, tt.wantInclude)
			}
			if !equalSlices(got.Exclude, tt.wantExclude) {
				t.Errorf("Exclude = %v, want %v", got.Exclude, tt.wantExclude)
			}
		})
	}
}

// An unset scope must stay empty rather than becoming a single empty pattern,
// which would otherwise match nothing and silently empty a collection.
func TestGetProfileConfig_EmptyScope(t *testing.T) {
	cfg := &SemanticConfig{CodeCollection: "code"}

	got := cfg.GetProfileConfig("code")
	if len(got.Include) != 0 {
		t.Errorf("Include = %v, want empty", got.Include)
	}
	if len(got.Exclude) != 0 {
		t.Errorf("Exclude = %v, want empty", got.Exclude)
	}
}

func TestGetProfileConfig_ScopeTrimsSpaces(t *testing.T) {
	cfg := &SemanticConfig{CodeExclude: "vendor, node_modules , dist"}

	got := cfg.GetProfileConfig("code")
	want := []string{"vendor", "node_modules", "dist"}
	if !equalSlices(got.Exclude, want) {
		t.Errorf("Exclude = %v, want %v", got.Exclude, want)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
