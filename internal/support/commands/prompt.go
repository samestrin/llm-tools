package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	promptText        string
	promptFile        string
	promptTemplate    string
	promptVars        []string
	promptLLM         string
	promptInstruct    string
	promptOutput      string
	promptRetries     int
	promptRetryDelay  int
	promptTimeout     int
	promptMinLen      int
	promptMustContain []string
	promptNoErrorChk  bool
	promptCache       bool
	promptCacheTTL    int
	promptRefresh     bool
	promptStrip       bool
)

// newPromptCmd creates the prompt command
func newPromptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prompt",
		Short: "Execute LLM prompt with template/retry support",
		Long: `Execute an LLM prompt with template substitution, retry logic, and validation.

Input Sources (mutually exclusive):
  --prompt  Direct prompt text
  --file    Read prompt from file
  --template  Use template file with [[var]] substitution

Features:
  - Template variable substitution with --var key=value
  - Retry with exponential backoff
  - Response validation (min-length, must-contain, error-check)
  - Optional caching`,
		RunE: runPrompt,
	}

	cmd.Flags().StringVar(&promptText, "prompt", "", "Direct prompt text")
	cmd.Flags().StringVar(&promptFile, "file", "", "Read prompt from file")
	cmd.Flags().StringVar(&promptTemplate, "template", "", "Template file with [[var]] placeholders")
	cmd.Flags().StringArrayVar(&promptVars, "var", nil, "Template variable (key=value or key=@file)")
	cmd.Flags().StringVar(&promptLLM, "llm", "", "LLM binary to use (default: from config or 'gemini')")
	cmd.Flags().StringVar(&promptInstruct, "instruction", "", "System instruction for the LLM")
	cmd.Flags().StringVar(&promptOutput, "output", "", "Output file (if not specified, prints to stdout)")
	cmd.Flags().IntVar(&promptRetries, "retries", 0, "Number of retries on failure")
	cmd.Flags().IntVar(&promptRetryDelay, "retry-delay", 2, "Initial retry delay in seconds")
	cmd.Flags().IntVar(&promptTimeout, "timeout", 120, "Timeout in seconds")
	cmd.Flags().IntVar(&promptMinLen, "min-length", 0, "Minimum response length")
	cmd.Flags().StringArrayVar(&promptMustContain, "must-contain", nil, "Required text in response")
	cmd.Flags().BoolVar(&promptNoErrorChk, "no-error-check", false, "Skip error pattern checking")
	cmd.Flags().BoolVar(&promptCache, "cache", false, "Enable response caching")
	cmd.Flags().IntVar(&promptCacheTTL, "cache-ttl", 3600, "Cache TTL in seconds")
	cmd.Flags().BoolVar(&promptRefresh, "refresh", false, "Force refresh cached response")
	cmd.Flags().BoolVar(&promptStrip, "strip", false, "Strip whitespace from file variable values")

	return cmd
}

func runPrompt(cmd *cobra.Command, args []string) error {
	// Validate input sources (mutually exclusive)
	inputCount := 0
	if promptText != "" {
		inputCount++
	}
	if promptFile != "" {
		inputCount++
	}
	if promptTemplate != "" {
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

	if promptText != "" {
		finalPrompt = promptText
	} else if promptFile != "" {
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %w", err)
		}
		finalPrompt = string(content)
	} else {
		// Template mode
		content, err := os.ReadFile(promptTemplate)
		if err != nil {
			return fmt.Errorf("failed to read template file: %w", err)
		}
		templateText := string(content)

		// Collect and apply variables
		variables := make(map[string]string)
		for _, varArg := range promptVars {
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
				if promptStrip {
					value = strings.TrimSpace(value)
				}
			}
			variables[key] = value
		}

		// Substitute variables
		finalPrompt = substituteTemplate(templateText, variables)

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

	// Get LLM binary
	llmBinary := promptLLM
	if llmBinary == "" {
		llmBinary = getDefaultLLM()
	}

	// Parse style from binary if specified (binary:style format)
	style := ""
	if strings.Contains(llmBinary, ":") {
		parts := strings.SplitN(llmBinary, ":", 2)
		llmBinary = parts[0]
		style = parts[1]
	} else {
		style = detectLLMStyle(llmBinary)
	}

	// Check cache if enabled
	var cacheKey string
	if promptCache {
		cacheKey = generateCacheKey(llmBinary, finalPrompt, promptInstruct)

		if !promptRefresh {
			response, cached, age := loadFromCache(cacheKey, promptCacheTTL)
			if cached {
				if promptOutput != "" {
					if err := os.WriteFile(promptOutput, []byte(response), 0644); err != nil {
						return fmt.Errorf("failed to write output: %w", err)
					}
					fmt.Fprintln(cmd.OutOrStdout(), "STATUS: SUCCESS")
					fmt.Fprintln(cmd.OutOrStdout(), "CACHED: TRUE")
					fmt.Fprintf(cmd.OutOrStdout(), "CACHE_AGE: %ds\n", age)
					fmt.Fprintf(cmd.OutOrStdout(), "OUTPUT_FILE: %s\n", promptOutput)
					fmt.Fprintf(cmd.OutOrStdout(), "OUTPUT_LENGTH: %d\n", len(response))
				} else {
					fmt.Fprint(cmd.OutOrStdout(), response)
				}
				return nil
			}
		}
	}

	// Execute with retries
	attempts := 0
	maxRetries := promptRetries
	retryDelay := promptRetryDelay
	var lastError string
	var response string

	for attempts <= maxRetries {
		attempts++

		resp, exitCode, stderr := executeLLM(llmBinary, style, finalPrompt, promptInstruct, promptTimeout)

		if exitCode == 127 {
			return fmt.Errorf("%s", stderr)
		}

		if exitCode == 124 {
			lastError = stderr
			if attempts <= maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				retryDelay *= 2
				continue
			}
			break
		}

		if exitCode != 0 {
			if stderr != "" {
				lastError = stderr
			} else {
				lastError = fmt.Sprintf("LLM returned exit code %d", exitCode)
			}
			if attempts <= maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				retryDelay *= 2
				continue
			}
			break
		}

		// Validate response
		valid, validationError := validateLLMResponse(resp, promptMinLen, promptMustContain, promptNoErrorChk)

		if valid {
			response = resp
			break
		}

		lastError = validationError
		if attempts <= maxRetries {
			time.Sleep(time.Duration(retryDelay) * time.Second)
			retryDelay *= 2
			continue
		}
	}

	if lastError != "" && response == "" {
		out := cmd.OutOrStdout()
		if promptOutput != "" {
			out = cmd.ErrOrStderr()
		}
		fmt.Fprintln(out, "STATUS: FAILED")
		fmt.Fprintf(out, "ATTEMPTS: %d\n", attempts)
		fmt.Fprintf(out, "LAST_ERROR: %s\n", lastError)
		return fmt.Errorf("prompt execution failed")
	}

	// Final validation
	valid, validationError := validateLLMResponse(response, promptMinLen, promptMustContain, promptNoErrorChk)
	if !valid {
		out := cmd.OutOrStdout()
		if promptOutput != "" {
			out = cmd.ErrOrStderr()
		}
		fmt.Fprintln(out, "STATUS: FAILED")
		fmt.Fprintf(out, "ATTEMPTS: %d\n", attempts)
		fmt.Fprintf(out, "LAST_ERROR: %s\n", validationError)
		return fmt.Errorf("response validation failed")
	}

	// Save to cache if enabled
	if promptCache && cacheKey != "" {
		saveToCache(cacheKey, response)
	}

	// Output
	if promptOutput != "" {
		if err := os.WriteFile(promptOutput, []byte(response), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "STATUS: SUCCESS")
		fmt.Fprintf(cmd.OutOrStdout(), "ATTEMPTS: %d\n", attempts)
		fmt.Fprintln(cmd.OutOrStdout(), "CACHED: FALSE")
		fmt.Fprintf(cmd.OutOrStdout(), "OUTPUT_FILE: %s\n", promptOutput)
		fmt.Fprintf(cmd.OutOrStdout(), "OUTPUT_LENGTH: %d\n", len(response))
	} else {
		fmt.Fprint(cmd.OutOrStdout(), response)
	}

	return nil
}

func substituteTemplate(template string, variables map[string]string) string {
	result := template
	for key, value := range variables {
		result = strings.ReplaceAll(result, "[["+key+"]]", value)
	}
	return result
}

func getDefaultLLM() string {
	// Check environment variable
	if llm := os.Getenv("LLM_SUPPORT_LLM"); llm != "" {
		return llm
	}

	// Check config files
	configPaths := []string{
		".planning/.config/helper_llm",
		filepath.Join(os.Getenv("HOME"), ".config", "llm-support", "default_llm"),
	}

	for _, path := range configPaths {
		if content, err := os.ReadFile(path); err == nil {
			return strings.TrimSpace(string(content))
		}
	}

	return "gemini"
}

func detectLLMStyle(binary string) string {
	binary = strings.ToLower(filepath.Base(binary))

	if strings.Contains(binary, "gemini") {
		return "gemini"
	}
	if strings.Contains(binary, "claude") {
		return "claude"
	}
	if strings.Contains(binary, "gpt") || strings.Contains(binary, "openai") {
		return "openai"
	}
	// Check ollama before llama since ollama contains "llama"
	if strings.Contains(binary, "ollama") {
		return "ollama"
	}
	if strings.Contains(binary, "llama") {
		return "llama"
	}

	return "generic"
}

func executeLLM(binary, style, prompt, instruction string, timeout int) (string, int, string) {
	var args []string

	switch style {
	case "gemini":
		args = []string{"-p", prompt}
		if instruction != "" {
			args = append(args, "--instruction", instruction)
		}
	case "claude":
		args = []string{"-p", prompt}
		if instruction != "" {
			args = append(args, "--system", instruction)
		}
	case "openai":
		args = []string{prompt}
		if instruction != "" {
			args = append(args, "--system", instruction)
		}
	default:
		args = []string{prompt}
	}

	cmd := exec.Command(binary, args...)

	// Set timeout using a simple approach
	done := make(chan error, 1)
	var output []byte
	var err error

	go func() {
		output, err = cmd.Output()
		done <- err
	}()

	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", 124, fmt.Sprintf("LLM execution timeout after %ds", timeout)
	case <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return "", exitErr.ExitCode(), string(exitErr.Stderr)
			}
			if os.IsNotExist(err) {
				return "", 127, fmt.Sprintf("LLM CLI not found: %s", binary)
			}
			return "", 1, err.Error()
		}
		return string(output), 0, ""
	}
}

func validateLLMResponse(response string, minLength int, mustContain []string, noErrorCheck bool) (bool, string) {
	if !noErrorCheck {
		hasError, errorMsg := checkErrorPatterns(response)
		if hasError {
			return false, errorMsg
		}
	}

	if minLength > 0 && len(strings.TrimSpace(response)) < minLength {
		return false, fmt.Sprintf("Response too short (%d < %d)", len(strings.TrimSpace(response)), minLength)
	}

	for _, required := range mustContain {
		if !strings.Contains(response, required) {
			return false, fmt.Sprintf("Response missing required text: %q", required)
		}
	}

	return true, ""
}

func checkErrorPatterns(response string) (bool, string) {
	errorPatterns := []string{
		"ERROR:",
		"FAILED:",
		"Exception:",
		"Traceback (most recent",
		"panic:",
		"fatal error:",
	}

	lower := strings.ToLower(response)
	for _, pattern := range errorPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			// Check if it's in the first 500 chars (likely an error message, not content)
			if strings.Contains(strings.ToLower(response[:min(500, len(response))]), strings.ToLower(pattern)) {
				return true, fmt.Sprintf("Response contains error pattern: %s", pattern)
			}
		}
	}

	return false, ""
}

func generateCacheKey(binary, prompt, instruction string) string {
	data := binary + "|" + prompt + "|" + instruction
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func getCacheDir() string {
	cacheDir := filepath.Join(os.TempDir(), "llm-support-cache")
	os.MkdirAll(cacheDir, 0755)
	return cacheDir
}

func loadFromCache(key string, ttl int) (string, bool, int) {
	cachePath := filepath.Join(getCacheDir(), key)

	info, err := os.Stat(cachePath)
	if err != nil {
		return "", false, 0
	}

	age := int(time.Since(info.ModTime()).Seconds())
	if age > ttl {
		return "", false, 0
	}

	content, err := os.ReadFile(cachePath)
	if err != nil {
		return "", false, 0
	}

	return string(content), true, age
}

func saveToCache(key, response string) {
	cachePath := filepath.Join(getCacheDir(), key)
	os.WriteFile(cachePath, []byte(response), 0644)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	RootCmd.AddCommand(newPromptCmd())
}
