package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

// newTOMLCmd creates the toml parent command with subcommands
func newTOMLCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toml",
		Short: "TOML manipulation commands",
		Long:  `Commands for parsing, querying, and validating TOML files.`,
	}

	cmd.AddCommand(newTOMLParseCmd())
	cmd.AddCommand(newTOMLQueryCmd())
	cmd.AddCommand(newTOMLValidateCmd())

	return cmd
}

func newTOMLParseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse <file>",
		Short: "Parse and display TOML",
		Args:  cobra.ExactArgs(1),
		RunE:  runTOMLParse,
	}
	return cmd
}

func newTOMLQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <file> <path>",
		Short: "Query TOML with dot-notation path",
		Long: `Query TOML using dot-notation.
Examples:
  toml query config.toml "server"
  toml query config.toml "server.port"`,
		Args: cobra.ExactArgs(2),
		RunE: runTOMLQuery,
	}
	return cmd
}

func newTOMLValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate TOML syntax",
		Args:  cobra.ExactArgs(1),
		RunE:  runTOMLValidate,
	}
	return cmd
}

func runTOMLParse(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data map[string]interface{}
	if _, err := toml.Decode(string(content), &data); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	// Pretty print the parsed TOML
	printTOMLValue(cmd, data, 0)

	return nil
}

func printTOMLValue(cmd *cobra.Command, data interface{}, indent int) {
	prefix := strings.Repeat("  ", indent)

	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			switch innerVal := val.(type) {
			case map[string]interface{}:
				fmt.Fprintf(cmd.OutOrStdout(), "%s[%s]\n", prefix, key)
				printTOMLValue(cmd, innerVal, indent+1)
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s = %v\n", prefix, key, val)
			}
		}
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "%s%v\n", prefix, v)
	}
}

func runTOMLQuery(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data map[string]interface{}
	if _, err := toml.Decode(string(content), &data); err != nil {
		return fmt.Errorf("invalid TOML: %w", err)
	}

	path := args[1]
	result, err := queryTOML(data, path)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "QUERY: %s\n", path)
	fmt.Fprintln(cmd.OutOrStdout(), "RESULT:")
	printTOMLValue(cmd, result, 0)

	return nil
}

func queryTOML(data map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s", path)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot navigate through %T at %s", current, part)
		}
	}

	return current, nil
}

func runTOMLValidate(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data map[string]interface{}
	if _, err := toml.Decode(string(content), &data); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "✗ INVALID: %s\n", err.Error())
		return fmt.Errorf("validation failed")
	}

	// Count keys
	keyCount := countTOMLKeys(data)
	fmt.Fprintf(cmd.OutOrStdout(), "✓ VALID TOML\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Keys: %d\n", keyCount)

	return nil
}

func countTOMLKeys(data map[string]interface{}) int {
	count := len(data)
	for _, val := range data {
		if nested, ok := val.(map[string]interface{}); ok {
			count += countTOMLKeys(nested)
		}
	}
	return count
}

func init() {
	RootCmd.AddCommand(newTOMLCmd())
}
