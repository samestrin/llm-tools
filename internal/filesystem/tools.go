package filesystem

import (
	"encoding/json"
)

// ToolDefinition represents an MCP tool with its schema
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// GetToolDefinitions returns the 15 batch/specialized tool definitions
// NOTE: This legacy server exposes 15 tools. Single-file operations
// should use Claude's native Read, Write, and Edit tools for better performance.
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// Batch Reading
		{
			Name:        "llm_filesystem_read_multiple_files",
			Description: "Reads the content of multiple files simultaneously (supports sequential reading)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}, "description": "File paths to read"},
					"continuation_tokens": {"type": "object", "description": "Per-file continuation token"},
					"auto_continue": {"type": "boolean", "description": "Automatically read the entire file", "default": true},
					"chunk_size": {"type": "number", "description": "Chunk size (bytes)", "default": 1048576}
				},
				"required": ["paths"]
			}`),
		},
		{
			Name:        "llm_filesystem_extract_lines",
			Description: "Extracts specific lines from a file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"line_numbers": {"type": "array", "items": {"type": "number"}, "description": "Line numbers to extract"},
					"start_line": {"type": "number", "description": "Start line (for range extraction)"},
					"end_line": {"type": "number", "description": "End line (for range extraction)"},
					"pattern": {"type": "string", "description": "Extract lines by pattern"},
					"context_lines": {"type": "number", "description": "Context lines around pattern match", "default": 0}
				},
				"required": ["path"]
			}`),
		},
		// Batch Editing
		{
			Name:        "llm_filesystem_edit_blocks",
			Description: "Processes multiple precise block edits at once",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path of the file to edit"},
					"edits": {
						"type": "array",
						"description": "List of precise block edits",
						"items": {
							"type": "object",
							"properties": {
								"old_text": {"type": "string", "description": "Exact existing text to match"},
								"new_text": {"type": "string", "description": "The new text"},
								"expected_replacements": {"type": "number", "description": "Expected replacements", "default": 1}
							},
							"required": ["old_text", "new_text"]
						}
					},
					"backup": {"type": "boolean", "description": "Create a backup", "default": true}
				},
				"required": ["path", "edits"]
			}`),
		},
		{
			Name:        "llm_filesystem_search_and_replace",
			Description: "Regex search and replace across files",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search in"},
					"pattern": {"type": "string", "description": "Regex pattern to find"},
					"replacement": {"type": "string", "description": "Replacement text"},
					"file_pattern": {"type": "string", "description": "File pattern filter", "default": "*"},
					"case_sensitive": {"type": "boolean", "description": "Case-sensitive search", "default": true},
					"dry_run": {"type": "boolean", "description": "Preview without changes", "default": false},
					"backup": {"type": "boolean", "description": "Create backups", "default": true},
					"max_files": {"type": "number", "description": "Maximum files to process", "default": 100}
				},
				"required": ["path", "pattern", "replacement"]
			}`),
		},
		// Directory Operations
		{
			Name:        "llm_filesystem_list_directory",
			Description: "Lists the contents of a directory (with auto-chunking and pagination support)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory path"},
					"page": {"type": "number", "description": "Page number", "default": 1},
					"page_size": {"type": "number", "description": "Number of items per page"},
					"pattern": {"type": "string", "description": "Filename filter pattern"},
					"show_hidden": {"type": "boolean", "description": "Show hidden files", "default": false},
					"sort_by": {"type": "string", "description": "Sort by", "enum": ["name", "size", "modified", "type"], "default": "name"},
					"reverse": {"type": "boolean", "description": "Reverse sort order", "default": false},
					"continuation_token": {"type": "string", "description": "Continuation token"},
					"auto_chunk": {"type": "boolean", "description": "Enable auto-chunking", "default": true}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "llm_filesystem_get_directory_tree",
			Description: "Gets the directory tree structure",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Root directory path"},
					"max_depth": {"type": "number", "description": "Maximum depth", "default": 3},
					"show_hidden": {"type": "boolean", "description": "Show hidden files", "default": false},
					"include_files": {"type": "boolean", "description": "Include files in the tree", "default": true}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "llm_filesystem_create_directories",
			Description: "Creates multiple directories in a single operation",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}, "description": "Directory paths to create"},
					"recursive": {"type": "boolean", "description": "Create parent directories", "default": true}
				},
				"required": ["paths"]
			}`),
		},
		// Search Operations
		{
			Name:        "llm_filesystem_search_files",
			Description: "Searches for files (by name/content) - supports auto-chunking, regex, context, and line numbers",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search in"},
					"pattern": {"type": "string", "description": "Search pattern (regex supported)"},
					"content_search": {"type": "boolean", "description": "Search file content", "default": false},
					"case_sensitive": {"type": "boolean", "description": "Case-sensitive search", "default": false},
					"max_results": {"type": "number", "description": "Maximum number of results", "default": 100},
					"context_lines": {"type": "number", "description": "Context lines around match", "default": 0},
					"file_pattern": {"type": "string", "description": "Filename filter pattern", "default": ""},
					"include_binary": {"type": "boolean", "description": "Include binary files", "default": false},
					"continuation_token": {"type": "string", "description": "Continuation token"},
					"auto_chunk": {"type": "boolean", "description": "Enable auto-chunking", "default": true}
				},
				"required": ["path", "pattern"]
			}`),
		},
		{
			Name:        "llm_filesystem_search_code",
			Description: "Searches for code (ripgrep-style) - provides auto-chunking, line numbers, and context",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search in"},
					"pattern": {"type": "string", "description": "Search pattern (regex supported)"},
					"file_pattern": {"type": "string", "description": "File extension filter", "default": ""},
					"context_lines": {"type": "number", "description": "Context lines around match", "default": 2},
					"max_results": {"type": "number", "description": "Maximum number of results", "default": 50},
					"case_sensitive": {"type": "boolean", "description": "Case-sensitive search", "default": false},
					"include_hidden": {"type": "boolean", "description": "Include hidden files", "default": false},
					"max_file_size": {"type": "number", "description": "Maximum file size (MB)", "default": 10},
					"continuation_token": {"type": "string", "description": "Continuation token"},
					"auto_chunk": {"type": "boolean", "description": "Enable auto-chunking", "default": true}
				},
				"required": ["path", "pattern"]
			}`),
		},
		// File Operations
		{
			Name:        "llm_filesystem_copy_file",
			Description: "Copies a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {"type": "string", "description": "Source file/directory path"},
					"destination": {"type": "string", "description": "Destination path"},
					"overwrite": {"type": "boolean", "description": "Overwrite existing file", "default": false},
					"preserve_timestamps": {"type": "boolean", "description": "Preserve timestamps", "default": true},
					"recursive": {"type": "boolean", "description": "Recursively copy directory", "default": true},
					"create_dirs": {"type": "boolean", "description": "Create destination directories", "default": true}
				},
				"required": ["source", "destination"]
			}`),
		},
		{
			Name:        "llm_filesystem_move_file",
			Description: "Moves or renames a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {"type": "string", "description": "Source file/directory path"},
					"destination": {"type": "string", "description": "Destination path"},
					"overwrite": {"type": "boolean", "description": "Overwrite existing file", "default": false},
					"create_dirs": {"type": "boolean", "description": "Create destination directories", "default": true},
					"backup_if_exists": {"type": "boolean", "description": "Create backup if destination exists", "default": false}
				},
				"required": ["source", "destination"]
			}`),
		},
		{
			Name:        "llm_filesystem_delete_file",
			Description: "Deletes a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path to delete"},
					"recursive": {"type": "boolean", "description": "Recursively delete directory", "default": false},
					"force": {"type": "boolean", "description": "Force deletion", "default": false},
					"backup_before_delete": {"type": "boolean", "description": "Create backup before deleting", "default": false},
					"confirm_delete": {"type": "boolean", "description": "Confirm deletion", "default": true}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "llm_filesystem_batch_file_operations",
			Description: "Performs batch operations on multiple files",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"operations": {
						"type": "array",
						"description": "List of batch operations",
						"items": {
							"type": "object",
							"properties": {
								"operation": {"type": "string", "enum": ["copy", "move", "delete", "rename"], "description": "Operation type"},
								"source": {"type": "string", "description": "Source path"},
								"destination": {"type": "string", "description": "Destination path"},
								"overwrite": {"type": "boolean", "description": "Allow overwrite", "default": false}
							},
							"required": ["operation", "source"]
						}
					},
					"stop_on_error": {"type": "boolean", "description": "Stop on error", "default": true},
					"dry_run": {"type": "boolean", "description": "Preview without execution", "default": false},
					"create_backup": {"type": "boolean", "description": "Create backup before changes", "default": false}
				},
				"required": ["operations"]
			}`),
		},
		// Archive Operations
		{
			Name:        "llm_filesystem_compress_files",
			Description: "Compresses files or directories",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}, "description": "Paths to compress"},
					"output_path": {"type": "string", "description": "Output archive file path"},
					"format": {"type": "string", "enum": ["zip", "tar", "tar.gz", "tar.bz2"], "default": "zip", "description": "Archive format"},
					"compression_level": {"type": "number", "minimum": 0, "maximum": 9, "default": 6, "description": "Compression level"},
					"exclude_patterns": {"type": "array", "items": {"type": "string"}, "description": "Patterns to exclude", "default": []}
				},
				"required": ["paths", "output_path"]
			}`),
		},
		{
			Name:        "llm_filesystem_extract_archive",
			Description: "Extracts an archive file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"archive_path": {"type": "string", "description": "Archive file path"},
					"extract_to": {"type": "string", "description": "Directory to extract to", "default": "."},
					"overwrite": {"type": "boolean", "description": "Overwrite existing files", "default": false},
					"create_dirs": {"type": "boolean", "description": "Create directories", "default": true},
					"preserve_permissions": {"type": "boolean", "description": "Preserve permissions", "default": true},
					"extract_specific": {"type": "array", "items": {"type": "string"}, "description": "Extract only specific files"}
				},
				"required": ["archive_path"]
			}`),
		},
	}
}
