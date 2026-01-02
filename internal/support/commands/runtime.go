package commands

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	runtimeStart     int64
	runtimeEnd       int64
	runtimeFormat    string
	runtimePrecision int
	runtimeLabel     bool
	runtimeRaw       bool
	runtimeJSON      bool
	runtimeMinimal   bool
)

// RuntimeResult holds the runtime command result
type RuntimeResult struct {
	// Full output fields
	Start       int64   `json:"start,omitempty"`
	End         int64   `json:"end,omitempty"`
	ElapsedSecs float64 `json:"elapsed_secs,omitempty"`
	Formatted   string  `json:"formatted,omitempty"`

	// Minimal output fields
	S   int64   `json:"s,omitempty"`   // start
	E   int64   `json:"e,omitempty"`   // end
	ES  float64 `json:"es,omitempty"`  // elapsed_secs
	F   string  `json:"f,omitempty"`   // formatted
	Raw float64 `json:"raw,omitempty"` // raw value (when --raw)
}

// newRuntimeCmd creates the runtime command
func newRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Calculate and format elapsed time from epoch timestamps",
		Long: `Calculate elapsed time between two epoch timestamps and format the result.

Useful for tracking execution time in scripts and LLM prompts.

Format options:
  secs       Raw seconds (e.g., "127.5s")
  mins       Raw minutes (e.g., "2.1m")
  mins-secs  Minutes and seconds breakdown (e.g., "2m 7.5s")
  hms        Hours, minutes, seconds (e.g., "1h 23m 45.0s")
  human      Auto-scaled based on magnitude (default)
  compact    No spaces (e.g., "2m7s")

Examples:
  # Basic usage (calculates from start to now)
  llm-support runtime --start 1735800000

  # Calculate between two timestamps
  llm-support runtime --start 1735800000 --end 1735800127

  # Get raw seconds with no formatting
  llm-support runtime --start 1735800000 --raw --format secs

  # JSON output for programmatic use
  llm-support runtime --start 1735800000 --json`,
		RunE: runRuntime,
	}

	cmd.Flags().Int64Var(&runtimeStart, "start", 0, "Start epoch timestamp (required)")
	cmd.Flags().Int64Var(&runtimeEnd, "end", 0, "End epoch timestamp (default: now)")
	cmd.Flags().StringVar(&runtimeFormat, "format", "human", "Output format: secs, mins, mins-secs, hms, human, compact")
	cmd.Flags().IntVar(&runtimePrecision, "precision", 1, "Decimal precision for output")
	cmd.Flags().BoolVar(&runtimeLabel, "label", false, "Include 'Runtime: ' prefix")
	cmd.Flags().BoolVar(&runtimeRaw, "raw", false, "Output raw number without unit suffix")
	cmd.Flags().BoolVar(&runtimeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&runtimeMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.MarkFlagRequired("start")

	return cmd
}

func runRuntime(cmd *cobra.Command, args []string) error {
	start := runtimeStart
	end := runtimeEnd

	// Default end to now if not specified
	if end == 0 {
		end = time.Now().Unix()
	}

	// Validate start time
	if start <= 0 {
		return fmt.Errorf("invalid start timestamp: %d", start)
	}

	// Calculate elapsed seconds
	elapsedSecs := float64(end - start)

	// Format the result
	formatted := formatDuration(elapsedSecs, runtimeFormat, runtimePrecision, runtimeRaw)

	// Add label if requested
	if runtimeLabel && !runtimeRaw {
		formatted = "Runtime: " + formatted
	}

	// Build result
	var result RuntimeResult
	if runtimeMinimal {
		result = RuntimeResult{
			S:  start,
			E:  end,
			ES: elapsedSecs,
			F:  formatted,
		}
		if runtimeRaw {
			result.Raw = getRawValue(elapsedSecs, runtimeFormat)
		}
	} else {
		result = RuntimeResult{
			Start:       start,
			End:         end,
			ElapsedSecs: elapsedSecs,
			Formatted:   formatted,
		}
	}

	// Output
	formatter := output.New(runtimeJSON, runtimeMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintln(w, formatted)
	})
}

// formatDuration formats elapsed seconds according to the specified format
func formatDuration(secs float64, format string, precision int, raw bool) string {
	// Handle negative durations
	negative := secs < 0
	if negative {
		secs = -secs
	}
	prefix := ""
	if negative {
		prefix = "-"
	}

	switch format {
	case "secs":
		if raw {
			return prefix + formatFloat(secs, precision)
		}
		return prefix + formatFloat(secs, precision) + "s"

	case "mins":
		mins := secs / 60
		if raw {
			return prefix + formatFloat(mins, precision)
		}
		return prefix + formatFloat(mins, precision) + "m"

	case "mins-secs":
		mins := int(secs / 60)
		remainSecs := math.Mod(secs, 60)
		if raw {
			return prefix + formatFloat(secs, precision)
		}
		if mins == 0 {
			return prefix + formatFloat(remainSecs, precision) + "s"
		}
		return prefix + fmt.Sprintf("%dm %s", mins, formatFloat(remainSecs, precision)+"s")

	case "hms":
		hours := int(secs / 3600)
		mins := int(math.Mod(secs, 3600) / 60)
		remainSecs := math.Mod(secs, 60)
		if raw {
			return prefix + formatFloat(secs, precision)
		}
		if hours == 0 && mins == 0 {
			return prefix + formatFloat(remainSecs, precision) + "s"
		}
		if hours == 0 {
			return prefix + fmt.Sprintf("%dm %s", mins, formatFloat(remainSecs, precision)+"s")
		}
		return prefix + fmt.Sprintf("%dh %dm %s", hours, mins, formatFloat(remainSecs, precision)+"s")

	case "compact":
		hours := int(secs / 3600)
		mins := int(math.Mod(secs, 3600) / 60)
		remainSecs := math.Mod(secs, 60)
		if raw {
			return prefix + formatFloat(secs, precision)
		}
		if hours == 0 && mins == 0 {
			return prefix + formatFloat(remainSecs, precision) + "s"
		}
		if hours == 0 {
			return prefix + fmt.Sprintf("%dm%s", mins, formatFloat(remainSecs, precision)+"s")
		}
		return prefix + fmt.Sprintf("%dh%dm%s", hours, mins, formatFloat(remainSecs, precision)+"s")

	case "human":
		fallthrough
	default:
		return prefix + formatHuman(secs, precision, raw)
	}
}

// formatHuman auto-scales the output based on magnitude
func formatHuman(secs float64, precision int, raw bool) string {
	switch {
	case secs < 60:
		// Less than 1 minute: show seconds
		if raw {
			return formatFloat(secs, precision)
		}
		return formatFloat(secs, precision) + "s"

	case secs < 3600:
		// Less than 1 hour: show minutes and seconds
		mins := int(secs / 60)
		remainSecs := math.Mod(secs, 60)
		if raw {
			return formatFloat(secs/60, precision)
		}
		if remainSecs < 0.5 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm %s", mins, formatFloat(remainSecs, precision)+"s")

	case secs < 86400:
		// Less than 1 day: show hours and minutes
		hours := int(secs / 3600)
		mins := int(math.Mod(secs, 3600) / 60)
		if raw {
			return formatFloat(secs/3600, precision)
		}
		if mins == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh %dm", hours, mins)

	default:
		// 1 day or more: show days and hours
		days := int(secs / 86400)
		hours := int(math.Mod(secs, 86400) / 3600)
		if raw {
			return formatFloat(secs/86400, precision)
		}
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

// formatFloat formats a float with the given precision
func formatFloat(val float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, val)
}

// getRawValue returns the raw numeric value based on format
func getRawValue(secs float64, format string) float64 {
	switch format {
	case "mins":
		return secs / 60
	case "human":
		// For human format, determine appropriate unit
		switch {
		case secs < 60:
			return secs
		case secs < 3600:
			return secs / 60
		case secs < 86400:
			return secs / 3600
		default:
			return secs / 86400
		}
	default:
		return secs
	}
}

func init() {
	RootCmd.AddCommand(newRuntimeCmd())
}
