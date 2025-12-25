package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

var (
	jsonIndent  int
	jsonCompact bool
)

// newJSONCmd creates the json parent command with subcommands
func newJSONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "json",
		Short: "JSON manipulation commands",
		Long:  `Commands for parsing, querying, validating, and merging JSON files.`,
	}

	cmd.AddCommand(newJSONParseCmd())
	cmd.AddCommand(newJSONQueryCmd())
	cmd.AddCommand(newJSONValidateCmd())
	cmd.AddCommand(newJSONMergeCmd())

	return cmd
}

func newJSONParseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse <file>",
		Short: "Parse and pretty-print JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runJSONParse,
	}
	cmd.Flags().IntVar(&jsonIndent, "indent", 2, "Indentation spaces")
	cmd.Flags().BoolVar(&jsonCompact, "compact", false, "Compact output")
	return cmd
}

func newJSONQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <file> <path>",
		Short: "Query JSON with path expression",
		Long: `Query JSON using gjson path syntax.
Examples:
  json query file.json "users"
  json query file.json "users.0.name"
  json query file.json "users.#.name"`,
		Args: cobra.ExactArgs(2),
		RunE: runJSONQuery,
	}
	return cmd
}

func newJSONValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate JSON syntax",
		Args:  cobra.ExactArgs(1),
		RunE:  runJSONValidate,
	}
	return cmd
}

func newJSONMergeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge <file1> <file2> [files...]",
		Short: "Merge multiple JSON files",
		Args:  cobra.MinimumNArgs(2),
		RunE:  runJSONMerge,
	}
	return cmd
}

func runJSONParse(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	var output []byte
	if jsonCompact {
		output, err = json.Marshal(data)
	} else {
		indent := strings.Repeat(" ", jsonIndent)
		output, err = json.MarshalIndent(data, "", indent)
	}

	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func runJSONQuery(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	if !gjson.ValidBytes(content) {
		return fmt.Errorf("invalid JSON in file")
	}

	path := args[1]
	result := gjson.GetBytes(content, path)

	if !result.Exists() {
		return fmt.Errorf("path not found: %s", path)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "QUERY: %s\n", path)
	fmt.Fprintln(cmd.OutOrStdout(), "RESULT:")

	// Pretty print the result
	if result.IsArray() || result.IsObject() {
		var data interface{}
		json.Unmarshal([]byte(result.Raw), &data)
		output, _ := json.MarshalIndent(data, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), result.String())
	}

	return nil
}

func runJSONValidate(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "✗ INVALID: %s\n", err.Error())
		return fmt.Errorf("validation failed")
	}

	// Count elements
	count := countJSONElements(data)
	fmt.Fprintf(cmd.OutOrStdout(), "✓ VALID JSON\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Elements: %d\n", count)

	return nil
}

func countJSONElements(data interface{}) int {
	count := 1
	switch v := data.(type) {
	case map[string]interface{}:
		for _, val := range v {
			count += countJSONElements(val)
		}
	case []interface{}:
		for _, val := range v {
			count += countJSONElements(val)
		}
	}
	return count
}

func runJSONMerge(cmd *cobra.Command, args []string) error {
	result := make(map[string]interface{})

	for _, file := range args {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(content, &data); err != nil {
			return fmt.Errorf("invalid JSON in %s: %w", file, err)
		}

		// Merge: later files override earlier ones
		for k, v := range data {
			result[k] = v
		}
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format result: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

// queryJSON navigates a JSON path and returns the value
func queryJSON(data interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", part)
			}
			current = val
		case []interface{}:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid array index: %s", part)
			}
			if idx < 0 || idx >= len(v) {
				return nil, fmt.Errorf("array index out of range: %d", idx)
			}
			current = v[idx]
		default:
			return nil, fmt.Errorf("cannot navigate through %T", current)
		}
	}

	return current, nil
}

func init() {
	RootCmd.AddCommand(newJSONCmd())
}
