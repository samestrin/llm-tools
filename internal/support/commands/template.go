package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	templateData   string
	templateVars   []string
	templateEnv    bool
	templateStrict bool
	templateSyntax string
	templateOutput string
	templateStrip  bool
)

// newTemplateCmd creates the template command
func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template <file>",
		Short: "Variable substitution in templates",
		Long: `Perform variable substitution in template files.

Supports {{variable}} or [[variable]] syntax with optional defaults.
Example: {{name|Guest}} uses "Guest" if name is not defined.`,
		Args: cobra.ExactArgs(1),
		RunE: runTemplate,
	}

	cmd.Flags().StringVar(&templateData, "data", "", "JSON file with variable values")
	cmd.Flags().StringArrayVar(&templateVars, "var", nil, "Variable in KEY=VALUE format")
	cmd.Flags().BoolVar(&templateEnv, "env", false, "Include environment variables")
	cmd.Flags().BoolVar(&templateStrict, "strict", false, "Error on undefined variables")
	cmd.Flags().StringVar(&templateSyntax, "syntax", "braces", "Syntax: braces ({{var}}) or brackets ([[var]])")
	cmd.Flags().StringVarP(&templateOutput, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().BoolVar(&templateStrip, "strip", false, "Strip whitespace from file values")

	return cmd
}

func runTemplate(cmd *cobra.Command, args []string) error {
	// Read template
	var template string
	if args[0] == "-" {
		// Read from stdin (not implemented for simplicity)
		return fmt.Errorf("stdin not supported, use file path")
	}

	content, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}
	template = string(content)

	// Collect variables
	variables := make(map[string]string)

	// From JSON file (lowest priority)
	if templateData != "" {
		data, err := os.ReadFile(templateData)
		if err != nil {
			return fmt.Errorf("failed to read data file: %w", err)
		}
		var jsonData map[string]interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return fmt.Errorf("invalid JSON in data file: %w", err)
		}
		for k, v := range jsonData {
			variables[k] = fmt.Sprintf("%v", v)
		}
	}

	// From environment (medium priority)
	if templateEnv {
		for _, env := range os.Environ() {
			if idx := strings.Index(env, "="); idx != -1 {
				variables[env[:idx]] = env[idx+1:]
			}
		}
	}

	// From command line (highest priority)
	for _, v := range templateVars {
		if idx := strings.Index(v, "="); idx != -1 {
			key := v[:idx]
			value := v[idx+1:]

			// Check if value is a file reference (@path)
			if strings.HasPrefix(value, "@") {
				filePath := value[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file for variable %s: %w", key, err)
				}
				value = string(fileContent)
				if templateStrip {
					value = strings.TrimSpace(value)
				}
			}
			variables[key] = value
		}
	}

	// Replace function
	replaceVar := func(match string, varContent string) string {
		varName := varContent
		defaultValue := ""
		hasDefault := false

		// Check for default value syntax: var|default
		if idx := strings.Index(varContent, "|"); idx != -1 {
			varName = strings.TrimSpace(varContent[:idx])
			defaultValue = strings.TrimSpace(varContent[idx+1:])
			hasDefault = true
		} else {
			varName = strings.TrimSpace(varContent)
		}

		if val, ok := variables[varName]; ok {
			return val
		} else if hasDefault {
			return defaultValue
		} else if templateStrict {
			fmt.Fprintf(cmd.ErrOrStderr(), "ERROR: Undefined variable: %s\n", varName)
			return match // Keep original
		}
		return match // Keep placeholder if not found
	}

	// Replace variables based on syntax
	var result string
	if templateSyntax == "brackets" {
		pattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
		result = pattern.ReplaceAllStringFunc(template, func(match string) string {
			content := match[2 : len(match)-2] // Remove [[ and ]]
			return replaceVar(match, content)
		})
	} else {
		pattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
		result = pattern.ReplaceAllStringFunc(template, func(match string) string {
			content := match[2 : len(match)-2] // Remove {{ and }}
			return replaceVar(match, content)
		})
	}

	// Output
	if templateOutput != "" {
		if err := os.WriteFile(templateOutput, []byte(result), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ Template written to: %s\n", templateOutput)
	} else {
		fmt.Fprint(cmd.OutOrStdout(), result)
	}

	return nil
}

func init() {
	RootCmd.AddCommand(newTemplateCmd())
}
