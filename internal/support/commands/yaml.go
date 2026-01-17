package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// newYamlCmd creates the yaml parent command
func newYamlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "yaml",
		Short: "Manage YAML configuration files",
		Long: `Manage YAML configuration files with dot notation access.

The yaml command provides a way to read, write, and manage structured
YAML configuration files using dot notation for nested keys.

Subcommands:
  init      Initialize a new YAML config file
  get       Retrieve a value by key
  set       Store a value at a key
  multiget  Retrieve multiple values
  multiset  Store multiple values atomically
  list      List all keys
  delete    Remove a key
  validate  Validate YAML syntax
  push      Append value to array
  pop       Remove and return last array element`,
	}

	// Add subcommands
	cmd.AddCommand(newYamlInitCmd())
	cmd.AddCommand(newYamlGetCmd())
	cmd.AddCommand(newYamlSetCmd())
	cmd.AddCommand(newYamlMultigetCmd())
	cmd.AddCommand(newYamlMultisetCmd())
	cmd.AddCommand(newYamlListCmd())
	cmd.AddCommand(newYamlDeleteCmd())
	cmd.AddCommand(newYamlValidateCmd())
	cmd.AddCommand(newYamlPushCmd())
	cmd.AddCommand(newYamlPopCmd())

	return cmd
}

// newYamlInitCmd creates the yaml init subcommand
func newYamlInitCmd() *cobra.Command {
	var file string
	var force bool
	var template string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new YAML config file",
		Long: `Initialize a new YAML config file with a template structure.

Templates:
  planning  Full planning workflow template with all sections
  minimal   Minimal template with empty config
  <path>    Custom template from file

Output:
  CONFIG_FILE: path to config file
  STATUS: CREATED | EXISTS
  KEYS: number of keys

Examples:
  yaml init --file config.yaml
  yaml init --file config.yaml --template planning
  yaml init --file config.yaml --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			// Check if file exists
			status := "CREATED"
			if _, err := os.Stat(file); err == nil {
				if !force {
					status = "EXISTS"
					// Parse existing file to count keys
					data, err := readYAMLAsMap(file)
					if err != nil {
						return fmt.Errorf("failed to read existing file: %w", err)
					}
					keyCount := countKeys(data)

					if jsonOutput {
						if minOutput {
							// --json --min: minimal JSON
							output := map[string]interface{}{"f": file, "s": status}
							jsonBytes, _ := json.Marshal(output)
							fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
						} else {
							// --json: full JSON
							output := map[string]interface{}{
								"config_file": file,
								"status":      status,
								"keys":        keyCount,
							}
							jsonBytes, _ := json.Marshal(output)
							fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
						}
					} else if minOutput {
						fmt.Fprintln(cmd.OutOrStdout(), file)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "CONFIG_FILE: %s\n", file)
						fmt.Fprintf(cmd.OutOrStdout(), "STATUS: %s\n", status)
						fmt.Fprintf(cmd.OutOrStdout(), "KEYS: %d\n", keyCount)
					}
					return nil
				}
			}

			// Get template content
			content, err := getTemplate(template)
			if err != nil {
				return err
			}

			// Ensure parent directory exists
			dir := filepath.Dir(file)
			if dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			// Write file
			if err := os.WriteFile(file, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			// Count keys in new file
			data, _ := readYAMLAsMap(file)
			keyCount := countKeys(data)

			if jsonOutput {
				if minOutput {
					// --json --min: minimal JSON
					output := map[string]interface{}{"f": file, "s": status}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON
					output := map[string]interface{}{
						"config_file": file,
						"status":      status,
						"keys":        keyCount,
					}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), file)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "CONFIG_FILE: %s\n", file)
				fmt.Fprintf(cmd.OutOrStdout(), "STATUS: %s\n", status)
				fmt.Fprintf(cmd.OutOrStdout(), "KEYS: %d\n", keyCount)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing file")
	cmd.Flags().StringVar(&template, "template", "", "Template: planning, minimal, or file path")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output (just file path)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlGetCmd creates the yaml get subcommand
func newYamlGetCmd() *cobra.Command {
	var file string
	var defaultValue string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Retrieve a value by key",
		Long: `Retrieve a value from the YAML config file by dot-notation key.

Key Format:
  helper.llm      - nested key
  items[0]        - array index
  deeply.nested.value - deep nesting

Output Formats:
  default: key: value
  --json:  {"key": "...", "value": "..."}
  --min:   value (just the value)

Examples:
  yaml get --file config.yaml helper.llm
  yaml get --file config.yaml helper.llm --min
  yaml get --file config.yaml missing.key --default "fallback"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			key := args[0]

			// Acquire read lock
			lock, err := yamlFileLock(file, false)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Read file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s (hint: run 'llm-support yaml init --file %s' first)", file, file)
				}
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Get value
			value, found := getValueAtPath(data, key)
			if !found {
				if defaultValue != "" {
					value = defaultValue
				} else {
					return fmt.Errorf("key not found: %s", key)
				}
			}

			// Format output
			if jsonOutput {
				if minOutput {
					// --json --min: just the value as JSON
					jsonBytes, _ := json.Marshal(value)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON with key and value
					output := map[string]interface{}{"key": key, "value": value}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), formatValue(value))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, formatValue(value))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().StringVar(&defaultValue, "default", "", "Default value if key not found")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output (just value)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlSetCmd creates the yaml set subcommand
func newYamlSetCmd() *cobra.Command {
	var file string
	var create bool
	var jsonOutput bool
	var minOutput bool
	var dryRun bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Store a value at a key",
		Long: `Store a value in the YAML config file at the specified key.

Creates intermediate keys if they don't exist.
Preserves comments where possible.

Use '-' as VALUE to read from stdin (for piping values or multi-line input).
Use --dry-run to preview changes without writing to the file.
Use --quiet to suppress success messages (errors still output).

Examples:
  yaml set --file config.yaml helper.llm claude
  yaml set --file config.yaml helper.max_lines 2500
  yaml set --file config.yaml new.nested.key value --create
  echo "piped" | yaml set --file config.yaml key -
  yaml set --file config.yaml key value --dry-run`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			key := args[0]
			value := args[1]

			// Handle stdin input when value is "-"
			if value == "-" {
				stdinValue, err := readFromStdin(cmd)
				if err != nil {
					return fmt.Errorf("failed to read value from stdin: %w", err)
				}
				value = stdinValue
			}

			// Check if file exists
			if _, err := os.Stat(file); os.IsNotExist(err) {
				if !create {
					return fmt.Errorf("config file not found: %s (hint: use --create to create it, or run 'llm-support yaml init --file %s' first)", file, file)
				}
				// Create empty file (unless dry-run)
				if !dryRun {
					if err := os.WriteFile(file, []byte(""), 0644); err != nil {
						return fmt.Errorf("failed to create config file: %w", err)
					}
				}
			}

			// Set value (convert numeric strings to numbers)
			var typedValue interface{} = value
			if num, err := parseNumber(value); err == nil {
				typedValue = num
			}

			// Handle dry-run mode - preview without writing
			if dryRun {
				var oldValue interface{}
				if _, err := os.Stat(file); err == nil {
					data, err := readYAMLAsMap(file)
					if err == nil {
						oldValue, _ = getValueAtPath(data, key)
					}
				}
				return outputDryRunPreview(cmd, file, key, oldValue, typedValue, jsonOutput, minOutput)
			}

			// Acquire write lock
			lock, err := yamlFileLock(file, true)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Use comment-preserving set (uses AST manipulation)
			if err := setValuePreservingComments(file, key, typedValue); err != nil {
				return fmt.Errorf("failed to set value: %w", err)
			}

			// Skip success output if quiet
			if quiet {
				return nil
			}

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: minimal JSON
					output := map[string]interface{}{"k": key, "s": "set"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON
					output := map[string]interface{}{"key": key, "value": value, "status": "set"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), key)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "SET: %s = %s\n", key, value)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&create, "create", false, "Create file if it doesn't exist")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing to file")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress success messages (errors still output)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlMultigetCmd creates the yaml multiget subcommand
func newYamlMultigetCmd() *cobra.Command {
	var file string
	var defaults string
	var separator string
	var jsonOutput bool
	var minOutput bool
	var requiredFile string

	cmd := &cobra.Command{
		Use:   "multiget [KEY1 KEY2 ...]",
		Short: "Retrieve multiple values",
		Long: `Retrieve multiple values from the YAML config file in a single operation.

Values are returned in argument order.

Keys can be specified via:
  - Positional arguments
  - --required-file (one key per line, # comments supported)

Output Formats:
  default: key=value (one per line)
  --json:  {"key1": "value1", "key2": "value2"}
  --min:   value1\nvalue2 (values only)

Examples:
  yaml multiget --file config.yaml helper.llm project.type
  yaml multiget --file config.yaml helper.llm missing.key --defaults '{"missing.key": "default"}'
  yaml multiget --file config.yaml helper.llm project.type --min
  yaml multiget --file config.yaml --required-file keys.txt`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			// Collect keys from all sources
			var keys []string

			// From positional args
			keys = append(keys, args...)

			// From --required-file
			if requiredFile != "" {
				fileKeys, err := parseRequiredKeysFile(requiredFile)
				if err != nil {
					return fmt.Errorf("failed to read required keys file: %w", err)
				}
				keys = append(keys, fileKeys...)
			}

			// Deduplicate keys while preserving order
			keys = yamlUniqueStrings(keys)

			if len(keys) == 0 {
				return fmt.Errorf("no keys specified (use positional args or --required-file)")
			}

			// Parse defaults if provided
			var defaultsMap map[string]string
			if defaults != "" {
				if err := json.Unmarshal([]byte(defaults), &defaultsMap); err != nil {
					return fmt.Errorf("invalid --defaults JSON: %w", err)
				}
			}

			// Acquire read lock
			lock, err := yamlFileLock(file, false)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Read file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", file)
				}
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Get all values
			results := make(map[string]string)
			var orderedKeys []string
			for _, key := range keys {
				orderedKeys = append(orderedKeys, key)
				value, found := getValueAtPath(data, key)
				if found {
					results[key] = formatValue(value)
				} else if defaultsMap != nil {
					if def, ok := defaultsMap[key]; ok {
						results[key] = def
					} else {
						return fmt.Errorf("key not found: %s", key)
					}
				} else {
					return fmt.Errorf("key not found: %s", key)
				}
			}

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: values as JSON array (preserves order)
					orderedValues := make([]string, len(orderedKeys))
					for i, key := range orderedKeys {
						orderedValues[i] = results[key]
					}
					jsonBytes, _ := json.Marshal(orderedValues)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON object
					jsonBytes, _ := json.Marshal(results)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				for _, key := range orderedKeys {
					fmt.Fprintln(cmd.OutOrStdout(), results[key])
				}
			} else {
				for _, key := range orderedKeys {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, results[key])
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().StringVar(&defaults, "defaults", "", "JSON object of default values")
	cmd.Flags().StringVar(&separator, "separator", "\n", "Value separator for --min output")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output (values only)")
	cmd.Flags().StringVar(&requiredFile, "required-file", "", "File containing required keys (one per line, # comments supported)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlMultisetCmd creates the yaml multiset subcommand
func newYamlMultisetCmd() *cobra.Command {
	var file string
	var create bool
	var jsonOutput bool
	var minOutput bool
	var dryRun bool
	var quiet bool

	cmd := &cobra.Command{
		Use:   "multiset KEY1 VALUE1 [KEY2 VALUE2 ...]",
		Short: "Store multiple values atomically",
		Long: `Store multiple key-value pairs in the YAML config file in a single atomic operation.

All keys are validated before any writes occur.
Arguments must be in KEY VALUE pairs.
Use --dry-run to preview changes without writing to the file.
Use --quiet to suppress success messages (errors still output).

Examples:
  yaml multiset --file config.yaml helper.llm claude helper.max_lines 2500
  yaml multiset --file config.yaml --create new.key value1 other.key value2
  yaml multiset --file config.yaml key1 val1 key2 val2 --dry-run`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			// Validate even number of arguments
			if len(args)%2 != 0 {
				return fmt.Errorf("multiset requires KEY VALUE pairs (got %d arguments)", len(args))
			}

			// Parse key-value pairs
			pairs := make([]struct{ key, value string }, len(args)/2)
			for i := 0; i < len(args); i += 2 {
				pairs[i/2] = struct{ key, value string }{
					key:   args[i],
					value: args[i+1],
				}
			}

			// Check if file exists
			if _, err := os.Stat(file); os.IsNotExist(err) {
				if !create {
					return fmt.Errorf("config file not found: %s (hint: use --create)", file)
				}
				// Create empty file (unless dry-run)
				if !dryRun {
					if err := os.WriteFile(file, []byte(""), 0644); err != nil {
						return fmt.Errorf("failed to create config file: %w", err)
					}
				}
			}

			// Read file for dry-run comparison or actual update
			var data map[string]interface{}
			var err error
			if _, statErr := os.Stat(file); statErr == nil {
				data, err = readYAMLAsMap(file)
				if err != nil {
					return fmt.Errorf("failed to read config file: %w", err)
				}
			} else {
				data = make(map[string]interface{})
			}

			// Handle dry-run mode - preview without writing
			if dryRun {
				var changes []dryRunChange
				for _, pair := range pairs {
					var typedValue interface{} = pair.value
					if num, parseErr := parseNumber(pair.value); parseErr == nil {
						typedValue = num
					}

					oldValue, _ := getValueAtPath(data, pair.key)
					changes = append(changes, dryRunChange{
						Key:      pair.key,
						OldValue: oldValue,
						NewValue: typedValue,
					})
				}
				return outputMultiDryRunPreview(cmd, file, changes, jsonOutput, minOutput)
			}

			// Acquire write lock
			lock, err := yamlFileLock(file, true)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Set all values
			var keys []string
			for _, pair := range pairs {
				var typedValue interface{} = pair.value
				if num, parseErr := parseNumber(pair.value); parseErr == nil {
					typedValue = num
				}

				if err := setValueAtPath(data, pair.key, typedValue); err != nil {
					return fmt.Errorf("failed to set %s: %w", pair.key, err)
				}
				keys = append(keys, pair.key)
			}

			// Write file atomically
			if err := writeYAMLFile(file, data); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			// Skip success output if quiet
			if quiet {
				return nil
			}

			// Output
			// NOTE: Output format intentionally matches context_multiset for consistency
			if jsonOutput {
				if minOutput {
					// --json --min: minimal JSON with count and status
					output := map[string]interface{}{"count": len(keys), "status": "ok"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON with keys, count, and status
					output := map[string]interface{}{"keys": keys, "count": len(keys), "status": "ok"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join(keys, ","))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "SET: %s\n", strings.Join(keys, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&create, "create", false, "Create file if it doesn't exist")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing to file")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress success messages (errors still output)")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlListCmd creates the yaml list subcommand
func newYamlListCmd() *cobra.Command {
	var file string
	var flat bool
	var values bool
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "list [PREFIX]",
		Short: "List all keys",
		Long: `List all keys in the YAML config file.

Output Formats:
  default: hierarchical YAML structure
  --flat:  dot-notation keys (one per line)
  --flat --values: key=value (one per line)
  --json:  JSON object

Examples:
  yaml list --file config.yaml
  yaml list --file config.yaml --flat
  yaml list --file config.yaml --flat --values
  yaml list --file config.yaml helper --flat`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			prefix := ""
			if len(args) > 0 {
				prefix = args[0]
			}

			// Acquire read lock
			lock, err := yamlFileLock(file, false)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Read file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", file)
				}
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Filter by prefix if provided
			if prefix != "" {
				value, found := getValueAtPath(data, prefix)
				if !found {
					return fmt.Errorf("prefix not found: %s", prefix)
				}
				if m, ok := value.(map[string]interface{}); ok {
					data = m
				} else {
					// Single value, output directly
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", prefix, formatValue(value))
					return nil
				}
			}

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: flattened keys as JSON array
					keys := flattenKeys(data, prefix)
					sort.Strings(keys)
					jsonBytes, _ := json.Marshal(keys)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON object
					jsonBytes, _ := json.Marshal(data)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if flat {
				if values {
					kvs := flattenKeysWithValues(data, prefix)
					var keys []string
					for k := range kvs {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", k, kvs[k])
					}
				} else {
					keys := flattenKeys(data, prefix)
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Fprintln(cmd.OutOrStdout(), k)
					}
				}
			} else if minOutput {
				keys := flattenKeys(data, prefix)
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintln(cmd.OutOrStdout(), k)
				}
			} else {
				// Default: YAML output
				yamlBytes, _ := yaml.Marshal(data)
				fmt.Fprint(cmd.OutOrStdout(), string(yamlBytes))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&flat, "flat", false, "Output flattened dot-notation keys")
	cmd.Flags().BoolVar(&values, "values", false, "Include values in output (with --flat)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlDeleteCmd creates the yaml delete subcommand
func newYamlDeleteCmd() *cobra.Command {
	var file string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "delete KEY",
		Short: "Remove a key",
		Long: `Delete a key from the YAML config file.

Examples:
  yaml delete --file config.yaml tools.deprecated
  yaml delete --file config.yaml helper.old_setting`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			key := args[0]

			// Acquire write lock
			lock, err := yamlFileLock(file, true)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Read file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", file)
				}
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Delete key
			if err := deleteValueAtPath(data, key); err != nil {
				return fmt.Errorf("failed to delete key: %w", err)
			}

			// Write file
			if err := writeYAMLFile(file, data); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: minimal JSON
					output := map[string]interface{}{"k": key, "s": "deleted"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON
					output := map[string]interface{}{"key": key, "status": "deleted"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), key)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "DELETED: %s\n", key)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlValidateCmd creates the yaml validate subcommand
func newYamlValidateCmd() *cobra.Command {
	var file string
	var required string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate YAML syntax",
		Long: `Validate YAML file syntax and optionally check for required keys.

Output:
  VALID: TRUE | FALSE
  KEYS: number of keys
  SECTIONS: top-level section names

Examples:
  yaml validate --file config.yaml
  yaml validate --file config.yaml --required "helper.llm,project.type"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			// Read and parse file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", file)
				}
				return err
			}

			// Check required keys
			if required != "" {
				requiredKeys := strings.Split(required, ",")
				var missing []string
				for _, key := range requiredKeys {
					key = strings.TrimSpace(key)
					if _, found := getValueAtPath(data, key); !found {
						missing = append(missing, key)
					}
				}
				if len(missing) > 0 {
					return fmt.Errorf("missing required keys: %s", strings.Join(missing, ", "))
				}
			}

			// Count keys and sections
			keyCount := countKeys(data)
			sections := getTopLevelSections(data)
			sort.Strings(sections)

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: minimal JSON
					output := map[string]interface{}{"ok": true, "keys": keyCount}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON
					output := map[string]interface{}{
						"valid":    true,
						"keys":     keyCount,
						"sections": sections,
					}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), "TRUE")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "VALID: TRUE")
				fmt.Fprintf(cmd.OutOrStdout(), "KEYS: %d\n", keyCount)
				fmt.Fprintf(cmd.OutOrStdout(), "SECTIONS: %s\n", strings.Join(sections, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().StringVar(&required, "required", "", "Comma-separated list of required keys")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlPushCmd creates the yaml push subcommand
func newYamlPushCmd() *cobra.Command {
	var file string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "push KEY VALUE",
		Short: "Append value to array",
		Long: `Append a value to an array at the specified key.

If the key doesn't exist, creates a new array.
If the key exists but is not an array, returns an error.

Examples:
  yaml push --file config.yaml items "new item"
  yaml push --file config.yaml plugins.enabled feature-x`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			key := args[0]
			value := args[1]

			// Acquire write lock
			lock, err := yamlFileLock(file, true)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Read file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", file)
				}
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Push value
			if err := pushToArray(data, key, value); err != nil {
				return err
			}

			// Write file
			if err := writeYAMLFile(file, data); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: minimal JSON
					output := map[string]interface{}{"k": key, "s": "pushed"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON
					output := map[string]interface{}{"key": key, "value": value, "status": "pushed"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), value)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "PUSHED: %s to %s\n", value, key)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.MarkFlagRequired("file")

	return cmd
}

// newYamlPopCmd creates the yaml pop subcommand
func newYamlPopCmd() *cobra.Command {
	var file string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "pop KEY",
		Short: "Remove and return last array element",
		Long: `Remove and return the last element from an array at the specified key.

Returns an error if the key is not an array or is empty.

Examples:
  yaml pop --file config.yaml items
  yaml pop --file config.yaml plugins.enabled --min`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return fmt.Errorf("--file flag is required")
			}

			key := args[0]

			// Acquire write lock
			lock, err := yamlFileLock(file, true)
			if err != nil {
				return err
			}
			defer lock.Unlock()

			// Read file
			data, err := readYAMLAsMap(file)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("config file not found: %s", file)
				}
				return fmt.Errorf("failed to read config file: %w", err)
			}

			// Pop value
			value, err := popFromArray(data, key)
			if err != nil {
				return err
			}

			// Write file
			if err := writeYAMLFile(file, data); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			// Output
			if jsonOutput {
				if minOutput {
					// --json --min: just the value as JSON
					jsonBytes, _ := json.Marshal(value)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				} else {
					// --json: full JSON
					output := map[string]interface{}{"key": key, "value": value, "status": "popped"}
					jsonBytes, _ := json.Marshal(output)
					fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
				}
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), formatValue(value))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "POPPED: %s from %s\n", formatValue(value), key)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "Path to YAML config file (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Minimal output")
	cmd.MarkFlagRequired("file")

	return cmd
}

// Helper functions

// formatValue converts a value to string representation
func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// parseNumber attempts to parse a string as int or float
func parseNumber(s string) (interface{}, error) {
	// Try int first
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil && fmt.Sprintf("%d", i) == s {
		return i, nil
	}

	// Try float
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err == nil {
		return f, nil
	}

	return nil, fmt.Errorf("not a number")
}

func init() {
	RootCmd.AddCommand(newYamlCmd())
}
