// Package output provides shared output formatting utilities for CLI commands.
// It supports four output modes:
//   - Default: Human-readable text output
//   - JSON: Pretty-printed JSON output
//   - Minimal: Token-optimized text output (no empty values, relative paths)
//   - Minimal+JSON: Single-line abbreviated JSON (minimal keys, no empty fields)
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// Formatter handles output formatting with support for JSON and minimal modes.
type Formatter struct {
	JSON    bool // Output as JSON
	Minimal bool // Output in minimal/token-optimized mode
	Writer  io.Writer
}

// New creates a new Formatter with the given options.
func New(jsonOutput, minimal bool, w io.Writer) *Formatter {
	if w == nil {
		w = os.Stdout
	}
	return &Formatter{
		JSON:    jsonOutput,
		Minimal: minimal,
		Writer:  w,
	}
}

// KeyAbbreviations maps full key names to abbreviated versions for minimal JSON output.
// NOTE: "count" intentionally not abbreviated - other tools output "count" directly
var KeyAbbreviations = map[string]string{
	"file":        "f",
	"line":        "l",
	"name":        "n",
	"type":        "t",
	"question":    "q",
	"answer":      "a",
	"occurrences": "o",
	"directories": "dirs",
	"path":        "p",
	"message":     "msg",
	"status":      "s",
	"value":       "v",
	"key":         "k",
	"result":      "r",
	"error":       "err",
	"success":     "ok",
	"files":       "fs",
	"matches":     "m",
}

// Print outputs the data according to the formatter's configuration.
// For JSON mode, it marshals the data. For text mode, it uses the textFunc if provided.
// textFunc is called for default text output; if nil, JSON is used as fallback.
func (f *Formatter) Print(data interface{}, textFunc func(io.Writer, interface{})) error {
	if f.JSON {
		return f.printJSON(data)
	}
	if textFunc != nil {
		textFunc(f.Writer, data)
		return nil
	}
	// Fallback to JSON if no text function provided
	return f.printJSON(data)
}

// printJSON outputs data as JSON, with formatting based on Minimal setting.
func (f *Formatter) printJSON(data interface{}) error {
	if f.Minimal {
		// Minimal JSON: single line, abbreviated keys, omit empty values
		processed := f.processForMinimalJSON(data)
		output, err := json.Marshal(processed)
		if err != nil {
			return err
		}
		fmt.Fprintln(f.Writer, string(output))
	} else {
		// Pretty-printed JSON
		output, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(f.Writer, string(output))
	}
	return nil
}

// processForMinimalJSON converts a struct/map to use abbreviated keys and omit empty values.
func (f *Formatter) processForMinimalJSON(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	val := reflect.ValueOf(data)

	// Handle pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		return f.processForMinimalJSON(val.Elem().Interface())
	}

	switch val.Kind() {
	case reflect.Struct:
		return f.processStruct(val)
	case reflect.Map:
		return f.processMap(val)
	case reflect.Slice, reflect.Array:
		return f.processSlice(val)
	default:
		return data
	}
}

func (f *Formatter) processStruct(val reflect.Value) map[string]interface{} {
	result := make(map[string]interface{})
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get JSON tag or use field name
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name := fieldType.Name
		omitEmpty := false
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				name = parts[0]
			}
			for _, opt := range parts[1:] {
				if opt == "omitempty" {
					omitEmpty = true
				}
			}
		}

		// Check if value is empty - only skip if field has omitempty tag
		// (minimal mode abbreviates keys but shouldn't omit meaningful zero values)
		if f.isEmpty(field) && omitEmpty {
			continue
		}

		// Abbreviate key name
		abbrevName := name
		if abbrev, ok := KeyAbbreviations[strings.ToLower(name)]; ok {
			abbrevName = abbrev
		}

		// Process nested values
		result[abbrevName] = f.processForMinimalJSON(field.Interface())
	}

	return result
}

func (f *Formatter) processMap(val reflect.Value) map[string]interface{} {
	result := make(map[string]interface{})

	for _, key := range val.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		value := val.MapIndex(key)

		// Maps don't have omitempty tags, so don't skip zero values
		// (they may be semantically meaningful like count=0)
		_ = f.Minimal // minimal mode only abbreviates keys for maps

		// Abbreviate key name
		abbrevKey := keyStr
		if abbrev, ok := KeyAbbreviations[strings.ToLower(keyStr)]; ok {
			abbrevKey = abbrev
		}

		result[abbrevKey] = f.processForMinimalJSON(value.Interface())
	}

	return result
}

func (f *Formatter) processSlice(val reflect.Value) []interface{} {
	result := make([]interface{}, 0, val.Len())

	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		if !f.isEmpty(elem) || !f.Minimal {
			result = append(result, f.processForMinimalJSON(elem.Interface()))
		}
	}

	return result
}

func (f *Formatter) isEmpty(val reflect.Value) bool {
	if !val.IsValid() {
		return true
	}

	switch val.Kind() {
	case reflect.String:
		return val.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return val.Float() == 0
	case reflect.Bool:
		return !val.Bool()
	case reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}

// RelativePath converts an absolute path to a path relative to the given base.
// If the path cannot be made relative, it returns the original path.
func RelativePath(path, base string) string {
	if path == "" || base == "" {
		return path
	}

	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}

	// Don't return relative paths that go up too many levels
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)+".."+string(filepath.Separator)+"..") {
		return path
	}

	return rel
}

// RelativePathCwd converts an absolute path to a path relative to the current working directory.
func RelativePathCwd(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	return RelativePath(path, cwd)
}

// FilterEmpty removes empty/zero values from a map.
func FilterEmpty(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		if !isEmptyValue(v) {
			result[k] = v
		}
	}
	return result
}

func isEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String:
		return val.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return val.Float() == 0
	case reflect.Bool:
		return !val.Bool()
	case reflect.Slice, reflect.Array, reflect.Map:
		return val.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}

// PrintLine prints a key-value pair to the writer, respecting minimal mode.
func (f *Formatter) PrintLine(key string, value interface{}) {
	if f.Minimal && isEmptyValue(value) {
		return
	}
	fmt.Fprintf(f.Writer, "%s: %v\n", key, value)
}

// PrintSection prints a section header.
func (f *Formatter) PrintSection(title string) {
	if !f.Minimal {
		fmt.Fprintf(f.Writer, "---\n%s\n", title)
	}
}

// PrintError outputs an error respecting JSON and Minimal modes.
// In JSON mode, outputs to stdout for parsing. In text mode, outputs to stderr.
// Returns the exit code (always 1 for errors).
func (f *Formatter) PrintError(err error) int {
	if f.JSON {
		if f.Minimal {
			// Minimal JSON: abbreviated keys, single line
			result := map[string]interface{}{
				"err": true,
				"msg": err.Error(),
			}
			output, _ := json.Marshal(result)
			fmt.Fprintln(f.Writer, string(output))
		} else {
			// Pretty JSON
			result := map[string]interface{}{
				"error":   true,
				"message": err.Error(),
			}
			output, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(f.Writer, string(output))
		}
	} else {
		// Text mode - use stderr
		w := f.Writer
		if w == os.Stdout {
			w = os.Stderr
		}
		if f.Minimal {
			// Minimal: just the error message
			fmt.Fprintln(w, err.Error())
		} else {
			// Default: "Error: " prefix
			fmt.Fprintf(w, "Error: %v\n", err)
		}
	}
	return 1
}

// ErrorResult is a helper for creating error responses in handlers.
type ErrorResult struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
}
