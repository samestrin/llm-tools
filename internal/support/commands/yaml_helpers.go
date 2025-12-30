package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/gofrs/flock"
)

// Built-in templates for yaml init

var planningTemplate = `# llm-support YAML configuration
# Created by: llm-support yaml init --template planning

helper:
  script: llm-support
  llm: gemini
  llm_cmd: "-p"
  max_lines: 2000

project:
  type: ""
  framework: ""
  package_manager: ""
  source_directory: src/

testing:
  runner: ""
  directory: ""
  cmd: ""
  coverage_cmd: ""

commands:
  lint: ""
  types: ""
  build: ""

tools:
  html2text: html2text
  clarification_script: ""
`

var minimalTemplate = `# llm-support YAML configuration
# Created by: llm-support yaml init --template minimal

config: {}
`

// getTemplate returns the template content for a given template name or file path
func getTemplate(templateName string) (string, error) {
	switch templateName {
	case "planning", "":
		return planningTemplate, nil
	case "minimal":
		return minimalTemplate, nil
	default:
		// Treat as file path
		content, err := os.ReadFile(templateName)
		if err != nil {
			return "", fmt.Errorf("failed to read template file: %w", err)
		}
		return string(content), nil
	}
}

// yamlFileLock acquires a file lock for concurrent safety
func yamlFileLock(filePath string, exclusive bool) (*flock.Flock, error) {
	lockFile := filePath + ".lock"
	fl := flock.New(lockFile)

	var err error
	if exclusive {
		err = fl.Lock()
	} else {
		err = fl.RLock()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return fl, nil
}

// readYAMLFile reads and parses a YAML file, returning the parsed AST file
func readYAMLFile(filePath string) (*ast.File, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	file, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	return file, nil
}

// readYAMLAsMap reads a YAML file and returns it as a nested map
func readYAMLAsMap(filePath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if data == nil {
		data = make(map[string]interface{})
	}

	return data, nil
}

// writeYAMLFile writes data to a YAML file atomically (temp file + rename)
func writeYAMLFile(filePath string, data interface{}) error {
	content, err := yaml.MarshalWithOptions(data, yaml.IndentSequence(true))
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	return writeYAMLFileRaw(filePath, content)
}

// writeYAMLFileRaw writes raw bytes to a YAML file atomically
func writeYAMLFileRaw(filePath string, content []byte) error {
	// Write to temp file first
	dir := filepath.Dir(filePath)
	tmpFile, err := os.CreateTemp(dir, ".yaml-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// convertDotPathToYAMLPath converts dot notation (helper.llm) to YAML path syntax ($.helper.llm)
func convertDotPathToYAMLPath(dotPath string) string {
	// Handle empty path
	if dotPath == "" {
		return "$"
	}

	// Handle array indices - convert items[0] to items.0
	// The goccy/go-yaml path syntax uses $.key[0] for arrays
	return "$." + dotPath
}

// getValueAtPath retrieves a value from YAML data using dot notation
func getValueAtPath(data map[string]interface{}, dotPath string) (interface{}, bool) {
	parts := parseDotPath(dotPath)
	if len(parts) == 0 {
		return data, true
	}

	current := interface{}(data)
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		case map[interface{}]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		case []interface{}:
			// Handle array index
			idx := parseArrayIndex(part)
			if idx < 0 || idx >= len(v) {
				return nil, false
			}
			current = v[idx]
		default:
			return nil, false
		}
	}

	return current, true
}

// setValueAtPath sets a value in YAML data using dot notation, creating intermediate keys
func setValueAtPath(data map[string]interface{}, dotPath string, value interface{}) error {
	parts := parseDotPath(dotPath)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	current := data
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		if existing, ok := current[part]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				current = m
			} else if m, ok := existing.(map[interface{}]interface{}); ok {
				// Convert to map[string]interface{}
				converted := make(map[string]interface{})
				for k, v := range m {
					if ks, ok := k.(string); ok {
						converted[ks] = v
					}
				}
				current[part] = converted
				current = converted
			} else {
				// Replace non-map value with a new map
				newMap := make(map[string]interface{})
				current[part] = newMap
				current = newMap
			}
		} else {
			// Create intermediate map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}

	// Set the final value
	current[parts[len(parts)-1]] = value
	return nil
}

// deleteValueAtPath deletes a value from YAML data using dot notation
func deleteValueAtPath(data map[string]interface{}, dotPath string) error {
	parts := parseDotPath(dotPath)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	if len(parts) == 1 {
		delete(data, parts[0])
		return nil
	}

	// Navigate to parent
	current := data
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if existing, ok := current[part]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				current = m
			} else if m, ok := existing.(map[interface{}]interface{}); ok {
				// Convert
				converted := make(map[string]interface{})
				for k, v := range m {
					if ks, ok := k.(string); ok {
						converted[ks] = v
					}
				}
				current = converted
			} else {
				return fmt.Errorf("path %s is not traversable", strings.Join(parts[:i+1], "."))
			}
		} else {
			return nil // Key doesn't exist, nothing to delete
		}
	}

	delete(current, parts[len(parts)-1])
	return nil
}

// parseDotPath splits a dot-notation path into parts, handling escaped dots
func parseDotPath(path string) []string {
	if path == "" {
		return nil
	}

	var parts []string
	current := strings.Builder{}

	for i := 0; i < len(path); i++ {
		ch := path[i]
		if ch == '\\' && i+1 < len(path) && path[i+1] == '.' {
			// Escaped dot
			current.WriteByte('.')
			i++
		} else if ch == '.' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseArrayIndex parses an array index from a path part like "0" or "[0]"
func parseArrayIndex(part string) int {
	// Remove brackets if present
	part = strings.TrimPrefix(part, "[")
	part = strings.TrimSuffix(part, "]")

	var idx int
	_, err := fmt.Sscanf(part, "%d", &idx)
	if err != nil {
		return -1
	}
	return idx
}

// flattenKeys recursively flattens a nested map into dot-notation keys
func flattenKeys(data map[string]interface{}, prefix string) []string {
	var keys []string

	for k, v := range data {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			keys = append(keys, flattenKeys(val, fullKey)...)
		case map[interface{}]interface{}:
			// Convert to map[string]interface{}
			converted := make(map[string]interface{})
			for mk, mv := range val {
				if ks, ok := mk.(string); ok {
					converted[ks] = mv
				}
			}
			keys = append(keys, flattenKeys(converted, fullKey)...)
		default:
			keys = append(keys, fullKey)
		}
	}

	return keys
}

// flattenKeysWithValues recursively flattens a nested map into key=value pairs
func flattenKeysWithValues(data map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)

	for k, v := range data {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			for fk, fv := range flattenKeysWithValues(val, fullKey) {
				result[fk] = fv
			}
		case map[interface{}]interface{}:
			// Convert to map[string]interface{}
			converted := make(map[string]interface{})
			for mk, mv := range val {
				if ks, ok := mk.(string); ok {
					converted[ks] = mv
				}
			}
			for fk, fv := range flattenKeysWithValues(converted, fullKey) {
				result[fk] = fv
			}
		default:
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}

	return result
}

// countKeys counts the total number of leaf keys in a nested map
func countKeys(data map[string]interface{}) int {
	count := 0

	for _, v := range data {
		switch val := v.(type) {
		case map[string]interface{}:
			count += countKeys(val)
		case map[interface{}]interface{}:
			converted := make(map[string]interface{})
			for mk, mv := range val {
				if ks, ok := mk.(string); ok {
					converted[ks] = mv
				}
			}
			count += countKeys(converted)
		default:
			count++
		}
	}

	return count
}

// getTopLevelSections returns the names of top-level sections in the YAML
func getTopLevelSections(data map[string]interface{}) []string {
	var sections []string
	for k := range data {
		sections = append(sections, k)
	}
	return sections
}

// pushToArray appends a value to an array at the given path
func pushToArray(data map[string]interface{}, dotPath string, value interface{}) error {
	existing, found := getValueAtPath(data, dotPath)
	if !found {
		// Create new array
		return setValueAtPath(data, dotPath, []interface{}{value})
	}

	arr, ok := existing.([]interface{})
	if !ok {
		return fmt.Errorf("path %s is not an array", dotPath)
	}

	arr = append(arr, value)
	return setValueAtPath(data, dotPath, arr)
}

// popFromArray removes and returns the last element from an array at the given path
func popFromArray(data map[string]interface{}, dotPath string) (interface{}, error) {
	existing, found := getValueAtPath(data, dotPath)
	if !found {
		return nil, fmt.Errorf("path %s not found", dotPath)
	}

	arr, ok := existing.([]interface{})
	if !ok {
		return nil, fmt.Errorf("path %s is not an array", dotPath)
	}

	if len(arr) == 0 {
		return nil, fmt.Errorf("array at %s is empty", dotPath)
	}

	lastValue := arr[len(arr)-1]
	arr = arr[:len(arr)-1]

	if err := setValueAtPath(data, dotPath, arr); err != nil {
		return nil, err
	}

	return lastValue, nil
}

// setValuePreservingComments modifies a YAML file while preserving comments
// Uses AST manipulation instead of map conversion
func setValuePreservingComments(filePath string, dotPath string, value interface{}) error {
	// Read the original file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Parse with comments
	file, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Convert dot path to YAML path syntax
	yamlPath := convertDotPathToYAMLPath(dotPath)

	// Create path object
	path, err := yaml.PathString(yamlPath)
	if err != nil {
		return fmt.Errorf("invalid path %s: %w", dotPath, err)
	}

	// Check if the path exists in the AST
	var existingValue interface{}
	pathExistsErr := path.Read(file, &existingValue)

	// Create the replacement node
	valueBytes, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// If path exists, use AST replacement to preserve comments
	if pathExistsErr == nil {
		err = path.ReplaceWithReader(file, strings.NewReader(strings.TrimSpace(string(valueBytes))))
		if err != nil {
			return fmt.Errorf("failed to replace value: %w", err)
		}
		// Write the modified AST back (preserves comments)
		return writeYAMLFileRaw(filePath, []byte(file.String()))
	}

	// Path doesn't exist - fall back to map-based approach
	// This handles creating new keys (less common case)
	data, mapErr := readYAMLAsMap(filePath)
	if mapErr != nil {
		return mapErr
	}
	if setErr := setValueAtPath(data, dotPath, value); setErr != nil {
		return setErr
	}
	return writeYAMLFile(filePath, data)
}
