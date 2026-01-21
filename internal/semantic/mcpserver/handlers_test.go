package mcpserver

import (
	"os"
	"reflect"
	"testing"
)

func TestBuildMemoryStoreArgs(t *testing.T) {
	args := map[string]interface{}{
		"question": "How to handle auth?",
		"answer":   "Use JWT tokens",
		"tags":     "auth,security",
		"source":   "manual",
		"storage":  "sqlite",
	}

	result := buildMemoryStoreArgs(args)

	// Check base command
	if len(result) < 2 || result[0] != "memory" || result[1] != "store" {
		t.Errorf("buildMemoryStoreArgs() should start with ['memory', 'store'], got %v", result[:2])
	}

	// Check all flags are present
	expected := []string{"--question", "How to handle auth?", "--answer", "Use JWT tokens", "--tags", "auth,security", "--source", "manual", "--storage", "sqlite"}
	for i := 0; i < len(expected); i += 2 {
		found := false
		for j := 2; j < len(result)-1; j++ {
			if result[j] == expected[i] && result[j+1] == expected[i+1] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildMemoryStoreArgs() missing %s %s", expected[i], expected[i+1])
		}
	}
}

func TestBuildMemorySearchArgs(t *testing.T) {
	args := map[string]interface{}{
		"query":     "authentication handling",
		"top_k":     float64(5),
		"threshold": 0.7,
		"tags":      "auth",
		"status":    "pending",
	}

	result := buildMemorySearchArgs(args)

	if len(result) < 3 || result[0] != "memory" || result[1] != "search" {
		t.Errorf("buildMemorySearchArgs() should start with ['memory', 'search'], got %v", result[:2])
	}

	if result[2] != "authentication handling" {
		t.Errorf("buildMemorySearchArgs() query = %s, want 'authentication handling'", result[2])
	}
}

func TestBuildMemoryPromoteArgs(t *testing.T) {
	args := map[string]interface{}{
		"id":      "mem-123",
		"target":  "CLAUDE.md",
		"section": "Custom Section",
		"force":   true,
	}

	result := buildMemoryPromoteArgs(args)

	if len(result) < 3 || result[0] != "memory" || result[1] != "promote" {
		t.Errorf("buildMemoryPromoteArgs() should start with ['memory', 'promote'], got %v", result[:2])
	}

	if result[2] != "mem-123" {
		t.Errorf("buildMemoryPromoteArgs() id = %s, want 'mem-123'", result[2])
	}

	// Check --force flag is present
	found := false
	for _, arg := range result {
		if arg == "--force" {
			found = true
			break
		}
	}
	if !found {
		t.Error("buildMemoryPromoteArgs() missing --force flag")
	}
}

func TestBuildMemoryListArgs(t *testing.T) {
	args := map[string]interface{}{
		"limit":  float64(20),
		"status": "promoted",
	}

	result := buildMemoryListArgs(args)

	if len(result) < 2 || result[0] != "memory" || result[1] != "list" {
		t.Errorf("buildMemoryListArgs() should start with ['memory', 'list'], got %v", result[:2])
	}
}

func TestBuildMemoryDeleteArgs(t *testing.T) {
	args := map[string]interface{}{
		"id":    "mem-456",
		"force": true,
	}

	result := buildMemoryDeleteArgs(args)

	expected := []string{"memory", "delete", "mem-456", "--force"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("buildMemoryDeleteArgs() = %v, want %v", result, expected)
	}
}

func TestBuildArgs_MemoryCommands(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		args    map[string]interface{}
		wantCmd []string
		wantErr bool
	}{
		{
			name:    "memory_store",
			cmdName: "memory_store",
			args:    map[string]interface{}{"question": "Q", "answer": "A"},
			wantCmd: []string{"memory", "store"},
			wantErr: false,
		},
		{
			name:    "memory_search",
			cmdName: "memory_search",
			args:    map[string]interface{}{"query": "test"},
			wantCmd: []string{"memory", "search"},
			wantErr: false,
		},
		{
			name:    "memory_promote",
			cmdName: "memory_promote",
			args:    map[string]interface{}{"id": "123", "target": "CLAUDE.md"},
			wantCmd: []string{"memory", "promote"},
			wantErr: false,
		},
		{
			name:    "memory_list",
			cmdName: "memory_list",
			args:    map[string]interface{}{},
			wantCmd: []string{"memory", "list"},
			wantErr: false,
		},
		{
			name:    "memory_delete",
			cmdName: "memory_delete",
			args:    map[string]interface{}{"id": "456"},
			wantCmd: []string{"memory", "delete"},
			wantErr: false,
		},
		{
			name:    "unknown_command",
			cmdName: "unknown_cmd",
			args:    map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildArgs(tt.cmdName, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Error("buildArgs() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("buildArgs() unexpected error: %v", err)
				return
			}

			if len(result) < 2 || result[0] != tt.wantCmd[0] || result[1] != tt.wantCmd[1] {
				t.Errorf("buildArgs() = %v, want command starting with %v", result, tt.wantCmd)
			}
		})
	}
}

func TestStripPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"llm_semantic_search", "search"},
		{"llm_semantic_memory_store", "memory_store"},
		{"llm_semantic_index", "index"},
		{"short", "short"},
	}

	for _, tt := range tests {
		result := stripPrefix(tt.input)
		if result != tt.expected {
			t.Errorf("stripPrefix(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestBuildSearchArgs_HybridParameters(t *testing.T) {
	args := map[string]interface{}{
		"query":        "authentication handling",
		"hybrid":       true,
		"fusion_k":     float64(80),
		"fusion_alpha": 0.6,
	}

	result := buildSearchArgs(args)

	// Check base command and query
	if len(result) < 2 || result[0] != "search" || result[1] != "authentication handling" {
		t.Errorf("buildSearchArgs() should start with ['search', '<query>'], got %v", result[:2])
	}

	// Check --hybrid flag
	foundHybrid := false
	for _, arg := range result {
		if arg == "--hybrid" {
			foundHybrid = true
			break
		}
	}
	if !foundHybrid {
		t.Error("buildSearchArgs() missing --hybrid flag")
	}

	// Check --fusion-k
	foundFusionK := false
	for i, arg := range result {
		if arg == "--fusion-k" && i+1 < len(result) && result[i+1] == "80" {
			foundFusionK = true
			break
		}
	}
	if !foundFusionK {
		t.Errorf("buildSearchArgs() missing or incorrect --fusion-k 80, got %v", result)
	}

	// Check --fusion-alpha
	foundFusionAlpha := false
	for i, arg := range result {
		if arg == "--fusion-alpha" && i+1 < len(result) {
			foundFusionAlpha = true
			break
		}
	}
	if !foundFusionAlpha {
		t.Errorf("buildSearchArgs() missing --fusion-alpha, got %v", result)
	}
}

func TestBuildSearchArgs_RecencyParameters(t *testing.T) {
	args := map[string]interface{}{
		"query":          "database connection",
		"recency_boost":  true,
		"recency_factor": 0.3,
		"recency_decay":  float64(14),
	}

	result := buildSearchArgs(args)

	// Check base command
	if len(result) < 2 || result[0] != "search" {
		t.Errorf("buildSearchArgs() should start with 'search', got %v", result)
	}

	// Check --recency-boost flag
	foundRecencyBoost := false
	for _, arg := range result {
		if arg == "--recency-boost" {
			foundRecencyBoost = true
			break
		}
	}
	if !foundRecencyBoost {
		t.Error("buildSearchArgs() missing --recency-boost flag")
	}

	// Check --recency-factor
	foundRecencyFactor := false
	for i, arg := range result {
		if arg == "--recency-factor" && i+1 < len(result) {
			foundRecencyFactor = true
			break
		}
	}
	if !foundRecencyFactor {
		t.Errorf("buildSearchArgs() missing --recency-factor, got %v", result)
	}

	// Check --recency-decay
	foundRecencyDecay := false
	for i, arg := range result {
		if arg == "--recency-decay" && i+1 < len(result) && result[i+1] == "14" {
			foundRecencyDecay = true
			break
		}
	}
	if !foundRecencyDecay {
		t.Errorf("buildSearchArgs() missing or incorrect --recency-decay 14, got %v", result)
	}
}

func TestBuildSearchArgs_AllParameters(t *testing.T) {
	// Test with all search parameters combined
	args := map[string]interface{}{
		"query":          "test query",
		"top_k":          float64(20),
		"threshold":      0.5,
		"type":           "function",
		"path":           "internal/",
		"hybrid":         true,
		"fusion_k":       float64(60),
		"fusion_alpha":   0.7,
		"recency_boost":  true,
		"recency_factor": 0.5,
		"recency_decay":  float64(7),
		"storage":        "sqlite",
		"collection":     "test_collection",
	}

	result := buildSearchArgs(args)

	// Expected flags (excluding query which is positional)
	expectedFlags := []string{
		"--top", "--threshold", "--type", "--path",
		"--hybrid", "--fusion-k", "--fusion-alpha",
		"--recency-boost", "--recency-factor", "--recency-decay",
		"--storage", "--collection",
	}

	for _, flag := range expectedFlags {
		found := false
		for _, arg := range result {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildSearchArgs() missing flag %s in result %v", flag, result)
		}
	}
}

// Profile configuration tests

func TestLoadConfig(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: my-code-index
  code_storage: qdrant
  docs_collection: my-docs-index
  docs_storage: sqlite
  memory_collection: my-memory-index
  memory_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	if cfg.Semantic.CodeCollection != "my-code-index" {
		t.Errorf("loadConfig() CodeCollection = %s, want my-code-index", cfg.Semantic.CodeCollection)
	}
	if cfg.Semantic.CodeStorage != "qdrant" {
		t.Errorf("loadConfig() CodeStorage = %s, want qdrant", cfg.Semantic.CodeStorage)
	}
	if cfg.Semantic.DocsCollection != "my-docs-index" {
		t.Errorf("loadConfig() DocsCollection = %s, want my-docs-index", cfg.Semantic.DocsCollection)
	}
	if cfg.Semantic.DocsStorage != "sqlite" {
		t.Errorf("loadConfig() DocsStorage = %s, want sqlite", cfg.Semantic.DocsStorage)
	}
	if cfg.Semantic.MemoryCollection != "my-memory-index" {
		t.Errorf("loadConfig() MemoryCollection = %s, want my-memory-index", cfg.Semantic.MemoryCollection)
	}
	if cfg.Semantic.MemoryStorage != "qdrant" {
		t.Errorf("loadConfig() MemoryStorage = %s, want qdrant", cfg.Semantic.MemoryStorage)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := loadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("loadConfig() expected error for missing file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/invalid.yaml"

	invalidContent := "semantic:\n  code_collection: [invalid yaml\n  broken: missing\n"
	if err := writeTestFile(configPath, invalidContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	_, err := loadConfig(configPath)
	if err == nil {
		t.Error("loadConfig() expected error for invalid YAML, got nil")
	}
}

func TestResolveProfileSettings_CodeProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: llm-tools-code
  code_storage: qdrant
  docs_collection: llm-tools-docs
  docs_storage: sqlite
  memory_collection: llm-tools-memory
  memory_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":   "test",
		"profile": "code",
		"config":  configPath,
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	if args["collection"] != "llm-tools-code" {
		t.Errorf("resolveProfileSettings() collection = %v, want llm-tools-code", args["collection"])
	}
	if args["storage"] != "qdrant" {
		t.Errorf("resolveProfileSettings() storage = %v, want qdrant", args["storage"])
	}
}

func TestResolveProfileSettings_DocsProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: llm-tools-code
  code_storage: qdrant
  docs_collection: llm-tools-docs
  docs_storage: sqlite
  memory_collection: llm-tools-memory
  memory_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":   "test",
		"profile": "docs",
		"config":  configPath,
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	if args["collection"] != "llm-tools-docs" {
		t.Errorf("resolveProfileSettings() collection = %v, want llm-tools-docs", args["collection"])
	}
	if args["storage"] != "sqlite" {
		t.Errorf("resolveProfileSettings() storage = %v, want sqlite", args["storage"])
	}
}

func TestResolveProfileSettings_MemoryProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: llm-tools-code
  code_storage: sqlite
  docs_collection: llm-tools-docs
  docs_storage: sqlite
  memory_collection: llm-tools-memory
  memory_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":   "test",
		"profile": "memory",
		"config":  configPath,
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	if args["collection"] != "llm-tools-memory" {
		t.Errorf("resolveProfileSettings() collection = %v, want llm-tools-memory", args["collection"])
	}
	if args["storage"] != "qdrant" {
		t.Errorf("resolveProfileSettings() storage = %v, want qdrant", args["storage"])
	}
}

func TestResolveProfileSettings_SprintsProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  sprints_collection: llm-tools-sprints
  sprints_storage: sqlite
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":   "test",
		"profile": "sprints",
		"config":  configPath,
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	if args["collection"] != "llm-tools-sprints" {
		t.Errorf("resolveProfileSettings() collection = %v, want llm-tools-sprints", args["collection"])
	}
	if args["storage"] != "sqlite" {
		t.Errorf("resolveProfileSettings() storage = %v, want sqlite", args["storage"])
	}
}

func TestResolveProfileSettings_ExplicitOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: llm-tools-code
  code_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Explicit collection and storage should NOT be overwritten by profile
	args := map[string]interface{}{
		"query":      "test",
		"profile":    "code",
		"config":     configPath,
		"collection": "my-custom-collection",
		"storage":    "sqlite",
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// Explicit values should be preserved
	if args["collection"] != "my-custom-collection" {
		t.Errorf("resolveProfileSettings() should preserve explicit collection, got %v", args["collection"])
	}
	if args["storage"] != "sqlite" {
		t.Errorf("resolveProfileSettings() should preserve explicit storage, got %v", args["storage"])
	}
}

func TestResolveProfileSettings_NoProfile(t *testing.T) {
	// When no profile is provided, nothing should change
	args := map[string]interface{}{
		"query": "test",
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// No collection or storage should be added
	if _, ok := args["collection"]; ok {
		t.Error("resolveProfileSettings() should not add collection when no profile")
	}
	if _, ok := args["storage"]; ok {
		t.Error("resolveProfileSettings() should not add storage when no profile")
	}
}

func TestResolveProfileSettings_ProfileWithoutConfig(t *testing.T) {
	// Profile without config should do nothing
	args := map[string]interface{}{
		"query":   "test",
		"profile": "code",
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// No collection or storage should be added
	if _, ok := args["collection"]; ok {
		t.Error("resolveProfileSettings() should not add collection when no config")
	}
}

func TestResolveProfileSettings_UnknownProfile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: llm-tools-code
  code_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":   "test",
		"profile": "unknown_profile",
		"config":  configPath,
	}

	err := resolveProfileSettings(args)
	if err == nil {
		t.Error("resolveProfileSettings() expected error for unknown profile, got nil")
	}
}

func TestResolveProfileSettings_MissingConfigFile(t *testing.T) {
	args := map[string]interface{}{
		"query":   "test",
		"profile": "code",
		"config":  "/nonexistent/config.yaml",
	}

	err := resolveProfileSettings(args)
	if err == nil {
		t.Error("resolveProfileSettings() expected error for missing config file, got nil")
	}
}

func TestResolveProfileSettings_EmptyValues(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	// Config with empty values
	configContent := `semantic:
  code_collection: ""
  code_storage: ""
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":   "test",
		"profile": "code",
		"config":  configPath,
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// Empty values from config should not be set
	if _, ok := args["collection"].(string); ok && args["collection"] != "" {
		t.Errorf("resolveProfileSettings() should not set empty collection")
	}
}

func TestResolveProfileSettings_PartialOverride(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: llm-tools-code
  code_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Only collection is explicitly set, storage should come from profile
	args := map[string]interface{}{
		"query":      "test",
		"profile":    "code",
		"config":     configPath,
		"collection": "my-custom-collection",
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// Explicit collection preserved
	if args["collection"] != "my-custom-collection" {
		t.Errorf("resolveProfileSettings() collection = %v, want my-custom-collection", args["collection"])
	}
	// Storage from profile
	if args["storage"] != "qdrant" {
		t.Errorf("resolveProfileSettings() storage = %v, want qdrant", args["storage"])
	}
}

// Helper function to write test files
func writeTestFile(path, content string) error {
	return writeFile(path, []byte(content))
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// Additional tests for coverage

func TestGetBool(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected bool
	}{
		{"true value", map[string]interface{}{"flag": true}, "flag", true},
		{"false value", map[string]interface{}{"flag": false}, "flag", false},
		{"missing key", map[string]interface{}{}, "flag", false},
		{"wrong type", map[string]interface{}{"flag": "true"}, "flag", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBool(tt.args, tt.key)
			if result != tt.expected {
				t.Errorf("getBool() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected int
		ok       bool
	}{
		{"int value", map[string]interface{}{"num": 42}, "num", 42, true},
		{"float64 value", map[string]interface{}{"num": float64(42)}, "num", 42, true},
		{"int64 value", map[string]interface{}{"num": int64(42)}, "num", 42, true},
		{"string numeric", map[string]interface{}{"num": "42"}, "num", 42, true},
		{"missing key", map[string]interface{}{}, "num", 0, false},
		{"non-numeric string", map[string]interface{}{"num": "not a number"}, "num", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := getInt(tt.args, tt.key)
			if ok != tt.ok {
				t.Errorf("getInt() ok = %v, want %v", ok, tt.ok)
			}
			if result != tt.expected {
				t.Errorf("getInt() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetFloat(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected float64
		ok       bool
	}{
		{"float64 value", map[string]interface{}{"num": float64(3.14)}, "num", 3.14, true},
		{"float32 value", map[string]interface{}{"num": float32(3.14)}, "num", float64(float32(3.14)), true},
		{"int value", map[string]interface{}{"num": 42}, "num", 42.0, true},
		{"string numeric", map[string]interface{}{"num": "3.14"}, "num", 3.14, true},
		{"missing key", map[string]interface{}{}, "num", 0, false},
		{"non-numeric string", map[string]interface{}{"num": "not a number"}, "num", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := getFloat(tt.args, tt.key)
			if ok != tt.ok {
				t.Errorf("getFloat() ok = %v, want %v", ok, tt.ok)
			}
			if result != tt.expected {
				t.Errorf("getFloat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBuildIndexArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		contains []string
	}{
		{
			name:     "basic path",
			args:     map[string]interface{}{"path": "/some/path"},
			contains: []string{"index", "/some/path"},
		},
		{
			name: "with includes",
			args: map[string]interface{}{
				"include": []interface{}{"*.go", "*.ts"},
			},
			contains: []string{"index", "--include", "*.go", "--include", "*.ts"},
		},
		{
			name: "with excludes",
			args: map[string]interface{}{
				"exclude": []interface{}{"vendor", "node_modules"},
			},
			contains: []string{"index", "--exclude", "vendor", "--exclude", "node_modules"},
		},
		{
			name:     "with force",
			args:     map[string]interface{}{"force": true},
			contains: []string{"index", "--force"},
		},
		{
			name: "with storage and collection",
			args: map[string]interface{}{
				"storage":    "qdrant",
				"collection": "my-collection",
			},
			contains: []string{"--storage", "qdrant", "--collection", "my-collection"},
		},
		{
			name: "with batch_size",
			args: map[string]interface{}{
				"batch_size": float64(50),
			},
			contains: []string{"index", "--batch-size", "50"},
		},
		{
			name: "batch_size zero is omitted",
			args: map[string]interface{}{
				"batch_size": float64(0),
			},
			contains: []string{"index"},
		},
		{
			name: "with parallel",
			args: map[string]interface{}{
				"parallel": float64(4),
			},
			contains: []string{"index", "--parallel", "4"},
		},
		{
			name: "parallel zero is omitted",
			args: map[string]interface{}{
				"parallel": float64(0),
			},
			contains: []string{"index"},
		},
		{
			name: "with batch_size and parallel",
			args: map[string]interface{}{
				"batch_size": float64(64),
				"parallel":   float64(4),
			},
			contains: []string{"index", "--batch-size", "64", "--parallel", "4"},
		},
		{
			name: "with exclude_tests",
			args: map[string]interface{}{
				"exclude_tests": true,
			},
			contains: []string{"index", "--exclude-tests"},
		},
		{
			name: "with file pattern in exclude",
			args: map[string]interface{}{
				"exclude": []interface{}{"*_test.go"},
			},
			contains: []string{"index", "--exclude", "*_test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildIndexArgs(tt.args)
			for _, expected := range tt.contains {
				found := false
				for _, arg := range result {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildIndexArgs() missing %q in %v", expected, result)
				}
			}
		})
	}
}

func TestBuildStatusArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		contains []string
	}{
		{
			name:     "basic",
			args:     map[string]interface{}{},
			contains: []string{"index-status"},
		},
		{
			name:     "with storage",
			args:     map[string]interface{}{"storage": "qdrant"},
			contains: []string{"index-status", "--storage", "qdrant"},
		},
		{
			name:     "with collection",
			args:     map[string]interface{}{"collection": "my-index"},
			contains: []string{"index-status", "--collection", "my-index"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildStatusArgs(tt.args)
			for _, expected := range tt.contains {
				found := false
				for _, arg := range result {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildStatusArgs() missing %q in %v", expected, result)
				}
			}
		})
	}
}

func TestBuildUpdateArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		contains []string
	}{
		{
			name:     "basic",
			args:     map[string]interface{}{},
			contains: []string{"index-update"},
		},
		{
			name:     "with path",
			args:     map[string]interface{}{"path": "/some/path"},
			contains: []string{"index-update", "/some/path"},
		},
		{
			name: "with includes",
			args: map[string]interface{}{
				"include": []interface{}{"*.go"},
			},
			contains: []string{"index-update", "--include", "*.go"},
		},
		{
			name: "with excludes",
			args: map[string]interface{}{
				"exclude": []interface{}{"vendor"},
			},
			contains: []string{"index-update", "--exclude", "vendor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildUpdateArgs(tt.args)
			for _, expected := range tt.contains {
				found := false
				for _, arg := range result {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildUpdateArgs() missing %q in %v", expected, result)
				}
			}
		})
	}
}

func TestBuildSearchArgs_Basic(t *testing.T) {
	args := map[string]interface{}{
		"query": "test query",
	}
	result := buildSearchArgs(args)

	if result[0] != "search" {
		t.Errorf("buildSearchArgs() first arg = %s, want search", result[0])
	}
	if result[1] != "test query" {
		t.Errorf("buildSearchArgs() query = %s, want 'test query'", result[1])
	}
}

func TestBuildSearchArgs_MinFlagAddedGlobally(t *testing.T) {
	// Note: --min is added globally by ExecuteHandler, not by buildSearchArgs
	// This test verifies buildSearchArgs does NOT duplicate it
	args := map[string]interface{}{
		"query": "test",
		"min":   true,
	}
	result := buildSearchArgs(args)

	count := 0
	for _, arg := range result {
		if arg == "--min" {
			count++
		}
	}
	// buildSearchArgs should NOT add --min (ExecuteHandler adds it globally)
	if count > 0 {
		t.Errorf("buildSearchArgs() should not add --min (added globally), but found %d instances", count)
	}
}

func TestBuildArgs_AllCommands(t *testing.T) {
	tests := []struct {
		cmdName     string
		args        map[string]interface{}
		wantCommand string
		wantErr     bool
	}{
		{"search", map[string]interface{}{"query": "test"}, "search", false},
		{"index", map[string]interface{}{}, "index", false},
		{"index_status", map[string]interface{}{}, "index-status", false},
		{"index_update", map[string]interface{}{}, "index-update", false},
		{"memory_store", map[string]interface{}{"question": "Q", "answer": "A"}, "memory", false},
		{"memory_search", map[string]interface{}{"query": "test"}, "memory", false},
		{"memory_promote", map[string]interface{}{"id": "123", "target": "CLAUDE.md"}, "memory", false},
		{"memory_list", map[string]interface{}{}, "memory", false},
		{"memory_delete", map[string]interface{}{"id": "123"}, "memory", false},
		{"invalid_command", map[string]interface{}{}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmdName, func(t *testing.T) {
			result, err := buildArgs(tt.cmdName, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Error("buildArgs() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("buildArgs() unexpected error: %v", err)
				return
			}

			if result[0] != tt.wantCommand {
				t.Errorf("buildArgs() first arg = %s, want %s", result[0], tt.wantCommand)
			}
		})
	}
}

func TestResolveProfileSettings_EmptyProfile(t *testing.T) {
	args := map[string]interface{}{
		"query":   "test",
		"profile": "",
		"config":  "/some/config.yaml",
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// Nothing should be added with empty profile
	if _, ok := args["collection"]; ok {
		t.Error("resolveProfileSettings() should not add collection with empty profile")
	}
}

func TestResolveProfileSettings_EmptyConfig(t *testing.T) {
	args := map[string]interface{}{
		"query":   "test",
		"profile": "code",
		"config":  "",
	}

	err := resolveProfileSettings(args)
	if err != nil {
		t.Fatalf("resolveProfileSettings() error = %v", err)
	}

	// Nothing should be added with empty config
	if _, ok := args["collection"]; ok {
		t.Error("resolveProfileSettings() should not add collection with empty config")
	}
}

func TestBuildMemoryStoreArgs_AllParams(t *testing.T) {
	args := map[string]interface{}{
		"question":   "Q",
		"answer":     "A",
		"tags":       "tag1,tag2",
		"source":     "manual",
		"storage":    "qdrant",
		"collection": "my-collection",
	}

	result := buildMemoryStoreArgs(args)

	expected := []string{
		"--question", "Q",
		"--answer", "A",
		"--tags", "tag1,tag2",
		"--source", "manual",
		"--storage", "qdrant",
		"--collection", "my-collection",
	}

	for i := 0; i < len(expected); i += 2 {
		found := false
		for j := 2; j < len(result)-1; j++ {
			if result[j] == expected[i] && result[j+1] == expected[i+1] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildMemoryStoreArgs() missing %s %s", expected[i], expected[i+1])
		}
	}
}

func TestBuildMemoryListArgs_AllParams(t *testing.T) {
	args := map[string]interface{}{
		"limit":      float64(50),
		"status":     "pending",
		"storage":    "sqlite",
		"collection": "memory-collection",
	}

	result := buildMemoryListArgs(args)

	if result[0] != "memory" || result[1] != "list" {
		t.Errorf("buildMemoryListArgs() should start with [memory, list], got %v", result[:2])
	}

	// Check limit
	foundLimit := false
	for i, arg := range result {
		if arg == "--limit" && i+1 < len(result) && result[i+1] == "50" {
			foundLimit = true
			break
		}
	}
	if !foundLimit {
		t.Errorf("buildMemoryListArgs() missing --limit 50")
	}
}

func TestBuildMemorySearchArgs_AllParams(t *testing.T) {
	args := map[string]interface{}{
		"query":      "search query",
		"top_k":      float64(10),
		"threshold":  0.5,
		"tags":       "tag1",
		"status":     "promoted",
		"storage":    "qdrant",
		"collection": "mem-collection",
	}

	result := buildMemorySearchArgs(args)

	expected := []string{
		"--top", "--threshold", "--tags", "--status", "--storage", "--collection",
	}

	for _, flag := range expected {
		found := false
		for _, arg := range result {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildMemorySearchArgs() missing flag %s", flag)
		}
	}
}

func TestBuildMemoryPromoteArgs_AllParams(t *testing.T) {
	args := map[string]interface{}{
		"id":         "mem-123",
		"target":     "CLAUDE.md",
		"section":    "My Section",
		"force":      true,
		"storage":    "qdrant",
		"collection": "mem-collection",
	}

	result := buildMemoryPromoteArgs(args)

	for _, flag := range []string{"--target", "--section", "--force", "--storage", "--collection"} {
		found := false
		for _, arg := range result {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildMemoryPromoteArgs() missing flag %s in %v", flag, result)
		}
	}
}

func TestBuildMemoryDeleteArgs_AllParams(t *testing.T) {
	args := map[string]interface{}{
		"id":         "mem-456",
		"force":      true,
		"storage":    "sqlite",
		"collection": "del-collection",
	}

	result := buildMemoryDeleteArgs(args)

	if result[0] != "memory" || result[1] != "delete" || result[2] != "mem-456" {
		t.Errorf("buildMemoryDeleteArgs() should start with [memory, delete, mem-456], got %v", result[:3])
	}

	for _, flag := range []string{"--force", "--storage", "--collection"} {
		found := false
		for _, arg := range result {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("buildMemoryDeleteArgs() missing flag %s", flag)
		}
	}
}

// Test search with profiles parameter
func TestBuildSearchArgs_Profiles(t *testing.T) {
	args := map[string]interface{}{
		"query":    "authentication",
		"profiles": []interface{}{"code", "docs"},
	}

	result := buildSearchArgs(args)

	if result[0] != "search" {
		t.Errorf("buildSearchArgs() should start with 'search', got %s", result[0])
	}

	// Check --profiles flag is present with joined value
	found := false
	for i, arg := range result {
		if arg == "--profiles" && i+1 < len(result) && result[i+1] == "code,docs" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("buildSearchArgs() should have --profiles code,docs, got %v", result)
	}
}

func TestBuildSearchArgs_RerankParameters(t *testing.T) {
	args := map[string]interface{}{
		"query":             "authentication middleware",
		"rerank":            true,
		"rerank_candidates": float64(100),
		"rerank_threshold":  0.5,
	}

	result := buildSearchArgs(args)

	// Check base command
	if len(result) < 2 || result[0] != "search" {
		t.Errorf("buildSearchArgs() should start with 'search', got %v", result)
	}

	// Check --rerank flag
	foundRerank := false
	for _, arg := range result {
		if arg == "--rerank" {
			foundRerank = true
			break
		}
	}
	if !foundRerank {
		t.Error("buildSearchArgs() missing --rerank flag")
	}

	// Check --rerank-candidates
	foundCandidates := false
	for i, arg := range result {
		if arg == "--rerank-candidates" && i+1 < len(result) && result[i+1] == "100" {
			foundCandidates = true
			break
		}
	}
	if !foundCandidates {
		t.Errorf("buildSearchArgs() missing or incorrect --rerank-candidates 100, got %v", result)
	}

	// Check --rerank-threshold
	foundThreshold := false
	for i, arg := range result {
		if arg == "--rerank-threshold" && i+1 < len(result) {
			foundThreshold = true
			break
		}
	}
	if !foundThreshold {
		t.Errorf("buildSearchArgs() missing --rerank-threshold, got %v", result)
	}
}

func TestBuildSearchArgs_NoRerank(t *testing.T) {
	args := map[string]interface{}{
		"query":     "authentication middleware",
		"no_rerank": true,
	}

	result := buildSearchArgs(args)

	// Check --no-rerank flag
	foundNoRerank := false
	for _, arg := range result {
		if arg == "--no-rerank" {
			foundNoRerank = true
			break
		}
	}
	if !foundNoRerank {
		t.Error("buildSearchArgs() missing --no-rerank flag")
	}
}

// Test getStringSlice helper
func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		key      string
		expected []string
	}{
		{
			name:     "interface slice",
			args:     map[string]interface{}{"profiles": []interface{}{"code", "docs", "memory"}},
			key:      "profiles",
			expected: []string{"code", "docs", "memory"},
		},
		{
			name:     "string slice",
			args:     map[string]interface{}{"profiles": []string{"code", "docs"}},
			key:      "profiles",
			expected: []string{"code", "docs"},
		},
		{
			name:     "missing key",
			args:     map[string]interface{}{},
			key:      "profiles",
			expected: nil,
		},
		{
			name:     "empty slice",
			args:     map[string]interface{}{"profiles": []interface{}{}},
			key:      "profiles",
			expected: []string{},
		},
		{
			name:     "filters empty strings",
			args:     map[string]interface{}{"profiles": []interface{}{"code", "", "docs"}},
			key:      "profiles",
			expected: []string{"code", "docs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringSlice(tt.args, tt.key)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("getStringSlice() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test multisearch validation
func TestValidateMultisearchArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid single query",
			args:    map[string]interface{}{"queries": []interface{}{"auth"}},
			wantErr: false,
		},
		{
			name:    "valid multiple queries",
			args:    map[string]interface{}{"queries": []interface{}{"auth", "jwt", "login"}},
			wantErr: false,
		},
		{
			name:    "empty queries",
			args:    map[string]interface{}{"queries": []interface{}{}},
			wantErr: true,
			errMsg:  "at least one query",
		},
		{
			name:    "missing queries",
			args:    map[string]interface{}{},
			wantErr: true,
			errMsg:  "at least one query",
		},
		{
			name:    "too many queries",
			args:    map[string]interface{}{"queries": []interface{}{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}},
			wantErr: true,
			errMsg:  "up to 10 queries",
		},
		{
			name:    "empty query in list",
			args:    map[string]interface{}{"queries": []interface{}{"auth", "", "login"}},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "whitespace-only query",
			args:    map[string]interface{}{"queries": []interface{}{"auth", "   ", "login"}},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMultisearchArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMultisearchArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateMultisearchArgs() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// helper for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test multisearch args building
func TestBuildMultisearchArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		contains []string
	}{
		{
			name: "basic queries",
			args: map[string]interface{}{
				"queries": []interface{}{"auth", "jwt"},
			},
			contains: []string{"multisearch", "auth", "jwt"},
		},
		{
			name: "with all options",
			args: map[string]interface{}{
				"queries":    []interface{}{"auth", "login"},
				"top_k":      float64(20),
				"threshold":  0.5,
				"profiles":   []interface{}{"code", "docs"},
				"no_boost":   true,
				"no_dedupe":  true,
				"output":     "by_query",
				"storage":    "qdrant",
				"collection": "test_coll",
			},
			contains: []string{"multisearch", "auth", "login", "--top", "20", "--threshold", "--profiles", "code,docs", "--no-boost", "--no-dedupe", "--output", "by_query", "--storage", "qdrant", "--collection", "test_coll"},
		},
		{
			name: "output format by_collection",
			args: map[string]interface{}{
				"queries": []interface{}{"test"},
				"output":  "by_collection",
			},
			contains: []string{"multisearch", "--output", "by_collection"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildMultisearchArgs(tt.args)

			for _, expected := range tt.contains {
				found := false
				for _, arg := range result {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("buildMultisearchArgs() missing %q in %v", expected, result)
				}
			}
		})
	}
}

// Test convenience wrapper functions
func TestBuildSearchCodeArgs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: my-code-index
  code_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":  "authentication middleware",
		"config": configPath,
		"top_k":  float64(5),
	}

	result := buildSearchCodeArgs(args)

	// Should start with search command
	if result[0] != "search" {
		t.Errorf("buildSearchCodeArgs() first arg = %s, want 'search'", result[0])
	}

	// Profile should be set to code
	if args["profile"] != "code" {
		t.Errorf("buildSearchCodeArgs() should set profile to 'code', got %v", args["profile"])
	}

	// Collection should be resolved from config
	if args["collection"] != "my-code-index" {
		t.Errorf("buildSearchCodeArgs() collection = %v, want 'my-code-index'", args["collection"])
	}

	// Storage should be resolved from config
	if args["storage"] != "qdrant" {
		t.Errorf("buildSearchCodeArgs() storage = %v, want 'qdrant'", args["storage"])
	}
}

func TestBuildSearchDocsArgs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  docs_collection: my-docs-index
  docs_storage: sqlite
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":  "API authentication",
		"config": configPath,
	}

	result := buildSearchDocsArgs(args)

	// Should start with search command
	if result[0] != "search" {
		t.Errorf("buildSearchDocsArgs() first arg = %s, want 'search'", result[0])
	}

	// Profile should be set to docs
	if args["profile"] != "docs" {
		t.Errorf("buildSearchDocsArgs() should set profile to 'docs', got %v", args["profile"])
	}

	// Collection should be resolved from config
	if args["collection"] != "my-docs-index" {
		t.Errorf("buildSearchDocsArgs() collection = %v, want 'my-docs-index'", args["collection"])
	}
}

func TestBuildSearchMemoryArgs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  memory_collection: my-memory-index
  memory_storage: qdrant
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	args := map[string]interface{}{
		"query":  "authentication decisions",
		"config": configPath,
		"tags":   "auth,security",
	}

	result := buildSearchMemoryArgs(args)

	// Should start with memory search command
	if result[0] != "memory" || result[1] != "search" {
		t.Errorf("buildSearchMemoryArgs() should start with ['memory', 'search'], got %v", result[:2])
	}

	// Profile should be set to memory
	if args["profile"] != "memory" {
		t.Errorf("buildSearchMemoryArgs() should set profile to 'memory', got %v", args["profile"])
	}

	// Collection should be resolved from config
	if args["collection"] != "my-memory-index" {
		t.Errorf("buildSearchMemoryArgs() collection = %v, want 'my-memory-index'", args["collection"])
	}
}

func TestBuildSearchCodeArgs_WithoutConfig(t *testing.T) {
	// Test that it works even without config (just sets profile)
	args := map[string]interface{}{
		"query": "test query",
	}

	result := buildSearchCodeArgs(args)

	if result[0] != "search" {
		t.Errorf("buildSearchCodeArgs() first arg = %s, want 'search'", result[0])
	}

	if args["profile"] != "code" {
		t.Errorf("buildSearchCodeArgs() should set profile to 'code', got %v", args["profile"])
	}
}

func TestBuildArgs_ConvenienceWrappers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"

	configContent := `semantic:
  code_collection: code-idx
  code_storage: sqlite
  docs_collection: docs-idx
  docs_storage: qdrant
  memory_collection: memory-idx
  memory_storage: sqlite
`
	if err := writeTestFile(configPath, configContent); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	tests := []struct {
		cmdName     string
		args        map[string]interface{}
		wantCommand string
		wantProfile string
	}{
		{
			cmdName:     "search_code",
			args:        map[string]interface{}{"query": "test", "config": configPath},
			wantCommand: "search",
			wantProfile: "code",
		},
		{
			cmdName:     "search_docs",
			args:        map[string]interface{}{"query": "test", "config": configPath},
			wantCommand: "search",
			wantProfile: "docs",
		},
		{
			cmdName:     "search_memory",
			args:        map[string]interface{}{"query": "test", "config": configPath},
			wantCommand: "memory",
			wantProfile: "memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.cmdName, func(t *testing.T) {
			result, err := buildArgs(tt.cmdName, tt.args)
			if err != nil {
				t.Errorf("buildArgs(%s) error = %v", tt.cmdName, err)
				return
			}

			if result[0] != tt.wantCommand {
				t.Errorf("buildArgs(%s) first arg = %s, want %s", tt.cmdName, result[0], tt.wantCommand)
			}

			if tt.args["profile"] != tt.wantProfile {
				t.Errorf("buildArgs(%s) profile = %v, want %s", tt.cmdName, tt.args["profile"], tt.wantProfile)
			}
		})
	}
}

// Test multisearch in command registry
func TestBuildArgs_MultisearchCommand(t *testing.T) {
	args := map[string]interface{}{
		"queries": []interface{}{"authentication", "authorization"},
	}

	result, err := buildArgs("multisearch", args)
	if err != nil {
		t.Errorf("buildArgs(multisearch) error = %v", err)
		return
	}

	if result[0] != "multisearch" {
		t.Errorf("buildArgs(multisearch) first arg = %s, want 'multisearch'", result[0])
	}
}

func TestBuildIndexArgs_BatchSizeZeroNotIncluded(t *testing.T) {
	// When batch_size is 0, it should NOT be included in args (0 = unlimited)
	args := map[string]interface{}{
		"batch_size": float64(0),
	}

	result := buildIndexArgs(args)

	for _, arg := range result {
		if arg == "--batch-size" {
			t.Error("buildIndexArgs() should NOT include --batch-size when value is 0")
		}
	}
}

func TestBuildIndexArgs_BatchSizePositive(t *testing.T) {
	// When batch_size is positive, it should be included
	args := map[string]interface{}{
		"batch_size": float64(100),
	}

	result := buildIndexArgs(args)

	foundBatchSize := false
	for i, arg := range result {
		if arg == "--batch-size" && i+1 < len(result) && result[i+1] == "100" {
			foundBatchSize = true
			break
		}
	}

	if !foundBatchSize {
		t.Errorf("buildIndexArgs() should include --batch-size 100, got %v", result)
	}
}

func TestBuildIndexArgs_ParallelZeroNotIncluded(t *testing.T) {
	// When parallel is 0, it should NOT be included in args (0 = sequential)
	args := map[string]interface{}{
		"parallel": float64(0),
	}

	result := buildIndexArgs(args)

	for _, arg := range result {
		if arg == "--parallel" {
			t.Error("buildIndexArgs() should NOT include --parallel when value is 0")
		}
	}
}

func TestBuildIndexArgs_ParallelPositive(t *testing.T) {
	// When parallel is positive, it should be included
	args := map[string]interface{}{
		"parallel": float64(4),
	}

	result := buildIndexArgs(args)

	foundParallel := false
	for i, arg := range result {
		if arg == "--parallel" && i+1 < len(result) && result[i+1] == "4" {
			foundParallel = true
			break
		}
	}

	if !foundParallel {
		t.Errorf("buildIndexArgs() should include --parallel 4, got %v", result)
	}
}

func TestBuildIndexArgs_BatchSizeAndParallel(t *testing.T) {
	// Test both batch_size and parallel together
	args := map[string]interface{}{
		"batch_size": float64(64),
		"parallel":   float64(4),
	}

	result := buildIndexArgs(args)

	foundBatchSize := false
	foundParallel := false
	for i, arg := range result {
		if arg == "--batch-size" && i+1 < len(result) && result[i+1] == "64" {
			foundBatchSize = true
		}
		if arg == "--parallel" && i+1 < len(result) && result[i+1] == "4" {
			foundParallel = true
		}
	}

	if !foundBatchSize {
		t.Errorf("buildIndexArgs() should include --batch-size 64, got %v", result)
	}
	if !foundParallel {
		t.Errorf("buildIndexArgs() should include --parallel 4, got %v", result)
	}
}

func TestBuildIndexArgs_ExcludeTests(t *testing.T) {
	// When exclude_tests is true, it should include --exclude-tests
	args := map[string]interface{}{
		"exclude_tests": true,
	}

	result := buildIndexArgs(args)

	found := false
	for _, arg := range result {
		if arg == "--exclude-tests" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("buildIndexArgs() should include --exclude-tests, got %v", result)
	}
}

func TestBuildIndexArgs_ExcludeTestsFalse(t *testing.T) {
	// When exclude_tests is false, it should NOT include --exclude-tests
	args := map[string]interface{}{
		"exclude_tests": false,
	}

	result := buildIndexArgs(args)

	for _, arg := range result {
		if arg == "--exclude-tests" {
			t.Error("buildIndexArgs() should NOT include --exclude-tests when false")
		}
	}
}

func TestBuildIndexArgs_ExcludeWithFilePatterns(t *testing.T) {
	// Test that exclude patterns work with file patterns
	args := map[string]interface{}{
		"exclude": []interface{}{"vendor", "*_test.go", "*.spec.ts"},
	}

	result := buildIndexArgs(args)

	expectedExcludes := []string{"vendor", "*_test.go", "*.spec.ts"}
	foundCount := 0
	for i, arg := range result {
		if arg == "--exclude" && i+1 < len(result) {
			for _, expected := range expectedExcludes {
				if result[i+1] == expected {
					foundCount++
					break
				}
			}
		}
	}

	if foundCount != len(expectedExcludes) {
		t.Errorf("buildIndexArgs() should include all exclude patterns, got %v", result)
	}
}

func TestBuildIndexArgs_EmbedBatchSizePositive(t *testing.T) {
	// Test that embed_batch_size parameter is passed when positive
	args := map[string]interface{}{
		"embed_batch_size": float64(64), // JSON numbers are float64
	}

	result := buildIndexArgs(args)

	foundEmbedBatchSize := false
	for i, arg := range result {
		if arg == "--embed-batch-size" && i+1 < len(result) {
			if result[i+1] == "64" {
				foundEmbedBatchSize = true
			}
		}
	}

	if !foundEmbedBatchSize {
		t.Errorf("buildIndexArgs() should include --embed-batch-size 64, got %v", result)
	}
}

func TestBuildIndexArgs_EmbedBatchSizeZeroNotIncluded(t *testing.T) {
	// Test that embed_batch_size=0 is not included (0 means per-file batching)
	args := map[string]interface{}{
		"embed_batch_size": float64(0),
	}

	result := buildIndexArgs(args)

	for _, arg := range result {
		if arg == "--embed-batch-size" {
			t.Errorf("buildIndexArgs() should not include --embed-batch-size when value is 0, got %v", result)
		}
	}
}

func TestBuildIndexArgs_EmbedBatchSizeWithOtherOptions(t *testing.T) {
	// Test that embed_batch_size works with other batching options
	args := map[string]interface{}{
		"embed_batch_size": float64(128),
		"batch_size":       float64(64),
		"parallel":         float64(4),
	}

	result := buildIndexArgs(args)

	foundEmbedBatchSize := false
	foundBatchSize := false
	foundParallel := false

	for i, arg := range result {
		if arg == "--embed-batch-size" && i+1 < len(result) && result[i+1] == "128" {
			foundEmbedBatchSize = true
		}
		if arg == "--batch-size" && i+1 < len(result) && result[i+1] == "64" {
			foundBatchSize = true
		}
		if arg == "--parallel" && i+1 < len(result) && result[i+1] == "4" {
			foundParallel = true
		}
	}

	if !foundEmbedBatchSize {
		t.Errorf("buildIndexArgs() should include --embed-batch-size 128, got %v", result)
	}
	if !foundBatchSize {
		t.Errorf("buildIndexArgs() should include --batch-size 64, got %v", result)
	}
	if !foundParallel {
		t.Errorf("buildIndexArgs() should include --parallel 4, got %v", result)
	}
}
