package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

type validationResult struct {
	Path   string `json:"path"`
	Valid  bool   `json:"valid"`
	Format string `json:"format"`
	Error  string `json:"error,omitempty"`
}

// newValidateCmd creates the validate command
func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <files...>",
		Short: "Validate files of multiple formats",
		Long: `Validate files of various formats including JSON, TOML, YAML, CSV, and Markdown.

Supported formats:
  .json      - JSON syntax validation
  .toml      - TOML syntax validation
  .yml/.yaml - YAML structure check
  .csv       - CSV structure validation
  .md        - Markdown non-empty check`,
		Args: cobra.MinimumNArgs(1),
		RunE: runValidate,
	}

	return cmd
}

func runValidate(cmd *cobra.Command, args []string) error {
	var results []validationResult
	hasError := false

	for _, filePath := range args {
		result := validateFile(filePath)
		results = append(results, result)
		if !result.Valid {
			hasError = true
		}
	}

	// Print results
	for _, result := range results {
		check := "✓"
		if !result.Valid {
			check = "✗"
		}

		if result.Valid {
			if result.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: VALID %s (⚠ %s)\n", check, result.Path, result.Format, result.Error)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: VALID %s\n", check, result.Path, result.Format)
			}
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s %s: INVALID %s\n", check, result.Path, result.Format)
			if result.Error != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "    ERROR: %s\n", result.Error)
			}
		}
	}

	// Print summary
	validCount := 0
	for _, r := range results {
		if r.Valid {
			validCount++
		}
	}
	invalidCount := len(results) - validCount

	fmt.Fprintln(cmd.OutOrStdout())
	if invalidCount == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "ALL_VALID: TRUE")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "ALL_VALID: FALSE")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "VALID_COUNT: %d\n", validCount)
	fmt.Fprintf(cmd.OutOrStdout(), "INVALID_COUNT: %d\n", invalidCount)

	if hasError {
		return fmt.Errorf("validation failed for %d file(s)", invalidCount)
	}

	return nil
}

func validateFile(filePath string) validationResult {
	result := validationResult{
		Path:   filePath,
		Valid:  false,
		Format: "UNKNOWN",
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		result.Error = "File not found"
		return result
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		result.Format = "JSON"
		content, err := os.ReadFile(filePath)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		var data interface{}
		if err := json.Unmarshal(content, &data); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Valid = true

	case ".md", ".markdown":
		result.Format = "MARKDOWN"
		content, err := os.ReadFile(filePath)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			result.Error = "Empty file"
			return result
		}
		result.Valid = true

	case ".toml":
		result.Format = "TOML"
		content, err := os.ReadFile(filePath)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		var data map[string]interface{}
		if _, err := toml.Decode(string(content), &data); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Valid = true

	case ".yml", ".yaml":
		result.Format = "YAML"
		content, err := os.ReadFile(filePath)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			result.Error = "Empty file"
			return result
		}
		var data interface{}
		if err := yaml.Unmarshal(content, &data); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Valid = true

	case ".csv":
		result.Format = "CSV"
		file, err := os.Open(filePath)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		defer file.Close()
		reader := csv.NewReader(file)
		if _, err := reader.ReadAll(); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Valid = true

	default:
		result.Error = "Unsupported format (supported: .json, .md, .toml, .yml, .csv)"
	}

	return result
}

func init() {
	RootCmd.AddCommand(newValidateCmd())
}
