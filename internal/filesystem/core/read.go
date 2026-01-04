package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// DefaultMaxSize is the default maximum JSON output size in characters (70K)
// This accounts for Claude's ~75K character limit on tool responses
const DefaultMaxSize = 70000

// EstimateJSONStringSize estimates the size of a string after JSON encoding
// JSON escapes: \n, \t, \r, ", \, and control characters
func EstimateJSONStringSize(s string) int {
	size := len(s)
	for _, c := range s {
		switch c {
		case '"', '\\':
			size++ // These become \" or \\
		case '\n', '\t', '\r':
			size++ // These become \n, \t, \r
		default:
			// Control characters (< 0x20) become \uXXXX (6 chars)
			if c < 0x20 {
				size += 5 // Original char counts as 1, \uXXXX is 6, so add 5
			}
		}
	}
	return size
}

// SizeExceededError represents an error when estimated JSON output size exceeds the limit
type SizeExceededError struct {
	Message           string `json:"message"`
	Path              string `json:"path"`
	Size              int64  `json:"size"`                // Raw file size in bytes
	EstimatedJSONSize int64  `json:"estimated_json_size"` // Estimated size after JSON encoding
	MaxSize           int64  `json:"max_size"`            // Maximum allowed JSON size
}

func (e *SizeExceededError) Error() string {
	return e.Message
}

// ToJSON returns the error as a JSON object with error: true
func (e *SizeExceededError) ToJSON() string {
	result := map[string]interface{}{
		"error":               true,
		"message":             e.Message,
		"path":                e.Path,
		"size":                e.Size,
		"estimated_json_size": e.EstimatedJSONSize,
		"max_size":            e.MaxSize,
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// TotalSizeExceededError represents an error when combined estimated JSON size exceeds the limit
type TotalSizeExceededError struct {
	Message           string          `json:"message"`
	TotalSize         int64           `json:"total_size"`          // Raw total size in bytes
	EstimatedJSONSize int64           `json:"estimated_json_size"` // Estimated size after JSON encoding
	MaxTotalSize      int64           `json:"max_total_size"`      // Maximum allowed JSON size
	Files             []FileSizeEntry `json:"files"`
}

// FileSizeEntry represents a file and its size
type FileSizeEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func (e *TotalSizeExceededError) Error() string {
	return e.Message
}

// ToJSON returns the error as a JSON object with error: true
func (e *TotalSizeExceededError) ToJSON() string {
	result := map[string]interface{}{
		"error":               true,
		"message":             e.Message,
		"total_size":          e.TotalSize,
		"estimated_json_size": e.EstimatedJSONSize,
		"max_total_size":      e.MaxTotalSize,
		"files":               e.Files,
	}
	jsonBytes, _ := json.Marshal(result)
	return string(jsonBytes)
}

// ReadFileOptions contains input parameters for ReadFile
type ReadFileOptions struct {
	Path             string
	StartOffset      int
	MaxSize          int
	LineStart        int
	LineCount        int
	AllowedDirs      []string
	SizeCheckMaxSize int64 // Maximum allowed JSON output size (0 = use default, -1 = no limit, >0 = custom)
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

	// Determine max size limit (0 = use default, -1 = no limit, >0 = custom)
	maxSize := opts.SizeCheckMaxSize
	if maxSize == 0 {
		maxSize = DefaultMaxSize
	}
	// maxSize is now: DefaultMaxSize, -1 (no limit), or custom value

	// Pre-check: fail-fast if raw file size already exceeds limit
	// JSON encoding only adds overhead, never reduces size, so if raw > limit, JSON will definitely > limit
	// Skip check if maxSize == -1 (no limit)
	if maxSize > 0 && info.Size() > maxSize {
		return nil, &SizeExceededError{
			Message:           fmt.Sprintf("File size (%d bytes) exceeds max_size (%d chars)", info.Size(), maxSize),
			Path:              path,
			Size:              info.Size(),
			EstimatedJSONSize: 0, // Not calculated for fail-fast
			MaxSize:           maxSize,
		}
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

	// Post-read check: estimate JSON size to catch files with high encoding overhead
	if maxSize > 0 {
		estimatedJSONSize := int64(EstimateJSONStringSize(content))
		if estimatedJSONSize > maxSize {
			return nil, &SizeExceededError{
				Message:           fmt.Sprintf("Estimated JSON size (%d chars) exceeds max_size (%d chars). Raw file: %d bytes", estimatedJSONSize, maxSize, len(content)),
				Path:              path,
				Size:              int64(len(content)),
				EstimatedJSONSize: estimatedJSONSize,
				MaxSize:           maxSize,
			}
		}
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
	Paths                 []string
	AllowedDirs           []string
	SizeCheckMaxTotalSize int64 // Maximum allowed total JSON output size (0 = use default, -1 = no limit, >0 = custom)
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

	// Determine max total size (0 = use default, -1 = no limit, >0 = custom)
	maxTotalSize := opts.SizeCheckMaxTotalSize
	if maxTotalSize == 0 {
		maxTotalSize = DefaultMaxSize
	}
	// maxTotalSize is now: DefaultMaxSize, -1 (no limit), or custom value

	// Pre-check: fail-fast if total raw file size already exceeds limit
	// JSON encoding only adds overhead, so if raw total > limit, JSON total will definitely > limit
	// Skip check if maxTotalSize == -1 (no limit)
	if maxTotalSize > 0 {
		var totalRawSize int64
		fileSizes := make([]FileSizeEntry, 0, len(opts.Paths))

		for _, path := range opts.Paths {
			resolved, _ := ResolveSymlink(path)
			if resolved != "" {
				path = resolved
			}

			info, err := os.Stat(path)
			if err != nil {
				fileSizes = append(fileSizes, FileSizeEntry{Path: path, Size: 0})
				continue
			}
			if info.IsDir() {
				fileSizes = append(fileSizes, FileSizeEntry{Path: path, Size: 0})
				continue
			}

			totalRawSize += info.Size()
			fileSizes = append(fileSizes, FileSizeEntry{Path: path, Size: info.Size()})
		}

		if totalRawSize > maxTotalSize {
			return nil, &TotalSizeExceededError{
				Message:           fmt.Sprintf("Total file size (%d bytes) exceeds max_total_size (%d chars)", totalRawSize, maxTotalSize),
				TotalSize:         totalRawSize,
				EstimatedJSONSize: 0, // Not calculated for fail-fast
				MaxTotalSize:      maxTotalSize,
				Files:             fileSizes,
			}
		}
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

	// Check estimated total JSON size after reading all files (smarter than raw byte limits)
	if maxTotalSize > 0 {
		var totalRawSize int64
		var totalEstimatedJSONSize int64
		fileSizes := make([]FileSizeEntry, 0, len(results))

		for _, r := range results {
			if r.Error == "" {
				totalRawSize += r.Size
				totalEstimatedJSONSize += int64(EstimateJSONStringSize(r.Content))
			}
			fileSizes = append(fileSizes, FileSizeEntry{Path: r.Path, Size: r.Size})
		}

		if totalEstimatedJSONSize > maxTotalSize {
			return nil, &TotalSizeExceededError{
				Message:           fmt.Sprintf("Estimated total JSON size (%d chars) exceeds max_total_size (%d chars). Raw total: %d bytes", totalEstimatedJSONSize, maxTotalSize, totalRawSize),
				TotalSize:         totalRawSize,
				EstimatedJSONSize: totalEstimatedJSONSize,
				MaxTotalSize:      maxTotalSize,
				Files:             fileSizes,
			}
		}
	}

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
