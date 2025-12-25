package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mdHeaderLevel     string
	mdHeaderPlain     bool
	mdTasksSummary    bool
	mdSectionHeader   bool
	mdFrontmatterJSON bool
	mdCodeLanguage    string
	mdCodeListOnly    bool
)

// newMarkdownCmd creates the markdown parent command with subcommands
func newMarkdownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "markdown",
		Short: "Markdown parsing and extraction",
		Long:  `Commands for parsing and extracting content from Markdown files.`,
	}

	cmd.AddCommand(newMdHeadersCmd())
	cmd.AddCommand(newMdTasksCmd())
	cmd.AddCommand(newMdSectionCmd())
	cmd.AddCommand(newMdFrontmatterCmd())
	cmd.AddCommand(newMdCodeblocksCmd())

	return cmd
}

func newMdHeadersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headers <file>",
		Short: "Extract headers from markdown",
		Args:  cobra.ExactArgs(1),
		RunE:  runMdHeaders,
	}
	cmd.Flags().StringVar(&mdHeaderLevel, "level", "", "Filter by level (e.g., '2' or '1,2')")
	cmd.Flags().BoolVar(&mdHeaderPlain, "plain", false, "Output header text only")
	return cmd
}

func newMdTasksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tasks <file>",
		Short: "Extract and count tasks",
		Args:  cobra.ExactArgs(1),
		RunE:  runMdTasks,
	}
	cmd.Flags().BoolVar(&mdTasksSummary, "summary", false, "Show summary only")
	return cmd
}

func newMdSectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "section <file> <title>",
		Short: "Extract section by title",
		Args:  cobra.ExactArgs(2),
		RunE:  runMdSection,
	}
	cmd.Flags().BoolVar(&mdSectionHeader, "include-header", false, "Include section header")
	return cmd
}

func newMdFrontmatterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frontmatter <file>",
		Short: "Extract YAML frontmatter",
		Args:  cobra.ExactArgs(1),
		RunE:  runMdFrontmatter,
	}
	cmd.Flags().BoolVar(&mdFrontmatterJSON, "json", false, "Output as JSON")
	return cmd
}

func newMdCodeblocksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codeblocks <file>",
		Short: "Extract code blocks",
		Args:  cobra.ExactArgs(1),
		RunE:  runMdCodeblocks,
	}
	cmd.Flags().StringVar(&mdCodeLanguage, "language", "", "Filter by language")
	cmd.Flags().BoolVar(&mdCodeListOnly, "list", false, "List blocks only")
	return cmd
}

func runMdHeaders(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse level filter
	var levels map[int]bool
	if mdHeaderLevel != "" {
		levels = make(map[int]bool)
		for _, l := range strings.Split(mdHeaderLevel, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(l))
			if err == nil && n >= 1 && n <= 6 {
				levels[n] = true
			}
		}
	}

	pattern := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		hashes := match[1]
		text := match[2]
		level := len(hashes)

		// Filter by level if specified
		if levels != nil && !levels[level] {
			continue
		}

		if mdHeaderPlain {
			fmt.Fprintln(cmd.OutOrStdout(), text)
		} else {
			indent := strings.Repeat("  ", level-1)
			fmt.Fprintf(cmd.OutOrStdout(), "%s%s %s\n", indent, hashes, text)
		}
	}

	return nil
}

func runMdTasks(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern := regexp.MustCompile(`- \[([ xX])\] (.+)`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	total := len(matches)
	completed := 0
	for _, match := range matches {
		if strings.ToLower(match[1]) == "x" {
			completed++
		}
	}

	if mdTasksSummary {
		percentage := 0.0
		if total > 0 {
			percentage = float64(completed) / float64(total) * 100
		}
		fmt.Fprintf(cmd.OutOrStdout(), "TOTAL_TASKS: %d\n", total)
		fmt.Fprintf(cmd.OutOrStdout(), "COMPLETED: %d\n", completed)
		fmt.Fprintf(cmd.OutOrStdout(), "INCOMPLETE: %d\n", total-completed)
		fmt.Fprintf(cmd.OutOrStdout(), "COMPLETION_RATE: %.1f%%\n", percentage)
	} else {
		for _, match := range matches {
			done := strings.ToLower(match[1]) == "x"
			task := match[2]
			status := "☐"
			if done {
				status = "✓"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", status, task)
		}

		if total > 0 {
			percentage := float64(completed) / float64(total) * 100
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "TOTAL_TASKS: %d\n", total)
			fmt.Fprintf(cmd.OutOrStdout(), "COMPLETED: %d (%.1f%%)\n", completed, percentage)
			fmt.Fprintf(cmd.OutOrStdout(), "INCOMPLETE: %d\n", total-completed)
		}
	}

	return nil
}

func runMdSection(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	sectionTitle := args[1]
	text := string(content)

	// Find the section header
	pattern := regexp.MustCompile(`(?im)^(#{1,6})\s+` + regexp.QuoteMeta(sectionTitle) + `\s*$`)
	match := pattern.FindStringSubmatchIndex(text)

	if match == nil {
		return fmt.Errorf("section '%s' not found", sectionTitle)
	}

	headerLevel := len(text[match[2]:match[3]]) // Length of # symbols
	startPos := match[1]

	// Find next header of same or higher level
	nextPattern := regexp.MustCompile(`(?m)^#{1,` + strconv.Itoa(headerLevel) + `}\s+.+$`)
	remaining := text[startPos:]
	nextMatch := nextPattern.FindStringIndex(remaining)

	var sectionContent string
	if nextMatch != nil {
		sectionContent = remaining[0:nextMatch[0]]
	} else {
		sectionContent = remaining
	}

	sectionContent = strings.TrimSpace(sectionContent)

	if mdSectionHeader {
		fmt.Fprintln(cmd.OutOrStdout(), text[match[0]:match[1]])
		fmt.Fprintln(cmd.OutOrStdout())
	}

	fmt.Fprintln(cmd.OutOrStdout(), sectionContent)
	return nil
}

func runMdFrontmatter(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	match := pattern.FindStringSubmatch(string(content))

	if match == nil {
		return fmt.Errorf("no frontmatter found")
	}

	frontmatter := match[1]

	if mdFrontmatterJSON {
		// Parse simple YAML frontmatter to JSON
		result := make(map[string]string)
		scanner := bufio.NewScanner(strings.NewReader(frontmatter))
		for scanner.Scan() {
			line := scanner.Text()
			if idx := strings.Index(line, ":"); idx != -1 {
				key := strings.TrimSpace(line[:idx])
				value := strings.TrimSpace(line[idx+1:])
				value = strings.Trim(value, "\"'")
				result[key] = value
			}
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), frontmatter)
	}

	return nil
}

func runMdCodeblocks(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	blockNum := 0
	for _, match := range matches {
		language := match[1]
		code := match[2]

		// Filter by language if specified
		if mdCodeLanguage != "" && !strings.EqualFold(language, mdCodeLanguage) {
			continue
		}

		blockNum++

		if mdCodeListOnly {
			lang := language
			if lang == "" {
				lang = "(no language)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Block %d: %s\n", blockNum, lang)
		} else {
			lang := language
			if lang == "" {
				lang = "(no language)"
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))
			fmt.Fprintf(cmd.OutOrStdout(), "CODE BLOCK %d: %s\n", blockNum, lang)
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("=", 60))
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(code, "\n"))
			fmt.Fprintln(cmd.OutOrStdout())
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newMarkdownCmd())
}
