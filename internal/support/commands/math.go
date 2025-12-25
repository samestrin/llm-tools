package commands

import (
	"fmt"
	"math"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/spf13/cobra"
)

// newMathCmd creates the math command
func newMathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "math [expression]",
		Short: "Evaluate mathematical expressions safely",
		Long: `Evaluate mathematical expressions safely using the expr library.

Supported operators:
  + - * / % ** (power)

Supported functions:
  abs, round, min, max, floor, ceil, sqrt, pow`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true, // Allow expressions like "-5 + 10"
		RunE:               runMath,
	}
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

	// Format output
	switch v := result.(type) {
	case float64:
		if v == float64(int64(v)) {
			fmt.Fprintf(cmd.OutOrStdout(), "RESULT: %d\n", int64(v))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "RESULT: %.6g\n", v)
		}
	case int, int64:
		fmt.Fprintf(cmd.OutOrStdout(), "RESULT: %d\n", v)
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "RESULT: %v\n", v)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newMathCmd())
}
