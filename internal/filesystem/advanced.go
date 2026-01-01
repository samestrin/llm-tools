package filesystem

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	ignore "github.com/sabhiram/go-gitignore"
)

// DiskUsageResult represents disk usage information
type DiskUsageResult struct {
	Path       string `json:"path"`
	TotalSize  int64  `json:"total_size"`
	TotalFiles int    `json:"total_files"`
	TotalDirs  int    `json:"total_dirs"`
}

// LargeFileInfo represents a large file found
type LargeFileInfo struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// LargeFilesResult represents find large files result
type LargeFilesResult struct {
	Path       string          `json:"path"`
	MinSize    int64           `json:"min_size"`
	Files      []LargeFileInfo `json:"files"`
	Total      int             `json:"total"`
	TotalCount int             `json:"total_count"`
	TotalSize  int64           `json:"total_size"`
}

// CompressResult represents compression result
type CompressResult struct {
	Output     string `json:"output"`
	Format     string `json:"format"`
	Size       int64  `json:"size"`
	FilesAdded int    `json:"files_added"`
	Success    bool   `json:"success"`
}

// ExtractResult represents extraction result
type ExtractResult struct {
	Archive        string `json:"archive"`
	Destination    string `json:"destination"`
	FilesExtracted int    `json:"files_extracted"`
	Success        bool   `json:"success"`
}

// SyncResult represents sync result
type SyncResult struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	FilesCopied int    `json:"files_copied"`
	DirsCreated int    `json:"dirs_created"`
	Success     bool   `json:"success"`
}

func (s *Server) handleGetDiskUsage(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	var totalSize int64
	var totalFiles, totalDirs int

	err = filepath.Walk(normalizedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			totalDirs++
		} else {
			totalFiles++
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to calculate disk usage: %w", err)
	}

	result := DiskUsageResult{
		Path:       normalizedPath,
		TotalSize:  totalSize,
		TotalFiles: totalFiles,
		TotalDirs:  totalDirs,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func (s *Server) handleFindLargeFiles(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Parse min_size as string (supports "100MB", "1GB", "500KB", or plain numbers)
	minSize, err := parseMinSize(args)
	if err != nil {
		return "", fmt.Errorf("invalid min_size: %w", err)
	}

	limit := GetInt(args, "max_results", 100)
	// Also support legacy 'limit' parameter
	if GetInt(args, "limit", 0) > 0 {
		limit = GetInt(args, "limit", 100)
	}

	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Load gitignore patterns
	gitignoreFile := loadGitignore(normalizedPath)

	var files []LargeFileInfo

	err = filepath.Walk(normalizedPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip .git directory
		if strings.Contains(path, string(os.PathSeparator)+".git"+string(os.PathSeparator)) {
			return nil
		}

		// Apply gitignore filtering
		if gitignoreFile != nil {
			relPath, _ := filepath.Rel(normalizedPath, path)
			if gitignoreFile.MatchesPath(relPath) {
				return nil
			}
		}

		if info.Size() >= minSize {
			files = append(files, LargeFileInfo{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
			})
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to find large files: %w", err)
	}

	// Sort by size descending
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	// Apply limit
	totalCount := len(files)
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}

	result := LargeFilesResult{
		Path:       normalizedPath,
		MinSize:    minSize,
		Files:      files,
		Total:      len(files),
		TotalCount: totalCount,
		TotalSize:  totalSize,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

// parseMinSize parses the min_size parameter which can be a string like "100MB" or a number
func parseMinSize(args map[string]interface{}) (int64, error) {
	// Try to get as string first
	minSizeStr := GetString(args, "min_size", "")
	if minSizeStr != "" {
		// Try parsing as human-readable size (e.g., "100MB", "1GB")
		bytes, err := humanize.ParseBytes(minSizeStr)
		if err == nil {
			return int64(bytes), nil
		}
		// Try parsing as plain number string
		if num, err := strconv.ParseInt(minSizeStr, 10, 64); err == nil {
			return num, nil
		}
		return 0, fmt.Errorf("cannot parse size: %s", minSizeStr)
	}

	// Fall back to integer parameter for backward compatibility
	return int64(GetInt(args, "min_size", 0)), nil
}

// loadGitignore loads the .gitignore file from the given path or any parent directory
func loadGitignore(path string) *ignore.GitIgnore {
	// Check if path is a file, get directory
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	searchPath := path
	if !info.IsDir() {
		searchPath = filepath.Dir(path)
	}

	// Walk up to find .gitignore
	for searchPath != "/" && searchPath != "." {
		gitignorePath := filepath.Join(searchPath, ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			gi, err := ignore.CompileIgnoreFile(gitignorePath)
			if err == nil {
				return gi
			}
		}

		// Check for .git directory (root of repo)
		gitDir := filepath.Join(searchPath, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			// Found git root, stop searching
			break
		}

		searchPath = filepath.Dir(searchPath)
	}

	return nil
}

func (s *Server) handleCompressFiles(args map[string]interface{}) (string, error) {
	paths := GetStringSlice(args, "paths")
	output := GetString(args, "output", "")
	format := GetString(args, "format", "zip")

	if len(paths) == 0 {
		return "", fmt.Errorf("paths is required")
	}
	if output == "" {
		return "", fmt.Errorf("output is required")
	}

	normalizedOutput, err := NormalizePath(output)
	if err != nil {
		return "", fmt.Errorf("invalid output path: %w", err)
	}

	if err := ValidatePath(normalizedOutput, s.allowedDirs); err != nil {
		return "", err
	}

	// Validate input paths
	for i, p := range paths {
		normalized, err := NormalizePath(p)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		if err := ValidatePath(normalized, s.allowedDirs); err != nil {
			return "", err
		}
		paths[i] = normalized
	}

	var filesAdded int
	var archiveSize int64

	switch format {
	case "zip":
		filesAdded, err = createZipArchive(paths, normalizedOutput)
	case "tar.gz", "tgz":
		filesAdded, err = createTarGzArchive(paths, normalizedOutput)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return "", fmt.Errorf("failed to create archive: %w", err)
	}

	info, _ := os.Stat(normalizedOutput)
	if info != nil {
		archiveSize = info.Size()
	}

	result := CompressResult{
		Output:     normalizedOutput,
		Format:     format,
		Size:       archiveSize,
		FilesAdded: filesAdded,
		Success:    true,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func createZipArchive(paths []string, output string) (int, error) {
	file, err := os.Create(output)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	count := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if info.IsDir() {
			filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				relPath, _ := filepath.Rel(filepath.Dir(path), filePath)
				if err := addToZip(zipWriter, filePath, relPath); err != nil {
					return nil
				}
				count++
				return nil
			})
		} else {
			if err := addToZip(zipWriter, path, filepath.Base(path)); err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

func addToZip(zipWriter *zip.Writer, filePath, name string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	w, err := zipWriter.Create(name)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, file)
	return err
}

func createTarGzArchive(paths []string, output string) (int, error) {
	file, err := os.Create(output)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	count := 0
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if info.IsDir() {
			filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				relPath, _ := filepath.Rel(filepath.Dir(path), filePath)
				if err := addToTar(tarWriter, filePath, relPath); err != nil {
					return nil
				}
				count++
				return nil
			})
		} else {
			if err := addToTar(tarWriter, path, filepath.Base(path)); err != nil {
				continue
			}
			count++
		}
	}

	return count, nil
}

func addToTar(tarWriter *tar.Writer, filePath, name string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name: name,
		Mode: int64(info.Mode()),
		Size: info.Size(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}

func (s *Server) handleExtractArchive(args map[string]interface{}) (string, error) {
	archive := GetString(args, "archive", "")
	destination := GetString(args, "destination", "")

	if archive == "" {
		return "", fmt.Errorf("archive is required")
	}
	if destination == "" {
		return "", fmt.Errorf("destination is required")
	}

	normalizedArchive, err := NormalizePath(archive)
	if err != nil {
		return "", fmt.Errorf("invalid archive path: %w", err)
	}

	normalizedDest, err := NormalizePath(destination)
	if err != nil {
		return "", fmt.Errorf("invalid destination path: %w", err)
	}

	if err := ValidatePath(normalizedArchive, s.allowedDirs); err != nil {
		return "", err
	}
	if err := ValidatePath(normalizedDest, s.allowedDirs); err != nil {
		return "", err
	}

	// Create destination directory
	if err := os.MkdirAll(normalizedDest, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	var filesExtracted int

	// Detect format
	if strings.HasSuffix(normalizedArchive, ".zip") {
		filesExtracted, err = extractZip(normalizedArchive, normalizedDest)
	} else if strings.HasSuffix(normalizedArchive, ".tar.gz") || strings.HasSuffix(normalizedArchive, ".tgz") {
		filesExtracted, err = extractTarGz(normalizedArchive, normalizedDest)
	} else {
		return "", fmt.Errorf("unsupported archive format")
	}

	if err != nil {
		return "", fmt.Errorf("failed to extract archive: %w", err)
	}

	result := ExtractResult{
		Archive:        normalizedArchive,
		Destination:    normalizedDest,
		FilesExtracted: filesExtracted,
		Success:        true,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func extractZip(archive, destination string) (int, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return 0, err
	}
	defer reader.Close()

	count := 0
	for _, file := range reader.File {
		path := filepath.Join(destination, file.Name)

		// Prevent zip slip vulnerability
		if !strings.HasPrefix(path, filepath.Clean(destination)+string(os.PathSeparator)) {
			continue
		}

		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			continue
		}

		outFile, err := os.Create(path)
		if err != nil {
			rc.Close()
			continue
		}

		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		count++
	}

	return count, nil
}

func extractTarGz(archive, destination string) (int, error) {
	file, err := os.Open(archive)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return 0, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	count := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}

		path := filepath.Join(destination, header.Name)

		// Prevent tar slip vulnerability
		if !strings.HasPrefix(path, filepath.Clean(destination)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, os.FileMode(header.Mode))
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				continue
			}
			outFile, err := os.Create(path)
			if err != nil {
				continue
			}
			io.Copy(outFile, tarReader)
			outFile.Close()
			count++
		}
	}

	return count, nil
}

func (s *Server) handleSyncDirectories(args map[string]interface{}) (string, error) {
	source := GetString(args, "source", "")
	destination := GetString(args, "destination", "")

	if source == "" {
		return "", fmt.Errorf("source is required")
	}
	if destination == "" {
		return "", fmt.Errorf("destination is required")
	}

	normalizedSrc, err := NormalizePath(source)
	if err != nil {
		return "", fmt.Errorf("invalid source path: %w", err)
	}

	normalizedDst, err := NormalizePath(destination)
	if err != nil {
		return "", fmt.Errorf("invalid destination path: %w", err)
	}

	if err := ValidatePath(normalizedSrc, s.allowedDirs); err != nil {
		return "", err
	}
	if err := ValidatePath(normalizedDst, s.allowedDirs); err != nil {
		return "", err
	}

	var filesCopied, dirsCreated int

	err = filepath.Walk(normalizedSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(normalizedSrc, path)
		dstPath := filepath.Join(normalizedDst, relPath)

		if info.IsDir() {
			if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
				return nil
			}
			dirsCreated++
		} else {
			if err := copyFile(path, dstPath); err != nil {
				return nil
			}
			filesCopied++
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to sync directories: %w", err)
	}

	result := SyncResult{
		Source:      normalizedSrc,
		Destination: normalizedDst,
		FilesCopied: filesCopied,
		DirsCreated: dirsCreated,
		Success:     true,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}
