package mcpserver

import (
	"encoding/json"
)

// ToolPrefix is the prefix for all llm-semantic tools
const ToolPrefix = "llm_semantic_"

// ToolDefinition defines a tool for the MCP SDK
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// GetToolDefinitions returns tool definitions for the official MCP SDK
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// 1. Semantic search
		{
			Name:        ToolPrefix + "search",
			Description: "Search code using natural language queries. Returns semantically similar code chunks ranked by relevance.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Natural language search query (e.g., 'authentication middleware' or 'database connection handling')"
					},
					"top_k": {
						"type": "integer",
						"description": "Maximum number of results to return (default: 10)"
					},
					"threshold": {
						"type": "number",
						"description": "Minimum similarity score 0.0-1.0 (default: 0.0)"
					},
					"type": {
						"type": "string",
						"enum": ["function", "method", "struct", "interface", "file"],
						"description": "Filter results by chunk type"
					},
					"path": {
						"type": "string",
						"description": "Filter results by path prefix"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON (default: true for MCP)"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - only file, name, line, score"
					}
				},
				"required": ["query"]
			}`),
		},

		// 2. Index status
		{
			Name:        ToolPrefix + "status",
			Description: "Show semantic index status including file count, chunk count, and last update time.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					}
				}
			}`),
		},

		// 3. Index build
		{
			Name:        ToolPrefix + "index",
			Description: "Build or rebuild the semantic code index for a directory. Parses code files and generates embeddings.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory path to index (default: current directory)"
					},
					"include": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Glob patterns to include (e.g., ['*.go', '*.py'])"
					},
					"exclude": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Directories to exclude (default: ['vendor', 'node_modules', '.git'])"
					},
					"force": {
						"type": "boolean",
						"description": "Force re-index all files even if unchanged"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					}
				}
			}`),
		},

		// 4. Index update (incremental)
		{
			Name:        ToolPrefix + "update",
			Description: "Incrementally update the semantic index with changed files since last indexing.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory path to update (default: current directory)"
					},
					"include": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Glob patterns to include"
					},
					"exclude": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Directories to exclude"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					}
				}
			}`),
		},
	}
}
