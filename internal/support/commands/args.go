package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newArgsCmd creates the args command
func newArgsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "args [arguments...]",
		Short: "Parse arguments into structured format",
		Long: `Parse command-line arguments into a structured format.
Separates positional arguments from flags and key-value pairs.

Output format:
  POSITIONAL: arg1 arg2 ...
  FLAG_NAME: value
  BOOLEAN_FLAG: true`,
		DisableFlagParsing: true, // Don't let Cobra parse flags - we handle them
		RunE:               runArgs,
	}
	return cmd
}

func runArgs(cmd *cobra.Command, args []string) error {
	positional := []string{}
	flags := make(map[string]string)

	i := 0
	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			// It's a flag or key-value pair
			key := strings.ReplaceAll(arg[2:], "-", "_")

			// Check if next arg is a value or another flag
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[key] = args[i+1]
				i += 2
			} else {
				flags[key] = "true"
				i++
			}
		} else {
			// Positional argument
			positional = append(positional, arg)
			i++
		}
	}

	// Output in LLM-friendly format
	var outputLines []string

	if len(positional) > 0 {
		outputLines = append(outputLines, fmt.Sprintf("POSITIONAL: %s", strings.Join(positional, " ")))
	}

	for key, value := range flags {
		outputLines = append(outputLines, fmt.Sprintf("%s: %s", strings.ToUpper(key), value))
	}

	if len(outputLines) == 0 {
		outputLines = append(outputLines, "NO_ARGS")
	}

	fmt.Fprintln(cmd.OutOrStdout(), strings.Join(outputLines, "\n"))
	return nil
}

func init() {
	RootCmd.AddCommand(newArgsCmd())
}
