package multireview

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRegistryDir(t *testing.T) {
	dir := DefaultRegistryDir()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	expected := filepath.Join(home, ".config", "llm-tools", "agents")
	if dir != expected {
		t.Errorf("DefaultRegistryDir() = %q, want %q", dir, expected)
	}
}

func TestLoadRegistry_ValidYAML(t *testing.T) {
	dir := t.TempDir()

	// Write valid registry.yaml
	registryYAML := `
providers:
  openai:
    api_key_env: OPENAI_API_KEY
    base_url: https://api.openai.com/v1
  anthropic:
    api_key_env: ANTHROPIC_API_KEY
    base_url: https://api.anthropic.com/v1

agents:
  bruce:
    provider: openai
    model: gpt-4o
    timeout_secs: 600
    temperature: 0.3
  greta:
    provider: anthropic
    model: claude-sonnet-4-20250514
    timeout_secs: 900
    rate_limited: true
    temperature: 0.2
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	// Write agent persona files
	if err := os.WriteFile(filepath.Join(dir, "bruce.md"), []byte("You are Bruce, a meticulous reviewer."), 0o644); err != nil {
		t.Fatalf("write bruce.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_base.md"), []byte("You are a code reviewer."), 0o644); err != nil {
		t.Fatalf("write _base.md: %v", err)
	}

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	// Check providers
	if len(reg.Providers) != 2 {
		t.Errorf("len(Providers) = %d, want 2", len(reg.Providers))
	}
	if p, ok := reg.Providers["openai"]; !ok {
		t.Error("missing provider: openai")
	} else {
		if p.APIKeyEnv != "OPENAI_API_KEY" {
			t.Errorf("openai.APIKeyEnv = %q, want OPENAI_API_KEY", p.APIKeyEnv)
		}
		if p.BaseURL != "https://api.openai.com/v1" {
			t.Errorf("openai.BaseURL = %q", p.BaseURL)
		}
	}

	// Check agents
	if len(reg.Agents) != 2 {
		t.Errorf("len(Agents) = %d, want 2", len(reg.Agents))
	}
	if a, ok := reg.Agents["bruce"]; !ok {
		t.Error("missing agent: bruce")
	} else {
		if a.Provider != "openai" {
			t.Errorf("bruce.Provider = %q, want openai", a.Provider)
		}
		if a.Model != "gpt-4o" {
			t.Errorf("bruce.Model = %q, want gpt-4o", a.Model)
		}
		if a.TimeoutSecs != 600 {
			t.Errorf("bruce.TimeoutSecs = %d, want 600", a.TimeoutSecs)
		}
		if a.Temperature != 0.3 {
			t.Errorf("bruce.Temperature = %f, want 0.3", a.Temperature)
		}
		if a.SystemPrompt != "You are Bruce, a meticulous reviewer." {
			t.Errorf("bruce.SystemPrompt = %q", a.SystemPrompt)
		}
	}

	// Check greta uses _base.md fallback
	if a, ok := reg.Agents["greta"]; !ok {
		t.Error("missing agent: greta")
	} else {
		if !a.RateLimited {
			t.Error("greta.RateLimited = false, want true")
		}
		// greta.md doesn't exist, should fall back to _base.md
		if a.SystemPrompt != "You are a code reviewer." {
			t.Errorf("greta.SystemPrompt = %q, want fallback from _base.md", a.SystemPrompt)
		}
	}
}

func TestLoadRegistry_MissingFile_UsesDefaults(t *testing.T) {
	dir := t.TempDir()
	// Empty directory, no registry.yaml

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	// Should have embedded defaults
	if len(reg.Providers) == 0 {
		t.Error("expected embedded default providers")
	}
	if _, ok := reg.Providers["openai"]; !ok {
		t.Error("expected default openai provider")
	}
}

func TestLoadRegistry_InvalidYAML(t *testing.T) {
	dir := t.TempDir()

	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("invalid: [yaml: content"), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	_, err := LoadRegistry(dir)
	if err == nil {
		t.Error("LoadRegistry() expected error for invalid YAML")
	}
}

func TestGetAgent_Found(t *testing.T) {
	reg := &Registry{
		Agents: map[string]AgentConfig{
			"bruce": {
				Name:        "bruce",
				Provider:    "openai",
				Model:       "gpt-4o",
				TimeoutSecs: 600,
			},
		},
	}

	agent, err := reg.GetAgent("bruce")
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if agent.Name != "bruce" {
		t.Errorf("agent.Name = %q, want bruce", agent.Name)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	reg := &Registry{
		Agents: map[string]AgentConfig{},
	}

	_, err := reg.GetAgent("nonexistent")
	if err == nil {
		t.Error("GetAgent() expected error for nonexistent agent")
	}
}

func TestResolveProvider_Success(t *testing.T) {
	reg := &Registry{
		Providers: map[string]ProviderConfig{
			"openai": {
				Name:      "openai",
				APIKeyEnv: "TEST_OPENAI_KEY",
				BaseURL:   "https://api.openai.com/v1",
			},
		},
	}

	// Set env var for test
	os.Setenv("TEST_OPENAI_KEY", "sk-test-key")
	defer os.Unsetenv("TEST_OPENAI_KEY")

	cfg, err := reg.ResolveProvider("openai")
	if err != nil {
		t.Fatalf("ResolveProvider() error = %v", err)
	}
	if cfg.APIKey != "sk-test-key" {
		t.Errorf("cfg.APIKey = %q, want sk-test-key", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("cfg.BaseURL = %q", cfg.BaseURL)
	}
}

func TestResolveProvider_MissingEnvVar(t *testing.T) {
	reg := &Registry{
		Providers: map[string]ProviderConfig{
			"openai": {
				Name:      "openai",
				APIKeyEnv: "NONEXISTENT_API_KEY_12345",
				BaseURL:   "https://api.openai.com/v1",
			},
		},
	}

	// Ensure env var is not set
	os.Unsetenv("NONEXISTENT_API_KEY_12345")

	_, err := reg.ResolveProvider("openai")
	if err == nil {
		t.Error("ResolveProvider() expected error for missing env var")
	}
}

func TestResolveProvider_NotFound(t *testing.T) {
	reg := &Registry{
		Providers: map[string]ProviderConfig{},
	}

	_, err := reg.ResolveProvider("nonexistent")
	if err == nil {
		t.Error("ResolveProvider() expected error for nonexistent provider")
	}
}

func TestLoadRegistry_AgentTimeoutDefaults(t *testing.T) {
	dir := t.TempDir()

	// Agent without timeout_secs should get default
	registryYAML := `
providers:
  openai:
    api_key_env: OPENAI_API_KEY
    base_url: https://api.openai.com/v1

agents:
  notimeout:
    provider: openai
    model: gpt-4o
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	agent, ok := reg.Agents["notimeout"]
	if !ok {
		t.Fatal("missing agent: notimeout")
	}

	// Default timeout should be applied
	if agent.TimeoutSecs == 0 {
		t.Error("agent.TimeoutSecs should have default, got 0")
	}
}

func TestLoadRegistry_AgentTemperatureDefaults(t *testing.T) {
	dir := t.TempDir()

	// Agent without temperature should get default
	registryYAML := `
providers:
  openai:
    api_key_env: OPENAI_API_KEY
    base_url: https://api.openai.com/v1

agents:
  notemp:
    provider: openai
    model: gpt-4o
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	agent, ok := reg.Agents["notemp"]
	if !ok {
		t.Fatal("missing agent: notemp")
	}

	// Default temperature should be applied (not 0.0)
	if agent.Temperature == 0.0 {
		t.Error("agent.Temperature should have default, got 0.0")
	}
}

func TestLoadRegistry_AgentFallback(t *testing.T) {
	dir := t.TempDir()

	// Agent with fallback configured
	registryYAML := `
providers:
  openrouter:
    api_key_env: OPENROUTER_API_KEY
    base_url: https://openrouter.ai/api/v1
  ollama:
    api_key_env: OLLAMA_API_KEY
    base_url: http://localhost:11434/v1

agents:
  dax:
    provider: openrouter
    model: deepseek/deepseek-r1
    fallback: dax-local
  dax-local:
    provider: ollama
    model: deepseek-r1:14b
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(registryYAML), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}

	// Check dax has fallback
	dax, ok := reg.Agents["dax"]
	if !ok {
		t.Fatal("missing agent: dax")
	}
	if dax.Fallback != "dax-local" {
		t.Errorf("dax.Fallback = %q, want dax-local", dax.Fallback)
	}

	// Check dax-local has no fallback
	daxLocal, ok := reg.Agents["dax-local"]
	if !ok {
		t.Fatal("missing agent: dax-local")
	}
	if daxLocal.Fallback != "" {
		t.Errorf("dax-local.Fallback = %q, want empty", daxLocal.Fallback)
	}
}
