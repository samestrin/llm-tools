// Package main is the entry point for the llm-clarification CLI tool.
package main

import (
	"os"

	"github.com/samestrin/llm-tools/internal/clarification/commands"
	"github.com/samestrin/llm-tools/pkg/output"
)

func main() {
	if err := commands.Execute(); err != nil {
		f := output.New(commands.GlobalJSONOutput, commands.GlobalMinOutput, os.Stdout)
		os.Exit(f.PrintError(err))
	}
}
