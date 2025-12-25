package commands

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	extractUnique bool
	extractCount  bool
)

// newExtractCmd creates the extract command
func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract <type> <file>",
		Short: "Extract patterns from text files",
		Long: `Extract various patterns from text files.

Types:
  urls       - Extract URLs (http/https)
  paths      - Extract file paths
  variables  - Extract template variables {{var}}
  todos      - Extract TODO checkboxes
  emails     - Extract email addresses
  ips        - Extract IP addresses`,
		Args: cobra.ExactArgs(2),
		RunE: runExtract,
	}

	cmd.Flags().BoolVar(&extractUnique, "unique", false, "Remove duplicates")
	cmd.Flags().BoolVar(&extractCount, "count", false, "Show count only")

	return cmd
}

func runExtract(cmd *cobra.Command, args []string) error {
	extractType := args[0]
	filePath := args[1]

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	text := string(content)
	var results []string

	switch extractType {
	case "urls":
		pattern := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
		results = pattern.FindAllString(text, -1)

	case "paths":
		pattern := regexp.MustCompile(`(?:\.{0,2}/|~/|/)[^\s:;,<>"|*?]+`)
		results = pattern.FindAllString(text, -1)

	case "variables":
		pattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			// Get variable name (before |)
			varName := strings.Split(match[1], "|")[0]
			results = append(results, strings.TrimSpace(varName))
		}

	case "todos":
		pattern := regexp.MustCompile(`- \[([ xX])\] (.+)`)
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			done := "x"
			if strings.TrimSpace(match[1]) == "" {
				done = " "
			}
			results = append(results, fmt.Sprintf("[%s] %s", done, match[2]))
		}

	case "emails":
		pattern := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
		results = pattern.FindAllString(text, -1)

	case "ips":
		pattern := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
		results = pattern.FindAllString(text, -1)

	default:
		return fmt.Errorf("unknown extraction type: %s (supported: urls, paths, variables, todos, emails, ips)", extractType)
	}

	// Remove duplicates if requested
	if extractUnique {
		seen := make(map[string]bool)
		unique := []string{}
		for _, r := range results {
			if !seen[r] {
				seen[r] = true
				unique = append(unique, r)
			}
		}
		results = unique
	}

	// Output
	if extractCount {
		fmt.Fprintf(cmd.OutOrStdout(), "COUNT: %d\n", len(results))
	} else {
		for _, r := range results {
			fmt.Fprintln(cmd.OutOrStdout(), r)
		}
		if len(results) == 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "No %s found\n", extractType)
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newExtractCmd())
}
