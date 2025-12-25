package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var validatePlanJSON bool

// PlanValidationResult holds the validation results
type PlanValidationResult struct {
	Path          string            `json:"path"`
	Valid         bool              `json:"valid"`
	RequiredFiles []FileStatus      `json:"required_files"`
	OptionalFiles []FileStatus      `json:"optional_files"`
	Warnings      []string          `json:"warnings,omitempty"`
	Errors        []string          `json:"errors,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// FileStatus represents the status of a required/optional file
type FileStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// Required files for a valid plan
var requiredPlanFiles = []string{
	"plan.md",
}

var requiredPlanDirs = []string{
	"user-stories",
	"acceptance-criteria",
}

var optionalPlanFiles = []string{
	"original-request.md",
	"sprint-design.md",
	"metadata.md",
	"package-recommendations.md",
	"test-planning-matrix.md",
	"README.md",
}

var optionalPlanDirs = []string{
	"documentation",
	"tasks",
}

// newValidatePlanCmd creates the validate-plan command
func newValidatePlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-plan <plan_path>",
		Short: "Validate plan directory structure",
		Long: `Validate that a plan directory has the required structure.

Required files:
  - plan.md

Required directories:
  - user-stories/
  - acceptance-criteria/

Optional files:
  - original-request.md
  - sprint-design.md
  - metadata.md
  - README.md

Examples:
  llm-support validate-plan .planning/plans/my-plan
  llm-support validate-plan .planning/sprints/active/1.0_sprint --json`,
		Args: cobra.ExactArgs(1),
		RunE: runValidatePlan,
	}

	cmd.Flags().BoolVar(&validatePlanJSON, "json", false, "Output as JSON")

	return cmd
}

func runValidatePlan(cmd *cobra.Command, args []string) error {
	planPath := args[0]

	// Check if path exists
	info, err := os.Stat(planPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("plan directory not found: %s", planPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", planPath)
	}

	absPath, _ := filepath.Abs(planPath)

	result := PlanValidationResult{
		Path:     absPath,
		Valid:    true,
		Metadata: make(map[string]string),
	}

	// Check required files
	for _, file := range requiredPlanFiles {
		status := FileStatus{Path: file}
		fullPath := filepath.Join(planPath, file)
		if _, err := os.Stat(fullPath); err == nil {
			status.Exists = true
		} else {
			status.Exists = false
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("missing required file: %s", file))
		}
		result.RequiredFiles = append(result.RequiredFiles, status)
	}

	// Check required directories
	for _, dir := range requiredPlanDirs {
		status := FileStatus{Path: dir + "/"}
		fullPath := filepath.Join(planPath, dir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			status.Exists = true

			// Validate directory contents
			warnings := validatePlanDirectory(fullPath, dir)
			result.Warnings = append(result.Warnings, warnings...)
		} else {
			status.Exists = false
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("missing required directory: %s/", dir))
		}
		result.RequiredFiles = append(result.RequiredFiles, status)
	}

	// Check optional files
	for _, file := range optionalPlanFiles {
		status := FileStatus{Path: file}
		fullPath := filepath.Join(planPath, file)
		if _, err := os.Stat(fullPath); err == nil {
			status.Exists = true
		}
		result.OptionalFiles = append(result.OptionalFiles, status)
	}

	// Check optional directories
	for _, dir := range optionalPlanDirs {
		status := FileStatus{Path: dir + "/"}
		fullPath := filepath.Join(planPath, dir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			status.Exists = true
		}
		result.OptionalFiles = append(result.OptionalFiles, status)
	}

	// Try to extract metadata from plan.md
	planMD := filepath.Join(planPath, "plan.md")
	if content, err := os.ReadFile(planMD); err == nil {
		result.Metadata = extractPlanMetadata(string(content))
	}

	// Output
	if validatePlanJSON {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		printValidationResult(cmd, result)
	}

	// Return error if validation failed
	if !result.Valid {
		return fmt.Errorf("plan validation failed")
	}

	return nil
}

func validatePlanDirectory(path, dirType string) []string {
	var warnings []string

	files, err := os.ReadDir(path)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("cannot read %s/: %v", dirType, err))
		return warnings
	}

	if len(files) == 0 {
		warnings = append(warnings, fmt.Sprintf("%s/ is empty", dirType))
		return warnings
	}

	// Check naming conventions
	var storyPattern, acPattern *regexp.Regexp

	if dirType == "user-stories" {
		storyPattern = regexp.MustCompile(`^\d+-[\w-]+\.md$`)
	} else if dirType == "acceptance-criteria" {
		acPattern = regexp.MustCompile(`^\d+-\d+-[\w-]+\.md$`)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		if dirType == "user-stories" && storyPattern != nil && !storyPattern.MatchString(name) {
			warnings = append(warnings, fmt.Sprintf("user story file naming: expected NN-name.md, got: %s", name))
		}
		if dirType == "acceptance-criteria" && acPattern != nil && !acPattern.MatchString(name) {
			warnings = append(warnings, fmt.Sprintf("acceptance criteria file naming: expected NN-MM-name.md, got: %s", name))
		}
	}

	return warnings
}

func extractPlanMetadata(content string) map[string]string {
	metadata := make(map[string]string)

	// Try to extract YAML frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			lines := strings.Split(frontmatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if idx := strings.Index(line, ":"); idx > 0 {
					key := strings.TrimSpace(line[:idx])
					value := strings.TrimSpace(line[idx+1:])
					// Remove quotes
					value = strings.Trim(value, `"'`)
					if key != "" && value != "" {
						metadata[key] = value
					}
				}
			}
		}
	}

	// Try to extract from markdown headers
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for "**Key:** Value" patterns
		if strings.HasPrefix(line, "**") && strings.Contains(line, ":**") {
			parts := strings.SplitN(line, ":**", 2)
			if len(parts) == 2 {
				key := strings.Trim(parts[0], "* ")
				value := strings.TrimSpace(parts[1])
				if key != "" && value != "" {
					metadata[key] = value
				}
			}
		}
	}

	return metadata
}

func printValidationResult(cmd *cobra.Command, result PlanValidationResult) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "PATH: %s\n", result.Path)

	if result.Valid {
		fmt.Fprintln(out, "STATUS: ✅ VALID")
	} else {
		fmt.Fprintln(out, "STATUS: ❌ INVALID")
	}

	fmt.Fprintln(out, "---")

	// Required files
	fmt.Fprintln(out, "REQUIRED FILES:")
	for _, f := range result.RequiredFiles {
		if f.Exists {
			fmt.Fprintf(out, "  ✅ %s\n", f.Path)
		} else {
			fmt.Fprintf(out, "  ❌ %s (missing)\n", f.Path)
		}
	}

	// Optional files
	fmt.Fprintln(out, "OPTIONAL FILES:")
	for _, f := range result.OptionalFiles {
		if f.Exists {
			fmt.Fprintf(out, "  ✅ %s\n", f.Path)
		} else {
			fmt.Fprintf(out, "  ⚪ %s\n", f.Path)
		}
	}

	// Errors
	if len(result.Errors) > 0 {
		fmt.Fprintln(out, "---")
		fmt.Fprintln(out, "ERRORS:")
		for _, e := range result.Errors {
			fmt.Fprintf(out, "  ❌ %s\n", e)
		}
	}

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Fprintln(out, "---")
		fmt.Fprintln(out, "WARNINGS:")
		for _, w := range result.Warnings {
			fmt.Fprintf(out, "  ⚠️  %s\n", w)
		}
	}

	// Metadata
	if len(result.Metadata) > 0 {
		fmt.Fprintln(out, "---")
		fmt.Fprintln(out, "METADATA:")
		for k, v := range result.Metadata {
			fmt.Fprintf(out, "  %s: %s\n", k, v)
		}
	}
}

func init() {
	RootCmd.AddCommand(newValidatePlanCmd())
}
