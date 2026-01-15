// Package config provides configuration file support for llm-semantic commands.
// It enables YAML-based configuration with profile switching for code/docs/memory indexes.
package config

import (
	"fmt"
	"os"

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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var wrapper configWrapper
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
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
	return []string{"code", "docs", "memory"}
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
