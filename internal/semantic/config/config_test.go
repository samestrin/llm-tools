package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_ValidYAML(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `
semantic:
  enabled: true
  auto_update: true
  max_results: 10
  min_score: 0.7
  stale_days: 7
  code_collection: project-code
  code_enabled: true
  code_storage: qdrant
  docs_collection: project-docs
  docs_enabled: true
  docs_storage: qdrant
  memory_collection: project-memory
  memory_enabled: true
  memory_storage: sqlite
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify general settings
	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if !cfg.AutoUpdate {
		t.Error("expected AutoUpdate to be true")
	}
	if cfg.MaxResults != 10 {
		t.Errorf("expected MaxResults=10, got %d", cfg.MaxResults)
	}
	if cfg.MinScore != 0.7 {
		t.Errorf("expected MinScore=0.7, got %f", cfg.MinScore)
	}
	if cfg.StaleDays != 7 {
		t.Errorf("expected StaleDays=7, got %d", cfg.StaleDays)
	}

	// Verify code profile
	if cfg.CodeCollection != "project-code" {
		t.Errorf("expected CodeCollection='project-code', got %q", cfg.CodeCollection)
	}
	if !cfg.CodeEnabled {
		t.Error("expected CodeEnabled to be true")
	}
	if cfg.CodeStorage != "qdrant" {
		t.Errorf("expected CodeStorage='qdrant', got %q", cfg.CodeStorage)
	}

	// Verify docs profile
	if cfg.DocsCollection != "project-docs" {
		t.Errorf("expected DocsCollection='project-docs', got %q", cfg.DocsCollection)
	}

	// Verify memory profile
	if cfg.MemoryCollection != "project-memory" {
		t.Errorf("expected MemoryCollection='project-memory', got %q", cfg.MemoryCollection)
	}
	if cfg.MemoryStorage != "sqlite" {
		t.Errorf("expected MemoryStorage='sqlite', got %q", cfg.MemoryStorage)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `
semantic:
  enabled: true
  invalid yaml here
  : broken
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	// Config with only some fields set - others should use defaults
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `
semantic:
  code_collection: my-code
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.CodeCollection != "my-code" {
		t.Errorf("expected CodeCollection='my-code', got %q", cfg.CodeCollection)
	}
	// Verify defaults for unset fields
	if cfg.MaxResults != 0 && cfg.MaxResults != 10 {
		// Allow 0 (Go default) or 10 (our default if we implement it)
		t.Errorf("unexpected MaxResults: %d", cfg.MaxResults)
	}
}

func TestLoadConfig_EmptySemanticSection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := `
other:
  key: value
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Empty semantic section should return zero-valued config
	if cfg == nil {
		t.Error("expected non-nil config even for empty semantic section")
	}
}

func TestGetProfileConfig_Code(t *testing.T) {
	cfg := &SemanticConfig{
		CodeCollection: "code-idx",
		CodeStorage:    "qdrant",
		CodeEnabled:    true,
		MinScore:       0.5,
		MaxResults:     20,
	}

	profile := cfg.GetProfileConfig("code")
	if profile.Collection != "code-idx" {
		t.Errorf("expected Collection='code-idx', got %q", profile.Collection)
	}
	if profile.Storage != "qdrant" {
		t.Errorf("expected Storage='qdrant', got %q", profile.Storage)
	}
	if !profile.Enabled {
		t.Error("expected Enabled to be true")
	}
	if profile.MinScore != 0.5 {
		t.Errorf("expected MinScore=0.5, got %f", profile.MinScore)
	}
	if profile.MaxResults != 20 {
		t.Errorf("expected MaxResults=20, got %d", profile.MaxResults)
	}
}

func TestGetProfileConfig_Docs(t *testing.T) {
	cfg := &SemanticConfig{
		DocsCollection: "docs-idx",
		DocsStorage:    "sqlite",
		DocsEnabled:    true,
	}

	profile := cfg.GetProfileConfig("docs")
	if profile.Collection != "docs-idx" {
		t.Errorf("expected Collection='docs-idx', got %q", profile.Collection)
	}
	if profile.Storage != "sqlite" {
		t.Errorf("expected Storage='sqlite', got %q", profile.Storage)
	}
}

func TestGetProfileConfig_Memory(t *testing.T) {
	cfg := &SemanticConfig{
		MemoryCollection: "mem-idx",
		MemoryStorage:    "qdrant",
		MemoryEnabled:    false,
	}

	profile := cfg.GetProfileConfig("memory")
	if profile.Collection != "mem-idx" {
		t.Errorf("expected Collection='mem-idx', got %q", profile.Collection)
	}
	if profile.Storage != "qdrant" {
		t.Errorf("expected Storage='qdrant', got %q", profile.Storage)
	}
	if profile.Enabled {
		t.Error("expected Enabled to be false")
	}
}

func TestGetProfileConfig_DefaultIsCode(t *testing.T) {
	cfg := &SemanticConfig{
		CodeCollection:   "code-default",
		DocsCollection:   "docs-idx",
		MemoryCollection: "mem-idx",
	}

	// Empty string should default to "code"
	profile := cfg.GetProfileConfig("")
	if profile.Collection != "code-default" {
		t.Errorf("expected default profile to be code, got collection %q", profile.Collection)
	}
}

func TestGetProfileConfig_UnknownProfile(t *testing.T) {
	cfg := &SemanticConfig{
		CodeCollection: "code-idx",
	}

	// Unknown profile should return error or fall back to code
	profile := cfg.GetProfileConfig("unknown")
	// We expect it to fall back to code profile
	if profile.Collection != "code-idx" {
		t.Errorf("expected unknown profile to fall back to code, got %q", profile.Collection)
	}
}

func TestValidProfiles(t *testing.T) {
	profiles := ValidProfiles()
	expected := []string{"code", "docs", "memory"}

	if len(profiles) != len(expected) {
		t.Errorf("expected %d profiles, got %d", len(expected), len(profiles))
	}

	for _, p := range expected {
		found := false
		for _, vp := range profiles {
			if vp == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected profile %q to be in valid profiles", p)
		}
	}
}

func TestIsValidProfile(t *testing.T) {
	tests := []struct {
		profile string
		valid   bool
	}{
		{"code", true},
		{"docs", true},
		{"memory", true},
		{"", true}, // empty defaults to code
		{"unknown", false},
		{"CODE", false}, // case sensitive
	}

	for _, tt := range tests {
		got := IsValidProfile(tt.profile)
		if got != tt.valid {
			t.Errorf("IsValidProfile(%q) = %v, want %v", tt.profile, got, tt.valid)
		}
	}
}

func TestResolveValue_ExplicitOverridesConfig(t *testing.T) {
	cfg := &SemanticConfig{
		CodeCollection: "config-collection",
		CodeStorage:    "qdrant",
		MinScore:       0.5,
	}

	// Test that ResolveValue returns explicit value when provided
	explicit := "explicit-collection"
	result := ResolveValue(explicit, cfg.CodeCollection)
	if result != explicit {
		t.Errorf("expected explicit value %q, got %q", explicit, result)
	}

	// Test that ResolveValue returns config value when explicit is empty
	result = ResolveValue("", cfg.CodeCollection)
	if result != cfg.CodeCollection {
		t.Errorf("expected config value %q, got %q", cfg.CodeCollection, result)
	}
}

func TestResolveFloatValue(t *testing.T) {
	cfg := &SemanticConfig{
		MinScore: 0.5,
	}

	// Explicit non-zero overrides
	result := ResolveFloatValue(0.8, cfg.MinScore, 0.0)
	if result != 0.8 {
		t.Errorf("expected 0.8, got %f", result)
	}

	// Zero explicit falls back to config
	result = ResolveFloatValue(0.0, cfg.MinScore, 0.0)
	if result != 0.5 {
		t.Errorf("expected 0.5, got %f", result)
	}

	// Both zero falls back to default
	result = ResolveFloatValue(0.0, 0.0, 0.7)
	if result != 0.7 {
		t.Errorf("expected 0.7, got %f", result)
	}
}

func TestResolveIntValue(t *testing.T) {
	cfg := &SemanticConfig{
		MaxResults: 20,
	}

	// Explicit non-zero overrides
	result := ResolveIntValue(50, cfg.MaxResults, 10)
	if result != 50 {
		t.Errorf("expected 50, got %d", result)
	}

	// Zero explicit falls back to config
	result = ResolveIntValue(0, cfg.MaxResults, 10)
	if result != 20 {
		t.Errorf("expected 20, got %d", result)
	}

	// Both zero falls back to default
	result = ResolveIntValue(0, 0, 10)
	if result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
}
