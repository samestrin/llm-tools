package commands

import (
	"fmt"
	"io"
	"sort"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	pcFile    string
	pcJSON    bool
	pcMinimal bool
)

// ComponentTesting holds resolved testing commands for a component.
type ComponentTesting struct {
	Cmd              string      `json:"cmd,omitempty"`
	CoverageCmd      string      `json:"coverage_cmd,omitempty"`
	ChangedCmd       string      `json:"changed_cmd,omitempty"`
	Runner           string      `json:"runner,omitempty"`
	CoverageBaseline interface{} `json:"coverage_baseline,omitempty"`
}

// ComponentCommands holds resolved build/lint/etc commands for a component.
type ComponentCommands struct {
	Lint   string `json:"lint,omitempty"`
	Build  string `json:"build,omitempty"`
	Types  string `json:"types,omitempty"`
	Format string `json:"format,omitempty"`
	Start  string `json:"start,omitempty"`
}

// ComponentResult represents a single project component.
type ComponentResult struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Framework       string            `json:"framework"`
	PackageManager  string            `json:"package_manager"`
	SourceDirectory string            `json:"source_directory"`
	Testing         ComponentTesting  `json:"testing"`
	Commands        ComponentCommands `json:"commands"`
}

// ProjectComponentsResult is the top-level response.
type ProjectComponentsResult struct {
	Components     []ComponentResult `json:"components"`
	IsMonorepo     bool              `json:"is_monorepo"`
	ComponentCount int               `json:"component_count"`
}

func newProjectComponentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project-components",
		Short: "Read project config and return normalized component list",
		Long: `Read config.yaml and return a normalized list of project components
with resolved testing and commands. Handles both single-project (flat)
and monorepo (nested) config shapes.

Flat config (project.type exists):
  Returns single component named "default"

Nested config (project.<name>.type exists):
  Returns N components keyed by name

Testing/commands resolution per component:
  1. Check testing.<component>.cmd (per-component override)
  2. Fallback to testing.cmd (flat/shared)`,
		Args: cobra.NoArgs,
		RunE: runProjectComponents,
	}

	cmd.Flags().StringVar(&pcFile, "file", ".planning/.config/config.yaml", "Path to config.yaml")
	cmd.Flags().BoolVar(&pcJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&pcMinimal, "min", false, "Minimal output")

	return cmd
}

func runProjectComponents(cmd *cobra.Command, args []string) error {
	data, err := readYAMLAsMap(pcFile)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	result := buildProjectComponents(data)

	formatter := output.New(pcJSON, pcMinimal, cmd.OutOrStdout())
	return formatter.Print(result, printProjectComponentsText)
}

func buildProjectComponents(data map[string]interface{}) ProjectComponentsResult {
	projectRaw, ok := getValueAtPath(data, "project")
	if !ok {
		return ProjectComponentsResult{
			Components:     []ComponentResult{},
			IsMonorepo:     false,
			ComponentCount: 0,
		}
	}

	projectMap, ok := projectRaw.(map[string]interface{})
	if !ok {
		return ProjectComponentsResult{
			Components:     []ComponentResult{},
			IsMonorepo:     false,
			ComponentCount: 0,
		}
	}

	// Detect shape: if "type" key exists as a string, it's flat
	if typeVal, hasType := projectMap["type"]; hasType {
		if _, isStr := typeVal.(string); isStr {
			comp := buildFlatComponent(projectMap, data)
			return ProjectComponentsResult{
				Components:     []ComponentResult{comp},
				IsMonorepo:     false,
				ComponentCount: 1,
			}
		}
	}

	// Nested: iterate children looking for maps with "type" key
	var components []ComponentResult
	for name, val := range projectMap {
		childMap, isMap := val.(map[string]interface{})
		if !isMap {
			continue
		}
		if _, hasType := childMap["type"]; !hasType {
			continue
		}
		comp := buildNestedComponent(name, childMap, data)
		components = append(components, comp)
	}

	// Sort by name for deterministic output
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	return ProjectComponentsResult{
		Components:     components,
		IsMonorepo:     len(components) > 1,
		ComponentCount: len(components),
	}
}

func buildFlatComponent(projectMap, data map[string]interface{}) ComponentResult {
	comp := ComponentResult{
		Name:            "default",
		Type:            stringVal(projectMap, "type"),
		Framework:       stringVal(projectMap, "framework"),
		PackageManager:  stringVal(projectMap, "package_manager"),
		SourceDirectory: stringVal(projectMap, "source_directory"),
	}

	// For flat config, testing comes from testing.* directly
	comp.Testing = resolveTesting("", data)
	comp.Commands = resolveCommands("", data)

	return comp
}

func buildNestedComponent(name string, childMap map[string]interface{}, data map[string]interface{}) ComponentResult {
	comp := ComponentResult{
		Name:            name,
		Type:            stringVal(childMap, "type"),
		Framework:       stringVal(childMap, "framework"),
		PackageManager:  stringVal(childMap, "package_manager"),
		SourceDirectory: stringVal(childMap, "source_directory"),
	}

	// For nested config, try testing.<name>.* first, fallback to testing.*
	comp.Testing = resolveTesting(name, data)
	comp.Commands = resolveCommands(name, data)

	return comp
}

// resolveTesting resolves testing config for a component.
// If componentName is empty, reads flat testing.* keys.
// Otherwise tries testing.<componentName>.* first, falls back to testing.*.
func resolveTesting(componentName string, data map[string]interface{}) ComponentTesting {
	t := ComponentTesting{}

	if componentName != "" {
		// Try per-component first
		prefix := "testing." + componentName + "."
		if v := getStringPath(data, prefix+"cmd"); v != "" {
			t.Cmd = v
		}
		if v := getStringPath(data, prefix+"coverage_cmd"); v != "" {
			t.CoverageCmd = v
		}
		if v := getStringPath(data, prefix+"changed_cmd"); v != "" {
			t.ChangedCmd = v
		}
		if v := getStringPath(data, prefix+"runner"); v != "" {
			t.Runner = v
		}
		if v, ok := getValueAtPath(data, prefix+"coverage_baseline"); ok {
			t.CoverageBaseline = v
		}
	}

	// Fallback to flat testing.* for any empty fields
	if t.Cmd == "" {
		t.Cmd = getStringPath(data, "testing.cmd")
	}
	if t.CoverageCmd == "" {
		t.CoverageCmd = getStringPath(data, "testing.coverage_cmd")
	}
	if t.ChangedCmd == "" {
		t.ChangedCmd = getStringPath(data, "testing.changed_cmd")
	}
	if t.Runner == "" {
		t.Runner = getStringPath(data, "testing.runner")
	}
	if t.CoverageBaseline == nil {
		if v, ok := getValueAtPath(data, "testing.coverage_baseline"); ok {
			t.CoverageBaseline = v
		}
	}

	return t
}

// resolveCommands resolves commands config for a component.
func resolveCommands(componentName string, data map[string]interface{}) ComponentCommands {
	c := ComponentCommands{}

	if componentName != "" {
		prefix := "commands." + componentName + "."
		if v := getStringPath(data, prefix+"lint"); v != "" {
			c.Lint = v
		}
		if v := getStringPath(data, prefix+"build"); v != "" {
			c.Build = v
		}
		if v := getStringPath(data, prefix+"types"); v != "" {
			c.Types = v
		}
		if v := getStringPath(data, prefix+"format"); v != "" {
			c.Format = v
		}
		if v := getStringPath(data, prefix+"start"); v != "" {
			c.Start = v
		}
	}

	// Fallback to flat commands.*
	if c.Lint == "" {
		c.Lint = getStringPath(data, "commands.lint")
	}
	if c.Build == "" {
		c.Build = getStringPath(data, "commands.build")
	}
	if c.Types == "" {
		c.Types = getStringPath(data, "commands.types")
	}
	if c.Format == "" {
		c.Format = getStringPath(data, "commands.format")
	}
	if c.Start == "" {
		c.Start = getStringPath(data, "commands.start")
	}

	return c
}

// getStringPath is a convenience wrapper around getValueAtPath that returns a string.
func getStringPath(data map[string]interface{}, path string) string {
	v, ok := getValueAtPath(data, path)
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// stringVal extracts a string value from a map, returning "" if missing or wrong type.
func stringVal(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func printProjectComponentsText(w io.Writer, data interface{}) {
	r := data.(ProjectComponentsResult)
	fmt.Fprintf(w, "IS_MONOREPO: %v\n", r.IsMonorepo)
	fmt.Fprintf(w, "COMPONENT_COUNT: %d\n", r.ComponentCount)
	fmt.Fprintln(w, "---")

	for _, c := range r.Components {
		fmt.Fprintf(w, "COMPONENT: %s\n", c.Name)
		fmt.Fprintf(w, "  TYPE: %s\n", c.Type)
		fmt.Fprintf(w, "  FRAMEWORK: %s\n", c.Framework)
		fmt.Fprintf(w, "  PACKAGE_MANAGER: %s\n", c.PackageManager)
		fmt.Fprintf(w, "  SOURCE_DIRECTORY: %s\n", c.SourceDirectory)
		if c.Testing.Cmd != "" {
			fmt.Fprintf(w, "  TESTING_CMD: %s\n", c.Testing.Cmd)
		}
		if c.Testing.CoverageCmd != "" {
			fmt.Fprintf(w, "  TESTING_COVERAGE_CMD: %s\n", c.Testing.CoverageCmd)
		}
		if c.Testing.ChangedCmd != "" {
			fmt.Fprintf(w, "  TESTING_CHANGED_CMD: %s\n", c.Testing.ChangedCmd)
		}
		if c.Testing.Runner != "" {
			fmt.Fprintf(w, "  TESTING_RUNNER: %s\n", c.Testing.Runner)
		}
		if c.Testing.CoverageBaseline != nil {
			fmt.Fprintf(w, "  TESTING_COVERAGE_BASELINE: %v\n", c.Testing.CoverageBaseline)
		}
		if c.Commands.Lint != "" {
			fmt.Fprintf(w, "  COMMANDS_LINT: %s\n", c.Commands.Lint)
		}
		if c.Commands.Build != "" {
			fmt.Fprintf(w, "  COMMANDS_BUILD: %s\n", c.Commands.Build)
		}
		if c.Commands.Types != "" {
			fmt.Fprintf(w, "  COMMANDS_TYPES: %s\n", c.Commands.Types)
		}
		if c.Commands.Format != "" {
			fmt.Fprintf(w, "  COMMANDS_FORMAT: %s\n", c.Commands.Format)
		}
		if c.Commands.Start != "" {
			fmt.Fprintf(w, "  COMMANDS_START: %s\n", c.Commands.Start)
		}
		fmt.Fprintln(w, "---")
	}
}

func init() {
	RootCmd.AddCommand(newProjectComponentsCmd())
}
