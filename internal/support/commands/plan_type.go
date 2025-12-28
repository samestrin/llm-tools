package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	planTypePath string
	planTypeJSON bool
	planTypeMin  bool
)

// PlanTypeResult holds the plan type information
type PlanTypeResult struct {
	Type                string `json:"type"`
	Label               string `json:"label"`
	Icon                string `json:"icon"`
	RequiresUserStories bool   `json:"requires_user_stories"`
	WorkSource          string `json:"work_source"`
}

// planTypeInfo contains metadata for all valid plan types
var planTypeInfo = map[string]PlanTypeResult{
	"feature":          {"feature", "Feature Development", "✨", true, "user-stories"},
	"bugfix":           {"bugfix", "Bug Fix", "🐛", false, "tasks"},
	"test-remediation": {"test-remediation", "Test Remediation", "🧪", false, "tasks"},
	"tech-debt":        {"tech-debt", "Technical Debt", "🔧", false, "tasks"},
	"infrastructure":   {"infrastructure", "Infrastructure", "🏗️", false, "tasks"},
}

// newPlanTypeCmd creates the plan-type command
func newPlanTypeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan-type",
		Short: "Extract plan type from planning metadata",
		Long: `Extract plan type from metadata.md or plan.md with intelligent fallback
and structured output.

The command looks for "Plan Type:" in metadata.md first, then falls back to
plan.md. If neither file contains a plan type, it defaults to "feature".

Output modes:
  Default:    Human-readable text with all metadata
  --json:     JSON object with all fields
  --min:      Just the type string (e.g., "feature")
  --json --min: Minimal JSON (e.g., {"type":"feature"})

Valid plan types: feature, bugfix, test-remediation, tech-debt, infrastructure

Examples:
  llm-support plan-type --path .planning/plans/my-plan/
  llm-support plan-type --json
  llm-support plan-type --min
  llm-support plan-type --json --min`,
		Args: cobra.NoArgs,
		RunE: runPlanType,
	}

	cmd.Flags().StringVar(&planTypePath, "path", ".", "Plan directory path")
	cmd.Flags().BoolVar(&planTypeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&planTypeMin, "min", false, "Minimal output (type only)")

	return cmd
}

func runPlanType(cmd *cobra.Command, args []string) error {
	// Resolve and validate path
	path, err := resolvePlanPath(planTypePath)
	if err != nil {
		return err
	}

	// Extract plan type from files
	planType, warning, err := extractPlanType(path)
	if err != nil {
		return err
	}

	// Output warning if not in minimal mode
	if warning != "" && !planTypeMin {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}

	// Normalize and validate the plan type
	normalizedType := normalizePlanType(planType)
	info, ok := planTypeInfo[normalizedType]
	if !ok {
		validTypes := getValidPlanTypes()
		return fmt.Errorf("Invalid plan type '%s'. Valid types: %s", planType, strings.Join(validTypes, ", "))
	}

	// Output based on flags
	outputPlanType(cmd, info)

	return nil
}

// resolvePlanPath resolves and validates the plan directory path
func resolvePlanPath(inputPath string) (string, error) {
	// Handle empty path
	if inputPath == "" {
		inputPath = "."
	}

	// Expand home directory
	if strings.HasPrefix(inputPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to expand home directory: %w", err)
		}
		inputPath = filepath.Join(home, inputPath[2:])
	}

	// Clean and resolve to absolute path
	cleanPath := filepath.Clean(inputPath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("Directory does not exist: %s", absPath)
	}
	if err != nil {
		return "", fmt.Errorf("unable to access path: %w", err)
	}

	// Check if path is a directory
	if !info.IsDir() {
		return "", fmt.Errorf("Path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// extractPlanType extracts the plan type from metadata.md or plan.md
func extractPlanType(dir string) (planType string, warning string, err error) {
	// Try metadata.md first
	metadataPath := filepath.Join(dir, "metadata.md")
	planType, err = extractTypeFromFile(metadataPath)
	if err == nil && planType != "" {
		return planType, "", nil
	}

	// Fallback to plan.md
	planMdPath := filepath.Join(dir, "plan.md")
	planType, err = extractTypeFromFile(planMdPath)
	if err == nil && planType != "" {
		return planType, "", nil
	}

	// Check if neither file exists
	_, metaErr := os.Stat(metadataPath)
	_, planErr := os.Stat(planMdPath)
	if os.IsNotExist(metaErr) && os.IsNotExist(planErr) {
		return "feature", "Warning: No metadata.md or plan.md found. Defaulting to 'feature' plan type.", nil
	}

	// Files exist but no Plan Type field found
	return "feature", "Warning: Plan Type field not found in metadata.md or plan.md. Defaulting to 'feature' plan type.", nil
}

// extractTypeFromFile extracts the Plan Type value from a file
func extractTypeFromFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	maxLines := 50 // Only scan first 50 lines for performance

	for scanner.Scan() && lineCount < maxLines {
		line := scanner.Text()
		lineCount++

		// Look for "Plan Type:" (case-insensitive)
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "plan type:") {
			// Extract value after colon
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				// Strip markdown formatting (bold, italic, etc.)
				value = stripMarkdownFormatting(value)
				if value != "" {
					return value, nil
				}
			}
		}
	}

	return "", nil
}

// stripMarkdownFormatting removes common markdown formatting characters
func stripMarkdownFormatting(s string) string {
	// Remove bold/italic markers (**text** or __text__ or *text* or _text_)
	result := s
	// Strip leading/trailing ** or __
	result = strings.TrimPrefix(result, "**")
	result = strings.TrimSuffix(result, "**")
	result = strings.TrimPrefix(result, "__")
	result = strings.TrimSuffix(result, "__")
	result = strings.TrimPrefix(result, "*")
	result = strings.TrimSuffix(result, "*")
	result = strings.TrimPrefix(result, "_")
	result = strings.TrimSuffix(result, "_")
	// Remove backticks
	result = strings.TrimPrefix(result, "`")
	result = strings.TrimSuffix(result, "`")
	// Trim any remaining whitespace
	return strings.TrimSpace(result)
}

// normalizePlanType normalizes the plan type to lowercase and standardized format
func normalizePlanType(planType string) string {
	// Trim whitespace and convert to lowercase
	normalized := strings.ToLower(strings.TrimSpace(planType))

	// Convert underscores to hyphens
	normalized = strings.ReplaceAll(normalized, "_", "-")

	return normalized
}

// getValidPlanTypes returns a list of valid plan type names
func getValidPlanTypes() []string {
	return []string{"feature", "bugfix", "test-remediation", "tech-debt", "infrastructure"}
}

// outputPlanType outputs the plan type in the requested format
func outputPlanType(cmd *cobra.Command, info PlanTypeResult) {
	out := cmd.OutOrStdout()

	if planTypeJSON && planTypeMin {
		// Compact JSON with only type
		fmt.Fprintf(out, "{\"type\":%q}\n", info.Type)
	} else if planTypeJSON {
		// Pretty-printed JSON with all fields
		output, _ := json.MarshalIndent(info, "", "  ")
		fmt.Fprintln(out, string(output))
	} else if planTypeMin {
		// Just the type string
		fmt.Fprintln(out, info.Type)
	} else {
		// Human-readable text format
		fmt.Fprintf(out, "Plan Type: %s\n", info.Type)
		fmt.Fprintf(out, "Label: %s\n", info.Label)
		fmt.Fprintf(out, "Icon: %s\n", info.Icon)
		fmt.Fprintf(out, "Requires User Stories: %v\n", info.RequiresUserStories)
		fmt.Fprintf(out, "Work Source: %s\n", info.WorkSource)
	}
}

func init() {
	RootCmd.AddCommand(newPlanTypeCmd())
}
