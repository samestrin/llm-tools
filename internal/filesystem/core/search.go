package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileSearchResult represents a found file
type FileSearchResult struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime string `json:"mod_time"`
}

// SearchFilesOptions contains input parameters for SearchFiles
type SearchFilesOptions struct {
	Path        string
	Pattern     string
	Recursive   bool
	ShowHidden  bool
	MaxResults  int
	AllowedDirs []string
}

// SearchFilesResult represents the result of a file search
type SearchFilesResult struct {
	Pattern string             `json:"pattern"`
	Path    string             `json:"path"`
	Matches []FileSearchResult `json:"matches"`
	Total   int                `json:"total"`
}

// SearchFiles searches for files by name pattern
func SearchFiles(opts SearchFilesOptions) (*SearchFilesResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 1000
	}

	var matches []FileSearchResult

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		name := info.Name()

		// Skip hidden unless requested
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories in non-recursive mode
		if !opts.Recursive && info.IsDir() && filePath != normalizedPath {
			return filepath.SkipDir
		}

		// Match pattern against filename
		matched, _ := filepath.Match(opts.Pattern, name)
		if !matched {
			return nil
		}

		if len(matches) >= maxResults {
			return filepath.SkipAll
		}

		matches = append(matches, FileSearchResult{
			Path:    filePath,
			Name:    name,
			Size:    info.Size(),
			IsDir:   info.IsDir(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		})

		return nil
	}

	if err := filepath.Walk(normalizedPath, walkFn); err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	return &SearchFilesResult{
		Pattern: opts.Pattern,
		Path:    normalizedPath,
		Matches: matches,
		Total:   len(matches),
	}, nil
}

// CodeMatch represents a single code match
type CodeMatch struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Content string   `json:"content"`
	Context []string `json:"context,omitempty"`
}

// SearchCodeOptions contains input parameters for SearchCode
type SearchCodeOptions struct {
	Path            string
	Pattern         string
	CaseInsensitive bool
	Regex           bool
	ContextLines    int
	FileTypes       []string
	MaxResults      int
	ShowHidden      bool
	AllowedDirs     []string
}

// SearchCodeResult represents the result of a code search
type SearchCodeResult struct {
	Pattern      string      `json:"pattern"`
	Path         string      `json:"path"`
	Matches      []CodeMatch `json:"matches"`
	TotalFiles   int         `json:"total_files"`
	TotalMatches int         `json:"total_matches"`
}

// SearchCode searches for patterns in file contents
func SearchCode(opts SearchCodeOptions) (*SearchCodeResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}
	if opts.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	maxResults := opts.MaxResults
	if maxResults == 0 {
		maxResults = 1000
	}

	// Prepare search pattern
	var searchFn func(line string) bool

	if opts.Regex {
		flags := ""
		if opts.CaseInsensitive {
			flags = "(?i)"
		}
		re, err := regexp.Compile(flags + opts.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		searchFn = func(line string) bool {
			return re.MatchString(line)
		}
	} else {
		searchPattern := opts.Pattern
		if opts.CaseInsensitive {
			searchPattern = strings.ToLower(opts.Pattern)
			searchFn = func(line string) bool {
				return strings.Contains(strings.ToLower(line), searchPattern)
			}
		} else {
			searchFn = func(line string) bool {
				return strings.Contains(line, opts.Pattern)
			}
		}
	}

	var matches []CodeMatch
	filesWithMatches := make(map[string]bool)

	walkFn := func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		name := info.Name()

		// Skip hidden unless requested
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			return nil
		}

		// Check file type filter
		if len(opts.FileTypes) > 0 {
			ext := filepath.Ext(name)
			matched := false
			for _, ft := range opts.FileTypes {
				if ext == ft || "."+ext == ft || ext == "."+ft {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// Skip binary files (simple heuristic)
		if isBinaryFile(filePath) {
			return nil
		}

		// Search in file
		fileMatches, err := searchInFile(filePath, searchFn, opts.ContextLines)
		if err != nil {
			return nil // Skip files that can't be read
		}

		for _, m := range fileMatches {
			if len(matches) >= maxResults {
				return filepath.SkipAll
			}
			m.File = filePath
			matches = append(matches, m)
			filesWithMatches[filePath] = true
		}

		return nil
	}

	if err := filepath.Walk(normalizedPath, walkFn); err != nil {
		return nil, fmt.Errorf("failed to search code: %w", err)
	}

	return &SearchCodeResult{
		Pattern:      opts.Pattern,
		Path:         normalizedPath,
		Matches:      matches,
		TotalFiles:   len(filesWithMatches),
		TotalMatches: len(matches),
	}, nil
}

func searchInFile(filePath string, matchFn func(string) bool, contextLines int) ([]CodeMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []CodeMatch
	var lines []string
	scanner := bufio.NewScanner(file)

	// Read all lines if context is needed
	if contextLines > 0 {
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		for i, line := range lines {
			if matchFn(line) {
				start := max(0, i-contextLines)
				end := min(len(lines), i+contextLines+1)
				context := make([]string, 0, end-start)
				for j := start; j < end; j++ {
					context = append(context, lines[j])
				}
				matches = append(matches, CodeMatch{
					Line:    i + 1,
					Content: line,
					Context: context,
				})
			}
		}
	} else {
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if matchFn(line) {
				matches = append(matches, CodeMatch{
					Line:    lineNum,
					Content: line,
				})
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}

	return matches, nil
}

func isBinaryFile(path string) bool {
	// Check common binary extensions
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".bin": true, ".dat": true, ".db": true, ".sqlite": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".pdf": true, ".doc": true, ".docx": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
	}
	if binaryExts[ext] {
		return true
	}

	// Quick check of file content (first 512 bytes)
	file, err := os.Open(path)
	if err != nil {
		return true // Assume binary if can't read
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return true
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	return false
}
