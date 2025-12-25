package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
)

var (
	depsType string
	depsJSON bool
)

// Dependency represents a single dependency with its version
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"` // prod, dev, optional
}

// DepsResult holds the parsed dependencies
type DepsResult struct {
	Manifest     string       `json:"manifest"`
	ManifestType string       `json:"manifest_type"`
	Dependencies []Dependency `json:"dependencies"`
}

// newDepsCmd creates the deps command
func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <manifest>",
		Short: "Parse package manifest dependencies",
		Long: `Parse and list dependencies from package manifest files.

Supported manifests:
  - package.json (Node.js)
  - go.mod (Go)
  - requirements.txt (Python)
  - Cargo.toml (Rust)
  - Gemfile (Ruby)
  - pom.xml (Maven)
  - pyproject.toml (Python)

Examples:
  llm-support deps package.json
  llm-support deps go.mod --type prod
  llm-support deps requirements.txt --json`,
		Args: cobra.ExactArgs(1),
		RunE: runDeps,
	}

	cmd.Flags().StringVar(&depsType, "type", "all", "Dependency type: all, prod, dev")
	cmd.Flags().BoolVar(&depsJSON, "json", false, "Output as JSON")

	return cmd
}

func runDeps(cmd *cobra.Command, args []string) error {
	manifest := args[0]

	// Validate type flag
	if depsType != "all" && depsType != "prod" && depsType != "dev" {
		return fmt.Errorf("type must be: all, prod, or dev")
	}

	// Check file exists
	if _, err := os.Stat(manifest); os.IsNotExist(err) {
		return fmt.Errorf("manifest file not found: %s", manifest)
	}

	// Detect manifest type and parse
	basename := filepath.Base(manifest)
	var result DepsResult
	var err error

	switch {
	case basename == "package.json":
		result, err = parsePackageJSON(manifest)
	case basename == "go.mod":
		result, err = parseGoMod(manifest)
	case basename == "requirements.txt":
		result, err = parseRequirementsTxt(manifest)
	case basename == "Cargo.toml":
		result, err = parseCargoToml(manifest)
	case basename == "Gemfile":
		result, err = parseGemfile(manifest)
	case basename == "pyproject.toml":
		result, err = parsePyprojectToml(manifest)
	case basename == "pom.xml":
		result, err = parsePomXML(manifest)
	default:
		return fmt.Errorf("unsupported manifest type: %s", basename)
	}

	if err != nil {
		return err
	}

	// Filter by type
	if depsType != "all" {
		filtered := make([]Dependency, 0)
		for _, dep := range result.Dependencies {
			if dep.Type == depsType {
				filtered = append(filtered, dep)
			}
		}
		result.Dependencies = filtered
	}

	// Output
	if depsJSON {
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "MANIFEST: %s\n", result.Manifest)
		fmt.Fprintf(cmd.OutOrStdout(), "TYPE: %s\n", result.ManifestType)
		fmt.Fprintf(cmd.OutOrStdout(), "DEPENDENCIES: %d\n", len(result.Dependencies))
		fmt.Fprintln(cmd.OutOrStdout(), "---")
		for _, dep := range result.Dependencies {
			if dep.Version != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", dep.Name, dep.Version, dep.Type)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", dep.Name, dep.Type)
			}
		}
	}

	return nil
}

func parsePackageJSON(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "package.json",
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return result, fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Parse dependencies
	if deps, ok := pkg["dependencies"].(map[string]interface{}); ok {
		for name, version := range deps {
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    name,
				Version: fmt.Sprintf("%v", version),
				Type:    "prod",
			})
		}
	}

	// Parse devDependencies
	if deps, ok := pkg["devDependencies"].(map[string]interface{}); ok {
		for name, version := range deps {
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    name,
				Version: fmt.Sprintf("%v", version),
				Type:    "dev",
			})
		}
	}

	// Parse optionalDependencies
	if deps, ok := pkg["optionalDependencies"].(map[string]interface{}); ok {
		for name, version := range deps {
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    name,
				Version: fmt.Sprintf("%v", version),
				Type:    "optional",
			})
		}
	}

	return result, nil
}

func parseGoMod(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "go.mod",
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	inRequire := false
	requireRe := regexp.MustCompile(`^\s*([^\s]+)\s+([^\s]+)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}

		// Single-line require
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				result.Dependencies = append(result.Dependencies, Dependency{
					Name:    parts[1],
					Version: parts[2],
					Type:    "prod",
				})
			}
			continue
		}

		// Multi-line require block
		if inRequire {
			matches := requireRe.FindStringSubmatch(line)
			if len(matches) >= 3 && !strings.HasPrefix(matches[1], "//") {
				result.Dependencies = append(result.Dependencies, Dependency{
					Name:    matches[1],
					Version: matches[2],
					Type:    "prod",
				})
			}
		}
	}

	return result, nil
}

func parseRequirementsTxt(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "requirements.txt",
	}

	file, err := os.Open(path)
	if err != nil {
		return result, fmt.Errorf("failed to read requirements.txt: %w", err)
	}
	defer file.Close()

	// Regex patterns for different requirement formats
	pkgRe := regexp.MustCompile(`^([a-zA-Z0-9_-]+)([<>=!~]+.+)?$`)
	editableRe := regexp.MustCompile(`^-e\s+(.+)$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip options like -r, -c, etc.
		if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "-e") {
			continue
		}

		// Handle editable installs
		if matches := editableRe.FindStringSubmatch(line); len(matches) >= 2 {
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    matches[1],
				Version: "editable",
				Type:    "prod",
			})
			continue
		}

		// Handle regular packages
		if matches := pkgRe.FindStringSubmatch(line); len(matches) >= 2 {
			version := ""
			if len(matches) >= 3 && matches[2] != "" {
				version = matches[2]
			}
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    matches[1],
				Version: version,
				Type:    "prod",
			})
		}
	}

	return result, nil
}

func parseCargoToml(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "Cargo.toml",
	}

	var cargo map[string]interface{}
	if _, err := toml.DecodeFile(path, &cargo); err != nil {
		return result, fmt.Errorf("failed to parse Cargo.toml: %w", err)
	}

	// Parse [dependencies]
	if deps, ok := cargo["dependencies"].(map[string]interface{}); ok {
		for name, value := range deps {
			version := extractCargoVersion(value)
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    name,
				Version: version,
				Type:    "prod",
			})
		}
	}

	// Parse [dev-dependencies]
	if deps, ok := cargo["dev-dependencies"].(map[string]interface{}); ok {
		for name, value := range deps {
			version := extractCargoVersion(value)
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    name,
				Version: version,
				Type:    "dev",
			})
		}
	}

	return result, nil
}

func extractCargoVersion(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if ver, ok := v["version"].(string); ok {
			return ver
		}
		if git, ok := v["git"].(string); ok {
			return "git:" + git
		}
		if path, ok := v["path"].(string); ok {
			return "path:" + path
		}
	}
	return ""
}

func parseGemfile(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "Gemfile",
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("failed to read Gemfile: %w", err)
	}

	// Parse gem 'name', 'version'
	gemRe := regexp.MustCompile(`gem\s+['"]([^'"]+)['"]\s*(?:,\s*['"]([^'"]+)['"])?`)
	groupRe := regexp.MustCompile(`group\s+:(\w+)`)

	lines := strings.Split(string(content), "\n")
	currentType := "prod"

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Track group context
		if matches := groupRe.FindStringSubmatch(line); len(matches) >= 2 {
			group := matches[1]
			if group == "development" || group == "test" {
				currentType = "dev"
			} else {
				currentType = "prod"
			}
			continue
		}

		// Reset to prod on 'end'
		if line == "end" {
			currentType = "prod"
			continue
		}

		// Parse gem declarations
		if matches := gemRe.FindStringSubmatch(line); len(matches) >= 2 {
			version := ""
			if len(matches) >= 3 {
				version = matches[2]
			}
			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    matches[1],
				Version: version,
				Type:    currentType,
			})
		}
	}

	return result, nil
}

func parsePyprojectToml(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "pyproject.toml",
	}

	var pyproject map[string]interface{}
	if _, err := toml.DecodeFile(path, &pyproject); err != nil {
		return result, fmt.Errorf("failed to parse pyproject.toml: %w", err)
	}

	// Parse [project.dependencies] (PEP 621)
	if project, ok := pyproject["project"].(map[string]interface{}); ok {
		if deps, ok := project["dependencies"].([]interface{}); ok {
			for _, dep := range deps {
				if s, ok := dep.(string); ok {
					name, version := parseRequirementSpec(s)
					result.Dependencies = append(result.Dependencies, Dependency{
						Name:    name,
						Version: version,
						Type:    "prod",
					})
				}
			}
		}

		// Parse [project.optional-dependencies]
		if optDeps, ok := project["optional-dependencies"].(map[string]interface{}); ok {
			for group, deps := range optDeps {
				depType := "optional"
				if group == "dev" || group == "test" || group == "testing" {
					depType = "dev"
				}
				if depList, ok := deps.([]interface{}); ok {
					for _, dep := range depList {
						if s, ok := dep.(string); ok {
							name, version := parseRequirementSpec(s)
							result.Dependencies = append(result.Dependencies, Dependency{
								Name:    name,
								Version: version,
								Type:    depType,
							})
						}
					}
				}
			}
		}
	}

	// Parse [tool.poetry.dependencies] (Poetry)
	if tool, ok := pyproject["tool"].(map[string]interface{}); ok {
		if poetry, ok := tool["poetry"].(map[string]interface{}); ok {
			if deps, ok := poetry["dependencies"].(map[string]interface{}); ok {
				for name, value := range deps {
					if name == "python" {
						continue // Skip python version constraint
					}
					version := extractPoetryVersion(value)
					result.Dependencies = append(result.Dependencies, Dependency{
						Name:    name,
						Version: version,
						Type:    "prod",
					})
				}
			}

			// Parse [tool.poetry.dev-dependencies] or [tool.poetry.group.dev.dependencies]
			if devDeps, ok := poetry["dev-dependencies"].(map[string]interface{}); ok {
				for name, value := range devDeps {
					version := extractPoetryVersion(value)
					result.Dependencies = append(result.Dependencies, Dependency{
						Name:    name,
						Version: version,
						Type:    "dev",
					})
				}
			}
		}
	}

	return result, nil
}

func parseRequirementSpec(spec string) (name, version string) {
	// Parse "package>=1.0,<2.0" format
	re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)(.*)$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(spec))
	if len(matches) >= 2 {
		name = matches[1]
		if len(matches) >= 3 {
			version = strings.TrimSpace(matches[2])
		}
	}
	return
}

func extractPoetryVersion(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if ver, ok := v["version"].(string); ok {
			return ver
		}
		if git, ok := v["git"].(string); ok {
			return "git:" + git
		}
		if path, ok := v["path"].(string); ok {
			return "path:" + path
		}
	}
	return ""
}

func parsePomXML(path string) (DepsResult, error) {
	result := DepsResult{
		Manifest:     path,
		ManifestType: "pom.xml",
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	// Simple regex-based parsing for Maven dependencies
	// Full XML parsing would be more robust but this handles common cases
	depRe := regexp.MustCompile(`<dependency>\s*<groupId>([^<]+)</groupId>\s*<artifactId>([^<]+)</artifactId>\s*(?:<version>([^<]+)</version>)?(?:\s*<scope>([^<]+)</scope>)?`)

	matches := depRe.FindAllStringSubmatch(string(content), -1)
	for _, match := range matches {
		if len(match) >= 3 {
			name := match[1] + ":" + match[2]
			version := ""
			depType := "prod"

			if len(match) >= 4 {
				version = match[3]
			}
			if len(match) >= 5 && match[4] != "" {
				scope := match[4]
				if scope == "test" || scope == "provided" {
					depType = "dev"
				}
			}

			result.Dependencies = append(result.Dependencies, Dependency{
				Name:    name,
				Version: version,
				Type:    depType,
			})
		}
	}

	return result, nil
}

func init() {
	RootCmd.AddCommand(newDepsCmd())
}
