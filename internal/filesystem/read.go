package filesystem

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/samestrin/llm-tools/internal/filesystem/core"
)

// ReadFileResult represents the result of a file read operation
type ReadFileResult struct {
	Path              string `json:"path"`
	Content           string `json:"content"`
	Size              int64  `json:"size"`
	Encoding          string `json:"encoding"`
	TotalSize         int64  `json:"total_size,omitempty"`
	Lines             int    `json:"lines,omitempty"`
	Truncated         bool   `json:"truncated,omitempty"`
	AutoChunked       bool   `json:"auto_chunked,omitempty"`
	ChunkIndex        int    `json:"chunk_index,omitempty"`
	TotalChunks       int    `json:"total_chunks,omitempty"`
	ContinuationToken string `json:"continuation_token,omitempty"`
	HasMore           bool   `json:"has_more,omitempty"`
	Error             string `json:"error,omitempty"`
}

// ReadMultipleFilesResult represents results from reading multiple files
type ReadMultipleFilesResult struct {
	Files   []ReadFileResult `json:"files"`
	Success int              `json:"success"`
	Failed  int              `json:"failed"`
}

// Default chunk size for auto-chunking (1MB)
const DefaultChunkSize = 1024 * 1024

func (s *Server) handleReadFile(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve symlinks and normalize path
	resolved, err := ResolveSymlink(path)
	if err == nil {
		path = resolved
	}

	// Validate path against allowed directories
	if err := ValidatePath(path, s.allowedDirs); err != nil {
		return "", err
	}

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}

	totalSize := info.Size()

	// Read options
	startOffset := GetInt(args, "start_offset", 0)
	maxSize := GetInt(args, "max_size", 0)
	lineStart := GetInt(args, "line_start", 0)
	lineCount := GetInt(args, "line_count", 0)
	autoChunk := GetBool(args, "auto_chunk", true)
	continuationTokenStr := GetString(args, "continuation_token", "")

	// Determine if size checking should be applied
	// If max_size > 0 is provided, it's used for chunking (read this many bytes)
	// If max_size == 0 explicitly, it means "no limit, read everything"
	// If max_size is not provided at all, apply default size limit
	_, maxSizeProvided := args["max_size"]

	// Pre-check: fail-fast if raw file size already exceeds limit
	// JSON encoding only adds overhead, so if raw > limit, JSON will definitely > limit
	if startOffset == 0 && lineStart == 0 && lineCount == 0 && continuationTokenStr == "" && !maxSizeProvided {
		if totalSize > core.DefaultMaxSize {
			return "", &core.SizeExceededError{
				Message:           fmt.Sprintf("File size (%d bytes) exceeds max_size (%d chars)", totalSize, core.DefaultMaxSize),
				Path:              path,
				Size:              totalSize,
				EstimatedJSONSize: 0, // Not calculated for fail-fast
				MaxSize:           core.DefaultMaxSize,
			}
		}
	}

	// If continuation token provided, decode and use its offset
	if continuationTokenStr != "" {
		token, err := DecodeContinuationToken(continuationTokenStr)
		if err != nil {
			return "", fmt.Errorf("invalid continuation token: %w", err)
		}
		if err := ValidateToken(token, path, "read"); err != nil {
			return "", err
		}
		startOffset = int(token.Offset)
	}

	// Determine effective max size for auto-chunking
	effectiveMaxSize := maxSize
	if autoChunk && effectiveMaxSize == 0 && totalSize > int64(DefaultChunkSize) {
		effectiveMaxSize = DefaultChunkSize
	}

	var content string
	var lines int

	// Read by lines or bytes
	if lineStart > 0 || lineCount > 0 {
		content, lines, err = readFileByLines(path, lineStart, lineCount)
	} else if startOffset > 0 || effectiveMaxSize > 0 {
		content, err = readFileByBytes(path, startOffset, effectiveMaxSize)
	} else {
		content, lines, err = readEntireFile(path)
	}

	if err != nil {
		return "", err
	}

	// Post-read check: estimate JSON size to catch files with high encoding overhead
	// Only apply when reading whole file without explicit chunking params
	if startOffset == 0 && lineStart == 0 && lineCount == 0 && continuationTokenStr == "" && !maxSizeProvided {
		estimatedJSONSize := int64(core.EstimateJSONStringSize(content))
		if estimatedJSONSize > core.DefaultMaxSize {
			return "", &core.SizeExceededError{
				Message:           fmt.Sprintf("Estimated JSON size (%d chars) exceeds max_size (%d chars). Raw file: %d bytes", estimatedJSONSize, core.DefaultMaxSize, len(content)),
				Path:              path,
				Size:              int64(len(content)),
				EstimatedJSONSize: estimatedJSONSize,
				MaxSize:           core.DefaultMaxSize,
			}
		}
	}

	result := ReadFileResult{
		Path:      path,
		Content:   content,
		Size:      int64(len(content)),
		Encoding:  "utf-8",
		TotalSize: totalSize,
		Lines:     lines,
	}

	// Calculate chunking metadata
	bytesRead := len(content)
	nextOffset := int64(startOffset) + int64(bytesRead)

	if autoChunk && effectiveMaxSize > 0 && totalSize > int64(effectiveMaxSize) {
		result.AutoChunked = true
		result.ChunkIndex = startOffset / effectiveMaxSize
		result.TotalChunks = int((totalSize + int64(effectiveMaxSize) - 1) / int64(effectiveMaxSize))

		// Generate continuation token if more data available
		if nextOffset < totalSize {
			result.HasMore = true
			result.ContinuationToken, _ = CreateReadToken(path, nextOffset)
		}
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
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

func (s *Server) handleReadMultipleFiles(args map[string]interface{}) (string, error) {
	paths := GetStringSlice(args, "paths")
	if len(paths) == 0 {
		return "", fmt.Errorf("paths is required")
	}

	// Check total size limit (0 = use default, -1 = no limit, >0 = custom)
	sizeLimit := int64(0) // Use default
	if maxSizeVal, ok := args["max_total_size"]; ok {
		if ms, ok := maxSizeVal.(float64); ok {
			sizeLimit = int64(ms)
		}
	}

	// Determine effective limit
	effectiveLimit := sizeLimit
	if effectiveLimit == 0 {
		effectiveLimit = core.DefaultMaxSize
	}
	// effectiveLimit is now: DefaultMaxSize, -1 (no limit), or custom value

	// If size checking is enabled, stat all files first
	if effectiveLimit > 0 {
		var totalSize int64
		fileSizes := make([]core.FileSizeEntry, 0, len(paths))

		for _, path := range paths {
			// Resolve symlink
			resolved, _ := ResolveSymlink(path)
			if resolved != "" {
				path = resolved
			}

			info, err := os.Stat(path)
			if err != nil {
				// Skip files that can't be stat'd - they'll error during read
				fileSizes = append(fileSizes, core.FileSizeEntry{Path: path, Size: 0})
				continue
			}

			if info.IsDir() {
				fileSizes = append(fileSizes, core.FileSizeEntry{Path: path, Size: 0})
				continue
			}

			totalSize += info.Size()
			fileSizes = append(fileSizes, core.FileSizeEntry{Path: path, Size: info.Size()})
		}

		// Pre-check: fail-fast if raw total size already exceeds limit
		if totalSize > effectiveLimit {
			return "", &core.TotalSizeExceededError{
				Message:           fmt.Sprintf("Total file size (%d bytes) exceeds max_total_size (%d chars)", totalSize, effectiveLimit),
				TotalSize:         totalSize,
				EstimatedJSONSize: 0, // Not calculated for fail-fast
				MaxTotalSize:      effectiveLimit,
				Files:             fileSizes,
			}
		}
	}

	results := make([]ReadFileResult, len(paths))
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	failed := 0

	for i, path := range paths {
		wg.Add(1)
		go func(idx int, filePath string) {
			defer wg.Done()

			// Resolve and validate
			resolved, _ := ResolveSymlink(filePath)
			if resolved != "" {
				filePath = resolved
			}

			result := ReadFileResult{Path: filePath, Encoding: "utf-8"}

			if err := ValidatePath(filePath, s.allowedDirs); err != nil {
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

	// Post-read check: estimate total JSON size to catch files with high encoding overhead
	if effectiveLimit > 0 {
		var totalRawSize int64
		var totalEstimatedJSONSize int64
		fileSizes := make([]core.FileSizeEntry, 0, len(results))

		for _, r := range results {
			if r.Error == "" {
				totalRawSize += r.Size
				totalEstimatedJSONSize += int64(core.EstimateJSONStringSize(r.Content))
			}
			fileSizes = append(fileSizes, core.FileSizeEntry{Path: r.Path, Size: r.Size})
		}

		if totalEstimatedJSONSize > effectiveLimit {
			return "", &core.TotalSizeExceededError{
				Message:           fmt.Sprintf("Estimated total JSON size (%d chars) exceeds max_total_size (%d chars). Raw total: %d bytes", totalEstimatedJSONSize, effectiveLimit, totalRawSize),
				TotalSize:         totalRawSize,
				EstimatedJSONSize: totalEstimatedJSONSize,
				MaxTotalSize:      effectiveLimit,
				Files:             fileSizes,
			}
		}
	}

	response := ReadMultipleFilesResult{
		Files:   results,
		Success: success,
		Failed:  failed,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleExtractLines(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Validate path
	resolved, _ := ResolveSymlink(path)
	if resolved != "" {
		path = resolved
	}
	if err := ValidatePath(path, s.allowedDirs); err != nil {
		return "", err
	}

	// Get extraction parameters
	lineNumbers := GetIntSlice(args, "line_numbers")
	startLine := GetInt(args, "start_line", 0)
	endLine := GetInt(args, "end_line", 0)
	pattern := GetString(args, "pattern", "")
	contextLines := GetInt(args, "context_lines", 0)

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var result strings.Builder
	scanner := bufio.NewScanner(file)
	currentLine := 0
	var allLines []string

	// Read all lines first if we need context or pattern matching
	if pattern != "" || contextLines > 0 {
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
	}

	// Extract by line numbers
	if len(lineNumbers) > 0 {
		lineSet := make(map[int]bool)
		for _, ln := range lineNumbers {
			lineSet[ln] = true
		}

		if len(allLines) == 0 {
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
			for scanner.Scan() {
				currentLine++
				if lineSet[currentLine] {
					result.WriteString(fmt.Sprintf("%d: %s\n", currentLine, scanner.Text()))
				}
			}
		} else {
			for ln := range lineSet {
				if ln > 0 && ln <= len(allLines) {
					result.WriteString(fmt.Sprintf("%d: %s\n", ln, allLines[ln-1]))
				}
			}
		}
	} else if startLine > 0 && endLine > 0 {
		// Extract by range
		if len(allLines) == 0 {
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
			for scanner.Scan() {
				currentLine++
				if currentLine >= startLine && currentLine <= endLine {
					result.WriteString(fmt.Sprintf("%d: %s\n", currentLine, scanner.Text()))
				}
				if currentLine > endLine {
					break
				}
			}
		} else {
			for i := startLine; i <= endLine && i <= len(allLines); i++ {
				if i > 0 {
					result.WriteString(fmt.Sprintf("%d: %s\n", i, allLines[i-1]))
				}
			}
		}
	} else if pattern != "" {
		// Extract by pattern - simplified implementation
		for i, line := range allLines {
			if strings.Contains(line, pattern) {
				lineNum := i + 1
				start := max(0, i-contextLines)
				end := min(len(allLines), i+contextLines+1)
				for j := start; j < end; j++ {
					result.WriteString(fmt.Sprintf("%d: %s\n", j+1, allLines[j]))
				}
				if end < len(allLines) {
					result.WriteString(fmt.Sprintf("-- match at line %d --\n", lineNum))
				}
			}
		}
	}

	return result.String(), nil
}

// GetIntSlice extracts an int slice from args map
func GetIntSlice(args map[string]interface{}, key string) []int {
	if v, ok := args[key].([]interface{}); ok {
		result := make([]int, 0, len(v))
		for _, item := range v {
			switch n := item.(type) {
			case float64:
				result = append(result, int(n))
			case int:
				result = append(result, n)
			}
		}
		return result
	}
	return nil
}

// ResolvePath expands ~ and cleans path
func ResolvePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return filepath.Clean(path), nil
}
