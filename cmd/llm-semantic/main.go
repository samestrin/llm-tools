package main

import (
	"fmt"
	"os"

	"github.com/samestrin/llm-tools/internal/semantic/commands"
	"github.com/samestrin/llm-tools/pkg/output"
)

var (
	version = "1.5.0"
	commit  = "dev"
)

func main() {
	rootCmd := commands.RootCmd()
	rootCmd.Version = fmt.Sprintf("%s (%s)", version, commit)

	if err := rootCmd.Execute(); err != nil {
		f := output.New(commands.GlobalJSONOutput, commands.GlobalMinOutput, os.Stdout)
		os.Exit(f.PrintError(err))
	}
}
