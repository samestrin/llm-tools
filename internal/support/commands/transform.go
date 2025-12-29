package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	transformCaseTo       string
	transformCaseJSON     bool
	transformCaseMinimal  bool
	transformSortReverse  bool
	transformSortUnique   bool
	transformSortNoEmpty  bool
	transformSortJSON     bool
	transformSortMinimal  bool
	transformFilterInvert bool
	transformFilterJSON   bool
	transformFilterMin    bool
)

// TransformCaseResult holds the case transformation result
type TransformCaseResult struct {
	Input  string `json:"input,omitempty"`
	I      string `json:"i,omitempty"`
	Output string `json:"output,omitempty"`
	O      string `json:"o,omitempty"`
	ToCase string `json:"to_case,omitempty"`
	TC     string `json:"tc,omitempty"`
}

// TransformSortResult holds the sort result
type TransformSortResult struct {
	File   string   `json:"file,omitempty"`
	F      string   `json:"f,omitempty"`
	Lines  []string `json:"lines,omitempty"`
	L      []string `json:"l,omitempty"`
	Count  int      `json:"count,omitempty"`
	Cnt    *int     `json:"cnt,omitempty"`
}

// TransformFilterResult holds the filter result
type TransformFilterResult struct {
	File    string   `json:"file,omitempty"`
	F       string   `json:"f,omitempty"`
	Pattern string   `json:"pattern,omitempty"`
	P       string   `json:"p,omitempty"`
	Lines   []string `json:"lines,omitempty"`
	L       []string `json:"l,omitempty"`
	Count   int      `json:"count,omitempty"`
	Cnt     *int     `json:"cnt,omitempty"`
}

// newTransformCmd creates the transform parent command with subcommands
func newTransformCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transform",
		Short: "Text and data transformation",
		Long:  `Commands for transforming text and data formats.`,
	}

	cmd.AddCommand(newTransformCSVToJSONCmd())
	cmd.AddCommand(newTransformJSONToCSVCmd())
	cmd.AddCommand(newTransformCaseCmd())
	cmd.AddCommand(newTransformSortCmd())
	cmd.AddCommand(newTransformFilterCmd())

	return cmd
}

func newTransformCSVToJSONCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csv-to-json <file>",
		Short: "Convert CSV to JSON",
		Args:  cobra.ExactArgs(1),
		RunE:  runTransformCSVToJSON,
	}
	return cmd
}

func newTransformJSONToCSVCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "json-to-csv <file>",
		Short: "Convert JSON array to CSV",
		Args:  cobra.ExactArgs(1),
		RunE:  runTransformJSONToCSV,
	}
	return cmd
}

func newTransformCaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "case <text> --to <case>",
		Short: "Transform text case",
		Long: `Transform text to different case formats.
Supported formats: camelCase, PascalCase, snake_case, kebab-case, UPPERCASE, lowercase, Title Case`,
		Args: cobra.ExactArgs(1),
		RunE: runTransformCase,
	}
	cmd.Flags().StringVar(&transformCaseTo, "to", "", "Target case format (required)")
	cmd.Flags().BoolVar(&transformCaseJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&transformCaseMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("to")
	return cmd
}

func newTransformSortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sort <file>",
		Short: "Sort lines in file",
		Args:  cobra.ExactArgs(1),
		RunE:  runTransformSort,
	}
	cmd.Flags().BoolVar(&transformSortReverse, "reverse", false, "Sort in reverse order")
	cmd.Flags().BoolVar(&transformSortUnique, "unique", false, "Remove duplicate lines")
	cmd.Flags().BoolVar(&transformSortNoEmpty, "no-empty", false, "Remove empty lines")
	cmd.Flags().BoolVar(&transformSortJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&transformSortMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func newTransformFilterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter <file> <pattern>",
		Short: "Filter lines by regex pattern",
		Args:  cobra.ExactArgs(2),
		RunE:  runTransformFilter,
	}
	cmd.Flags().BoolVar(&transformFilterInvert, "invert", false, "Invert match (exclude matching lines)")
	cmd.Flags().BoolVar(&transformFilterJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&transformFilterMin, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runTransformCSVToJSON(cmd *cobra.Command, args []string) error {
	file, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 1 {
		return fmt.Errorf("empty CSV file")
	}

	headers := records[0]
	var result []map[string]string

	for _, row := range records[1:] {
		record := make(map[string]string)
		for i, val := range row {
			if i < len(headers) {
				record[headers[i]] = val
			}
		}
		result = append(result, record)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(output))
	return nil
}

func runTransformJSONToCSV(cmd *cobra.Command, args []string) error {
	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data []map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("empty JSON array")
	}

	// Get all keys from all objects
	keysMap := make(map[string]bool)
	for _, item := range data {
		for key := range item {
			keysMap[key] = true
		}
	}

	var keys []string
	for key := range keysMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	writer := csv.NewWriter(cmd.OutOrStdout())
	defer writer.Flush()

	// Write header
	writer.Write(keys)

	// Write rows
	for _, item := range data {
		var row []string
		for _, key := range keys {
			if val, ok := item[key]; ok {
				row = append(row, fmt.Sprintf("%v", val))
			} else {
				row = append(row, "")
			}
		}
		writer.Write(row)
	}

	return nil
}

func runTransformCase(cmd *cobra.Command, args []string) error {
	text := args[0]
	toCase := strings.ToLower(transformCaseTo)

	var result string

	switch toCase {
	case "camelcase":
		// snake_case or kebab-case to camelCase
		parts := regexp.MustCompile(`[-_\s]+`).Split(text, -1)
		if len(parts) > 0 {
			result = strings.ToLower(parts[0])
			for _, word := range parts[1:] {
				if word != "" {
					result += strings.Title(strings.ToLower(word))
				}
			}
		}

	case "pascalcase":
		// any to PascalCase
		parts := regexp.MustCompile(`[-_\s]+`).Split(text, -1)
		for _, word := range parts {
			if word != "" {
				result += strings.Title(strings.ToLower(word))
			}
		}

	case "snake_case":
		// Handle PascalCase/camelCase
		s1 := regexp.MustCompile(`(.)([ A-Z][a-z]+)`).ReplaceAllString(text, "${1}_${2}")
		s2 := regexp.MustCompile(`([a-z0-9])([A-Z])`).ReplaceAllString(s1, "${1}_${2}")
		result = regexp.MustCompile(`[-\s]+`).ReplaceAllString(s2, "_")
		result = strings.ToLower(result)

	case "kebab-case":
		// any to kebab-case
		s1 := regexp.MustCompile(`(.)([A-Z][a-z]+)`).ReplaceAllString(text, "${1}-${2}")
		s2 := regexp.MustCompile(`([a-z0-9])([A-Z])`).ReplaceAllString(s1, "${1}-${2}")
		result = regexp.MustCompile(`[_\s]+`).ReplaceAllString(s2, "-")
		result = strings.ToLower(result)

	case "uppercase":
		result = strings.ToUpper(text)

	case "lowercase":
		result = strings.ToLower(text)

	case "titlecase", "title case":
		result = strings.Title(text)

	default:
		return fmt.Errorf("unknown case type: %s (supported: camelCase, PascalCase, snake_case, kebab-case, UPPERCASE, lowercase, Title Case)", toCase)
	}

	// Build result
	var res TransformCaseResult
	if transformCaseMinimal {
		res = TransformCaseResult{I: text, O: result, TC: toCase}
	} else {
		res = TransformCaseResult{Input: text, Output: result, ToCase: toCase}
	}

	formatter := output.New(transformCaseJSON, transformCaseMinimal, cmd.OutOrStdout())
	return formatter.Print(res, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "INPUT: %s\n", text)
		fmt.Fprintf(w, "OUTPUT: %s\n", result)
	})
}

func runTransformSort(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Remove empty lines if requested
	if transformSortNoEmpty {
		var filtered []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}

	// Sort
	if transformSortReverse {
		sort.Sort(sort.Reverse(sort.StringSlice(lines)))
	} else {
		sort.Strings(lines)
	}

	// Remove duplicates if requested
	if transformSortUnique {
		seen := make(map[string]bool)
		var unique []string
		for _, line := range lines {
			if !seen[line] {
				seen[line] = true
				unique = append(unique, line)
			}
		}
		lines = unique
	}

	// Build result
	count := len(lines)
	var result TransformSortResult
	if transformSortMinimal {
		result = TransformSortResult{F: filePath, L: lines, Cnt: &count}
	} else {
		result = TransformSortResult{File: filePath, Lines: lines, Count: count}
	}

	formatter := output.New(transformSortJSON, transformSortMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		for _, line := range lines {
			fmt.Fprintln(w, line)
		}
	})
}

func runTransformFilter(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	patternStr := args[1]
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	pattern, err := regexp.Compile(patternStr)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	allLines := strings.Split(string(content), "\n")
	var filteredLines []string

	for _, line := range allLines {
		matches := pattern.MatchString(line)
		if transformFilterInvert {
			if !matches {
				filteredLines = append(filteredLines, line)
			}
		} else {
			if matches {
				filteredLines = append(filteredLines, line)
			}
		}
	}

	// Build result
	count := len(filteredLines)
	var result TransformFilterResult
	if transformFilterMin {
		result = TransformFilterResult{F: filePath, P: patternStr, L: filteredLines, Cnt: &count}
	} else {
		result = TransformFilterResult{File: filePath, Pattern: patternStr, Lines: filteredLines, Count: count}
	}

	formatter := output.New(transformFilterJSON, transformFilterMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		for _, line := range filteredLines {
			fmt.Fprintln(w, line)
		}
	})
}

func init() {
	RootCmd.AddCommand(newTransformCmd())
}
