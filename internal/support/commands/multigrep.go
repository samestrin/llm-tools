package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	multigrepPath        string
	multigrepKeywords    string
	multigrepExtensions  string
	multigrepIgnoreCase  bool
	multigrepMaxPerKw    int
	multigrepNoExclude   bool
	multigrepJSON        bool
	multigrepMinimal     bool
	multigrepDefinitions bool
	multigrepOutputDir   string
)

type matchInfo struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type keywordResult struct {
	MatchCount        int         `json:"match_count,omitempty"`
	MC                *int        `json:"mc,omitempty"`
	FilesMatched      []string    `json:"files_matched,omitempty"`
	FM                []string    `json:"fm,omitempty"`
	DefinitionMatches []matchInfo `json:"definition_matches,omitempty"`
	DM                []matchInfo `json:"dm,omitempty"`
	OtherMatches      []matchInfo `json:"other_matches,omitempty"`
	OM                []matchInfo `json:"om,omitempty"`
	Truncated         bool        `json:"truncated,omitempty"`
	T                 *bool       `json:"t,omitempty"`
}

// MultigrepResult represents the overall multigrep result
type MultigrepResult struct {
	KeywordsSearched    int                       `json:"keywords_searched,omitempty"`
	KS                  *int                      `json:"ks,omitempty"`
	KeywordsWithMatches int                       `json:"keywords_with_matches,omitempty"`
	KWM                 *int                      `json:"kwm,omitempty"`
	TotalMatches        int                       `json:"total_matches,omitempty"`
	TM                  *int                      `json:"tm,omitempty"`
	SearchPath          string                    `json:"search_path,omitempty"`
	SP                  string                    `json:"sp,omitempty"`
	FilesSearched       int                       `json:"files_searched,omitempty"`
	FS                  *int                      `json:"fs,omitempty"`
	Results             map[string]*keywordResult `json:"results,omitempty"`
	R                   map[string]*keywordResult `json:"r,omitempty"`
}

// newMultigrepCmd creates the multigrep command
func newMultigrepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "multigrep",
		Short: "Search for multiple keywords in parallel",
		Long: `Search for multiple keywords in parallel with intelligent output management.
Prioritizes definition matches (function, class, const declarations) over usage matches.

Example:
  llm-support multigrep --path ./src --keywords "useState,useEffect" --extensions "ts,tsx"`,
		RunE: runMultigrep,
	}

	cmd.Flags().StringVar(&multigrepPath, "path", "", "Path to search in (required)")
	cmd.Flags().StringVar(&multigrepKeywords, "keywords", "", "Comma-separated keywords to search (required)")
	cmd.Flags().StringVar(&multigrepExtensions, "extensions", "", "Filter by file extensions (e.g., 'ts,tsx,js')")
	cmd.Flags().BoolVarP(&multigrepIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	cmd.Flags().IntVar(&multigrepMaxPerKw, "max-per-keyword", 10, "Max matches per keyword")
	cmd.Flags().BoolVar(&multigrepNoExclude, "no-exclude", false, "Don't exclude common directories")
	cmd.Flags().BoolVar(&multigrepJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&multigrepMinimal, "min", false, "Output in minimal/token-optimized format")
	cmd.Flags().BoolVarP(&multigrepDefinitions, "definitions", "d", false, "Show only definition matches")
	cmd.Flags().StringVarP(&multigrepOutputDir, "output-dir", "o", "", "Write results to directory (one file per keyword)")

	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("keywords")

	return cmd
}

func runMultigrep(cmd *cobra.Command, args []string) error {
	keywords := parseKeywords(multigrepKeywords)
	if len(keywords) == 0 {
		return fmt.Errorf("no keywords provided")
	}

	path, err := filepath.Abs(multigrepPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	// Parse extensions
	var extensions []string
	if multigrepExtensions != "" {
		for _, ext := range strings.Split(multigrepExtensions, ",") {
			ext = strings.TrimSpace(ext)
			ext = strings.TrimPrefix(ext, ".")
			if ext != "" {
				extensions = append(extensions, ext)
			}
		}
	}

	// Collect files to search
	files := collectFiles(path, extensions)

	// Definition patterns
	defPatterns := []string{
		`^\s*(export\s+)?(const|let|var|function|class|interface|type|enum)\s+%s\b`,
		`^\s*(export\s+)?async\s+function\s+%s\b`,
		`^\s*(public|private|protected)?\s*(static\s+)?(async\s+)?%s\s*[(<]`,
		`^\s*def\s+%s\s*\(`,
		`^\s*class\s+%s\s*[:(]`,
		`^\s*func\s+%s\s*\(`,
		`^\s*func\s+\([^)]*\)\s+%s\s*\(`, // Go method
	}

	// Search in parallel
	results := make(map[string]*keywordResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, kw := range keywords {
		wg.Add(1)
		go func(keyword string) {
			defer wg.Done()
			result := searchKeyword(path, files, keyword, defPatterns)
			mu.Lock()
			results[keyword] = result
			mu.Unlock()
		}(kw)
	}

	wg.Wait()

	// Calculate totals
	totalMatches := 0
	keywordsWithMatches := 0
	for _, r := range results {
		totalMatches += r.MatchCount
		if r.MatchCount > 0 {
			keywordsWithMatches++
		}
	}

	// Write output files if requested
	if multigrepOutputDir != "" {
		if err := os.MkdirAll(multigrepOutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		for keyword, r := range results {
			// Safe filename - replace non-alphanumeric with underscore
			safeKeyword := regexp.MustCompile(`[^\w\-]`).ReplaceAllString(keyword, "_")
			keywordFile := filepath.Join(multigrepOutputDir, fmt.Sprintf("keyword_%s.txt", safeKeyword))

			var lines []string
			for _, m := range r.DefinitionMatches {
				lines = append(lines, fmt.Sprintf("DEF: %s:%d: %s", m.File, m.Line, m.Content))
			}
			for _, m := range r.OtherMatches {
				lines = append(lines, fmt.Sprintf("USE: %s:%d: %s", m.File, m.Line, m.Content))
			}

			content := strings.Join(lines, "\n")
			if len(lines) > 0 {
				content += "\n"
			}
			if err := os.WriteFile(keywordFile, []byte(content), 0644); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to write %s: %v\n", keywordFile, err)
			}
		}
	}

	// Build result struct
	filesSearched := len(files)
	keywordsSearchedCount := len(keywords)
	var mgResult MultigrepResult
	if multigrepMinimal {
		mgResult = MultigrepResult{
			KS:  &keywordsSearchedCount,
			KWM: &keywordsWithMatches,
			TM:  &totalMatches,
			SP:  path,
			FS:  &filesSearched,
			R:   results,
		}
	} else {
		mgResult = MultigrepResult{
			KeywordsSearched:    keywordsSearchedCount,
			KeywordsWithMatches: keywordsWithMatches,
			TotalMatches:        totalMatches,
			SearchPath:          path,
			FilesSearched:       filesSearched,
			Results:             results,
		}
	}

	formatter := output.New(multigrepJSON, multigrepMinimal, cmd.OutOrStdout())
	return formatter.Print(mgResult, func(w io.Writer, data interface{}) {
		fmt.Fprintln(w, strings.Repeat("=", 70))
		fmt.Fprintln(w, "MULTIGREP RESULTS")
		fmt.Fprintln(w, strings.Repeat("=", 70))
		fmt.Fprintf(w, "Search Path: %s\n", path)
		fmt.Fprintf(w, "Files Searched: %d\n", len(files))
		fmt.Fprintf(w, "Keywords: %d\n", len(keywords))
		fmt.Fprintf(w, "Total Matches: %d\n", totalMatches)
		fmt.Fprintln(w, strings.Repeat("=", 70))
		fmt.Fprintln(w)

		for _, kw := range keywords {
			r := results[kw]
			fmt.Fprintf(w, "KEYWORD: %s\n", kw)
			fmt.Fprintf(w, "  Matches: %d in %d files\n", r.MatchCount, len(r.FilesMatched))

			if len(r.DefinitionMatches) > 0 {
				fmt.Fprintln(w, "  DEFINITIONS:")
				for _, m := range r.DefinitionMatches {
					fmt.Fprintf(w, "    → %s:%d: %s\n", m.File, m.Line, truncate(m.Content, 80))
				}
			}

			if !multigrepDefinitions && len(r.OtherMatches) > 0 {
				fmt.Fprintln(w, "  USAGES (sample):")
				for _, m := range r.OtherMatches {
					fmt.Fprintf(w, "    - %s:%d: %s\n", m.File, m.Line, truncate(m.Content, 80))
				}
			}

			if r.Truncated {
				fmt.Fprintln(w, "  (results truncated)")
			}
			fmt.Fprintln(w)
		}

		fmt.Fprintf(w, "KEYWORDS_SEARCHED: %d\n", len(keywords))
		fmt.Fprintf(w, "KEYWORDS_WITH_MATCHES: %d\n", keywordsWithMatches)
		fmt.Fprintf(w, "TOTAL_MATCHES: %d\n", totalMatches)
	})
}

func parseKeywords(s string) []string {
	var keywords []string
	for _, kw := range strings.Split(s, ",") {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			keywords = append(keywords, kw)
		}
	}
	return keywords
}

func collectFiles(basePath string, extensions []string) []string {
	var files []string
	binaryExts := map[string]bool{
		"exe": true, "dll": true, "so": true, "dylib": true, "bin": true,
		"pyc": true, "pyo": true, "class": true, "jar": true, "zip": true,
		"tar": true, "gz": true, "png": true, "jpg": true, "jpeg": true,
		"gif": true, "ico": true, "svg": true, "woff": true, "woff2": true,
	}

	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// Skip excluded directories
			if info != nil && info.IsDir() && !multigrepNoExclude {
				if utils.IsExcludedDir(info.Name()) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.TrimPrefix(filepath.Ext(path), ".")

		// Skip binary files
		if binaryExts[strings.ToLower(ext)] {
			return nil
		}

		// Filter by extension if specified
		if len(extensions) > 0 {
			found := false
			for _, e := range extensions {
				if strings.EqualFold(ext, e) {
					found = true
					break
				}
			}
			if !found {
				return nil
			}
		}

		files = append(files, path)
		return nil
	})

	return files
}

func searchKeyword(basePath string, files []string, keyword string, defPatterns []string) *keywordResult {
	result := &keywordResult{
		FilesMatched:      []string{},
		DefinitionMatches: []matchInfo{},
		OtherMatches:      []matchInfo{},
	}

	// Compile definition regexes
	var defRegexes []*regexp.Regexp
	for _, pattern := range defPatterns {
		re, err := regexp.Compile(fmt.Sprintf(pattern, regexp.QuoteMeta(keyword)))
		if err == nil {
			defRegexes = append(defRegexes, re)
		}
	}

	filesMatchedSet := make(map[string]bool)

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for lineNum, line := range lines {
			var matches bool
			if multigrepIgnoreCase {
				matches = strings.Contains(strings.ToLower(line), strings.ToLower(keyword))
			} else {
				matches = strings.Contains(line, keyword)
			}

			if matches {
				result.MatchCount++
				relPath, _ := filepath.Rel(basePath, filePath)
				filesMatchedSet[relPath] = true

				// Check if it's a definition
				isDefinition := false
				for _, re := range defRegexes {
					if re.MatchString(line) {
						isDefinition = true
						break
					}
				}

				info := matchInfo{
					File:    relPath,
					Line:    lineNum + 1,
					Content: strings.TrimSpace(line),
				}

				if isDefinition {
					if len(result.DefinitionMatches) < multigrepMaxPerKw {
						result.DefinitionMatches = append(result.DefinitionMatches, info)
					}
				} else {
					if len(result.OtherMatches) < multigrepMaxPerKw {
						result.OtherMatches = append(result.OtherMatches, info)
					}
				}

				if len(result.DefinitionMatches) >= multigrepMaxPerKw &&
					len(result.OtherMatches) >= multigrepMaxPerKw {
					result.Truncated = true
				}
			}
		}
	}

	for f := range filesMatchedSet {
		result.FilesMatched = append(result.FilesMatched, f)
	}

	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func init() {
	RootCmd.AddCommand(newMultigrepCmd())
}
