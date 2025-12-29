package core

import (
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

// ListDirectoryOptions contains input parameters for ListDirectory
type ListDirectoryOptions struct {
	Path        string
	ShowHidden  bool
	Pattern     string
	SortBy      string
	Reverse     bool
	Page        int
	PageSize    int
	AllowedDirs []string
}

// ListDirectoryResult represents the result of a directory listing
type ListDirectoryResult struct {
	Path       string           `json:"path"`
	Entries    []DirectoryEntry `json:"entries"`
	Total      int              `json:"total"`
	Page       int              `json:"page,omitempty"`
	PageSize   int              `json:"page_size,omitempty"`
	TotalPages int              `json:"total_pages,omitempty"`
}

// ListDirectory lists directory contents
func ListDirectory(opts ListDirectoryOptions) (*ListDirectoryResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	// Check if directory exists
	info, err := os.Stat(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory")
	}

	// Read directory entries
	dirEntries, err := os.ReadDir(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var entries []DirectoryEntry
	for _, de := range dirEntries {
		name := de.Name()

		// Skip hidden files unless requested
		if !opts.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Apply pattern filter
		if opts.Pattern != "" {
			matched, _ := filepath.Match(opts.Pattern, name)
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
	sortEntries(entries, opts.SortBy, opts.Reverse)

	total := len(entries)

	// Apply pagination
	var totalPages int
	if opts.Page > 0 && opts.PageSize > 0 {
		totalPages = (total + opts.PageSize - 1) / opts.PageSize
		start := (opts.Page - 1) * opts.PageSize
		end := start + opts.PageSize
		if start >= total {
			entries = []DirectoryEntry{}
		} else {
			if end > total {
				end = total
			}
			entries = entries[start:end]
		}
	}

	return &ListDirectoryResult{
		Path:       normalizedPath,
		Entries:    entries,
		Total:      total,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		TotalPages: totalPages,
	}, nil
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

// TreeNode represents a node in the directory tree
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// GetDirectoryTreeOptions contains input parameters for GetDirectoryTree
type GetDirectoryTreeOptions struct {
	Path         string
	MaxDepth     int
	ShowHidden   bool
	IncludeFiles bool
	Pattern      string
	AllowedDirs  []string
}

// GetDirectoryTreeResult represents the result of a directory tree operation
type GetDirectoryTreeResult struct {
	Root       *TreeNode `json:"root"`
	TotalDirs  int       `json:"total_dirs"`
	TotalFiles int       `json:"total_files"`
	TotalSize  int64     `json:"total_size"`
}

// GetDirectoryTree gets the directory tree structure
func GetDirectoryTree(opts GetDirectoryTreeOptions) (*GetDirectoryTreeResult, error) {
	if opts.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Normalize and validate path
	normalizedPath, err := NormalizePath(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if err := ValidatePath(normalizedPath, opts.AllowedDirs); err != nil {
		return nil, err
	}

	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = 5
	}

	var totalDirs, totalFiles int
	var totalSize int64

	root, err := buildTree(normalizedPath, 0, maxDepth, opts.ShowHidden, opts.IncludeFiles, opts.Pattern, &totalDirs, &totalFiles, &totalSize)
	if err != nil {
		return nil, err
	}

	return &GetDirectoryTreeResult{
		Root:       root,
		TotalDirs:  totalDirs,
		TotalFiles: totalFiles,
		TotalSize:  totalSize,
	}, nil
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
