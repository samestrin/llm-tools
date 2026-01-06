package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"

	"github.com/samestrin/llm-tools/pkg/llmapi"
	"github.com/samestrin/llm-tools/pkg/output"
)

var (
	extractRelevantContext     string
	extractRelevantConcurrency int
	extractRelevantOutput      string
	extractRelevantTimeout     int
	extractRelevantJSON        bool
	extractRelevantMinimal     bool
	extractRelevantPath        string
)

// ExtractRelevantResult holds the extraction result
type ExtractRelevantResult struct {
	Path            string   `json:"path,omitempty"`
	P               string   `json:"p,omitempty"`
	URL             string   `json:"url,omitempty"`
	U               string   `json:"u,omitempty"`
	Context         string   `json:"context,omitempty"`
	Ctx             string   `json:"ctx,omitempty"`
	ExtractedParts  []string `json:"extracted_parts,omitempty"`
	EP              []string `json:"ep,omitempty"`
	TotalFiles      int      `json:"total_files,omitempty"`
	TF              *int     `json:"tf,omitempty"`
	ProcessedFiles  int      `json:"processed_files,omitempty"`
	PF              *int     `json:"pf,omitempty"`
	Error           string   `json:"error,omitempty"`
	E               string   `json:"e,omitempty"`
	ProcessingTimeS float64  `json:"processing_time_s,omitempty"`
	PTS             *float64 `json:"pts,omitempty"`
}

// newExtractRelevantCmd creates the extract-relevant command
func newExtractRelevantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract-relevant",
		Short: "Extract relevant content using LLM API",
		Long: `Extract relevant content from files, directories, or URLs using an LLM API.

The command sends content to the LLM with a context describing what
to extract, and returns only the relevant portions.

Supports:
  - Local files and directories
  - URLs (HTTP/HTTPS) - HTML is automatically converted to clean text

Examples:
  # Local files and directories
  llm-support extract-relevant --path ./src --context "API endpoint definitions"
  llm-support extract-relevant --path ./docs --context "Configuration options" --concurrency 4
  llm-support extract-relevant --path ./file.md --context "Code examples" -o output.md

  # URLs (HTML automatically stripped)
  llm-support extract-relevant --path https://example.com/docs --context "Installation steps"
  llm-support extract-relevant --path https://api.example.com/openapi.json --context "Authentication endpoints"

API Configuration:
  Set OPENAI_API_KEY environment variable or create .planning/.config/openai_api_key file.
  Optionally set OPENAI_BASE_URL and OPENAI_MODEL for custom endpoints.`,
		Args: cobra.NoArgs,
		RunE: runExtractRelevant,
	}

	cmd.Flags().StringVar(&extractRelevantPath, "path", ".", "File or directory path to process")
	cmd.Flags().StringVar(&extractRelevantContext, "context", "", "Context describing what content to extract (required)")
	cmd.Flags().IntVar(&extractRelevantConcurrency, "concurrency", 2, "Number of concurrent API calls for directory processing")
	cmd.Flags().StringVarP(&extractRelevantOutput, "output", "o", "", "Output file (default: stdout)")
	cmd.Flags().IntVar(&extractRelevantTimeout, "timeout", 60, "API call timeout in seconds")
	cmd.Flags().BoolVar(&extractRelevantJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&extractRelevantMinimal, "min", false, "Output in minimal/token-optimized format")

	cmd.MarkFlagRequired("context")

	return cmd
}

func runExtractRelevant(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	targetPath := extractRelevantPath

	// Validate context is provided
	if extractRelevantContext == "" {
		return fmt.Errorf("--context is required")
	}

	// Load API configuration
	config := llmapi.GetAPIConfig()
	if err := config.Validate(); err != nil {
		return err
	}

	// Create LLM client
	client := llmapi.NewLLMClientFromConfig(config)

	result := ExtractRelevantResult{
		Context: extractRelevantContext,
	}

	var err error

	// Check if target is a URL
	if isURL(targetPath) {
		result.URL = targetPath
		err = processURL(cmd, client, targetPath, &result)
	} else {
		result.Path = targetPath

		// Check if target exists
		info, statErr := os.Stat(targetPath)
		if os.IsNotExist(statErr) {
			return fmt.Errorf("path not found: %s", targetPath)
		}
		if statErr != nil {
			return fmt.Errorf("cannot access path: %w", statErr)
		}

		if info.IsDir() {
			// Process directory
			err = processDirectory(cmd, client, targetPath, &result)
		} else {
			// Process single file
			err = processSingleFile(cmd, client, targetPath, &result)
		}
	}

	if err != nil {
		result.Error = err.Error()
	}

	result.ProcessingTimeS = time.Since(startTime).Seconds()

	// Output result
	return outputExtractResult(cmd, result)
}

func processSingleFile(cmd *cobra.Command, client *llmapi.LLMClient, filePath string, result *ExtractRelevantResult) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	extracted, err := extractRelevantContent(client, string(content), extractRelevantContext, extractRelevantTimeout)
	if err != nil {
		return err
	}

	result.ExtractedParts = append(result.ExtractedParts, extracted)
	result.TotalFiles = 1
	result.ProcessedFiles = 1

	return nil
}

// isURL checks if the given path is an HTTP/HTTPS URL
func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// processURL fetches and processes content from a URL
func processURL(cmd *cobra.Command, client *llmapi.LLMClient, url string, result *ExtractRelevantResult) error {
	content, err := fetchURL(url, extractRelevantTimeout)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}

	extracted, err := extractRelevantContent(client, content, extractRelevantContext, extractRelevantTimeout)
	if err != nil {
		return err
	}

	result.ExtractedParts = append(result.ExtractedParts, extracted)
	result.TotalFiles = 1
	result.ProcessedFiles = 1

	return nil
}

// fetchURL fetches content from a URL and converts HTML to plain text
func fetchURL(url string, timeout int) (string, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "llm-support/1.0 (extract-relevant)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,text/plain;q=0.8,*/*;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")

	// Check if it's HTML content
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "application/xhtml") {
		return htmlToText(resp.Body)
	}

	// For non-HTML content, read as-is
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// htmlToText converts HTML content to clean plain text
func htmlToText(r io.Reader) (string, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Remove script, style, and other non-content elements
	doc.Find("script, style, noscript, iframe, svg, nav, footer, header").Remove()

	// Extract text content with structure preservation
	var buf strings.Builder

	// Try to get the page title
	if title := doc.Find("title").First().Text(); title != "" {
		buf.WriteString("# ")
		buf.WriteString(strings.TrimSpace(title))
		buf.WriteString("\n\n")
	}

	// Process main content (prefer article/main, fall back to body)
	var mainContent *goquery.Selection
	if article := doc.Find("article, main, [role='main']").First(); article.Length() > 0 {
		mainContent = article
	} else {
		mainContent = doc.Find("body")
	}

	// Extract text with basic structure
	mainContent.Find("h1, h2, h3, h4, h5, h6, p, li, pre, code, blockquote, td, th").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		tagName := goquery.NodeName(s)

		switch tagName {
		case "h1":
			buf.WriteString("\n# ")
			buf.WriteString(text)
			buf.WriteString("\n\n")
		case "h2":
			buf.WriteString("\n## ")
			buf.WriteString(text)
			buf.WriteString("\n\n")
		case "h3":
			buf.WriteString("\n### ")
			buf.WriteString(text)
			buf.WriteString("\n\n")
		case "h4", "h5", "h6":
			buf.WriteString("\n#### ")
			buf.WriteString(text)
			buf.WriteString("\n\n")
		case "li":
			buf.WriteString("- ")
			buf.WriteString(text)
			buf.WriteString("\n")
		case "pre", "code":
			buf.WriteString("\n```\n")
			buf.WriteString(text)
			buf.WriteString("\n```\n\n")
		case "blockquote":
			buf.WriteString("> ")
			buf.WriteString(text)
			buf.WriteString("\n\n")
		default:
			buf.WriteString(text)
			buf.WriteString("\n\n")
		}
	})

	result := buf.String()

	// Clean up excessive whitespace
	result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	result = strings.TrimSpace(result)

	if result == "" {
		// Fallback: just get all text from body
		result = strings.TrimSpace(doc.Find("body").Text())
	}

	return result, nil
}

func processDirectory(cmd *cobra.Command, client *llmapi.LLMClient, dirPath string, result *ExtractRelevantResult) error {
	// Collect files to process
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			// Skip common excluded directories
			if shouldExcludeDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip binary and non-text files
		if !hasTextExtension(path) {
			return nil
		}

		// Skip very large files
		if info.Size() > 100*1024 { // 100KB limit
			return nil
		}

		files = append(files, path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	result.TotalFiles = len(files)

	if len(files) == 0 {
		return fmt.Errorf("no suitable files found in directory")
	}

	// Process files concurrently using batch processor
	processor := llmapi.NewBatchProcessor(client, extractRelevantConcurrency)

	items := make([]interface{}, len(files))
	for i, f := range files {
		items[i] = f
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(extractRelevantTimeout*len(files))*time.Second)
	defer cancel()

	results := processor.ProcessItems(ctx, items, func(ctx context.Context, c *llmapi.LLMClient, item interface{}) (interface{}, error) {
		filePath := item.(string)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		// Create context-aware prompt for this file
		relPath, _ := filepath.Rel(dirPath, filePath)
		fileContext := fmt.Sprintf("File: %s\n\nContext: %s", relPath, extractRelevantContext)

		extracted, err := extractRelevantContent(c, string(content), fileContext, extractRelevantTimeout)
		if err != nil {
			return nil, err
		}

		if extracted == "" || strings.TrimSpace(extracted) == "NONE" {
			return nil, nil // No relevant content
		}

		return fmt.Sprintf("## %s\n\n%s", relPath, extracted), nil
	})

	// Collect successful results
	for _, r := range results {
		if r.Error == nil && r.Output != nil {
			output := r.Output.(string)
			if output != "" {
				result.ExtractedParts = append(result.ExtractedParts, output)
			}
		}
		if r.Error == nil {
			result.ProcessedFiles++
		}
	}

	return nil
}

func extractRelevantContent(client *llmapi.LLMClient, content, context string, timeout int) (string, error) {
	systemPrompt := `You are a content extraction assistant. Your task is to extract only the parts of the provided content that are relevant to the given context.

Rules:
1. Only extract content that directly relates to the specified context
2. Preserve the original formatting (code blocks, headers, lists, etc.)
3. If no relevant content is found, respond with "NONE"
4. Do not add explanations or summaries - just extract the relevant portions
5. Keep extracted sections in their original order
6. Include enough surrounding context to make the extracted content understandable`

	userPrompt := fmt.Sprintf("Context: %s\n\n---\n\nContent to extract from:\n\n%s", context, content)

	// Truncate content if too long (API limit considerations)
	if len(userPrompt) > 30000 {
		userPrompt = userPrompt[:30000] + "\n\n[Content truncated due to length]"
	}

	return client.CompleteWithSystem(systemPrompt, userPrompt, time.Duration(timeout)*time.Second)
}

func shouldExcludeDir(name string) bool {
	excludedDirs := map[string]bool{
		".git":          true,
		"node_modules":  true,
		"vendor":        true,
		"dist":          true,
		"build":         true,
		"__pycache__":   true,
		".pytest_cache": true,
		"target":        true,
		"coverage":      true,
		".stryker-tmp":  true,
		".next":         true,
		".nuxt":         true,
		"out":           true,
		".vscode":       true,
		".idea":         true,
	}
	return excludedDirs[name]
}

func hasTextExtension(path string) bool {
	textExts := map[string]bool{
		".md": true, ".txt": true, ".go": true, ".py": true, ".js": true,
		".ts": true, ".jsx": true, ".tsx": true, ".html": true, ".css": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
		".sh": true, ".bash": true, ".zsh": true, ".sql": true, ".rs": true,
		".java": true, ".kt": true, ".swift": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true, ".vue": true,
		".svelte": true, ".astro": true, ".prisma": true, ".graphql": true,
		".proto": true, ".dockerfile": true, ".gitignore": true, ".env": true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		// Check for known extensionless files
		base := strings.ToLower(filepath.Base(path))
		return base == "dockerfile" || base == "makefile" || base == "gemfile" ||
			base == "rakefile" || base == "readme" || base == "license"
	}

	return textExts[ext]
}

func outputExtractResult(cmd *cobra.Command, result ExtractRelevantResult) error {
	// Build the appropriate result struct
	var finalResult ExtractRelevantResult
	if extractRelevantMinimal {
		tf := result.TotalFiles
		pf := result.ProcessedFiles
		pts := result.ProcessingTimeS
		finalResult = ExtractRelevantResult{
			P:   result.Path,
			U:   result.URL,
			Ctx: result.Context,
			EP:  result.ExtractedParts,
			TF:  &tf,
			PF:  &pf,
			E:   result.Error,
			PTS: &pts,
		}
	} else {
		finalResult = result
	}

	// Write to file if output is specified
	if extractRelevantOutput != "" {
		var outputContent string

		if extractRelevantJSON || extractRelevantMinimal {
			// Use formatter to get JSON output
			var buf bytes.Buffer
			formatter := output.New(true, extractRelevantMinimal, &buf)
			formatter.Print(finalResult, nil)
			outputContent = buf.String()
		} else {
			// Plain text output - just the extracted content
			if result.Error != "" {
				outputContent = fmt.Sprintf("ERROR: %s\n", result.Error)
			} else if len(result.ExtractedParts) == 0 {
				outputContent = "No relevant content found.\n"
			} else {
				outputContent = strings.Join(result.ExtractedParts, "\n\n---\n\n")
			}
		}

		if err := os.WriteFile(extractRelevantOutput, []byte(outputContent), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Extracted content written to: %s\n", extractRelevantOutput)
		fmt.Fprintf(cmd.OutOrStdout(), "Files processed: %d/%d\n", result.ProcessedFiles, result.TotalFiles)
		fmt.Fprintf(cmd.OutOrStdout(), "Processing time: %.2fs\n", result.ProcessingTimeS)
		return nil
	}

	// Output to stdout
	formatter := output.New(extractRelevantJSON, extractRelevantMinimal, cmd.OutOrStdout())
	return formatter.Print(finalResult, func(w io.Writer, data interface{}) {
		if result.Error != "" {
			fmt.Fprintf(w, "ERROR: %s\n", result.Error)
		} else if len(result.ExtractedParts) == 0 {
			fmt.Fprintln(w, "No relevant content found.")
		} else {
			fmt.Fprint(w, strings.Join(result.ExtractedParts, "\n\n---\n\n"))
		}
	})
}

func init() {
	RootCmd.AddCommand(newExtractRelevantCmd())
}
