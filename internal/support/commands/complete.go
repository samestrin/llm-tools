package commands

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/pkg/llmapi"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	completePrompt      string
	completeFile        string
	completeTemplate    string
	completeVars        []string
	completeSystem      string
	completeModel       string
	completeTemperature float64
	completeMaxTokens   int
	completeTimeout     int
	completeRetries     int
	completeRetryDelay  int
	completeOutput      string
	completeStrip       bool
	completeJSON        bool
	completeMinimal     bool
)

// CompleteResult represents the result of an API completion
type CompleteResult struct {
	Status       string `json:"status,omitempty"`
	S            string `json:"s,omitempty"`
	Attempts     int    `json:"attempts,omitempty"`
	A            *int   `json:"a,omitempty"`
	Model        string `json:"model,omitempty"`
	M            string `json:"m,omitempty"`
	OutputFile   string `json:"output_file,omitempty"`
	OF           string `json:"of,omitempty"`
	OutputLength int    `json:"output_length,omitempty"`
	OL           *int   `json:"ol,omitempty"`
	Response     string `json:"response,omitempty"`
	R            string `json:"r,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	LE           string `json:"le,omitempty"`
}

// newCompleteCmd creates the complete command
func newCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete",
		Short: "Send prompt to OpenAI-compatible API",
		Long: `Send a prompt directly to an OpenAI-compatible API endpoint.

Uses environment variables for configuration:
  OPENAI_API_KEY    - API key (required)
  OPENAI_BASE_URL   - API endpoint (default: https://api.openai.com/v1)
  OPENAI_MODEL      - Model to use (default: gpt-4o-mini)

Input Sources (mutually exclusive):
  --prompt    Direct prompt text
  --file      Read prompt from file
  --template  Use template file with [[var]] substitution

Examples:
  # Simple prompt
  llm-support complete --prompt "Explain recursion in one sentence"

  # With system instruction
  llm-support complete --prompt "Hello" --system "You are a pirate"

  # Using a template
  llm-support complete --template prompt.txt --var code=@myfile.go --var task="review"

  # Custom model and parameters
  llm-support complete --prompt "Hello" --model gpt-4o --temperature 0.9

  # Output to file
  llm-support complete --prompt "Write a poem" --output poem.txt`,
		RunE: runComplete,
	}

	cmd.Flags().StringVar(&completePrompt, "prompt", "", "Direct prompt text")
	cmd.Flags().StringVar(&completeFile, "file", "", "Read prompt from file")
	cmd.Flags().StringVar(&completeTemplate, "template", "", "Template file with [[var]] placeholders")
	cmd.Flags().StringArrayVar(&completeVars, "var", nil, "Template variable (key=value or key=@file)")
	cmd.Flags().StringVar(&completeSystem, "system", "", "System instruction")
	cmd.Flags().StringVar(&completeModel, "model", "", "Model to use (overrides OPENAI_MODEL)")
	cmd.Flags().Float64Var(&completeTemperature, "temperature", 0.7, "Temperature (0.0-2.0)")
	cmd.Flags().IntVar(&completeMaxTokens, "max-tokens", 0, "Maximum tokens in response (0 = no limit)")
	cmd.Flags().IntVar(&completeTimeout, "timeout", 120, "Request timeout in seconds")
	cmd.Flags().IntVar(&completeRetries, "retries", 3, "Number of retries on failure")
	cmd.Flags().IntVar(&completeRetryDelay, "retry-delay", 2, "Initial retry delay in seconds")
	cmd.Flags().StringVar(&completeOutput, "output", "", "Output file (if not specified, prints to stdout)")
	cmd.Flags().BoolVar(&completeStrip, "strip", false, "Strip whitespace from file variable values")
	cmd.Flags().BoolVar(&completeJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&completeMinimal, "min", false, "Output in minimal/token-optimized format")

	return cmd
}

func runComplete(cmd *cobra.Command, args []string) error {
	// Validate input sources (mutually exclusive)
	inputCount := 0
	if completePrompt != "" {
		inputCount++
	}
	if completeFile != "" {
		inputCount++
	}
	if completeTemplate != "" {
		inputCount++
	}

	if inputCount == 0 {
		return fmt.Errorf("must specify one of: --prompt, --file, --template")
	}
	if inputCount > 1 {
		return fmt.Errorf("cannot specify multiple prompt sources")
	}

	// Build the prompt text
	var finalPrompt string

	if completePrompt != "" {
		finalPrompt = completePrompt
	} else if completeFile != "" {
		content, err := os.ReadFile(completeFile)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %w", err)
		}
		finalPrompt = string(content)
	} else {
		// Template mode
		content, err := os.ReadFile(completeTemplate)
		if err != nil {
			return fmt.Errorf("failed to read template file: %w", err)
		}
		templateText := string(content)

		// Collect and apply variables
		variables := make(map[string]string)
		for _, varArg := range completeVars {
			if !strings.Contains(varArg, "=") {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: Ignoring invalid variable format: %s\n", varArg)
				continue
			}
			parts := strings.SplitN(varArg, "=", 2)
			key, value := parts[0], parts[1]

			// Check for file reference
			if strings.HasPrefix(value, "@") {
				filePath := value[1:]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read variable file %s: %w", filePath, err)
				}
				value = string(fileContent)
				if completeStrip {
					value = strings.TrimSpace(value)
				}
			}
			variables[key] = value
		}

		// Substitute variables
		finalPrompt = substituteCompleteTemplate(templateText, variables)

		// Check for unsubstituted variables
		unsubPattern := regexp.MustCompile(`\[\[(\w+)\]\]`)
		matches := unsubPattern.FindAllStringSubmatch(finalPrompt, -1)
		if len(matches) > 0 {
			var missing []string
			for _, m := range matches {
				missing = append(missing, m[1])
			}
			return fmt.Errorf("template variable(s) not provided: %s", strings.Join(missing, ", "))
		}
	}

	// Get API configuration
	config := llmapi.GetAPIConfig()
	if err := config.Validate(); err != nil {
		return err
	}

	// Override model if specified
	if completeModel != "" {
		config.Model = completeModel
	}

	// Create client
	client := llmapi.NewLLMClientFromConfig(config)
	client.Temperature = completeTemperature
	client.MaxRetries = completeRetries
	client.RetryDelay = time.Duration(completeRetryDelay) * time.Second

	// Execute the request
	var response string
	var lastError string
	attempts := 0

	for attempt := 0; attempt <= completeRetries; attempt++ {
		attempts = attempt + 1

		resp, err := client.CompleteWithSystem(
			completeSystem,
			finalPrompt,
			time.Duration(completeTimeout)*time.Second,
		)

		if err == nil {
			response = resp
			break
		}

		lastError = err.Error()

		// Don't sleep after last attempt
		if attempt < completeRetries {
			delay := time.Duration(completeRetryDelay) * time.Second
			for i := 0; i < attempt; i++ {
				delay *= 2 // Exponential backoff
			}
			time.Sleep(delay)
		}
	}

	if response == "" && lastError != "" {
		outputCompleteResult(cmd, CompleteResult{
			Status:    "FAILED",
			Attempts:  attempts,
			Model:     config.Model,
			LastError: lastError,
		})
		return fmt.Errorf("API request failed: %s", lastError)
	}

	// Write to output file if specified
	if completeOutput != "" {
		if err := os.WriteFile(completeOutput, []byte(response), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return outputCompleteResult(cmd, CompleteResult{
		Status:       "SUCCESS",
		Attempts:     attempts,
		Model:        config.Model,
		OutputFile:   completeOutput,
		OutputLength: len(response),
		Response:     response,
	})
}

func outputCompleteResult(cmd *cobra.Command, result CompleteResult) error {
	var finalResult CompleteResult
	if completeMinimal {
		attempts := result.Attempts
		outputLen := result.OutputLength
		finalResult = CompleteResult{
			S:  result.Status,
			A:  &attempts,
			M:  result.Model,
			OF: result.OutputFile,
			OL: &outputLen,
			R:  result.Response,
			LE: result.LastError,
		}
	} else {
		finalResult = result
	}

	// If outputFile is set and not JSON mode, show metadata only
	if result.OutputFile != "" && !completeJSON && !completeMinimal {
		formatter := output.New(completeJSON, completeMinimal, cmd.OutOrStdout())
		return formatter.Print(finalResult, func(w io.Writer, data interface{}) {
			fmt.Fprintf(w, "STATUS: %s\n", result.Status)
			if result.Attempts > 0 {
				fmt.Fprintf(w, "ATTEMPTS: %d\n", result.Attempts)
			}
			if result.Model != "" {
				fmt.Fprintf(w, "MODEL: %s\n", result.Model)
			}
			if result.OutputFile != "" {
				fmt.Fprintf(w, "OUTPUT_FILE: %s\n", result.OutputFile)
			}
			if result.OutputLength > 0 {
				fmt.Fprintf(w, "OUTPUT_LENGTH: %d\n", result.OutputLength)
			}
			if result.LastError != "" {
				fmt.Fprintf(w, "LAST_ERROR: %s\n", result.LastError)
			}
		})
	}

	// Standard output (to stdout) or JSON/minimal mode
	formatter := output.New(completeJSON, completeMinimal, cmd.OutOrStdout())
	return formatter.Print(finalResult, func(w io.Writer, data interface{}) {
		// For text output without file, just print the response
		if result.OutputFile == "" && result.Response != "" {
			fmt.Fprint(w, result.Response)
		} else if result.Status == "FAILED" {
			fmt.Fprintf(w, "STATUS: %s\n", result.Status)
			if result.Attempts > 0 {
				fmt.Fprintf(w, "ATTEMPTS: %d\n", result.Attempts)
			}
			if result.Model != "" {
				fmt.Fprintf(w, "MODEL: %s\n", result.Model)
			}
			if result.LastError != "" {
				fmt.Fprintf(w, "LAST_ERROR: %s\n", result.LastError)
			}
		}
	})
}

func substituteCompleteTemplate(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		result = strings.ReplaceAll(result, "[["+key+"]]", value)
	}
	return result
}

func init() {
	RootCmd.AddCommand(newCompleteCmd())
}
