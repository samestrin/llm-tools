package main

import (
	"fmt"
	"os"

	"github.com/samestrin/llm-tools/internal/semantic/commands"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

func main() {
	rootCmd := commands.RootCmd()
	rootCmd.Version = fmt.Sprintf("%s (%s)", version, commit)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
