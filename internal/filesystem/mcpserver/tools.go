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
					"path": {"type": "string"},
					"line_start": {"type": "number"},
					"line_count": {"type": "number"},
					"start_offset": {"type": "number", "description": "Starting byte offset"},
					"max_size": {"type": "number", "description": "Max JSON output chars (0=default 70000, -1=unlimited)", "default": 70000}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        ToolPrefix + "write_file",
			Description: "Write or create a file with specified content",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string"},
					"content": {"type": "string"},
					"append": {"type": "boolean", "description": "Append instead of overwrite", "default": false},
					"create_dirs": {"type": "boolean", "description": "Create parent dirs", "default": true}
				},
				"required": ["path", "content"]
			}`),
		},
		// Batch Writing
		{
			Name:        ToolPrefix + "write_multiple_files",
			Description: "Write multiple files in a single operation with auto-mkdir",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"files": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"path": {"type": "string"},
								"content": {"type": "string"}
							},
							"required": ["path", "content"]
						}
					}
				},
				"required": ["files"]
			}`),
		},
		// Batch Reading
		{
			Name:        ToolPrefix + "read_multiple_files",
			Description: "Reads multiple files simultaneously",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {"type": "array", "items": {"type": "string"}},
					"max_total_size": {"type": "integer", "description": "Max combined JSON output chars (default: 70000, -1=unlimited)", "default": 70000},
					"chunk_size": {"type": "number", "default": 1048576}
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
					"path": {"type": "string"},
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
					"path": {"type": "string"},
					"edits": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"old_string": {"type": "string"},
								"new_string": {"type": "string"}
							},
							"required": ["old_string", "new_string"]
						}
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
					"path": {"type": "string"},
					"pattern": {"type": "string"},
					"replacement": {"type": "string"},
					"regex": {"type": "boolean", "default": false},
					"dry_run": {"type": "boolean", "default": false},
					"file_types": {"type": "array", "items": {"type": "string"}, "description": "Filter by extensions"}
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
					"path": {"type": "string"},
					"show_hidden": {"type": "boolean", "default": false},
					"pattern": {"type": "string", "description": "Filename filter"},
					"sort_by": {"type": "string", "enum": ["name", "size", "modified", "type"], "default": "name"},
					"reverse": {"type": "boolean", "default": false},
					"page": {"type": "number"},
					"page_size": {"type": "number"}
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
					"path": {"type": "string"},
					"max_depth": {"type": "number", "default": 5},
					"show_hidden": {"type": "boolean", "default": false},
					"include_files": {"type": "boolean", "default": false},
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
					"path": {"type": "string"},
					"pattern": {"type": "string"},
					"recursive": {"type": "boolean", "default": true},
					"show_hidden": {"type": "boolean", "default": false},
					"max_results": {"type": "number", "default": 1000}
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
					"path": {"type": "string"},
					"pattern": {"type": "string"},
					"ignore_case": {"type": "boolean", "default": false},
					"regex": {"type": "boolean", "default": false},
					"context": {"type": "number", "description": "Context lines around match", "default": 0},
					"file_types": {"type": "array", "items": {"type": "string"}, "description": "Filter by extensions"},
					"max_results": {"type": "number", "default": 1000},
					"show_hidden": {"type": "boolean", "default": false}
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
					"source": {"type": "string"},
					"destination": {"type": "string"}
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
					"source": {"type": "string"},
					"destination": {"type": "string"}
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
					"path": {"type": "string"},
					"recursive": {"type": "boolean", "default": false}
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
						}
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
					"paths": {"type": "array", "items": {"type": "string"}},
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
					"archive": {"type": "string"},
					"destination": {"type": "string"}
				},
				"required": ["archive", "destination"]
			}`),
		},
	}
}
