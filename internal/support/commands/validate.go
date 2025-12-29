package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	validateJSON    bool
	validateMinimal bool
)

type validationResult struct {
	Path   string `json:"path,omitempty"`
	P      string `json:"p,omitempty"`
	Valid  bool   `json:"valid"`
	V      *bool  `json:"v,omitempty"`
	Format string `json:"format,omitempty"`
	F      string `json:"f,omitempty"`
	Error  string `json:"error,omitempty"`
	E      string `json:"e,omitempty"`
}

// ValidateResult represents the overall validation result
type ValidateResult struct {
	Results      []validationResult `json:"results,omitempty"`
	R            []validationResult `json:"r,omitempty"`
	AllValid     bool               `json:"all_valid"`
	ValidCount   int                `json:"valid_count,omitempty"`
	VC           *int               `json:"vc,omitempty"`
	InvalidCount int                `json:"invalid_count,omitempty"`
	IC           *int               `json:"ic,omitempty"`
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

	cmd.Flags().BoolVar(&validateJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&validateMinimal, "min", false, "Output in minimal/token-optimized format")

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

	// Calculate summary
	validCount := 0
	for _, r := range results {
		if r.Valid {
			validCount++
		}
	}
	invalidCount := len(results) - validCount

	// Build result struct
	var finalResult ValidateResult
	if validateMinimal {
		// Convert results to minimal format
		minResults := make([]validationResult, len(results))
		for i, r := range results {
			v := r.Valid
			minResults[i] = validationResult{
				P: r.Path,
				V: &v,
				F: r.Format,
				E: r.Error,
			}
		}
		finalResult = ValidateResult{
			R:        minResults,
			AllValid: invalidCount == 0,
			VC:       &validCount,
			IC:       &invalidCount,
		}
	} else {
		finalResult = ValidateResult{
			Results:      results,
			AllValid:     invalidCount == 0,
			ValidCount:   validCount,
			InvalidCount: invalidCount,
		}
	}

	formatter := output.New(validateJSON, validateMinimal, cmd.OutOrStdout())
	err := formatter.Print(finalResult, func(w io.Writer, data interface{}) {
		res := data.(ValidateResult)
		resultList := res.Results
		if res.R != nil {
			resultList = res.R
		}

		for _, result := range resultList {
			check := "✓"
			path := result.Path
			if result.P != "" {
				path = result.P
			}
			format := result.Format
			if result.F != "" {
				format = result.F
			}
			errMsg := result.Error
			if result.E != "" {
				errMsg = result.E
			}
			valid := result.Valid
			if result.V != nil {
				valid = *result.V
			}

			if !valid {
				check = "✗"
			}

			if valid {
				if errMsg != "" {
					fmt.Fprintf(w, "%s %s: VALID %s (⚠ %s)\n", check, path, format, errMsg)
				} else {
					fmt.Fprintf(w, "%s %s: VALID %s\n", check, path, format)
				}
			} else {
				fmt.Fprintf(w, "%s %s: INVALID %s\n", check, path, format)
				if errMsg != "" {
					fmt.Fprintf(w, "    ERROR: %s\n", errMsg)
				}
			}
		}

		fmt.Fprintln(w)
		if res.AllValid {
			fmt.Fprintln(w, "ALL_VALID: TRUE")
		} else {
			fmt.Fprintln(w, "ALL_VALID: FALSE")
		}

		vc := res.ValidCount
		if res.VC != nil {
			vc = *res.VC
		}
		ic := res.InvalidCount
		if res.IC != nil {
			ic = *res.IC
		}
		fmt.Fprintf(w, "VALID_COUNT: %d\n", vc)
		fmt.Fprintf(w, "INVALID_COUNT: %d\n", ic)
	})

	if err != nil {
		return err
	}

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
