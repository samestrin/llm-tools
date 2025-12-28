package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/spf13/cobra"
)

var (
	contextDir string
)

// Key validation regex: must start with letter or underscore, then letters, digits, underscores
var validKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// newContextCmd creates the context parent command
func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Manage prompt context variables",
		Long: `Manage persistent key-value storage for prompt variables.

The context command provides a way to store and retrieve values across
prompt executions, solving the "forgotten timestamp" problem where LLMs
lose precise values during long-running prompts.

Subcommands:
  init   Initialize context file in a directory
  set    Store a key-value pair
  get    Retrieve a value by key
  list   List all stored values
  dump   Output in shell-sourceable format
  clear  Remove all values`,
	}

	// Add subcommands
	cmd.AddCommand(newContextInitCmd())
	cmd.AddCommand(newContextSetCmd())
	cmd.AddCommand(newContextGetCmd())
	cmd.AddCommand(newContextListCmd())
	cmd.AddCommand(newContextDumpCmd())
	cmd.AddCommand(newContextClearCmd())

	return cmd
}

// newContextInitCmd creates the context init subcommand
func newContextInitCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize context file",
		Long: `Initialize a context.env file in the specified directory.

Creates a new context.env file with header comments. If the file already
exists, it is preserved (not overwritten).

Output:
  CONTEXT_FILE: path to context file
  STATUS: CREATED | EXISTS`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir flag is required")
			}

			// Verify directory exists
			info, err := os.Stat(dir)
			if os.IsNotExist(err) {
				return fmt.Errorf("directory does not exist: %s (hint: use 'llm-support init-temp --name mycontext' to create a temp directory)", dir)
			}
			if err != nil {
				return fmt.Errorf("error accessing directory: %w", err)
			}
			if !info.IsDir() {
				return fmt.Errorf("not a directory: %s", dir)
			}

			contextFile := filepath.Join(dir, "context.env")
			status := "CREATED"

			// Check if file exists
			if _, err := os.Stat(contextFile); err == nil {
				status = "EXISTS"
			} else {
				// Create new file with header
				header := fmt.Sprintf("# llm-support context file\n# Created: %s\n\n",
					time.Now().Format("2006-01-02 15:04:05"))

				if err := os.WriteFile(contextFile, []byte(header), 0644); err != nil {
					return fmt.Errorf("failed to create context file: %w", err)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "CONTEXT_FILE: %s\n", contextFile)
			fmt.Fprintf(cmd.OutOrStdout(), "STATUS: %s\n", status)

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory for context file (required)")
	cmd.MarkFlagRequired("dir")

	return cmd
}

// newContextSetCmd creates the context set subcommand
func newContextSetCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Store a key-value pair",
		Long: `Store a key-value pair in the context file.

Keys are automatically uppercased. Valid keys must:
- Start with a letter or underscore
- Contain only letters, digits, and underscores

Values are stored with proper shell escaping using single quotes.

Examples:
  context set --dir /tmp MY_VAR "hello world"
  context set --dir /tmp MESSAGE "It's working"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir flag is required")
			}

			// Verify directory exists
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("directory does not exist: %s (hint: use 'llm-support init-temp --name mycontext' to create a temp directory)", dir)
			}

			key := args[0]
			value := args[1]

			// Validate key format
			if !validKeyRegex.MatchString(key) {
				return fmt.Errorf("invalid key format: %q (must start with letter or underscore, contain only letters, digits, underscores)", key)
			}

			// Uppercase the key
			key = strings.ToUpper(key)

			// Escape value for shell (single quotes with '\'' for embedded quotes)
			escapedValue := escapeShellValue(value)

			contextFile := filepath.Join(dir, "context.env")

			// Acquire file lock for concurrent write safety
			lockFile := contextFile + ".lock"
			fileLock := flock.New(lockFile)

			if err := fileLock.Lock(); err != nil {
				return fmt.Errorf("failed to acquire lock: %w", err)
			}
			defer fileLock.Unlock()

			// Append to file
			f, err := os.OpenFile(contextFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("context file not found: %s (hint: run 'llm-support context init --dir %s' first)", contextFile, dir)
				}
				return fmt.Errorf("failed to open context file: %w", err)
			}
			defer f.Close()

			line := fmt.Sprintf("%s=%s\n", key, escapedValue)
			if _, err := f.WriteString(line); err != nil {
				return fmt.Errorf("failed to write to context file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "SET: %s\n", key)

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory for context file (required)")
	cmd.MarkFlagRequired("dir")

	return cmd
}

// escapeShellValue escapes a value for shell sourcing using single quotes
// Single quotes within the value are escaped as '\”
func escapeShellValue(value string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(value, "'", `'\''`)
	return "'" + escaped + "'"
}

// parseContextFile reads a context file and returns a map of key-value pairs.
// Later values for the same key override earlier values.
func parseContextFile(contextFile string) (map[string]string, error) {
	result := make(map[string]string)

	f, err := os.Open(contextFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY='value' format
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}

		key := line[:idx]
		value := line[idx+1:]

		// Remove surrounding single quotes and unescape
		value = unescapeShellValue(value)

		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// unescapeShellValue reverses the escaping done by escapeShellValue
func unescapeShellValue(value string) string {
	// Remove surrounding single quotes
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		value = value[1 : len(value)-1]
	}
	// Unescape '\'' back to single quote
	value = strings.ReplaceAll(value, `'\''`, "'")
	return value
}

// newContextGetCmd creates the context get subcommand
func newContextGetCmd() *cobra.Command {
	var dir string
	var defaultValue string
	var jsonOutput bool
	var minOutput bool

	cmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Retrieve a value by key",
		Long: `Retrieve a value from the context file by key.

If the key is not found, returns an error unless --default is provided.

Output Formats:
  default: KEY: value
  --json:  {"key": "KEY", "value": "value"}
  --min:   value (just the value, no key)

Examples:
  context get --dir /tmp MY_VAR
  context get --dir /tmp MISSING --default "fallback"
  context get --dir /tmp MY_VAR --json
  context get --dir /tmp MY_VAR --min`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir flag is required")
			}

			key := strings.ToUpper(args[0])
			contextFile := filepath.Join(dir, "context.env")

			// Acquire shared lock for read consistency
			lockFile := contextFile + ".lock"
			fileLock := flock.New(lockFile)
			if err := fileLock.RLock(); err != nil {
				return fmt.Errorf("failed to acquire read lock: %w", err)
			}
			defer fileLock.Unlock()

			values, err := parseContextFile(contextFile)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("context file not found (hint: run 'llm-support context init --dir %s' first)", dir)
				}
				return fmt.Errorf("failed to read context file: %w", err)
			}

			value, found := values[key]
			if !found {
				if defaultValue != "" {
					value = defaultValue
				} else {
					return fmt.Errorf("key not found: %s", key)
				}
			}

			// Output based on format
			if jsonOutput {
				output := map[string]string{"key": key, "value": value}
				data, _ := json.Marshal(output)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else if minOutput {
				fmt.Fprintln(cmd.OutOrStdout(), value)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", key, value)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory for context file (required)")
	cmd.Flags().StringVar(&defaultValue, "default", "", "Default value if key not found")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.Flags().BoolVar(&minOutput, "min", false, "Output just the value")
	cmd.MarkFlagRequired("dir")

	return cmd
}

// newContextListCmd creates the context list subcommand
func newContextListCmd() *cobra.Command {
	var dir string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all stored values",
		Long: `List all key-value pairs in the context file.

Output Formats:
  default: KEY=value (one per line)
  --json:  {"KEY1": "value1", "KEY2": "value2"}

Examples:
  context list --dir /tmp
  context list --dir /tmp --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir flag is required")
			}

			contextFile := filepath.Join(dir, "context.env")

			// Acquire shared lock for read consistency
			lockFile := contextFile + ".lock"
			fileLock := flock.New(lockFile)
			if err := fileLock.RLock(); err != nil {
				return fmt.Errorf("failed to acquire read lock: %w", err)
			}
			defer fileLock.Unlock()

			values, err := parseContextFile(contextFile)
			if err != nil {
				return fmt.Errorf("failed to read context file: %w", err)
			}

			if len(values) == 0 {
				return nil // Empty output for empty context
			}

			if jsonOutput {
				data, _ := json.Marshal(values)
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				for key, value := range values {
					fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, value)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory for context file (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	cmd.MarkFlagRequired("dir")

	return cmd
}

// newContextDumpCmd creates the context dump subcommand
func newContextDumpCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Output in shell-sourceable format",
		Long: `Output all context values in a format that can be eval'd by a shell.

This produces output like:
  KEY1='value1'
  KEY2='value2'

Which can be used with:
  eval "$(llm-support context dump --dir /tmp)"

Examples:
  context dump --dir /tmp
  eval "$(context dump --dir /tmp)"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir flag is required")
			}

			contextFile := filepath.Join(dir, "context.env")

			// Acquire shared lock for read consistency
			lockFile := contextFile + ".lock"
			fileLock := flock.New(lockFile)
			if err := fileLock.RLock(); err != nil {
				return fmt.Errorf("failed to acquire read lock: %w", err)
			}
			defer fileLock.Unlock()

			values, err := parseContextFile(contextFile)
			if err != nil {
				return fmt.Errorf("failed to read context file: %w", err)
			}

			if len(values) == 0 {
				return nil // Empty output for empty context
			}

			for key, value := range values {
				escapedValue := escapeShellValue(value)
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, escapedValue)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory for context file (required)")
	cmd.MarkFlagRequired("dir")

	return cmd
}

// newContextClearCmd creates the context clear subcommand
func newContextClearCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove all values",
		Long: `Clear all values from the context file, preserving the header.

Examples:
  context clear --dir /tmp`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return fmt.Errorf("--dir flag is required")
			}

			contextFile := filepath.Join(dir, "context.env")

			// Acquire file lock for concurrent safety
			lockFile := contextFile + ".lock"
			fileLock := flock.New(lockFile)

			if err := fileLock.Lock(); err != nil {
				return fmt.Errorf("failed to acquire lock: %w", err)
			}
			defer fileLock.Unlock()

			// Read existing file to preserve header
			content, err := os.ReadFile(contextFile)
			if err != nil {
				return fmt.Errorf("failed to read context file: %w", err)
			}

			// Extract header lines (comments at the start)
			lines := strings.Split(string(content), "\n")
			var header []string
			for _, line := range lines {
				if strings.HasPrefix(line, "#") {
					header = append(header, line)
				} else if line == "" && len(header) > 0 {
					// Keep one empty line after header
					header = append(header, "")
					break
				} else {
					break
				}
			}

			// Write back just the header
			newContent := strings.Join(header, "\n")
			if !strings.HasSuffix(newContent, "\n") {
				newContent += "\n"
			}

			if err := os.WriteFile(contextFile, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("failed to write context file: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "CLEARED")

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "Directory for context file (required)")
	cmd.MarkFlagRequired("dir")

	return cmd
}

func init() {
	RootCmd.AddCommand(newContextCmd())
}
