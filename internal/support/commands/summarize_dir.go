package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/internal/support/gitignore"
	"github.com/samestrin/llm-tools/internal/support/utils"
	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	summarizeDirFormat      string
	summarizeDirRecursive   bool
	summarizeDirGlob        string
	summarizeDirMaxTokens   int
	summarizeDirMaxLines    int
	summarizeDirNoGitignore bool
	summarizeDirPath        string
	summarizeDirJSON        bool
	summarizeDirMinimal     bool
)

// SummarizeDirFileInfo represents file info in the result
type SummarizeDirFileInfo struct {
	Path string `json:"path,omitempty"`
	P    string `json:"p,omitempty"`
	Size int64  `json:"size,omitempty"`
	S    *int64 `json:"s,omitempty"`
	Ext  string `json:"ext,omitempty"`
	E    string `json:"e,omitempty"`
}

// SummarizeDirResult represents the summarize-dir result
type SummarizeDirResult struct {
	Directory   string                 `json:"directory,omitempty"`
	Dir         string                 `json:"dir,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Fmt         string                 `json:"fmt,omitempty"`
	Directories []string               `json:"directories,omitempty"`
	Dirs        []string               `json:"dirs,omitempty"`
	Files       []SummarizeDirFileInfo `json:"files,omitempty"`
	F           []SummarizeDirFileInfo `json:"f,omitempty"`
	FileCount   int                    `json:"file_count,omitempty"`
	FC          *int                   `json:"fc,omitempty"`
	DirCount    int                    `json:"dir_count,omitempty"`
	DC          *int                   `json:"dc,omitempty"`
	Contents    []FileContent          `json:"contents,omitempty"`
	C           []FileContent          `json:"c,omitempty"`
}

// FileContent represents file content for outline/full modes
type FileContent struct {
	Path    string `json:"path,omitempty"`
	P       string `json:"p,omitempty"`
	Content string `json:"content,omitempty"`
	C       string `json:"c,omitempty"`
}

// FileHeaders represents headers extracted from a markdown file
type FileHeaders struct {
	Path    string        `json:"path,omitempty"`
	P       string        `json:"p,omitempty"`
	Headers []HeaderEntry `json:"headers,omitempty"`
	H       []HeaderEntry `json:"h,omitempty"`
}

// HeaderEntry represents a single markdown header
type HeaderEntry struct {
	Level int    `json:"level,omitempty"`
	L     *int   `json:"l,omitempty"`
	Text  string `json:"text,omitempty"`
	T     string `json:"t,omitempty"`
}

// SummarizeDirHeadersResult represents the headers format result
type SummarizeDirHeadersResult struct {
	Directory string        `json:"directory,omitempty"`
	Dir       string        `json:"dir,omitempty"`
	Format    string        `json:"format,omitempty"`
	Fmt       string        `json:"fmt,omitempty"`
	Files     []FileHeaders `json:"files,omitempty"`
	F         []FileHeaders `json:"f,omitempty"`
	FileCount int           `json:"file_count,omitempty"`
	FC        *int          `json:"fc,omitempty"`
}

// newSummarizeDirCmd creates the summarize-dir command
func newSummarizeDirCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summarize-dir",
		Short: "Summarize directory contents",
		Long: `Generate a summary of directory contents.
Useful for providing context to LLMs about a codebase.

Formats:
  tree    - Directory tree with file types
  outline - Brief outline of each file (first N lines)
  headers - Extract markdown headers only (for .md files)
  full    - Full content (truncated)`,
		Args: cobra.NoArgs,
		RunE: runSummarizeDir,
	}
	cmd.Flags().StringVar(&summarizeDirPath, "path", ".", "Directory path to summarize")
	cmd.Flags().StringVar(&summarizeDirFormat, "format", "tree", "Output format: tree, outline, headers, full")
	cmd.Flags().BoolVarP(&summarizeDirRecursive, "recursive", "r", true, "Recursive scan")
	cmd.Flags().StringVar(&summarizeDirGlob, "glob", "", "File glob pattern")
	cmd.Flags().IntVar(&summarizeDirMaxTokens, "max-tokens", 4000, "Approximate max tokens")
	cmd.Flags().IntVar(&summarizeDirMaxLines, "lines", 10, "Max lines per file in outline mode")
	cmd.Flags().BoolVar(&summarizeDirNoGitignore, "no-gitignore", false, "Disable .gitignore filtering")
	cmd.Flags().BoolVar(&summarizeDirJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&summarizeDirMinimal, "min", false, "Output in minimal/token-optimized format")
	return cmd
}

func runSummarizeDir(cmd *cobra.Command, args []string) error {
	path, err := filepath.Abs(summarizeDirPath)
	if err != nil {
		return fmt.Errorf("invalid path: %s", summarizeDirPath)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", path)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	var ignorer *gitignore.Parser
	if !summarizeDirNoGitignore {
		ignorer, _ = gitignore.NewParser(path)
	}

	switch summarizeDirFormat {
	case "tree":
		return summarizeTree(cmd, path, ignorer)
	case "outline":
		return summarizeOutline(cmd, path, ignorer)
	case "headers":
		return summarizeHeaders(cmd, path, ignorer)
	case "full":
		return summarizeFull(cmd, path, ignorer)
	default:
		return fmt.Errorf("unknown format: %s (supported: tree, outline, headers, full)", summarizeDirFormat)
	}
}

func summarizeTree(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	type fileInfo struct {
		path string
		size int64
		ext  string
	}

	var files []fileInfo
	var dirs []string

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			dirs = append(dirs, relPath)
			if !summarizeDirRecursive {
				return filepath.SkipDir
			}
		} else {
			files = append(files, fileInfo{
				path: relPath,
				size: info.Size(),
				ext:  filepath.Ext(info.Name()),
			})
		}
		return nil
	}

	filepath.Walk(path, walkFn)

	sort.Strings(dirs)
	sort.Slice(files, func(i, j int) bool {
		return files[i].path < files[j].path
	})

	// Build result
	fileCount := len(files)
	dirCount := len(dirs)

	var result SummarizeDirResult
	if summarizeDirMinimal {
		fileInfos := make([]SummarizeDirFileInfo, len(files))
		for i, f := range files {
			size := f.size
			fileInfos[i] = SummarizeDirFileInfo{
				P: f.path,
				S: &size,
				E: f.ext,
			}
		}
		result = SummarizeDirResult{
			Dir:  path,
			Fmt:  "tree",
			Dirs: dirs,
			F:    fileInfos,
			FC:   &fileCount,
			DC:   &dirCount,
		}
	} else {
		fileInfos := make([]SummarizeDirFileInfo, len(files))
		for i, f := range files {
			fileInfos[i] = SummarizeDirFileInfo{
				Path: f.path,
				Size: f.size,
				Ext:  f.ext,
			}
		}
		result = SummarizeDirResult{
			Directory:   path,
			Format:      "tree",
			Directories: dirs,
			Files:       fileInfos,
			FileCount:   fileCount,
			DirCount:    dirCount,
		}
	}

	formatter := output.New(summarizeDirJSON, summarizeDirMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "DIRECTORY: %s\n\n", path)

		fmt.Fprintln(w, "DIRECTORIES:")
		for _, d := range dirs {
			fmt.Fprintf(w, "  %s/\n", d)
		}

		fmt.Fprintln(w, "\nFILES:")
		for _, f := range files {
			fmt.Fprintf(w, "  %s (%s)\n", f.path, utils.FormatSize(f.size))
		}

		fmt.Fprintf(w, "\nSUMMARY: %d directories, %d files\n", len(dirs), len(files))
	})
}

func summarizeOutline(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	var totalChars int
	maxChars := summarizeDirMaxTokens * 4 // Rough token-to-char ratio
	var contents []FileContent

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if totalChars >= maxChars {
			return filepath.SkipAll
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)

		file, err := os.Open(filePath)
		if err != nil {
			return nil
		}
		defer file.Close()

		// Read first N lines
		scanner := bufio.NewScanner(file)
		var lines []string
		for scanner.Scan() && len(lines) < summarizeDirMaxLines {
			lines = append(lines, scanner.Text())
		}

		if len(lines) > 0 {
			content := strings.Join(lines, "\n")
			contents = append(contents, FileContent{
				Path:    relPath,
				Content: content,
			})
			totalChars += len(content)
		}

		return nil
	}

	filepath.Walk(path, walkFn)

	// Build result
	var result SummarizeDirResult
	if summarizeDirMinimal {
		minContents := make([]FileContent, len(contents))
		for i, c := range contents {
			minContents[i] = FileContent{
				P: c.Path,
				C: c.Content,
			}
		}
		result = SummarizeDirResult{
			Dir: path,
			Fmt: "outline",
			C:   minContents,
		}
	} else {
		result = SummarizeDirResult{
			Directory: path,
			Format:    "outline",
			Contents:  contents,
		}
	}

	formatter := output.New(summarizeDirJSON, summarizeDirMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "DIRECTORY: %s\n", path)
		fmt.Fprintf(w, "FORMAT: outline (first %d lines per file)\n\n", summarizeDirMaxLines)

		for _, c := range contents {
			fmt.Fprintf(w, "--- %s ---\n", c.Path)
			fmt.Fprintln(w, c.Content)
			fmt.Fprintln(w)
		}
	})
}

func summarizeHeaders(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	var totalChars int
	maxChars := summarizeDirMaxTokens * 4
	var files []FileHeaders

	headerPattern := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if totalChars >= maxChars {
			return filepath.SkipAll
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			return nil
		}

		// Only process markdown files
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		// Apply glob filter if specified
		if summarizeDirGlob != "" {
			matched, _ := filepath.Match(summarizeDirGlob, info.Name())
			if !matched {
				return nil
			}
		}

		relPath, _ := filepath.Rel(path, filePath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		matches := headerPattern.FindAllStringSubmatch(string(content), -1)
		if len(matches) == 0 {
			return nil
		}

		var headers []HeaderEntry
		for _, match := range matches {
			level := len(match[1])
			text := match[2]
			headers = append(headers, HeaderEntry{
				Level: level,
				Text:  text,
			})
			totalChars += len(text) + level + 2 // account for # and space
		}

		files = append(files, FileHeaders{
			Path:    relPath,
			Headers: headers,
		})

		return nil
	}

	filepath.Walk(path, walkFn)

	// Build result
	fileCount := len(files)
	var result SummarizeDirHeadersResult
	if summarizeDirMinimal {
		minFiles := make([]FileHeaders, len(files))
		for i, f := range files {
			minHeaders := make([]HeaderEntry, len(f.Headers))
			for j, h := range f.Headers {
				lvl := h.Level
				minHeaders[j] = HeaderEntry{L: &lvl, T: h.Text}
			}
			minFiles[i] = FileHeaders{P: f.Path, H: minHeaders}
		}
		result = SummarizeDirHeadersResult{
			Dir: path,
			Fmt: "headers",
			F:   minFiles,
			FC:  &fileCount,
		}
	} else {
		result = SummarizeDirHeadersResult{
			Directory: path,
			Format:    "headers",
			Files:     files,
			FileCount: fileCount,
		}
	}

	formatter := output.New(summarizeDirJSON, summarizeDirMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "DIRECTORY: %s\n", path)
		fmt.Fprintln(w, "FORMAT: headers (markdown headers only)")
		fmt.Fprintln(w)

		for _, f := range files {
			fmt.Fprintf(w, "%s\n", f.Path)
			for _, h := range f.Headers {
				indent := strings.Repeat("  ", h.Level-1)
				hashes := strings.Repeat("#", h.Level)
				fmt.Fprintf(w, "%s%s %s\n", indent, hashes, h.Text)
			}
			fmt.Fprintln(w)
		}

		fmt.Fprintf(w, "SUMMARY: %d files with headers\n", fileCount)
	})
}

func summarizeFull(cmd *cobra.Command, path string, ignorer *gitignore.Parser) error {
	var totalChars int
	maxChars := summarizeDirMaxTokens * 4
	var contents []FileContent

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if totalChars >= maxChars {
			return filepath.SkipAll
		}

		// Skip hidden files
		if !summarizeDirNoGitignore && strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Check gitignore
		if ignorer != nil && ignorer.IsIgnored(filePath) {
			return nil
		}

		relPath, _ := filepath.Rel(path, filePath)

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		// Skip binary files
		if !isTextFile(content) {
			return nil
		}

		remaining := maxChars - totalChars
		text := string(content)
		if len(text) > remaining {
			text = text[:remaining] + "\n... (truncated)"
		}

		contents = append(contents, FileContent{
			Path:    relPath,
			Content: text,
		})

		totalChars += len(text)
		return nil
	}

	filepath.Walk(path, walkFn)

	// Build result
	var result SummarizeDirResult
	if summarizeDirMinimal {
		minContents := make([]FileContent, len(contents))
		for i, c := range contents {
			minContents[i] = FileContent{
				P: c.Path,
				C: c.Content,
			}
		}
		result = SummarizeDirResult{
			Dir: path,
			Fmt: "full",
			C:   minContents,
		}
	} else {
		result = SummarizeDirResult{
			Directory: path,
			Format:    "full",
			Contents:  contents,
		}
	}

	formatter := output.New(summarizeDirJSON, summarizeDirMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		fmt.Fprintf(w, "DIRECTORY: %s\n", path)
		fmt.Fprintln(w, "FORMAT: full (truncated to max tokens)")

		for _, c := range contents {
			fmt.Fprintf(w, "=== %s ===\n", c.Path)
			fmt.Fprintln(w, c.Content)
			fmt.Fprintln(w)
		}
	})
}

func init() {
	RootCmd.AddCommand(newSummarizeDirCmd())
}
