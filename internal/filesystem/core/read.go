package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ReadFileOptions contains input parameters for ReadFile
type ReadFileOptions struct {
	Path        string
	StartOffset int
	MaxSize     int
	LineStart   int
	LineCount   int
	AllowedDirs []string
}

// ReadFileResult represents the result of a file read operation
type ReadFileResult struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Lines     int    `json:"lines,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ReadFile reads a file with optional line range or byte offset
func ReadFile(opts ReadFileOptions) (*ReadFileResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Resolve symlinks and normalize path
	path := opts.Path
	resolved, err := ResolveSymlink(path)
	if err == nil {
		path = resolved
	}

	// Validate path against allowed directories
	if err := ValidatePath(path, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	var content string
	var lines int

	// Read by lines or bytes
	if opts.LineStart > 0 || opts.LineCount > 0 {
		content, lines, err = readFileByLines(path, opts.LineStart, opts.LineCount)
	} else if opts.StartOffset > 0 || opts.MaxSize > 0 {
		content, err = readFileByBytes(path, opts.StartOffset, opts.MaxSize)
	} else {
		content, lines, err = readEntireFile(path)
	}

	if err != nil {
		return nil, err
	}

	return &ReadFileResult{
		Path:    path,
		Content: content,
		Size:    int64(len(content)),
		Lines:   lines,
	}, nil
}

func readEntireFile(path string) (string, int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read file: %w", err)
	}
	lines := strings.Count(string(content), "\n")
	return string(content), lines, nil
}

func readFileByLines(path string, startLine, lineCount int) (string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(file)
	currentLine := 0
	linesRead := 0

	// Handle 1-indexed line numbers
	if startLine < 1 {
		startLine = 1
	}

	for scanner.Scan() {
		currentLine++
		if currentLine >= startLine {
			if lineCount > 0 && linesRead >= lineCount {
				break
			}
			result.WriteString(scanner.Text())
			result.WriteString("\n")
			linesRead++
		}
	}

	if err := scanner.Err(); err != nil {
		return "", 0, fmt.Errorf("error reading file: %w", err)
	}

	return result.String(), linesRead, nil
}

func readFileByBytes(path string, startOffset, maxSize int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if startOffset > 0 {
		_, err = file.Seek(int64(startOffset), 0)
		if err != nil {
			return "", fmt.Errorf("failed to seek: %w", err)
		}
	}

	var buffer []byte
	if maxSize > 0 {
		buffer = make([]byte, maxSize)
		n, err := file.Read(buffer)
		if err != nil && err.Error() != "EOF" {
			return "", fmt.Errorf("failed to read: %w", err)
		}
		buffer = buffer[:n]
	} else {
		buffer, err = os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		if startOffset > 0 && startOffset < len(buffer) {
			buffer = buffer[startOffset:]
		}
	}

	return string(buffer), nil
}

// ReadMultipleFilesOptions contains input parameters for ReadMultipleFiles
type ReadMultipleFilesOptions struct {
	Paths       []string
	AllowedDirs []string
}

// ReadMultipleFilesResult represents results from reading multiple files
type ReadMultipleFilesResult struct {
	Files   []ReadFileResult `json:"files"`
	Success int              `json:"success"`
	Failed  int              `json:"failed"`
}

// ReadMultipleFiles reads multiple files concurrently
func ReadMultipleFiles(opts ReadMultipleFilesOptions) (*ReadMultipleFilesResult, error) {
	if len(opts.Paths) == 0 {
		return nil, fmt.Errorf("paths is required")
	}

	results := make([]ReadFileResult, len(opts.Paths))
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	failed := 0

	for i, path := range opts.Paths {
		wg.Add(1)
		go func(idx int, filePath string) {
			defer wg.Done()

			// Resolve and validate
			resolved, _ := ResolveSymlink(filePath)
			if resolved != "" {
				filePath = resolved
			}

			result := ReadFileResult{Path: filePath}

			if err := ValidatePath(filePath, opts.AllowedDirs); err != nil {
				result.Error = err.Error()
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				content, lines, err := readEntireFile(filePath)
				if err != nil {
					result.Error = err.Error()
					mu.Lock()
					failed++
					mu.Unlock()
				} else {
					result.Content = content
					result.Size = int64(len(content))
					result.Lines = lines
					mu.Lock()
					success++
					mu.Unlock()
				}
			}

			mu.Lock()
			results[idx] = result
			mu.Unlock()
		}(i, path)
	}

	wg.Wait()

	return &ReadMultipleFilesResult{
		Files:   results,
		Success: success,
		Failed:  failed,
	}, nil
}

// ExtractLinesOptions contains input parameters for ExtractLines
type ExtractLinesOptions struct {
	Path         string
	LineNumbers  []int
	StartLine    int
	EndLine      int
	Pattern      string
	ContextLines int
	AllowedDirs  []string
}

// ExtractLinesResult represents the result of extracting lines
type ExtractLinesResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Lines   int    `json:"lines"`
}

// ExtractLines extracts specific lines from a file
func ExtractLines(opts ExtractLinesOptions) (*ExtractLinesResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Validate path
	path := opts.Path
	resolved, _ := ResolveSymlink(path)
	if resolved != "" {
		path = resolved
	}
	if err := ValidatePath(path, opts.AllowedDirs); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(file)
	currentLine := 0
	var allLines []string

	// Read all lines first if we need context or pattern matching
	if opts.Pattern != "" || opts.ContextLines > 0 {
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
	}

	linesExtracted := 0

	// Extract by line numbers
	if len(opts.LineNumbers) > 0 {
		lineSet := make(map[int]bool)
		for _, ln := range opts.LineNumbers {
			lineSet[ln] = true
		}

		if len(allLines) == 0 {
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
			for scanner.Scan() {
				currentLine++
				if lineSet[currentLine] {
					result.WriteString(fmt.Sprintf("%d: %s\n", currentLine, scanner.Text()))
					linesExtracted++
				}
			}
		} else {
			for ln := range lineSet {
				if ln > 0 && ln <= len(allLines) {
					result.WriteString(fmt.Sprintf("%d: %s\n", ln, allLines[ln-1]))
					linesExtracted++
				}
			}
		}
	} else if opts.StartLine > 0 && opts.EndLine > 0 {
		// Extract by range
		if len(allLines) == 0 {
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
			for scanner.Scan() {
				currentLine++
				if currentLine >= opts.StartLine && currentLine <= opts.EndLine {
					result.WriteString(fmt.Sprintf("%d: %s\n", currentLine, scanner.Text()))
					linesExtracted++
				}
				if currentLine > opts.EndLine {
					break
				}
			}
		} else {
			for i := opts.StartLine; i <= opts.EndLine && i <= len(allLines); i++ {
				if i > 0 {
					result.WriteString(fmt.Sprintf("%d: %s\n", i, allLines[i-1]))
					linesExtracted++
				}
			}
		}
	} else if opts.Pattern != "" {
		// Extract by pattern
		for i, line := range allLines {
			if strings.Contains(line, opts.Pattern) {
				lineNum := i + 1
				start := max(0, i-opts.ContextLines)
				end := min(len(allLines), i+opts.ContextLines+1)
				for j := start; j < end; j++ {
					result.WriteString(fmt.Sprintf("%d: %s\n", j+1, allLines[j]))
					linesExtracted++
				}
				if end < len(allLines) {
					result.WriteString(fmt.Sprintf("-- match at line %d --\n", lineNum))
				}
			}
		}
	}

	return &ExtractLinesResult{
		Path:    path,
		Content: result.String(),
		Lines:   linesExtracted,
	}, nil
}
