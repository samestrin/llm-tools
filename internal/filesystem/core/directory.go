package core

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
)

// DirectoryEntry represents a file or directory entry
type DirectoryEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Type         string `json:"type"` // "file" or "directory"
	IsDir        bool   `json:"is_dir"`
	Size         int64  `json:"size"`
	SizeReadable string `json:"size_readable"`
	Mode         string `json:"mode"`
	Permissions  uint32 `json:"permissions"`
	Modified     string `json:"modified"`
	Created      string `json:"created,omitempty"`
	Accessed     string `json:"accessed,omitempty"`
	Extension    string `json:"extension,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	IsReadable   bool   `json:"is_readable"`
	IsWritable   bool   `json:"is_writable"`
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
	Items      []DirectoryEntry `json:"items"`
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

		entryType := "file"
		if info.IsDir() {
			entryType = "directory"
		}

		// Get extension and MIME type for files
		var ext, mimeType string
		if !info.IsDir() {
			ext = filepath.Ext(name)
			if ext != "" {
				mimeType = mime.TypeByExtension(ext)
			}
		}

		// Check access permissions
		entryPath := filepath.Join(normalizedPath, name)
		isReadable, isWritable := checkAccess(entryPath)

		// Get all timestamps
		created, accessed, modified := GetFileTimestamps(info)

		entry := DirectoryEntry{
			Name:         name,
			Path:         entryPath,
			Type:         entryType,
			IsDir:        info.IsDir(),
			Size:         info.Size(),
			SizeReadable: humanize.Bytes(uint64(info.Size())),
			Mode:         info.Mode().String(),
			Permissions:  uint32(info.Mode().Perm()),
			Modified:     modified.Format("2006-01-02T15:04:05Z07:00"),
			Created:      created.Format("2006-01-02T15:04:05Z07:00"),
			Accessed:     accessed.Format("2006-01-02T15:04:05Z07:00"),
			Extension:    ext,
			MimeType:     mimeType,
			IsReadable:   isReadable,
			IsWritable:   isWritable,
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
		Items:      entries,
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
				return entries[i].Modified > entries[j].Modified
			}
			return entries[i].Modified < entries[j].Modified
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
	Tree       *TreeNode `json:"tree"`
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
		Tree:       root,
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

// checkAccess checks if a file/directory is readable and writable
func checkAccess(path string) (readable bool, writable bool) {
	// Check read access by attempting to open for reading
	file, err := os.Open(path)
	if err == nil {
		file.Close()
		readable = true
	}

	// Check write access by checking if we can open for writing
	// For directories, check if we can create a temp file
	info, err := os.Stat(path)
	if err != nil {
		return readable, false
	}

	if info.IsDir() {
		// For directories, check write permission via mode
		writable = info.Mode().Perm()&0200 != 0
	} else {
		// For files, try opening with write flag (O_WRONLY)
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err == nil {
			file.Close()
			writable = true
		}
	}

	return readable, writable
}
