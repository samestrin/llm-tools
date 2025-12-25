// Package main is the entry point for the llm-clarification CLI tool.
package main

import (
	"fmt"
	"os"

	"github.com/samestrin/llm-tools/internal/clarification/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
