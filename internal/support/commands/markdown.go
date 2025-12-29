package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	mdHeaderLevel     string
	mdHeaderPlain     bool
	mdHeaderJSON      bool
	mdHeaderMinimal   bool
	mdTasksSummary    bool
	mdTasksJSON       bool
	mdTasksMinimal    bool
	mdSectionHeader   bool
	mdSectionJSON     bool
	mdSectionMinimal  bool
	mdFrontmatterJSON bool
	mdFrontmatterMin  bool
	mdCodeLanguage    string
	mdCodeListOnly    bool
	mdCodeJSON        bool
	mdCodeMinimal     bool
)

// MdHeaderEntry represents a markdown header
type MdHeaderEntry struct {
	Level int    `json:"level,omitempty"`
	L     *int   `json:"l,omitempty"`
	Text  string `json:"text,omitempty"`
	T     string `json:"t,omitempty"`
}

// MdHeadersResult holds the headers extraction result
type MdHeadersResult struct {
	File    string          `json:"file,omitempty"`
	F       string          `json:"f,omitempty"`
	Headers []MdHeaderEntry `json:"headers,omitempty"`
	H       []MdHeaderEntry `json:"h,omitempty"`
}

// MdTasksResult holds the tasks extraction result
type MdTasksResult struct {
	File           string    `json:"file,omitempty"`
	F              string    `json:"f,omitempty"`
	TotalTasks     int       `json:"total_tasks,omitempty"`
	TT             *int      `json:"tt,omitempty"`
	Completed      int       `json:"completed,omitempty"`
	Cp             *int      `json:"cp,omitempty"`
	Incomplete     int       `json:"incomplete,omitempty"`
	Inc            *int      `json:"inc,omitempty"`
	CompletionRate float64   `json:"completion_rate,omitempty"`
	CR             *float64  `json:"cr,omitempty"`
	Tasks          []MdTask  `json:"tasks,omitempty"`
	Ts             []MdTask  `json:"ts,omitempty"`
}

// MdTask represents a single task
type MdTask struct {
	Text string `json:"text,omitempty"`
	T    string `json:"t,omitempty"`
	Done bool   `json:"done,omitempty"`
	D    *bool  `json:"d,omitempty"`
}

// MdSectionResult holds the section extraction result
type MdSectionResult struct {
	File    string `json:"file,omitempty"`
	F       string `json:"f,omitempty"`
	Title   string `json:"title,omitempty"`
	Ti      string `json:"ti,omitempty"`
	Content string `json:"content,omitempty"`
	C       string `json:"c,omitempty"`
}

// MdFrontmatterResult holds the frontmatter extraction result
type MdFrontmatterResult struct {
	File   string            `json:"file,omitempty"`
	F      string            `json:"f,omitempty"`
	Raw    string            `json:"raw,omitempty"`
	R      string            `json:"r,omitempty"`
	Parsed map[string]string `json:"parsed,omitempty"`
	P      map[string]string `json:"p,omitempty"`
}

// MdCodeBlock represents a code block
type MdCodeBlock struct {
	Number   int    `json:"number,omitempty"`
	N        *int   `json:"n,omitempty"`
	Language string `json:"language,omitempty"`
	L        string `json:"l,omitempty"`
	Code     string `json:"code,omitempty"`
	C        string `json:"c,omitempty"`
}

// MdCodeblocksResult holds the codeblocks extraction result
type MdCodeblocksResult struct {
	File   string        `json:"file,omitempty"`
	F      string        `json:"f,omitempty"`
	Blocks []MdCodeBlock `json:"blocks,omitempty"`
	B      []MdCodeBlock `json:"b,omitempty"`
}

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
	cmd.Flags().BoolVar(&mdHeaderJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&mdHeaderMinimal, "min", false, "Output in minimal/token-optimized format")
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
	cmd.Flags().BoolVar(&mdTasksJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&mdTasksMinimal, "min", false, "Output in minimal/token-optimized format")
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
	cmd.Flags().BoolVar(&mdSectionJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&mdSectionMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func newMdFrontmatterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frontmatter <file>",
		Short: "Extract YAML frontmatter",
		Args:  cobra.ExactArgs(1),
		RunE:  runMdFrontmatter,
	}
	cmd.Flags().BoolVar(&mdFrontmatterJSON, "json", false, "Output as JSON (also parses YAML to JSON)")
	cmd.Flags().BoolVar(&mdFrontmatterMin, "min", false, "Output in minimal/token-optimized format")
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
	cmd.Flags().BoolVar(&mdCodeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&mdCodeMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runMdHeaders(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
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

	// Collect headers
	var headers []MdHeaderEntry
	for _, match := range matches {
		hashes := match[1]
		text := match[2]
		level := len(hashes)

		// Filter by level if specified
		if levels != nil && !levels[level] {
			continue
		}

		headers = append(headers, MdHeaderEntry{
			Level: level,
			Text:  text,
		})
	}

	// Build result
	var result MdHeadersResult
	if mdHeaderMinimal {
		minHeaders := make([]MdHeaderEntry, len(headers))
		for i, h := range headers {
			lvl := h.Level
			minHeaders[i] = MdHeaderEntry{L: &lvl, T: h.Text}
		}
		result = MdHeadersResult{F: filePath, H: minHeaders}
	} else {
		result = MdHeadersResult{File: filePath, Headers: headers}
	}

	formatter := output.New(mdHeaderJSON, mdHeaderMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		for _, h := range headers {
			hashes := strings.Repeat("#", h.Level)
			if mdHeaderPlain {
				fmt.Fprintln(w, h.Text)
			} else {
				indent := strings.Repeat("  ", h.Level-1)
				fmt.Fprintf(w, "%s%s %s\n", indent, hashes, h.Text)
			}
		}
	})
}

func runMdTasks(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern := regexp.MustCompile(`- \[([ xX])\] (.+)`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	total := len(matches)
	completed := 0
	var tasks []MdTask
	for _, match := range matches {
		done := strings.ToLower(match[1]) == "x"
		if done {
			completed++
		}
		tasks = append(tasks, MdTask{Text: match[2], Done: done})
	}

	incomplete := total - completed
	percentage := 0.0
	if total > 0 {
		percentage = float64(completed) / float64(total) * 100
	}

	// Build result
	var result MdTasksResult
	if mdTasksMinimal {
		minTasks := make([]MdTask, len(tasks))
		for i, t := range tasks {
			d := t.Done
			minTasks[i] = MdTask{T: t.Text, D: &d}
		}
		result = MdTasksResult{
			F:   filePath,
			TT:  &total,
			Cp:  &completed,
			Inc: &incomplete,
			CR:  &percentage,
		}
		if !mdTasksSummary {
			result.Ts = minTasks
		}
	} else {
		result = MdTasksResult{
			File:           filePath,
			TotalTasks:     total,
			Completed:      completed,
			Incomplete:     incomplete,
			CompletionRate: percentage,
		}
		if !mdTasksSummary {
			result.Tasks = tasks
		}
	}

	formatter := output.New(mdTasksJSON, mdTasksMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if mdTasksSummary {
			fmt.Fprintf(w, "TOTAL_TASKS: %d\n", total)
			fmt.Fprintf(w, "COMPLETED: %d\n", completed)
			fmt.Fprintf(w, "INCOMPLETE: %d\n", incomplete)
			fmt.Fprintf(w, "COMPLETION_RATE: %.1f%%\n", percentage)
		} else {
			for _, t := range tasks {
				status := "☐"
				if t.Done {
					status = "✓"
				}
				fmt.Fprintf(w, "%s %s\n", status, t.Text)
			}

			if total > 0 {
				fmt.Fprintln(w)
				fmt.Fprintf(w, "TOTAL_TASKS: %d\n", total)
				fmt.Fprintf(w, "COMPLETED: %d (%.1f%%)\n", completed, percentage)
				fmt.Fprintf(w, "INCOMPLETE: %d\n", incomplete)
			}
		}
	})
}

func runMdSection(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
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
	headerLine := text[match[0]:match[1]]

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

	// Build result
	var result MdSectionResult
	if mdSectionMinimal {
		result = MdSectionResult{F: filePath, Ti: sectionTitle, C: sectionContent}
	} else {
		result = MdSectionResult{File: filePath, Title: sectionTitle, Content: sectionContent}
	}

	formatter := output.New(mdSectionJSON, mdSectionMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		if mdSectionHeader {
			fmt.Fprintln(w, headerLine)
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, sectionContent)
	})
}

func runMdFrontmatter(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`)
	match := pattern.FindStringSubmatch(string(content))

	if match == nil {
		return fmt.Errorf("no frontmatter found")
	}

	frontmatter := match[1]

	// Parse simple YAML frontmatter to map
	parsed := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, ":"); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			value = strings.Trim(value, "\"'")
			parsed[key] = value
		}
	}

	// Build result
	var result MdFrontmatterResult
	if mdFrontmatterMin {
		result = MdFrontmatterResult{F: filePath, R: frontmatter, P: parsed}
	} else {
		result = MdFrontmatterResult{File: filePath, Raw: frontmatter, Parsed: parsed}
	}

	formatter := output.New(mdFrontmatterJSON, mdFrontmatterMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintln(w, frontmatter)
	})
}

func runMdCodeblocks(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	matches := pattern.FindAllStringSubmatch(string(content), -1)

	var blocks []MdCodeBlock
	blockNum := 0
	for _, match := range matches {
		language := match[1]
		code := match[2]

		// Filter by language if specified
		if mdCodeLanguage != "" && !strings.EqualFold(language, mdCodeLanguage) {
			continue
		}

		blockNum++
		lang := language
		if lang == "" {
			lang = "(no language)"
		}

		blocks = append(blocks, MdCodeBlock{
			Number:   blockNum,
			Language: lang,
			Code:     strings.TrimRight(code, "\n"),
		})
	}

	// Build result
	var result MdCodeblocksResult
	if mdCodeMinimal {
		minBlocks := make([]MdCodeBlock, len(blocks))
		for i, b := range blocks {
			num := b.Number
			minBlocks[i] = MdCodeBlock{N: &num, L: b.Language}
			if !mdCodeListOnly {
				minBlocks[i].C = b.Code
			}
		}
		result = MdCodeblocksResult{F: filePath, B: minBlocks}
	} else {
		if mdCodeListOnly {
			listBlocks := make([]MdCodeBlock, len(blocks))
			for i, b := range blocks {
				listBlocks[i] = MdCodeBlock{Number: b.Number, Language: b.Language}
			}
			result = MdCodeblocksResult{File: filePath, Blocks: listBlocks}
		} else {
			result = MdCodeblocksResult{File: filePath, Blocks: blocks}
		}
	}

	formatter := output.New(mdCodeJSON, mdCodeMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		for _, b := range blocks {
			if mdCodeListOnly {
				fmt.Fprintf(w, "Block %d: %s\n", b.Number, b.Language)
			} else {
				fmt.Fprintln(w, strings.Repeat("=", 60))
				fmt.Fprintf(w, "CODE BLOCK %d: %s\n", b.Number, b.Language)
				fmt.Fprintln(w, strings.Repeat("=", 60))
				fmt.Fprintln(w, b.Code)
				fmt.Fprintln(w)
			}
		}
	})
}

func init() {
	RootCmd.AddCommand(newMarkdownCmd())
}
