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

// GetToolDefinitions returns all 27 tool definitions (25 v3.4.0 + 2 v3.5.1)
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// Core File Operations (Phase 1)
		{
			Name:        "llm_filesystem_read_file",
			Description: "Reads a file (with auto-chunking support)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to read"},
					"start_offset": {"type": "number", "description": "Starting byte offset"},
					"max_size": {"type": "number", "description": "Maximum size to read"},
					"line_start": {"type": "number", "description": "Starting line number"},
					"line_count": {"type": "number", "description": "Number of lines to read"},
					"encoding": {"type": "string", "description": "Text encoding", "default": "utf-8"},
					"continuation_token": {"type": "string", "description": "Continuation token from a previous call"},
					"auto_chunk": {"type": "boolean", "description": "Enable auto-chunking", "default": true}
				},
				"required": ["path"]
			}`),
		},
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
			Name:        "llm_filesystem_write_file",
			Description: "Writes or modifies a file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"content": {"type": "string", "description": "File content"},
					"encoding": {"type": "string", "description": "Text encoding", "default": "utf-8"},
					"create_dirs": {"type": "boolean", "description": "Automatically create directories", "default": true},
					"append": {"type": "boolean", "description": "Append mode", "default": false},
					"force_remove_emojis": {"type": "boolean", "description": "Force remove emojis", "default": false}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        "llm_filesystem_large_write_file",
			Description: "Reliably writes large files (with streaming, retry, backup, and verification features)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"content": {"type": "string", "description": "File content"},
					"encoding": {"type": "string", "description": "Text encoding", "default": "utf-8"},
					"create_dirs": {"type": "boolean", "description": "Automatically create directories", "default": true},
					"append": {"type": "boolean", "description": "Append mode", "default": false},
					"chunk_size": {"type": "number", "description": "Chunk size (bytes)", "default": 65536},
					"backup": {"type": "boolean", "description": "Create a backup", "default": true},
					"retry_attempts": {"type": "number", "description": "Number of retry attempts", "default": 3},
					"verify_write": {"type": "boolean", "description": "Verify after writing", "default": true},
					"force_remove_emojis": {"type": "boolean", "description": "Force remove emojis", "default": false}
				},
				"required": ["path", "content"]
			}`),
		},
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
			Name:        "llm_filesystem_get_file_info",
			Description: "Gets detailed information about a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path to get info for"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "llm_filesystem_create_directory",
			Description: "Creates a directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path of the directory to create"},
					"recursive": {"type": "boolean", "description": "Create parent directories", "default": true}
				},
				"required": ["path"]
			}`),
		},
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
		// Edit Operations
		{
			Name:        "llm_filesystem_edit_block",
			Description: "Precise block editing: safely replace exact matches",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path of the file to edit"},
					"old_text": {"type": "string", "description": "Exact existing text to match"},
					"new_text": {"type": "string", "description": "Replacement text"},
					"expected_replacements": {"type": "number", "description": "Expected number of replacements", "default": 1},
					"backup": {"type": "boolean", "description": "Create a backup", "default": true},
					"word_boundary": {"type": "boolean", "description": "Enforce word boundaries", "default": false},
					"preview_only": {"type": "boolean", "description": "Preview only", "default": false},
					"case_sensitive": {"type": "boolean", "description": "Match case sensitively", "default": true}
				},
				"required": ["path", "old_text", "new_text"]
			}`),
		},
		{
			Name:        "llm_filesystem_safe_edit",
			Description: "Safe smart editing: Detects risks and provides interactive confirmation",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path of the file to edit"},
					"old_text": {"type": "string", "description": "Text to be replaced"},
					"new_text": {"type": "string", "description": "The new text"},
					"safety_level": {"type": "string", "enum": ["strict", "moderate", "flexible"], "default": "moderate", "description": "Safety level"},
					"auto_add_context": {"type": "boolean", "description": "Automatically add context", "default": true},
					"require_confirmation": {"type": "boolean", "description": "Require confirmation on high risk", "default": true}
				},
				"required": ["path", "old_text", "new_text"]
			}`),
		},
		{
			Name:        "llm_filesystem_edit_multiple_blocks",
			Description: "Edits multiple parts of a file at once",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path of the file to edit"},
					"edits": {
						"type": "array",
						"description": "List of edit operations",
						"items": {
							"type": "object",
							"properties": {
								"old_text": {"type": "string", "description": "Existing text to find"},
								"new_text": {"type": "string", "description": "The new text"},
								"line_number": {"type": "number", "description": "Line number"},
								"mode": {"type": "string", "enum": ["replace", "insert_before", "insert_after", "delete_line"], "default": "replace"}
							}
						}
					},
					"backup": {"type": "boolean", "description": "Create a backup", "default": true}
				},
				"required": ["path", "edits"]
			}`),
		},
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
		// File Management Operations
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
		// System & Archive Operations
		{
			Name:        "llm_filesystem_get_disk_usage",
			Description: "Gets disk usage information",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path to check", "default": "/"}
				}
			}`),
		},
		{
			Name:        "llm_filesystem_find_large_files",
			Description: "Finds large files",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search in"},
					"min_size": {"type": "string", "description": "Minimum size (e.g., 100MB, 1GB)", "default": "100MB"},
					"max_results": {"type": "number", "description": "Maximum number of results", "default": 50}
				},
				"required": ["path"]
			}`),
		},
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
		{
			Name:        "llm_filesystem_sync_directories",
			Description: "Synchronizes two directories",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source_dir": {"type": "string", "description": "Source directory"},
					"target_dir": {"type": "string", "description": "Target directory"},
					"sync_mode": {"type": "string", "enum": ["mirror", "update", "merge"], "default": "update", "description": "Sync mode"},
					"delete_extra": {"type": "boolean", "description": "Delete files only in target", "default": false},
					"preserve_newer": {"type": "boolean", "description": "Preserve newer files", "default": true},
					"dry_run": {"type": "boolean", "description": "Preview without execution", "default": false},
					"exclude_patterns": {"type": "array", "items": {"type": "string"}, "description": "Patterns to exclude", "default": [".git", "node_modules", ".DS_Store"]}
				},
				"required": ["source_dir", "target_dir"]
			}`),
		},
		{
			Name:        "llm_filesystem_list_allowed_directories",
			Description: "Lists the allowed directories",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {},
				"required": []
			}`),
		},
		// v3.5.1 Tools
		{
			Name:        "llm_filesystem_edit_file",
			Description: "Line-based file editing (insert, replace, delete)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"line_number": {"type": "number", "description": "Line number to edit"},
					"mode": {"type": "string", "enum": ["insert", "replace", "delete"], "description": "Edit mode"},
					"content": {"type": "string", "description": "Content for insert/replace"},
					"backup": {"type": "boolean", "description": "Create backup", "default": true}
				},
				"required": ["path", "line_number", "mode"]
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
	}
}
