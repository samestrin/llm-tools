package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	reportTitle   string
	reportStatus  string
	reportStats   []string
	reportOutput  string
	reportJSON    bool
	reportMinimal bool
)

// ReportResult holds the report generation result
type ReportResult struct {
	Title      string            `json:"title,omitempty"`
	Ti         string            `json:"ti,omitempty"`
	Status     string            `json:"status,omitempty"`
	S          string            `json:"s,omitempty"`
	Stats      map[string]string `json:"stats,omitempty"`
	St         map[string]string `json:"st,omitempty"`
	Timestamp  string            `json:"timestamp,omitempty"`
	TS         string            `json:"ts,omitempty"`
	Content    string            `json:"content,omitempty"`
	C          string            `json:"c,omitempty"`
	OutputFile string            `json:"output_file,omitempty"`
	OF         string            `json:"of,omitempty"`
}

// newReportCmd creates the report command
func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate formatted status report",
		Long: `Generate a formatted markdown status report with title, statistics, and status.

Status values:
  success - Operation completed successfully
  partial - Operation completed with some issues
  failed  - Operation failed

Examples:
  llm-support report --title "Build Report" --status success
  llm-support report --title "Test Results" --stat tests=50 --stat passed=48 --stat failed=2 --status partial
  llm-support report --title "Deploy" --status success -o report.md`,
		RunE: runReport,
	}

	cmd.Flags().StringVar(&reportTitle, "title", "", "Report title (required)")
	cmd.Flags().StringVar(&reportStatus, "status", "", "Report status: success, partial, failed (required)")
	cmd.Flags().StringArrayVar(&reportStats, "stat", []string{}, "Statistics in KEY=VALUE format (can be repeated)")
	cmd.Flags().StringVarP(&reportOutput, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&reportJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&reportMinimal, "min", false, "Output in minimal/token-optimized format")

	cmd.MarkFlagRequired("title")
	cmd.MarkFlagRequired("status")

	return cmd
}

func runReport(cmd *cobra.Command, args []string) error {
	// Validate required flags
	if reportTitle == "" {
		return fmt.Errorf("--title is required")
	}
	if reportStatus == "" {
		return fmt.Errorf("--status is required")
	}

	// Validate status value
	validStatuses := map[string]bool{
		"success": true,
		"partial": true,
		"failed":  true,
	}
	if !validStatuses[reportStatus] {
		return fmt.Errorf("status must be: success, partial, or failed")
	}

	// Parse statistics
	stats := make(map[string]string)
	for _, stat := range reportStats {
		parts := strings.SplitN(stat, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("stat must be in KEY=VALUE format: %s", stat)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			return fmt.Errorf("stat must be in KEY=VALUE format: %s", stat)
		}
		stats[key] = value
	}

	// Generate report
	report := generateReport(reportTitle, reportStatus, stats)
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Output to file if specified
	if reportOutput != "" {
		if err := os.WriteFile(reportOutput, []byte(report), 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}

		var result ReportResult
		if reportMinimal {
			result = ReportResult{
				Ti: reportTitle,
				S:  reportStatus,
				OF: reportOutput,
			}
		} else {
			result = ReportResult{
				Title:      reportTitle,
				Status:     reportStatus,
				OutputFile: reportOutput,
			}
		}

		formatter := output.New(reportJSON, reportMinimal, cmd.OutOrStdout())
		return formatter.Print(result, func(w io.Writer, data interface{}) {
			fmt.Fprintf(w, "Report written to: %s\n", reportOutput)
		})
	}

	// Build result for stdout
	var result ReportResult
	if reportMinimal {
		result = ReportResult{
			Ti: reportTitle,
			S:  reportStatus,
			St: stats,
			TS: timestamp,
			C:  report,
		}
	} else {
		result = ReportResult{
			Title:     reportTitle,
			Status:    reportStatus,
			Stats:     stats,
			Timestamp: timestamp,
			Content:   report,
		}
	}

	formatter := output.New(reportJSON, reportMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprint(w, report)
	})
}

func generateReport(title, status string, stats map[string]string) string {
	var sb strings.Builder

	// Escape markdown special characters in title
	escapedTitle := escapeMarkdown(title)

	// Header
	sb.WriteString(fmt.Sprintf("# %s\n\n", escapedTitle))

	// Status with emoji
	statusEmoji := getStatusEmoji(status)
	statusLabel := strings.ToUpper(status)
	sb.WriteString(fmt.Sprintf("**Status:** %s %s\n", statusEmoji, statusLabel))

	// Timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n\n", timestamp))

	// Statistics table (if any)
	if len(stats) > 0 {
		sb.WriteString("## Statistics\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|--------|-------|\n")

		// Sort keys for consistent output
		keys := make([]string, 0, len(stats))
		for k := range stats {
			keys = append(keys, k)
		}

		for _, key := range keys {
			escapedKey := escapeMarkdown(key)
			escapedValue := escapeMarkdown(stats[key])
			sb.WriteString(fmt.Sprintf("| %s | %s |\n", escapedKey, escapedValue))
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("---\n")

	return sb.String()
}

func getStatusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "partial":
		return "⚠️"
	case "failed":
		return "❌"
	default:
		return "❓"
	}
}

func escapeMarkdown(s string) string {
	// Escape markdown special characters that could break tables or formatting
	replacer := strings.NewReplacer(
		"|", "\\|",
		"*", "\\*",
		"_", "\\_",
		"`", "\\`",
		"[", "\\[",
		"]", "\\]",
	)
	return replacer.Replace(s)
}

func init() {
	RootCmd.AddCommand(newReportCmd())
}
