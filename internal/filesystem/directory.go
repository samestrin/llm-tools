package filesystem

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirectoryEntry represents a file or directory entry
type DirectoryEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

// ListDirectoryResult represents the result of a directory listing
type ListDirectoryResult struct {
	Path              string           `json:"path"`
	Entries           []DirectoryEntry `json:"entries"`
	Total             int              `json:"total"`
	Page              int              `json:"page,omitempty"`
	PageSize          int              `json:"page_size,omitempty"`
	TotalPages        int              `json:"total_pages,omitempty"`
	ContinuationToken string           `json:"continuation_token,omitempty"`
	HasMore           bool             `json:"has_more,omitempty"`
}

// TreeNode represents a node in the directory tree
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// DirectoryTreeResult represents the result of a directory tree operation
type DirectoryTreeResult struct {
	Root       *TreeNode `json:"root"`
	TotalDirs  int       `json:"total_dirs"`
	TotalFiles int       `json:"total_files"`
	TotalSize  int64     `json:"total_size"`
}

func (s *Server) handleListDirectory(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Check if directory exists
	info, err := os.Stat(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	// Get options
	showHidden := GetBool(args, "show_hidden", false)
	pattern := GetString(args, "pattern", "")
	sortBy := GetString(args, "sort_by", "name")
	reverse := GetBool(args, "reverse", false)
	page := GetInt(args, "page", 0)
	pageSize := GetInt(args, "page_size", 0)
	continuationTokenStr := GetString(args, "continuation_token", "")

	// If continuation token provided, decode and use its page
	if continuationTokenStr != "" {
		token, err := DecodeContinuationToken(continuationTokenStr)
		if err != nil {
			return "", fmt.Errorf("invalid continuation token: %w", err)
		}
		if err := ValidateToken(token, normalizedPath, "list"); err != nil {
			return "", err
		}
		page = token.Page
	}

	// Read directory entries
	dirEntries, err := os.ReadDir(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	var entries []DirectoryEntry
	for _, de := range dirEntries {
		name := de.Name()

		// Skip hidden files unless requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Apply pattern filter
		if pattern != "" {
			matched, _ := filepath.Match(pattern, name)
			if !matched {
				continue
			}
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		entry := DirectoryEntry{
			Name:    name,
			Path:    filepath.Join(normalizedPath, name),
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		}
		entries = append(entries, entry)
	}

	// Sort entries
	sortEntries(entries, sortBy, reverse)

	total := len(entries)

	// Apply pagination
	var totalPages int
	var hasMore bool
	var nextToken string
	if page > 0 && pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
		start := (page - 1) * pageSize
		end := start + pageSize
		if start >= total {
			entries = []DirectoryEntry{}
		} else {
			if end > total {
				end = total
			}
			entries = entries[start:end]
		}
		// Generate continuation token if there are more pages
		hasMore = page < totalPages
		if hasMore {
			nextToken, _ = CreateListToken(normalizedPath, page+1)
		}
	}

	result := ListDirectoryResult{
		Path:              normalizedPath,
		Entries:           entries,
		Total:             total,
		Page:              page,
		PageSize:          pageSize,
		TotalPages:        totalPages,
		ContinuationToken: nextToken,
		HasMore:           hasMore,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func sortEntries(entries []DirectoryEntry, sortBy string, reverse bool) {
	switch sortBy {
	case "size":
		sort.Slice(entries, func(i, j int) bool {
			if reverse {
				return entries[i].Size > entries[j].Size
			}
			return entries[i].Size < entries[j].Size
		})
	case "modified", "time":
		sort.Slice(entries, func(i, j int) bool {
			if reverse {
				return entries[i].ModTime > entries[j].ModTime
			}
			return entries[i].ModTime < entries[j].ModTime
		})
	default: // name
		sort.Slice(entries, func(i, j int) bool {
			if reverse {
				return entries[i].Name > entries[j].Name
			}
			return entries[i].Name < entries[j].Name
		})
	}
}

func (s *Server) handleGetDirectoryTree(args map[string]interface{}) (string, error) {
	path := GetString(args, "path", "")
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, s.allowedDirs); err != nil {
		return "", err
	}

	// Get options
	maxDepth := GetInt(args, "max_depth", 5)
	showHidden := GetBool(args, "show_hidden", false)
	includeFiles := GetBool(args, "include_files", false)
	pattern := GetString(args, "pattern", "")

	var totalDirs, totalFiles int
	var totalSize int64

	root, err := buildTree(normalizedPath, 0, maxDepth, showHidden, includeFiles, pattern, &totalDirs, &totalFiles, &totalSize)
	if err != nil {
		return "", err
	}

	result := DirectoryTreeResult{
		Root:       root,
		TotalDirs:  totalDirs,
		TotalFiles: totalFiles,
		TotalSize:  totalSize,
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(jsonBytes), nil
}

func buildTree(path string, depth, maxDepth int, showHidden, includeFiles bool, pattern string, totalDirs, totalFiles *int, totalSize *int64) (*TreeNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat: %w", err)
	}

	node := &TreeNode{
		Name:  filepath.Base(path),
		Path:  path,
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}

	if !info.IsDir() {
		*totalFiles++
		*totalSize += info.Size()
		return node, nil
	}

	*totalDirs++

	if depth >= maxDepth {
		return node, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return node, nil // Return node without children on error
	}

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden unless requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		// Skip files if not requested
		if !entryInfo.IsDir() && !includeFiles {
			continue
		}

		// Apply pattern filter for files
		if !entryInfo.IsDir() && pattern != "" {
			matched, _ := filepath.Match(pattern, name)
			if !matched {
				continue
			}
		}

		childPath := filepath.Join(path, name)
		child, err := buildTree(childPath, depth+1, maxDepth, showHidden, includeFiles, pattern, totalDirs, totalFiles, totalSize)
		if err != nil {
			continue
		}

		node.Children = append(node.Children, child)
	}

	return node, nil
}
