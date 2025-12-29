package mcpserver

import (
	"encoding/json"
)

// ToolPrefix is the prefix for all llm-filesystem tools
const ToolPrefix = "fast_"

// ToolDefinition defines a tool for the MCP SDK
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// GetToolDefinitions returns tool definitions for the official MCP SDK
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// Core File Operations
		{
			Name:        ToolPrefix + "read_file",
			Description: "Reads a file with auto-chunking support",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to read"},
					"start_offset": {"type": "number", "description": "Starting byte offset"},
					"max_size": {"type": "number", "description": "Maximum size to read"},
					"line_start": {"type": "number", "description": "Starting line number"},
					"line_count": {"type": "number", "description": "Number of lines to read"},
					"encoding": {"type": "string", "description": "Text encoding", "default": "utf-8"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "read_multiple_files",
			Description: "Reads multiple files simultaneously",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}, "description": "File paths to read"},
					"chunk_size": {"type": "number", "description": "Chunk size in bytes", "default": 1048576}
				},
				"required": ["paths"]
			}`),
		},
		{
			Name:        ToolPrefix + "write_file",
			Description: "Writes content to a file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"content": {"type": "string", "description": "File content"},
					"encoding": {"type": "string", "description": "Text encoding", "default": "utf-8"},
					"create_dirs": {"type": "boolean", "description": "Create parent directories", "default": true},
					"append": {"type": "boolean", "description": "Append mode", "default": false}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        ToolPrefix + "large_write_file",
			Description: "Writes large files with backup and verification",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"content": {"type": "string", "description": "File content"},
					"encoding": {"type": "string", "description": "Text encoding", "default": "utf-8"},
					"create_dirs": {"type": "boolean", "description": "Create parent directories", "default": true},
					"backup": {"type": "boolean", "description": "Create backup", "default": true},
					"verify": {"type": "boolean", "description": "Verify write", "default": true}
				},
				"required": ["path", "content"]
			}`),
		},
		{
			Name:        ToolPrefix + "get_file_info",
			Description: "Gets detailed file information",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path to get info for"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "create_directory",
			Description: "Creates a directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory path to create"},
					"recursive": {"type": "boolean", "description": "Create parent directories", "default": true}
				},
				"required": ["path"]
			}`),
		},
		// Directory Operations
		{
			Name:        ToolPrefix + "list_directory",
			Description: "Lists directory contents with filtering and pagination",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory path"},
					"show_hidden": {"type": "boolean", "description": "Show hidden files", "default": false},
					"pattern": {"type": "string", "description": "Filename filter pattern"},
					"sort_by": {"type": "string", "enum": ["name", "size", "modified"], "default": "name"},
					"reverse": {"type": "boolean", "description": "Reverse sort order", "default": false},
					"page": {"type": "number", "description": "Page number"},
					"page_size": {"type": "number", "description": "Items per page"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "get_directory_tree",
			Description: "Gets directory tree structure",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Root directory path"},
					"depth": {"type": "number", "description": "Maximum depth", "default": 5},
					"show_hidden": {"type": "boolean", "description": "Show hidden files", "default": false},
					"include_files": {"type": "boolean", "description": "Include files", "default": false},
					"pattern": {"type": "string", "description": "File pattern filter"}
				},
				"required": ["path"]
			}`),
		},
		// Search Operations
		{
			Name:        ToolPrefix + "search_files",
			Description: "Search for files by name pattern",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search in"},
					"pattern": {"type": "string", "description": "Search pattern"},
					"recursive": {"type": "boolean", "description": "Search recursively", "default": true},
					"show_hidden": {"type": "boolean", "description": "Include hidden files", "default": false},
					"max_results": {"type": "number", "description": "Maximum results", "default": 1000}
				},
				"required": ["path", "pattern"]
			}`),
		},
		{
			Name:        ToolPrefix + "search_code",
			Description: "Search for patterns in file contents",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search in"},
					"pattern": {"type": "string", "description": "Search pattern"},
					"ignore_case": {"type": "boolean", "description": "Case insensitive", "default": false},
					"regex": {"type": "boolean", "description": "Use regex", "default": false},
					"context": {"type": "number", "description": "Context lines", "default": 0},
					"file_types": {"type": "array", "items": {"type": "string"}, "description": "File extensions"},
					"max_results": {"type": "number", "description": "Maximum results", "default": 1000},
					"show_hidden": {"type": "boolean", "description": "Include hidden files", "default": false}
				},
				"required": ["path", "pattern"]
			}`),
		},
		// Edit Operations
		{
			Name:        ToolPrefix + "edit_block",
			Description: "Replace a block of text in a file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"old_string": {"type": "string", "description": "Text to find"},
					"new_string": {"type": "string", "description": "Replacement text"}
				},
				"required": ["path", "old_string", "new_string"]
			}`),
		},
		{
			Name:        ToolPrefix + "edit_blocks",
			Description: "Apply multiple edits to a file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"edits": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"old_string": {"type": "string"},
								"new_string": {"type": "string"}
							},
							"required": ["old_string", "new_string"]
						},
						"description": "List of edits"
					}
				},
				"required": ["path", "edits"]
			}`),
		},
		{
			Name:        ToolPrefix + "safe_edit",
			Description: "Safe edit with backup and dry-run support",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"old_string": {"type": "string", "description": "Text to find"},
					"new_string": {"type": "string", "description": "Replacement text"},
					"backup": {"type": "boolean", "description": "Create backup", "default": true},
					"dry_run": {"type": "boolean", "description": "Preview only", "default": false}
				},
				"required": ["path", "old_string", "new_string"]
			}`),
		},
		{
			Name:        ToolPrefix + "edit_file",
			Description: "Line-based file editing (insert, replace, delete)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"operation": {"type": "string", "enum": ["insert", "replace", "delete"], "description": "Operation type"},
					"line": {"type": "number", "description": "Line number"},
					"content": {"type": "string", "description": "Content for insert/replace"}
				},
				"required": ["path", "operation", "line"]
			}`),
		},
		{
			Name:        ToolPrefix + "search_and_replace",
			Description: "Search and replace across multiple files",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search"},
					"pattern": {"type": "string", "description": "Search pattern"},
					"replacement": {"type": "string", "description": "Replacement text"},
					"regex": {"type": "boolean", "description": "Use regex", "default": false},
					"dry_run": {"type": "boolean", "description": "Preview only", "default": false},
					"file_types": {"type": "array", "items": {"type": "string"}, "description": "File extensions"}
				},
				"required": ["path", "pattern", "replacement"]
			}`),
		},
		{
			Name:        ToolPrefix + "extract_lines",
			Description: "Extract specific lines from a file",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path"},
					"start": {"type": "number", "description": "Start line"},
					"end": {"type": "number", "description": "End line"},
					"lines": {"type": "array", "items": {"type": "number"}, "description": "Specific line numbers"}
				},
				"required": ["path"]
			}`),
		},
		// File Operations
		{
			Name:        ToolPrefix + "copy_file",
			Description: "Copy a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {"type": "string", "description": "Source path"},
					"destination": {"type": "string", "description": "Destination path"}
				},
				"required": ["source", "destination"]
			}`),
		},
		{
			Name:        ToolPrefix + "move_file",
			Description: "Move or rename a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {"type": "string", "description": "Source path"},
					"destination": {"type": "string", "description": "Destination path"}
				},
				"required": ["source", "destination"]
			}`),
		},
		{
			Name:        ToolPrefix + "delete_file",
			Description: "Delete a file or directory",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path to delete"},
					"recursive": {"type": "boolean", "description": "Delete recursively", "default": false}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "batch_file_operations",
			Description: "Perform batch file operations",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"operations": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"operation": {"type": "string", "enum": ["copy", "move", "delete"]},
								"source": {"type": "string"},
								"destination": {"type": "string"}
							},
							"required": ["operation", "source"]
						},
						"description": "List of operations"
					}
				},
				"required": ["operations"]
			}`),
		},
		// Advanced Operations
		{
			Name:        ToolPrefix + "get_disk_usage",
			Description: "Get disk usage for a path",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Path to analyze"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "find_large_files",
			Description: "Find files larger than specified size",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Directory to search"},
					"min_size": {"type": "number", "description": "Minimum size in bytes", "default": 0},
					"limit": {"type": "number", "description": "Maximum results", "default": 100}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "compress_files",
			Description: "Compress files into an archive",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}, "description": "Paths to compress"},
					"output": {"type": "string", "description": "Output archive path"},
					"format": {"type": "string", "enum": ["zip", "tar.gz"], "default": "zip"}
				},
				"required": ["paths", "output"]
			}`),
		},
		{
			Name:        ToolPrefix + "extract_archive",
			Description: "Extract an archive",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"archive": {"type": "string", "description": "Archive file path"},
					"destination": {"type": "string", "description": "Destination directory"}
				},
				"required": ["archive", "destination"]
			}`),
		},
		{
			Name:        ToolPrefix + "sync_directories",
			Description: "Synchronize two directories",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {"type": "string", "description": "Source directory"},
					"destination": {"type": "string", "description": "Destination directory"}
				},
				"required": ["source", "destination"]
			}`),
		},
		{
			Name:        ToolPrefix + "list_allowed_directories",
			Description: "List directories the tool is allowed to access",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}
}
