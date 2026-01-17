package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
)

// invalidArrayIndex is a sentinel value for parseArrayIndex when input is not a valid integer.
// We use -999 because -1, -2, etc. are valid negative indices for accessing array elements from the end.
const invalidArrayIndex = -999

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
// Supports negative array indices: -1 = last, -2 = second to last, etc.
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
			// Handle array index (including negative indices)
			idx := parseArrayIndex(part)
			if idx == invalidArrayIndex {
				// Not a valid index
				return nil, false
			}
			// Handle negative indices: -1 = last, -2 = second to last, etc.
			if idx < 0 {
				idx = len(v) + idx
			}
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
// Supports array index traversal and setting values at array indices (including negative indices)
func setValueAtPath(data map[string]interface{}, dotPath string, value interface{}) error {
	parts := parseDotPath(dotPath)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	// Use interface{} to track current position (can be map or array)
	var currentVal interface{} = data
	var parentVal interface{}
	var parentKey string
	var parentIdx int = -1

	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]

		switch curr := currentVal.(type) {
		case map[string]interface{}:
			if existing, ok := curr[part]; ok {
				if m, ok := existing.(map[string]interface{}); ok {
					parentVal = currentVal
					parentKey = part
					parentIdx = -1
					currentVal = m
				} else if m, ok := existing.(map[interface{}]interface{}); ok {
					// Convert to map[string]interface{}
					converted := make(map[string]interface{})
					for k, v := range m {
						if ks, ok := k.(string); ok {
							converted[ks] = v
						}
					}
					curr[part] = converted
					parentVal = currentVal
					parentKey = part
					parentIdx = -1
					currentVal = converted
				} else if arr, ok := existing.([]interface{}); ok {
					// Next part should be an array index
					parentVal = currentVal
					parentKey = part
					parentIdx = -1
					currentVal = arr
				} else {
					// Replace non-map value with a new map
					newMap := make(map[string]interface{})
					curr[part] = newMap
					parentVal = currentVal
					parentKey = part
					parentIdx = -1
					currentVal = newMap
				}
			} else {
				// Create intermediate map
				newMap := make(map[string]interface{})
				curr[part] = newMap
				parentVal = currentVal
				parentKey = part
				parentIdx = -1
				currentVal = newMap
			}

		case []interface{}:
			// Handle array index traversal
			idx := parseArrayIndex(part)
			if idx == invalidArrayIndex {
				return fmt.Errorf("invalid array index: %s", part)
			}
			// Handle negative indices
			if idx < 0 {
				idx = len(curr) + idx
			}
			if idx < 0 || idx >= len(curr) {
				return fmt.Errorf("array index out of bounds: %d (length: %d)", idx, len(curr))
			}

			elem := curr[idx]
			if m, ok := elem.(map[string]interface{}); ok {
				parentVal = currentVal
				parentIdx = idx
				parentKey = ""
				currentVal = m
			} else if m, ok := elem.(map[interface{}]interface{}); ok {
				// Convert to map[string]interface{}
				converted := make(map[string]interface{})
				for k, v := range m {
					if ks, ok := k.(string); ok {
						converted[ks] = v
					}
				}
				curr[idx] = converted
				parentVal = currentVal
				parentIdx = idx
				parentKey = ""
				currentVal = converted
			} else if arr, ok := elem.([]interface{}); ok {
				parentVal = currentVal
				parentIdx = idx
				parentKey = ""
				currentVal = arr
			} else {
				// Need to traverse further but hit a scalar - replace with map
				newMap := make(map[string]interface{})
				curr[idx] = newMap
				parentVal = currentVal
				parentIdx = idx
				parentKey = ""
				currentVal = newMap
			}

		default:
			return fmt.Errorf("cannot traverse path at %s", part)
		}
	}

	// Set the final value
	finalPart := parts[len(parts)-1]

	switch curr := currentVal.(type) {
	case map[string]interface{}:
		curr[finalPart] = value
	case []interface{}:
		idx := parseArrayIndex(finalPart)
		if idx == invalidArrayIndex {
			return fmt.Errorf("invalid array index: %s", finalPart)
		}
		// Handle negative indices
		if idx < 0 {
			idx = len(curr) + idx
		}
		if idx < 0 || idx >= len(curr) {
			return fmt.Errorf("array index out of bounds: %d (length: %d)", idx, len(curr))
		}
		curr[idx] = value
		// Update parent reference since slice assignment doesn't modify the original
		if parentVal != nil {
			if parentIdx >= 0 {
				parentVal.([]interface{})[parentIdx] = curr
			} else if parentKey != "" {
				parentVal.(map[string]interface{})[parentKey] = curr
			}
		}
	default:
		return fmt.Errorf("cannot set value at path")
	}

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

// parseDotPath splits a dot-notation path into parts, handling escaped dots and bracket notation
// Supports: "a.b.c", "items[0].name", "items[-1]", "a\.b.c" (escaped dot), "a[0][1]" (nested arrays)
func parseDotPath(path string) []string {
	if path == "" {
		return nil
	}

	var parts []string
	current := strings.Builder{}

	for i := 0; i < len(path); i++ {
		ch := path[i]

		switch {
		case ch == '\\' && i+1 < len(path) && path[i+1] == '.':
			// Escaped dot - include literal dot
			current.WriteByte('.')
			i++

		case ch == '.':
			// Dot separator - flush current segment
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}

		case ch == '[':
			// Array bracket - flush current segment if any, then parse index
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			// Find closing bracket
			end := strings.Index(path[i:], "]")
			if end == -1 {
				// Malformed - treat as literal
				current.WriteByte(ch)
				continue
			}
			// Extract index (handles negative indices like -1)
			indexStr := path[i+1 : i+end]
			parts = append(parts, indexStr)
			i += end // Skip past closing bracket

		default:
			current.WriteByte(ch)
		}
	}

	// Flush final segment
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseArrayIndex parses an array index from a path part like "0", "[0]", "-1", or "[-1]"
// Returns invalidArrayIndex as a sentinel value for invalid input (since -1, -2, etc. are valid negative indices)
func parseArrayIndex(part string) int {
	// Remove brackets if present
	part = strings.TrimPrefix(part, "[")
	part = strings.TrimSuffix(part, "]")

	var idx int
	_, err := fmt.Sscanf(part, "%d", &idx)
	if err != nil {
		return invalidArrayIndex
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

	// Check if path contains negative indices - if so, we need to resolve them first
	// since the yaml library doesn't support negative indices
	if strings.Contains(dotPath, "-") {
		// Read current data to resolve negative indices
		data, mapErr := readYAMLAsMap(filePath)
		if mapErr != nil {
			return mapErr
		}

		// Resolve negative indices in the path
		resolvedPath, resolveErr := resolveNegativeIndices(data, dotPath)
		if resolveErr != nil {
			return resolveErr
		}
		dotPath = resolvedPath
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

// resolveNegativeIndices converts negative array indices to positive ones by examining the data
// For example: items[-1].name with a 3-element array becomes items[2].name
func resolveNegativeIndices(data map[string]interface{}, dotPath string) (string, error) {
	parts := parseDotPath(dotPath)
	if len(parts) == 0 {
		return dotPath, nil
	}

	current := interface{}(data)
	var resolvedParts []string

	for i, part := range parts {
		idx := parseArrayIndex(part)
		if idx != invalidArrayIndex && idx < 0 {
			// Negative index - need to resolve it
			arr, ok := current.([]interface{})
			if !ok {
				return "", fmt.Errorf("negative index %d used on non-array at %s", idx, strings.Join(parts[:i], "."))
			}
			resolvedIdx := len(arr) + idx
			if resolvedIdx < 0 || resolvedIdx >= len(arr) {
				return "", fmt.Errorf("array index out of bounds: %d (length: %d)", idx, len(arr))
			}
			resolvedParts = append(resolvedParts, fmt.Sprintf("[%d]", resolvedIdx))
			current = arr[resolvedIdx]
		} else {
			resolvedParts = append(resolvedParts, part)
			// Navigate to next level
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[part]
			case map[interface{}]interface{}:
				current = v[part]
			case []interface{}:
				if idx != invalidArrayIndex && idx >= 0 && idx < len(v) {
					current = v[idx]
				}
			}
		}
	}

	// Reconstruct path - parts that are numeric should use bracket notation
	var result strings.Builder
	for i, part := range resolvedParts {
		if strings.HasPrefix(part, "[") {
			result.WriteString(part)
		} else {
			if i > 0 {
				result.WriteString(".")
			}
			result.WriteString(part)
		}
	}
	return result.String(), nil
}

// maxStdinSize is the maximum allowed size for stdin input (10MB)
// This prevents memory exhaustion from malicious or accidental large inputs
const maxStdinSize = 10 * 1024 * 1024

// readFromStdin reads all content from stdin, trimming trailing newline
// Uses cmd.InOrStdin() for testability - tests can use cmd.SetIn()
// Enforces a maximum size limit to prevent memory exhaustion
func readFromStdin(cmd *cobra.Command) (string, error) {
	reader := cmd.InOrStdin()

	// Check if stdin has data (not a terminal)
	// Only check if it's actually os.Stdin (not a test buffer)
	if f, ok := reader.(*os.File); ok && f == os.Stdin {
		stat, err := f.Stat()
		if err != nil {
			return "", fmt.Errorf("failed to stat stdin: %w", err)
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return "", fmt.Errorf("no data piped to stdin (use 'echo value | command' or 'command < file')")
		}
	}

	// Read with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(reader, maxStdinSize+1)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("error reading stdin: %w", err)
	}

	// Check if we hit the limit
	if len(content) > maxStdinSize {
		return "", fmt.Errorf("stdin input exceeds maximum size of %d bytes", maxStdinSize)
	}

	// Validate UTF-8 encoding to prevent YAML corruption from binary data
	if !utf8.Valid(content) {
		return "", fmt.Errorf("stdin input is not valid UTF-8 (binary data not supported for YAML values)")
	}

	// Trim trailing newline (common from echo)
	// Handle both Unix (\n) and Windows (\r\n) line endings
	result := strings.TrimSuffix(string(content), "\n")
	result = strings.TrimSuffix(result, "\r")
	return result, nil
}

// parseRequiredKeysFile reads keys from a file, one per line
// Supports # comments and skips empty lines
// Validates the file path to prevent path traversal attacks
func parseRequiredKeysFile(filePath string) ([]string, error) {
	// Clean the path to resolve any . or .. components
	cleanPath := filepath.Clean(filePath)

	// Resolve to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Verify the file exists and is a regular file (not a symlink to something else)
	info, err := os.Lstat(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat required keys file: %w", err)
	}

	// Don't follow symlinks to prevent traversal via symlink
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("required keys file cannot be a symlink for security reasons")
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("required keys file must be a regular file")
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open required keys file: %w", err)
	}
	defer file.Close()

	var keys []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle keys longer than default 64KB limit
	// YAML keys with long paths (e.g., deeply.nested.keys.with.many.segments)
	// may exceed the default scanner buffer
	const maxKeyLength = 256 * 1024 // 256KB should be more than enough for any key
	buf := make([]byte, maxKeyLength)
	scanner.Buffer(buf, maxKeyLength)
	for scanner.Scan() {
		line := scanner.Text()

		// Strip inline comments
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}

		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		keys = append(keys, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading required keys file: %w", err)
	}

	return keys, nil
}

// yamlUniqueStrings removes duplicates from a string slice while preserving order
// Used for deduplicating keys from multiple sources in yaml-multiget
func yamlUniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(input))

	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// dryRunChange represents a single key change for dry-run preview
type dryRunChange struct {
	Key      string      `json:"key"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// outputDryRunPreview outputs a single change preview
func outputDryRunPreview(cmd *cobra.Command, filePath, key string, oldValue, newValue interface{}, jsonOutput, minOutput bool) error {
	if jsonOutput {
		return outputDryRunJSON(cmd, filePath, []dryRunChange{{
			Key:      key,
			OldValue: oldValue,
			NewValue: newValue,
		}}, minOutput)
	}

	if minOutput {
		// Minimal: just key: old → new
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %v → %v\n", key, formatDryRunValue(oldValue), formatDryRunValue(newValue))
		return nil
	}

	// Default text output
	fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN - No changes written")
	fmt.Fprintf(cmd.OutOrStdout(), "File: %s\n", filePath)
	fmt.Fprintf(cmd.OutOrStdout(), "Key: %s\n", key)
	fmt.Fprintf(cmd.OutOrStdout(), "Old: %s\n", formatDryRunValue(oldValue))
	fmt.Fprintf(cmd.OutOrStdout(), "New: %s\n", formatDryRunValue(newValue))

	return nil
}

// outputMultiDryRunPreview outputs multiple change previews
func outputMultiDryRunPreview(cmd *cobra.Command, filePath string, changes []dryRunChange, jsonOutput, minOutput bool) error {
	if jsonOutput {
		return outputDryRunJSON(cmd, filePath, changes, minOutput)
	}

	if minOutput {
		for _, c := range changes {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %v → %v\n", c.Key, formatDryRunValue(c.OldValue), formatDryRunValue(c.NewValue))
		}
		return nil
	}

	// Default text output
	fmt.Fprintln(cmd.OutOrStdout(), "DRY RUN - No changes written")
	fmt.Fprintf(cmd.OutOrStdout(), "File: %s\n", filePath)
	fmt.Fprintf(cmd.OutOrStdout(), "Changes (%d):\n", len(changes))
	for _, c := range changes {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s:\n", c.Key)
		fmt.Fprintf(cmd.OutOrStdout(), "    Old: %v\n", formatDryRunValue(c.OldValue))
		fmt.Fprintf(cmd.OutOrStdout(), "    New: %v\n", formatDryRunValue(c.NewValue))
	}

	return nil
}

// outputDryRunJSON outputs dry-run preview as JSON
func outputDryRunJSON(cmd *cobra.Command, filePath string, changes []dryRunChange, minOutput bool) error {
	if minOutput {
		// Minimal JSON: just array of changes
		output := map[string]interface{}{
			"dry_run": true,
			"changes": changes,
		}
		jsonBytes, _ := json.Marshal(output)
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
		return nil
	}

	// Full JSON with formatting
	output := map[string]interface{}{
		"dry_run": true,
		"file":    filePath,
		"changes": changes,
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// formatDryRunValue formats a value for display (handles nil, etc.)
func formatDryRunValue(v interface{}) string {
	if v == nil {
		return "<not set>"
	}
	return fmt.Sprintf("%v", v)
}
