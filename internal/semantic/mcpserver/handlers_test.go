package mcpserver

import (
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
