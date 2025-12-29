package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	foreachFiles     []string
	foreachGlob      string
	foreachTemplate  string
	foreachOutputDir string
	foreachOutputPat string
	foreachLLM       string
	foreachVars      []string
	foreachParallel  int
	foreachSkipExist bool
	foreachTimeout   int
	foreachJSON      bool
	foreachMinimal   bool
)

// ForeachResult holds the result of processing files
type ForeachResult struct {
	TotalFiles     int              `json:"total_files,omitempty"`
	TF             *int             `json:"tf,omitempty"`
	ProcessedFiles int              `json:"processed_files,omitempty"`
	PF             *int             `json:"pf,omitempty"`
	SkippedFiles   int              `json:"skipped_files,omitempty"`
	SF             *int             `json:"sf,omitempty"`
	FailedFiles    int              `json:"failed_files,omitempty"`
	FF             *int             `json:"ff,omitempty"`
	Results        []ForeachFileRes `json:"results,omitempty"`
	R              []ForeachFileRes `json:"r,omitempty"`
	ProcessingTime float64          `json:"processing_time_s,omitempty"`
	PT             *float64         `json:"pt,omitempty"`
}

// ForeachFileRes holds the result for a single file
type ForeachFileRes struct {
	InputFile  string `json:"input_file,omitempty"`
	IF         string `json:"if,omitempty"`
	OutputFile string `json:"output_file,omitempty"`
	OF         string `json:"of,omitempty"`
	Status     string `json:"status,omitempty"` // success, skipped, failed
	S          string `json:"s,omitempty"`
	Error      string `json:"error,omitempty"`
	E          string `json:"e,omitempty"`
}

// newForeachCmd creates the foreach command
func newForeachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "foreach",
		Short: "Batch process files with LLM",
		Long: `Process multiple files through an LLM using a template.

For each file, the template is populated with the file content and variables,
then sent to the specified LLM. Results are saved to output files.

Template Variables:
  [[CONTENT]]     - Content of the current file
  [[FILENAME]]    - Base name of the current file
  [[FILEPATH]]    - Full path of the current file
  [[EXTENSION]]   - File extension
  [[DIRNAME]]     - Directory name
  [[INDEX]]       - 1-based index of current file
  [[TOTAL]]       - Total number of files
  Custom variables via --var key=value

Examples:
  llm-support foreach --files "*.md" --template prompt.md --output-dir ./out
  llm-support foreach --glob "src/**/*.go" --template analyze.md --llm claude --parallel 4
  llm-support foreach --files file1.txt,file2.txt --template t.md --var LANG=Go --skip-existing`,
		RunE: runForeach,
	}

	cmd.Flags().StringSliceVar(&foreachFiles, "files", nil, "Files to process (comma-separated or repeated)")
	cmd.Flags().StringVar(&foreachGlob, "glob", "", "Glob pattern to match files")
	cmd.Flags().StringVar(&foreachTemplate, "template", "", "Template file with [[var]] placeholders (required)")
	cmd.Flags().StringVar(&foreachOutputDir, "output-dir", "", "Output directory for processed files")
	cmd.Flags().StringVar(&foreachOutputPat, "output-pattern", "", "Output filename pattern (e.g., '{{name}}-processed.md')")
	cmd.Flags().StringVar(&foreachLLM, "llm", "", "LLM binary to use (default: from config or 'gemini')")
	cmd.Flags().StringSliceVar(&foreachVars, "var", nil, "Template variable (key=value)")
	cmd.Flags().IntVar(&foreachParallel, "parallel", 1, "Number of parallel processes")
	cmd.Flags().BoolVar(&foreachSkipExist, "skip-existing", false, "Skip files where output already exists")
	cmd.Flags().IntVar(&foreachTimeout, "timeout", 120, "Timeout per file in seconds")
	cmd.Flags().BoolVar(&foreachJSON, "json", false, "Output results as JSON")
	cmd.Flags().BoolVar(&foreachMinimal, "min", false, "Output in minimal/token-optimized format")

	cmd.MarkFlagRequired("template")

	return cmd
}

func runForeach(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	// Validate template exists
	if foreachTemplate == "" {
		return fmt.Errorf("--template is required")
	}

	templateContent, err := os.ReadFile(foreachTemplate)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Collect files to process
	var files []string

	// From --files flag
	for _, f := range foreachFiles {
		// Handle comma-separated values
		for _, path := range strings.Split(f, ",") {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			// Expand glob if it contains wildcards
			if strings.ContainsAny(path, "*?[") {
				matches, err := filepath.Glob(path)
				if err == nil && len(matches) > 0 {
					files = append(files, matches...)
				}
			} else if _, err := os.Stat(path); err == nil {
				files = append(files, path)
			}
		}
	}

	// From --glob flag
	if foreachGlob != "" {
		matches, err := filepath.Glob(foreachGlob)
		if err != nil {
			return fmt.Errorf("invalid glob pattern: %w", err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files to process")
	}

	// Remove duplicates
	files = uniqueStrings(files)

	// Parse variables
	variables := make(map[string]string)
	for _, v := range foreachVars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			variables[parts[0]] = parts[1]
		}
	}

	// Get LLM binary
	llmBinary := foreachLLM
	if llmBinary == "" {
		llmBinary = getDefaultLLM()
	}

	result := ForeachResult{
		TotalFiles: len(files),
	}

	// Process files
	if foreachParallel <= 1 {
		// Sequential processing
		for i, file := range files {
			res := processFile(file, string(templateContent), variables, llmBinary, i+1, len(files))
			result.Results = append(result.Results, res)
			updateCounts(&result, res)
		}
	} else {
		// Parallel processing
		result.Results = processFilesParallel(files, string(templateContent), variables, llmBinary, foreachParallel)
		for _, res := range result.Results {
			updateCounts(&result, res)
		}
	}

	result.ProcessingTime = time.Since(startTime).Seconds()

	// Output results
	return outputForeachResult(cmd, result)
}

func processFile(filePath, template string, variables map[string]string, llmBinary string, index, total int) ForeachFileRes {
	result := ForeachFileRes{
		InputFile: filePath,
		Status:    "failed",
	}

	// Determine output path
	outputPath := determineOutputPath(filePath)
	result.OutputFile = outputPath

	// Check if should skip
	if foreachSkipExist && outputPath != "" {
		if _, err := os.Stat(outputPath); err == nil {
			result.Status = "skipped"
			return result
		}
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read file: %v", err)
		return result
	}

	// Build prompt from template
	prompt := substituteTemplateVars(template, filePath, string(content), variables, index, total)

	// Execute LLM
	llmOutput, exitCode, stderr := executeLLM(llmBinary, detectLLMStyle(llmBinary), prompt, "", foreachTimeout)

	if exitCode != 0 {
		result.Error = fmt.Sprintf("LLM failed (exit %d): %s", exitCode, stderr)
		return result
	}

	// Write output
	if outputPath != "" {
		// Ensure output directory exists
		if foreachOutputDir != "" {
			os.MkdirAll(foreachOutputDir, 0755)
		}

		if err := os.WriteFile(outputPath, []byte(llmOutput), 0644); err != nil {
			result.Error = fmt.Sprintf("failed to write output: %v", err)
			return result
		}
	}

	result.Status = "success"
	return result
}

func processFilesParallel(files []string, template string, variables map[string]string, llmBinary string, parallel int) []ForeachFileRes {
	results := make([]ForeachFileRes, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)

	for i, file := range files {
		wg.Add(1)
		go func(idx int, f string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = processFile(f, template, variables, llmBinary, idx+1, len(files))
		}(i, file)
	}

	wg.Wait()
	return results
}

func substituteTemplateVars(template, filePath, content string, variables map[string]string, index, total int) string {
	result := template

	// Built-in variables
	result = strings.ReplaceAll(result, "[[CONTENT]]", content)
	result = strings.ReplaceAll(result, "[[FILENAME]]", filepath.Base(filePath))
	result = strings.ReplaceAll(result, "[[FILEPATH]]", filePath)
	result = strings.ReplaceAll(result, "[[EXTENSION]]", filepath.Ext(filePath))
	result = strings.ReplaceAll(result, "[[DIRNAME]]", filepath.Dir(filePath))
	result = strings.ReplaceAll(result, "[[INDEX]]", fmt.Sprintf("%d", index))
	result = strings.ReplaceAll(result, "[[TOTAL]]", fmt.Sprintf("%d", total))

	// Custom variables
	for key, value := range variables {
		result = strings.ReplaceAll(result, "[["+key+"]]", value)
	}

	return result
}

func determineOutputPath(inputPath string) string {
	if foreachOutputDir == "" && foreachOutputPat == "" {
		return "" // No output file, just stdout
	}

	baseName := filepath.Base(inputPath)
	ext := filepath.Ext(baseName)
	nameNoExt := strings.TrimSuffix(baseName, ext)

	var outputName string
	if foreachOutputPat != "" {
		// Apply output pattern
		outputName = foreachOutputPat
		outputName = strings.ReplaceAll(outputName, "{{name}}", nameNoExt)
		outputName = strings.ReplaceAll(outputName, "{{ext}}", ext)
		outputName = strings.ReplaceAll(outputName, "{{filename}}", baseName)
	} else {
		// Default: same name in output dir
		outputName = baseName
	}

	if foreachOutputDir != "" {
		return filepath.Join(foreachOutputDir, outputName)
	}

	return filepath.Join(filepath.Dir(inputPath), outputName)
}

func updateCounts(result *ForeachResult, fileRes ForeachFileRes) {
	switch fileRes.Status {
	case "success":
		result.ProcessedFiles++
	case "skipped":
		result.SkippedFiles++
	case "failed":
		result.FailedFiles++
	}
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func outputForeachResult(cmd *cobra.Command, result ForeachResult) error {
	// Build appropriate result struct
	var finalResult ForeachResult
	if foreachMinimal {
		tf := result.TotalFiles
		pf := result.ProcessedFiles
		sf := result.SkippedFiles
		ff := result.FailedFiles
		pt := result.ProcessingTime
		minResults := make([]ForeachFileRes, len(result.Results))
		for i, r := range result.Results {
			minResults[i] = ForeachFileRes{
				IF: r.InputFile,
				OF: r.OutputFile,
				S:  r.Status,
				E:  r.Error,
			}
		}
		finalResult = ForeachResult{
			TF: &tf,
			PF: &pf,
			SF: &sf,
			FF: &ff,
			R:  minResults,
			PT: &pt,
		}
	} else {
		finalResult = result
	}

	formatter := output.New(foreachJSON, foreachMinimal, cmd.OutOrStdout())
	return formatter.Print(finalResult, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "PROCESSED: %d/%d\n", result.ProcessedFiles, result.TotalFiles)
		fmt.Fprintf(w, "SKIPPED: %d\n", result.SkippedFiles)
		fmt.Fprintf(w, "FAILED: %d\n", result.FailedFiles)
		fmt.Fprintf(w, "TIME: %.2fs\n", result.ProcessingTime)

		if result.FailedFiles > 0 {
			fmt.Fprintln(w, "\nFailed files:")
			for _, r := range result.Results {
				if r.Status == "failed" {
					fmt.Fprintf(w, "  %s: %s\n", r.InputFile, r.Error)
				}
			}
		}
	})
}

// executeForeachLLM is a wrapper for testing
var executeForeachLLM = func(binary string, args []string, timeout int) (string, int, string) {
	cmd := exec.Command(binary, args...)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	done := make(chan error, 1)
	var output []byte
	var err error

	go func() {
		output, err = cmd.Output()
		done <- err
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", 124, "timeout"
	case <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return "", exitErr.ExitCode(), string(exitErr.Stderr)
			}
			return "", 1, err.Error()
		}
		return string(output), 0, ""
	}
}

// checkUnsubstitutedVars checks for unsubstituted template variables
func checkUnsubstitutedVars(content string) []string {
	pattern := regexp.MustCompile(`\[\[(\w+)\]\]`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	var unsubstituted []string
	for _, m := range matches {
		unsubstituted = append(unsubstituted, m[1])
	}
	return unsubstituted
}

func init() {
	RootCmd.AddCommand(newForeachCmd())
}
