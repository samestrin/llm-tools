package commands

import (
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
				return fmt.Errorf("directory does not exist: %s", dir)
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
// Single quotes within the value are escaped as '\''
func escapeShellValue(value string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(value, "'", `'\''`)
	return "'" + escaped + "'"
}

func init() {
	RootCmd.AddCommand(newContextCmd())
}
