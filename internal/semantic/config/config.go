// Package config provides configuration file support for llm-semantic commands.
// It enables YAML-based configuration with profile switching for code/docs/memory indexes.
package config

import (
	"os"
	"strings"

	"github.com/goccy/go-yaml"
)

// SemanticConfig represents the semantic configuration from a YAML file.
// Configuration is read from the "semantic:" key in the YAML file.
type SemanticConfig struct {
	// General settings (apply to all profiles)
	Enabled    bool    `yaml:"enabled"`
	AutoUpdate bool    `yaml:"auto_update"`
	MaxResults int     `yaml:"max_results"`
	MinScore   float64 `yaml:"min_score"`
	StaleDays  int     `yaml:"stale_days"`

	// Code profile settings
	CodeCollection string `yaml:"code_collection"`
	CodeEnabled    bool   `yaml:"code_enabled"`
	CodeStorage    string `yaml:"code_storage"`

	// Docs profile settings
	DocsCollection string `yaml:"docs_collection"`
	DocsEnabled    bool   `yaml:"docs_enabled"`
	DocsStorage    string `yaml:"docs_storage"`

	// Memory profile settings
	MemoryCollection string `yaml:"memory_collection"`
	MemoryEnabled    bool   `yaml:"memory_enabled"`
	MemoryStorage    string `yaml:"memory_storage"`

	// Sprints profile settings
	SprintsCollection string `yaml:"sprints_collection"`
	SprintsEnabled    bool   `yaml:"sprints_enabled"`
	SprintsStorage    string `yaml:"sprints_storage"`
}

// ProfileConfig represents resolved configuration for a specific profile.
// It contains only the fields relevant for a single profile context.
type ProfileConfig struct {
	Collection string
	Storage    string
	Enabled    bool
	MinScore   float64
	MaxResults int
	StaleDays  int
}

// configWrapper is used to parse the "semantic:" section from a YAML file.
type configWrapper struct {
	Semantic SemanticConfig `yaml:"semantic"`
}

// LoadConfig loads semantic configuration from a YAML file.
// It reads the "semantic:" section and ignores other sections.
// Returns an error if the file doesn't exist or contains invalid YAML.
func LoadConfig(path string) (*SemanticConfig, error) {
	// Validate path is not empty or whitespace
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, ErrConfigPathEmpty()
	}

	data, err := os.ReadFile(trimmedPath)
	if err != nil {
		return nil, WrapReadError(trimmedPath, err)
	}

	// Check for empty file
	if len(data) == 0 {
		return nil, ErrConfigEmpty(trimmedPath)
	}

	var wrapper configWrapper
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, ErrConfigInvalidYAML(trimmedPath, err)
	}

	return &wrapper.Semantic, nil
}

// GetProfileConfig returns the resolved configuration for a specific profile.
// Valid profiles are: "code" (default), "docs", "memory".
// Unknown profiles fall back to "code".
func (c *SemanticConfig) GetProfileConfig(profile string) ProfileConfig {
	// Default empty profile to "code"
	if profile == "" {
		profile = "code"
	}

	pc := ProfileConfig{
		MinScore:   c.MinScore,
		MaxResults: c.MaxResults,
		StaleDays:  c.StaleDays,
	}

	switch profile {
	case "docs":
		pc.Collection = c.DocsCollection
		pc.Storage = c.DocsStorage
		pc.Enabled = c.DocsEnabled
	case "memory":
		pc.Collection = c.MemoryCollection
		pc.Storage = c.MemoryStorage
		pc.Enabled = c.MemoryEnabled
	case "sprints":
		pc.Collection = c.SprintsCollection
		pc.Storage = c.SprintsStorage
		pc.Enabled = c.SprintsEnabled
	case "code":
		fallthrough
	default:
		// Default to code profile for unknown profiles
		pc.Collection = c.CodeCollection
		pc.Storage = c.CodeStorage
		pc.Enabled = c.CodeEnabled
	}

	return pc
}

// ValidProfiles returns the list of valid profile names.
func ValidProfiles() []string {
	return []string{"code", "docs", "memory", "sprints"}
}

// IsValidProfile checks if the given profile name is valid.
// Empty string is valid (defaults to "code").
func IsValidProfile(profile string) bool {
	if profile == "" {
		return true // empty defaults to code
	}
	for _, p := range ValidProfiles() {
		if p == profile {
			return true
		}
	}
	return false
}

// ResolveValue returns the explicit value if non-empty, otherwise the config value.
// This implements the precedence: explicit > config > default.
func ResolveValue(explicit, configValue string) string {
	if explicit != "" {
		return explicit
	}
	return configValue
}

// ResolveFloatValue returns the first non-zero value from explicit, config, or default.
// This implements the precedence: explicit > config > default.
func ResolveFloatValue(explicit, configValue, defaultValue float64) float64 {
	if explicit != 0 {
		return explicit
	}
	if configValue != 0 {
		return configValue
	}
	return defaultValue
}

// ResolveIntValue returns the first non-zero value from explicit, config, or default.
// This implements the precedence: explicit > config > default.
func ResolveIntValue(explicit, configValue, defaultValue int) int {
	if explicit != 0 {
		return explicit
	}
	if configValue != 0 {
		return configValue
	}
	return defaultValue
}
