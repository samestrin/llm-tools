package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	routeTDFile         string
	routeTDContent      string
	routeTDQuickWinsMax int
	routeTDBacklogMax   int
	routeTDJSON         bool
	routeTDMinimal      bool
)

// Default threshold constants
const (
	defaultQuickWinsMax = 30   // Issues < 30 min go to quick_wins
	defaultBacklogMax   = 2880 // Issues >= 2880 min (2 days) go to td_files
)

// RouteTDInput represents the expected input structure
type RouteTDInput struct {
	Rows []map[string]interface{} `json:"rows"`
}

// RouteTDSummary provides routing statistics
type RouteTDSummary struct {
	TotalInput     int `json:"total_input"`
	TotalRouted    int `json:"total_routed"`
	QuickWinsCount int `json:"quick_wins_count"`
	BacklogCount   int `json:"backlog_count"`
	TDFilesCount   int `json:"td_files_count"`
}

// RouteTDResult holds the routing results
type RouteTDResult struct {
	QuickWins   []map[string]interface{} `json:"quick_wins"`
	Backlog     []map[string]interface{} `json:"backlog"`
	TDFiles     []map[string]interface{} `json:"td_files"`
	Summary     RouteTDSummary           `json:"routing_summary"`
	ParseErrors []ParseError             `json:"parse_errors,omitempty"`
}

// newRouteTDCmd creates the route-td command
func newRouteTDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route-td",
		Short: "Route technical debt issues by effort estimate",
		Long: `Route parsed technical debt issues to appropriate destinations based on EST_MINUTES.

Routing thresholds (configurable):
  quick_wins:  EST_MINUTES < 30 (quick fixes for README.md)
  backlog:     EST_MINUTES 30-2879 (medium effort for README.md backlog)
  td_files:    EST_MINUTES >= 2880 (sprint-sized work for td-##-*.md files)

Input: JSON with "rows" array from parse_stream or similar
Output: JSON with quick_wins, backlog, td_files arrays and routing_summary

Examples:
  llm-support route-td --file parsed_issues.json
  llm-support parse-stream --file issues.txt | llm-support route-td
  llm-support route-td --content '{"rows": [{"EST_MINUTES": 15}]}'`,
		RunE: runRouteTD,
	}

	cmd.Flags().StringVar(&routeTDFile, "file", "", "Input JSON file path")
	cmd.Flags().StringVar(&routeTDContent, "content", "", "Direct JSON content input")
	cmd.Flags().IntVar(&routeTDQuickWinsMax, "quick-wins-max", defaultQuickWinsMax, "Max minutes for quick_wins (default: 30)")
	cmd.Flags().IntVar(&routeTDBacklogMax, "backlog-max", defaultBacklogMax, "Min minutes for td_files (default: 2880)")
	cmd.Flags().BoolVar(&routeTDJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&routeTDMinimal, "min", false, "Minimal output format")

	return cmd
}

func runRouteTD(cmd *cobra.Command, args []string) error {
	// Validate thresholds
	if routeTDQuickWinsMax < 0 {
		return fmt.Errorf("quick-wins-max must be non-negative, got: %d", routeTDQuickWinsMax)
	}
	if routeTDBacklogMax < 0 {
		return fmt.Errorf("backlog-max must be non-negative, got: %d", routeTDBacklogMax)
	}
	if routeTDQuickWinsMax >= routeTDBacklogMax {
		return fmt.Errorf("quick-wins-max (%d) must be less than backlog-max (%d)", routeTDQuickWinsMax, routeTDBacklogMax)
	}

	// Get input content
	content, err := getRouteTDInput(cmd)
	if err != nil {
		return err
	}

	// Parse input JSON
	var input RouteTDInput
	if err := json.Unmarshal([]byte(content), &input); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}

	// Route the issues
	result := routeIssues(input.Rows, routeTDQuickWinsMax, routeTDBacklogMax)

	// Validate zero data loss
	totalRouted := len(result.QuickWins) + len(result.Backlog) + len(result.TDFiles)
	if totalRouted != len(input.Rows) {
		return fmt.Errorf("FATAL: Data loss detected - input:%d routed:%d - this is a bug, please report",
			len(input.Rows), totalRouted)
	}

	formatter := output.New(routeTDJSON, routeTDMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(RouteTDResult)
		fmt.Fprintf(w, "ROUTING_SUMMARY:\n")
		fmt.Fprintf(w, "  Total Input: %d\n", r.Summary.TotalInput)
		fmt.Fprintf(w, "  Quick Wins: %d\n", r.Summary.QuickWinsCount)
		fmt.Fprintf(w, "  Backlog: %d\n", r.Summary.BacklogCount)
		fmt.Fprintf(w, "  TD Files: %d\n", r.Summary.TDFilesCount)
		if len(r.ParseErrors) > 0 {
			fmt.Fprintf(w, "  Parse Warnings: %d\n", len(r.ParseErrors))
		}
	})
}

func getRouteTDInput(cmd *cobra.Command) (string, error) {
	// Priority: --content flag, then --file flag, then stdin
	if routeTDContent != "" {
		return routeTDContent, nil
	}

	if routeTDFile != "" {
		data, err := os.ReadFile(routeTDFile)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("file not found: %s", routeTDFile)
			}
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		return string(data), nil
	}

	// Try to read from stdin if available
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		if len(data) > 0 {
			return string(data), nil
		}
	}

	return "", fmt.Errorf("no input provided: specify --file, --content, or pipe JSON to stdin")
}

func routeIssues(rows []map[string]interface{}, quickWinsMax, backlogMax int) RouteTDResult {
	result := RouteTDResult{
		QuickWins:   []map[string]interface{}{},
		Backlog:     []map[string]interface{}{},
		TDFiles:     []map[string]interface{}{},
		ParseErrors: []ParseError{},
	}

	for i, row := range rows {
		estMinutes, err := extractEstMinutes(row)
		if err != nil {
			// Default to backlog if EST_MINUTES is missing or invalid
			result.ParseErrors = append(result.ParseErrors, ParseError{
				Line:    i + 1,
				Message: err.Error(),
			})
			result.Backlog = append(result.Backlog, row)
			continue
		}

		// Route based on thresholds
		if estMinutes < float64(quickWinsMax) {
			result.QuickWins = append(result.QuickWins, row)
		} else if estMinutes < float64(backlogMax) {
			result.Backlog = append(result.Backlog, row)
		} else {
			result.TDFiles = append(result.TDFiles, row)
		}
	}

	// Set summary
	result.Summary = RouteTDSummary{
		TotalInput:     len(rows),
		TotalRouted:    len(result.QuickWins) + len(result.Backlog) + len(result.TDFiles),
		QuickWinsCount: len(result.QuickWins),
		BacklogCount:   len(result.Backlog),
		TDFilesCount:   len(result.TDFiles),
	}

	return result
}

func extractEstMinutes(row map[string]interface{}) (float64, error) {
	val, ok := row["EST_MINUTES"]
	if !ok {
		return 0, fmt.Errorf("missing EST_MINUTES field")
	}

	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		// Try to parse as float
		s := strings.TrimSpace(v)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, nil
		}
		// Try to parse as int
		if i, err := strconv.Atoi(s); err == nil {
			return float64(i), nil
		}
		return 0, fmt.Errorf("invalid EST_MINUTES value '%s': not a number", v)
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, nil
		}
		return 0, fmt.Errorf("invalid EST_MINUTES json.Number: %s", v)
	default:
		return 0, fmt.Errorf("invalid EST_MINUTES type: %T", val)
	}
}

func init() {
	RootCmd.AddCommand(newRouteTDCmd())
}
