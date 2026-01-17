package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

// parseStreamMaxInputSize limits input to prevent OOM on large inputs (10MB)
const parseStreamMaxInputSize = 10 * 1024 * 1024

var (
	parseStreamFile      string
	parseStreamContent   string
	parseStreamFormat    string
	parseStreamDelimiter string
	parseStreamHeaders   string
	parseStreamJSON      bool
	parseStreamMinimal   bool

	// Compiled regex patterns for performance
	checklistDetectPattern = regexp.MustCompile(`^[\s]*[-*+]\s*\[[xX ]\]`)
	checklistParsePattern  = regexp.MustCompile(`^([\s]*)([-*+])\s*\[([xX ])\]\s*(.*)$`)
)

// ParseError represents a parsing error at a specific location
type ParseError struct {
	Line    int    `json:"line"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
}

// ParseStreamResult holds the result of stream parsing
type ParseStreamResult struct {
	Format      string                   `json:"format"`
	Delimiter   string                   `json:"delimiter,omitempty"`
	Headers     []string                 `json:"headers"`
	Rows        []map[string]interface{} `json:"rows"`
	RowCount    int                      `json:"row_count"`
	ParseErrors []ParseError             `json:"parse_errors"`
}

// newParseStreamCmd creates the parse-stream command
func newParseStreamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parse-stream",
		Short: "Parse structured data streams into JSON",
		Long: `Parse structured data formats (pipe-delimited, markdown checklists) into JSON.

Supported formats:
  auto               - Auto-detect format based on content
  pipe               - Pipe-delimited format (e.g., TD_STREAM)
  markdown-checklist - Markdown checkbox lists (- [ ] / - [x])

Examples:
  llm-support parse-stream --file data.txt --format pipe
  llm-support parse-stream --file tasks.md --format markdown-checklist
  cat data.txt | llm-support parse-stream --format pipe`,
		RunE: runParseStream,
	}

	cmd.Flags().StringVar(&parseStreamFile, "file", "", "Input file path")
	cmd.Flags().StringVar(&parseStreamContent, "content", "", "Direct content input (alternative to file/stdin)")
	cmd.Flags().StringVar(&parseStreamFormat, "format", "auto", "Format: auto, pipe, markdown-checklist")
	cmd.Flags().StringVar(&parseStreamDelimiter, "delimiter", "|", "Delimiter for pipe format")
	cmd.Flags().StringVar(&parseStreamHeaders, "headers", "", "Comma-separated header names (overrides auto-detection)")
	cmd.Flags().BoolVar(&parseStreamJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&parseStreamMinimal, "min", false, "Minimal output format")

	return cmd
}

func runParseStream(cmd *cobra.Command, args []string) error {
	// Get input content
	content, err := getParseStreamInput(cmd)
	if err != nil {
		return err
	}

	// Detect or use specified format
	format := parseStreamFormat
	if format == "auto" {
		format = detectFormat(content)
	}

	var result ParseStreamResult

	switch format {
	case "pipe":
		result, err = parsePipeDelimited(content)
	case "markdown-checklist":
		result, err = parseMarkdownChecklist(content)
	default:
		return fmt.Errorf("unknown format: %s. Valid formats: auto, pipe, markdown-checklist", format)
	}

	if err != nil {
		return err
	}

	formatter := output.New(parseStreamJSON, parseStreamMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(ParseStreamResult)
		fmt.Fprintf(w, "FORMAT: %s\n", r.Format)
		if r.Delimiter != "" {
			fmt.Fprintf(w, "DELIMITER: %s\n", r.Delimiter)
		}
		fmt.Fprintf(w, "HEADERS: %s\n", strings.Join(r.Headers, ", "))
		fmt.Fprintf(w, "ROW_COUNT: %d\n", r.RowCount)
		if len(r.ParseErrors) > 0 {
			fmt.Fprintf(w, "PARSE_ERRORS: %d\n", len(r.ParseErrors))
		}
	})
}

func getParseStreamInput(cmd *cobra.Command) (string, error) {
	// Priority: --content flag, then --file flag, then stdin
	if parseStreamContent != "" {
		if len(parseStreamContent) > parseStreamMaxInputSize {
			return "", fmt.Errorf("content exceeds maximum size of %d bytes", parseStreamMaxInputSize)
		}
		return parseStreamContent, nil
	}

	if parseStreamFile != "" {
		// Check file size before reading to prevent OOM
		info, err := os.Stat(parseStreamFile)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("file not found: %s", parseStreamFile)
			}
			return "", fmt.Errorf("failed to stat file: %w", err)
		}
		if info.Size() > parseStreamMaxInputSize {
			return "", fmt.Errorf("file size %d exceeds maximum of %d bytes", info.Size(), parseStreamMaxInputSize)
		}
		data, err := os.ReadFile(parseStreamFile)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		return string(data), nil
	}

	// Try to read from stdin if available
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Use LimitReader to prevent OOM on large stdin input
		limitedReader := io.LimitReader(os.Stdin, parseStreamMaxInputSize+1)
		data, err := io.ReadAll(limitedReader)
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		if len(data) > parseStreamMaxInputSize {
			return "", fmt.Errorf("stdin input exceeds maximum size of %d bytes", parseStreamMaxInputSize)
		}
		if len(data) > 0 {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("no input provided: specify --file, --content, or pipe data to stdin")
}

func detectFormat(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "pipe" // default
	}

	// Check for markdown checklist pattern
	for _, line := range lines {
		if checklistDetectPattern.MatchString(line) {
			return "markdown-checklist"
		}
	}

	// Default to pipe-delimited if contains pipe characters
	for _, line := range lines {
		if strings.Contains(line, "|") {
			return "pipe"
		}
	}

	return "pipe" // default
}

func parsePipeDelimited(content string) (ParseStreamResult, error) {
	result := ParseStreamResult{
		Format:      "pipe",
		Delimiter:   parseStreamDelimiter,
		Headers:     []string{},
		Rows:        []map[string]interface{}{},
		ParseErrors: []ParseError{},
	}

	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return result, nil
	}

	// Handle headers
	var headers []string
	startRow := 0

	if parseStreamHeaders != "" {
		// Use explicit headers
		headers = strings.Split(parseStreamHeaders, ",")
		for i := range headers {
			headers[i] = strings.TrimSpace(headers[i])
		}
	} else {
		// Auto-detect from first row
		headers = splitByDelimiter(lines[0], parseStreamDelimiter)
		startRow = 1
	}

	result.Headers = headers
	expectedCols := len(headers)

	// Parse data rows
	for i := startRow; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := splitByDelimiter(line, parseStreamDelimiter)

		// Track column count mismatches
		if len(fields) != expectedCols {
			result.ParseErrors = append(result.ParseErrors, ParseError{
				Line:    i + 1, // 1-indexed
				Message: fmt.Sprintf("expected %d columns, got %d", expectedCols, len(fields)),
			})
		}

		// Create row map
		row := make(map[string]interface{})
		for j, header := range headers {
			if j < len(fields) {
				row[header] = fields[j]
			} else {
				row[header] = "" // missing field
			}
		}

		// Handle extra fields
		if len(fields) > len(headers) {
			for j := len(headers); j < len(fields); j++ {
				row[fmt.Sprintf("_extra_%d", j-len(headers)+1)] = fields[j]
			}
		}

		result.Rows = append(result.Rows, row)
	}

	result.RowCount = len(result.Rows)
	return result, nil
}

func splitByDelimiter(line, delimiter string) []string {
	parts := strings.Split(line, delimiter)
	result := make([]string, len(parts))
	for i, part := range parts {
		result[i] = strings.TrimSpace(part)
	}
	return result
}

func parseMarkdownChecklist(content string) (ParseStreamResult, error) {
	result := ParseStreamResult{
		Format:      "markdown-checklist",
		Headers:     []string{"checked", "text", "indent"},
		Rows:        []map[string]interface{}{},
		ParseErrors: []ParseError{},
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		matches := checklistParsePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		indent := len(matches[1])
		checkMark := matches[3]
		text := strings.TrimSpace(matches[4])

		checked := strings.ToLower(checkMark) == "x"

		row := map[string]interface{}{
			"checked": checked,
			"text":    text,
			"indent":  indent,
			"line":    lineNum,
		}

		result.Rows = append(result.Rows, row)
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("error reading content: %w", err)
	}

	result.RowCount = len(result.Rows)
	return result, nil
}

func init() {
	RootCmd.AddCommand(newParseStreamCmd())
}
