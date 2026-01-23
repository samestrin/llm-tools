package mcpserver

import (
	"encoding/json"
)

// ToolPrefix is the prefix for all llm-filesystem tools
const ToolPrefix = "llm_filesystem_"

// ToolDefinition defines a tool for the MCP SDK
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// GetToolDefinitions returns tool definitions for the official MCP SDK
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// Single File Operations (for LLM compatibility)
		{
			Name:        ToolPrefix + "read_file",
			Description: "Read a file with optional line range or byte offset",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to read"},
					"line_start": {"type": "number", "description": "Starting line number"},
					"line_count": {"type": "number", "description": "Number of lines to read"},
					"start_offset": {"type": "number", "description": "Starting byte offset"},
					"max_size": {"type": "number", "description": "Maximum JSON output size in characters (0 = default 70000, -1 = no limit)", "default": 70000}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "write_file",
			Description: "Writes or modifies a file with the specified content",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to write"},
					"content": {"type": "string", "description": "Content to write"},
					"append": {"type": "boolean", "description": "Append to file instead of overwrite", "default": false},
					"create_dirs": {"type": "boolean", "description": "Create parent directories if needed", "default": true}
				},
				"required": ["path", "content"]
			}`),
		},
		// Batch Reading
		{
			Name:        ToolPrefix + "read_multiple_files",
			Description: "Reads multiple files simultaneously",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}, "description": "File paths to read"},
					"max_total_size": {"type": "integer", "description": "Maximum combined JSON output size in characters (default: 70000, -1 = no limit). Uses smart estimation to account for JSON encoding overhead.", "default": 70000},
					"chunk_size": {"type": "number", "description": "Chunk size in bytes", "default": 1048576}
				},
				"required": ["paths"]
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
		// Batch Editing
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
					"sort_by": {"type": "string", "enum": ["name", "size", "modified", "type"], "default": "name"},
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
					"max_depth": {"type": "number", "description": "Maximum depth", "default": 5},
					"show_hidden": {"type": "boolean", "description": "Show hidden files", "default": false},
					"include_files": {"type": "boolean", "description": "Include files", "default": false},
					"pattern": {"type": "string", "description": "File pattern filter"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "create_directories",
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
		// Archive Operations
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
	}
}
