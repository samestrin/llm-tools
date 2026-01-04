package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	analyzeDepsJSON    bool
	analyzeDepsMinimal bool
)

// newAnalyzeDepsCmd creates the analyze-deps command
func newAnalyzeDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze-deps <file>",
		Short: "Analyze file dependencies from markdown",
		Long: `Analyze file dependencies from a user story or task markdown file.
Extracts file references and categorizes them by action type.

Output:
  FILES_READ: files that need to be read
  FILES_MODIFY: files that need to be modified
  FILES_CREATE: files that need to be created
  DIRECTORIES: directories referenced
  TOTAL_FILES: total file count
  CONFIDENCE: high | medium | low`,
		Args: cobra.ExactArgs(1),
		RunE: runAnalyzeDeps,
	}

	cmd.Flags().BoolVar(&analyzeDepsJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&analyzeDepsMinimal, "min", false, "Output in minimal/token-optimized format")

	return cmd
}

// AnalyzeDepsResult represents the JSON output of the analyze-deps command.
type AnalyzeDepsResult struct {
	// Standard mode fields (always included when not minimal)
	FilesRead   []string `json:"files_read"`
	FilesModify []string `json:"files_modify"`
	FilesCreate []string `json:"files_create"`
	Directories []string `json:"directories"`
	TotalFiles  *int     `json:"total_files,omitempty"`
	Confidence  string   `json:"confidence"`
	// Minimal mode fields
	FR    []string `json:"fr,omitempty"`
	FM    []string `json:"fm,omitempty"`
	FC    []string `json:"fc,omitempty"`
	Dirs  []string `json:"dirs,omitempty"`
	Total *int     `json:"total,omitempty"`
	Conf  string   `json:"conf,omitempty"`
}

func runAnalyzeDeps(cmd *cobra.Command, args []string) error {
	filePath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	text := string(content)

	filesRead := make(map[string]bool)
	filesModify := make(map[string]bool)
	filesCreate := make(map[string]bool)
	directories := make(map[string]bool)

	// Patterns for file extraction
	backtickPattern := regexp.MustCompile("`([^`]+\\.[a-zA-Z]{1,10})`")
	dirPattern := regexp.MustCompile(`(?:^|[\s'"(])([a-zA-Z0-9_\-./]+/)(?:[\s'")]|$)`)

	// Action patterns
	createPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[Cc]reate\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Nn]ew\s+file[:\s]+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Aa]dd\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
	}

	modifyPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[Mm]odify\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Uu]pdate\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Ee]dit\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Cc]hange\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
	}

	readPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[Rr]ead\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Rr]eference\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
		regexp.MustCompile(`[Ss]ee\s+[` + "`" + `"']?([^` + "`" + `"']+\.[a-zA-Z]{1,10})[` + "`" + `"']?`),
	}

	// Extract files with action context
	for _, pattern := range createPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if isValidFilePath(match[1]) {
				filesCreate[match[1]] = true
			}
		}
	}

	for _, pattern := range modifyPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if isValidFilePath(match[1]) {
				filesModify[match[1]] = true
			}
		}
	}

	for _, pattern := range readPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if isValidFilePath(match[1]) {
				filesRead[match[1]] = true
			}
		}
	}

	// Extract backtick references (default to modify)
	matches := backtickPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		fileRef := match[1]
		if isValidFilePath(fileRef) && !filesCreate[fileRef] && !filesRead[fileRef] {
			filesModify[fileRef] = true
		}
	}

	// Extract directories
	dirMatches := dirPattern.FindAllStringSubmatch(text, -1)
	for _, match := range dirMatches {
		if isValidDirPath(match[1]) {
			directories[match[1]] = true
		}
	}

	// Remove overlap (create > modify > read)
	for f := range filesCreate {
		delete(filesModify, f)
		delete(filesRead, f)
	}
	for f := range filesModify {
		delete(filesRead, f)
	}

	// Convert to sorted slices
	readList := mapToSortedSlice(filesRead)
	modifyList := mapToSortedSlice(filesModify)
	createList := mapToSortedSlice(filesCreate)
	dirList := mapToSortedSlice(directories)

	totalFiles := len(readList) + len(modifyList) + len(createList)

	confidence := "low"
	if totalFiles >= 3 {
		confidence = "high"
	} else if totalFiles >= 1 {
		confidence = "medium"
	}

	// Build result - ensure empty slices are initialized (not nil) for JSON output
	if readList == nil {
		readList = []string{}
	}
	if modifyList == nil {
		modifyList = []string{}
	}
	if createList == nil {
		createList = []string{}
	}
	if dirList == nil {
		dirList = []string{}
	}

	var result AnalyzeDepsResult
	if analyzeDepsMinimal {
		result = AnalyzeDepsResult{
			FR:    readList,
			FM:    modifyList,
			FC:    createList,
			Dirs:  dirList,
			Total: &totalFiles,
			Conf:  confidence,
		}
	} else {
		result = AnalyzeDepsResult{
			FilesRead:   readList,
			FilesModify: modifyList,
			FilesCreate: createList,
			Directories: dirList,
			TotalFiles:  &totalFiles,
			Confidence:  confidence,
		}
	}

	formatter := output.New(analyzeDepsJSON, analyzeDepsMinimal, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(AnalyzeDepsResult)
		fr := r.FilesRead
		if r.FR != nil {
			fr = r.FR
		}
		fm := r.FilesModify
		if r.FM != nil {
			fm = r.FM
		}
		fc := r.FilesCreate
		if r.FC != nil {
			fc = r.FC
		}
		dirs := r.Directories
		if r.Dirs != nil {
			dirs = r.Dirs
		}
		var total int
		if r.TotalFiles != nil {
			total = *r.TotalFiles
		} else if r.Total != nil {
			total = *r.Total
		}
		conf := r.Confidence
		if r.Conf != "" {
			conf = r.Conf
		}

		fmt.Fprintf(w, "FILES_READ: %s\n", formatList(fr))
		fmt.Fprintf(w, "FILES_MODIFY: %s\n", formatList(fm))
		fmt.Fprintf(w, "FILES_CREATE: %s\n", formatList(fc))
		fmt.Fprintf(w, "DIRECTORIES: %s\n", formatList(dirs))
		fmt.Fprintf(w, "TOTAL_FILES: %d\n", total)
		fmt.Fprintf(w, "CONFIDENCE: %s\n", conf)
	})
}

func isValidFilePath(path string) bool {
	if len(path) < 3 || len(path) > 200 {
		return false
	}
	if !strings.Contains(path, ".") {
		return false
	}

	validExtensions := map[string]bool{
		".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true,
		".py": true, ".go": true, ".rs": true, ".java": true, ".kt": true,
		".md": true, ".json": true, ".toml": true, ".yaml": true, ".yml": true,
		".css": true, ".scss": true, ".html": true, ".vue": true, ".svelte": true,
		".sql": true, ".sh": true, ".bash": true, ".zsh": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rb": true, ".php": true, ".swift": true,
	}

	ext := filepath.Ext(path)
	return validExtensions[ext]
}

func isValidDirPath(path string) bool {
	if len(path) < 2 || len(path) > 100 {
		return false
	}
	if !strings.HasSuffix(path, "/") {
		return false
	}
	// Exclude common false positives
	exclude := map[string]bool{
		"http://": true, "https://": true, "file://": true,
	}
	return !exclude[path]
}

func mapToSortedSlice(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func init() {
	RootCmd.AddCommand(newAnalyzeDepsCmd())
}
