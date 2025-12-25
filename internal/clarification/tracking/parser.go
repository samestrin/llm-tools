package tracking

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// LoadTrackingFile loads and parses a YAML tracking file from the given path.
func LoadTrackingFile(path string) (*TrackingFile, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("tracking file not found: %s", path)
	}

	// Read file contents
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracking file: %w", err)
	}

	// Parse YAML
	var tf TrackingFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("failed to parse tracking file YAML: %w", err)
	}

	// Ensure entries is not nil
	if tf.Entries == nil {
		tf.Entries = []Entry{}
	}

	return &tf, nil
}

// SaveTrackingFile writes a TrackingFile to the given path as YAML.
func SaveTrackingFile(tf *TrackingFile, path string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Ensure entries is not nil for proper YAML output
	if tf.Entries == nil {
		tf.Entries = []Entry{}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(tf)
	if err != nil {
		return fmt.Errorf("failed to marshal tracking file: %w", err)
	}

	// Write to file with 0644 permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracking file: %w", err)
	}

	return nil
}

// FileExists checks if a file exists at the given path.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
