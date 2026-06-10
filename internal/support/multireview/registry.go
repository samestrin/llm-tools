package multireview

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/samestrin/llm-tools/pkg/llmapi"
	"gopkg.in/yaml.v3"
)

// ProviderConfig defines credentials and endpoint for an LLM provider.
type ProviderConfig struct {
	Name      string `yaml:"-"`           // Set during loading
	APIKeyEnv string `yaml:"api_key_env"` // Environment variable name for API key
	BaseURL   string `yaml:"base_url"`    // API endpoint URL
}

// AgentConfig defines a single reviewer agent.
type AgentConfig struct {
	Name         string  `yaml:"-"`            // Set during loading
	Provider     string  `yaml:"provider"`     // Provider name from ProviderConfig
	Model        string  `yaml:"model"`        // Model identifier
	SystemPrompt string  `yaml:"-"`            // Loaded from <agent>.md
	TimeoutSecs  int     `yaml:"timeout_secs"` // Per-agent timeout (default: 600)
	RateLimited  bool    `yaml:"rate_limited"` // If true, runs in serial lane
	Temperature  float64 `yaml:"temperature"`  // Generation temperature (default: 0.7)
	Fallback     string  `yaml:"fallback"`     // Fallback agent name if this agent fails
}

// Registry holds all provider and agent configurations.
type Registry struct {
	Providers map[string]ProviderConfig
	Agents    map[string]AgentConfig
}

// registryFile is the YAML structure on disk.
type registryFile struct {
	Providers map[string]ProviderConfig `yaml:"providers"`
	Agents    map[string]agentFileEntry `yaml:"agents"`
}

// agentFileEntry is the YAML structure for an agent entry.
type agentFileEntry struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	TimeoutSecs int     `yaml:"timeout_secs"`
	RateLimited bool    `yaml:"rate_limited"`
	Temperature float64 `yaml:"temperature"`
	Fallback    string  `yaml:"fallback"`
}

// DefaultRegistryDir returns the standard location for agent configurations.
// Returns ~/.config/llm-tools/agents/
func DefaultRegistryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "llm-tools", "agents")
}

// embeddedDefaults provides fallback configuration when registry.yaml is missing.
func embeddedDefaults() *Registry {
	return &Registry{
		Providers: map[string]ProviderConfig{
			"openai": {
				Name:      "openai",
				APIKeyEnv: "OPENAI_API_KEY",
				BaseURL:   "https://api.openai.com/v1",
			},
			"anthropic": {
				Name:      "anthropic",
				APIKeyEnv: "ANTHROPIC_API_KEY",
				BaseURL:   "https://api.anthropic.com/v1",
			},
			"google": {
				Name:      "google",
				APIKeyEnv: "GOOGLE_API_KEY",
				BaseURL:   "https://generativelanguage.googleapis.com/v1beta/openai",
			},
			"openrouter": {
				Name:      "openrouter",
				APIKeyEnv: "OPENROUTER_API_KEY",
				BaseURL:   "https://openrouter.ai/api/v1",
			},
		},
		Agents: map[string]AgentConfig{},
	}
}

// LoadRegistry reads registry.yaml and per-agent system prompts from the given directory.
// Falls back to embedded defaults if registry.yaml is missing.
func LoadRegistry(dir string) (*Registry, error) {
	registryPath := filepath.Join(dir, "registry.yaml")

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Use embedded defaults
			return embeddedDefaults(), nil
		}
		return nil, fmt.Errorf("read registry.yaml: %w", err)
	}

	var rf registryFile
	if err := yaml.Unmarshal(data, &rf); err != nil {
		return nil, fmt.Errorf("parse registry.yaml: %w", err)
	}

	reg := &Registry{
		Providers: make(map[string]ProviderConfig),
		Agents:    make(map[string]AgentConfig),
	}

	// Copy providers, setting Name field
	for name, p := range rf.Providers {
		p.Name = name
		reg.Providers[name] = p
	}

	// Load base system prompt (fallback)
	basePrompt := loadPromptFile(filepath.Join(dir, "_base.md"))

	// Convert agents, loading system prompts
	for name, a := range rf.Agents {
		agent := AgentConfig{
			Name:        name,
			Provider:    a.Provider,
			Model:       a.Model,
			TimeoutSecs: a.TimeoutSecs,
			RateLimited: a.RateLimited,
			Temperature: a.Temperature,
			Fallback:    a.Fallback,
		}

		// Apply defaults
		if agent.TimeoutSecs == 0 {
			agent.TimeoutSecs = 600 // 10 minutes
		}
		if agent.Temperature == 0.0 {
			agent.Temperature = 0.7
		}

		// Load agent-specific prompt, fall back to base
		agentPromptPath := filepath.Join(dir, name+".md")
		if prompt := loadPromptFile(agentPromptPath); prompt != "" {
			agent.SystemPrompt = prompt
		} else {
			agent.SystemPrompt = basePrompt
		}

		reg.Agents[name] = agent
	}

	// Fail fast on broken fallback configuration — discovering a typo'd
	// fallback name mid-run, after the primary agent has already burned its
	// timeout budget, costs minutes; failing at load costs nothing. Walk each
	// agent's full fallback chain so cycles (which would recurse forever at
	// invoke time) are rejected too.
	for name, agent := range reg.Agents {
		visited := map[string]bool{name: true}
		current := agent
		for current.Fallback != "" {
			next, ok := reg.Agents[current.Fallback]
			if !ok {
				return nil, fmt.Errorf("agent %q: fallback %q is not defined in registry.yaml (agents must reference an existing agent name)", current.Name, current.Fallback)
			}
			if visited[current.Fallback] {
				return nil, fmt.Errorf("agent %q: fallback chain contains a cycle through %q", name, current.Fallback)
			}
			visited[current.Fallback] = true
			current = next
		}
	}

	return reg, nil
}

// loadPromptFile reads a markdown file and returns its contents.
// Returns empty string if file doesn't exist or can't be read.
func loadPromptFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// GetAgent returns the configuration for an agent by name.
func (r *Registry) GetAgent(name string) (AgentConfig, error) {
	agent, ok := r.Agents[name]
	if !ok {
		return AgentConfig{}, fmt.Errorf("agent not found: %s", name)
	}
	return agent, nil
}

// ResolveProvider returns an API configuration with credentials resolved from environment variables.
func (r *Registry) ResolveProvider(name string) (*llmapi.APIConfig, error) {
	provider, ok := r.Providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}

	apiKey := os.Getenv(provider.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s not set for provider %s", provider.APIKeyEnv, name)
	}

	return &llmapi.APIConfig{
		APIKey:  apiKey,
		BaseURL: provider.BaseURL,
		Model:   "", // Set by caller based on agent config
	}, nil
}
