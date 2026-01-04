package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	argsJSON    bool
	argsMinimal bool
)

// ArgsResult holds parsed argument results
type ArgsResult struct {
	Positional []string          `json:"positional,omitempty"`
	Flags      map[string]string `json:"flags,omitempty"`
}

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
		RunE: runArgs,
	}

	cmd.Flags().BoolVar(&argsJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&argsMinimal, "min", false, "Output in minimal/token-optimized format")

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

	result := ArgsResult{
		Positional: positional,
		Flags:      flags,
	}

	formatter := output.New(argsJSON, argsMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(ArgsResult)
		if len(r.Positional) > 0 {
			fmt.Fprintf(w, "POSITIONAL: %s\n", strings.Join(r.Positional, " "))
		}
		for key, value := range r.Flags {
			fmt.Fprintf(w, "%s: %s\n", strings.ToUpper(key), value)
		}
		if len(r.Positional) == 0 && len(r.Flags) == 0 {
			fmt.Fprintln(w, "NO_ARGS")
		}
	})
}

func init() {
	RootCmd.AddCommand(newArgsCmd())
}
