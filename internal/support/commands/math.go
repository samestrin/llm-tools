package commands

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/spf13/cobra"
)

var (
	mathJSON bool
	mathMin  bool
)

// newMathCmd creates the math command
func newMathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "math <expression>",
		Short: "Evaluate mathematical expressions safely",
		Long: `Evaluate mathematical expressions safely using the expr library.

Supported operators:
  + - * / % ** (power)

Supported functions:
  abs, round, min, max, floor, ceil, sqrt, pow

Note: For expressions starting with -, use quotes or = prefix:
  llm-support math "(-5 + 10)"
  llm-support math -- -5 + 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: runMath,
	}

	cmd.Flags().BoolVar(&mathJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&mathMin, "min", false, "Minimal output - just the value")

	return cmd
}

func runMath(cmd *cobra.Command, args []string) error {
	expression := strings.Join(args, " ")

	// Replace ** with pow() for power operations
	expression = strings.ReplaceAll(expression, "**", "^")

	// Define environment with math functions
	env := map[string]interface{}{
		"abs": func(x float64) float64 {
			if x < 0 {
				return -x
			}
			return x
		},
		"round": math.Round,
		"floor": math.Floor,
		"ceil":  math.Ceil,
		"sqrt":  math.Sqrt,
		"pow":   math.Pow,
		"min": func(args ...float64) float64 {
			if len(args) == 0 {
				return 0
			}
			m := args[0]
			for _, v := range args[1:] {
				if v < m {
					m = v
				}
			}
			return m
		},
		"max": func(args ...float64) float64 {
			if len(args) == 0 {
				return 0
			}
			m := args[0]
			for _, v := range args[1:] {
				if v > m {
					m = v
				}
			}
			return m
		},
	}

	// Compile and run expression
	// Disable builtins that conflict with Go 1.24's new builtin functions
	program, err := expr.Compile(expression,
		expr.Env(env),
		expr.DisableBuiltin("min"),
		expr.DisableBuiltin("max"),
		expr.DisableBuiltin("ceil"),
		expr.DisableBuiltin("floor"),
		expr.DisableBuiltin("abs"),
		expr.DisableBuiltin("round"),
	)
	if err != nil {
		return fmt.Errorf("invalid expression: %v", err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return fmt.Errorf("evaluation error: %v", err)
	}

	// Format result value
	var resultStr string
	switch v := result.(type) {
	case float64:
		if v == float64(int64(v)) {
			resultStr = fmt.Sprintf("%d", int64(v))
		} else {
			resultStr = fmt.Sprintf("%.6g", v)
		}
	case int:
		resultStr = fmt.Sprintf("%d", v)
	case int64:
		resultStr = fmt.Sprintf("%d", v)
	default:
		resultStr = fmt.Sprintf("%v", v)
	}

	// Output based on format flags
	if mathJSON {
		output := map[string]interface{}{
			"expression": expression,
			"result":     result,
		}
		jsonBytes, _ := json.Marshal(output)
		fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
	} else if mathMin {
		fmt.Fprintln(cmd.OutOrStdout(), resultStr)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "RESULT: %s\n", resultStr)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newMathCmd())
}
