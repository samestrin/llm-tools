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

// schemaProperty defines a JSON Schema property
type schemaProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Maximum     *float64 `json:"maximum,omitempty"`
	Items       *struct {
		Type string `json:"type"`
	} `json:"items,omitempty"`
}

// commonProperties returns the shared schema properties used by most tools.
// This centralizes the definitions to avoid duplication.
func commonProperties() map[string]schemaProperty {
	return map[string]schemaProperty{
		"json": {
			Type:        "boolean",
			Description: "Output as JSON",
		},
		"min": {
			Type:        "boolean",
			Description: "Minimal output format",
		},
		"storage": {
			Type:        "string",
			Description: "Storage backend (default: sqlite)",
			Enum:        []string{"sqlite", "qdrant"},
		},
		"collection": {
			Type:        "string",
			Description: "Collection name for qdrant storage (default: llm_semantic)",
		},
		"profile": {
			Type:        "string",
			Description: "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config",
		},
		"config": {
			Type:        "string",
			Description: "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')",
		},
	}
}

// mergeProperties combines tool-specific properties with common properties.
// Common properties can be overridden by tool-specific ones.
func mergeProperties(toolProps, common map[string]schemaProperty) map[string]schemaProperty {
	result := make(map[string]schemaProperty)
	for k, v := range common {
		result[k] = v
	}
	for k, v := range toolProps {
		result[k] = v
	}
	return result
}

// buildSchema creates a JSON Schema object with merged properties.
func buildSchema(toolProps map[string]schemaProperty, required []string) json.RawMessage {
	props := mergeProperties(toolProps, commonProperties())
	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	data, _ := json.Marshal(schema)
	return data
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
						"minimum": 0.0,
						"maximum": 1.0,
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
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					},
					"hybrid": {
						"type": "boolean",
						"description": "Enable hybrid search (dense + lexical with RRF fusion)"
					},
					"fusion_k": {
						"type": "integer",
						"description": "RRF fusion k parameter (higher = smoother ranking, default: 60)"
					},
					"fusion_alpha": {
						"type": "number",
						"description": "Fusion weight: 1.0 = dense only, 0.0 = lexical only (default: 0.7)"
					},
					"recency_boost": {
						"type": "boolean",
						"description": "Enable recency boost (recently modified files ranked higher)"
					},
					"recency_factor": {
						"type": "number",
						"description": "Recency boost factor, max boost = 1+factor (default: 0.5)"
					},
					"recency_decay": {
						"type": "integer",
						"description": "Recency half-life in days (default: 7)"
					},
					"profiles": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Profiles to search across (e.g., ['code', 'docs']). Searches each profile's collection in parallel and merges results."
					}
				},
				"required": ["query"]
			}`),
		},

		// 1b. Multisearch - batch semantic search with deduplication and boosting
		{
			Name:        ToolPrefix + "multisearch",
			Description: "Execute multiple semantic queries with intelligent deduplication and multi-match boosting. Results matching multiple queries receive boosted scores. Supports multiple output formats.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"queries": {
						"type": "array",
						"items": {"type": "string"},
						"description": "1-10 search queries to execute in parallel (e.g., ['authentication', 'JWT tokens'])"
					},
					"top_k": {
						"type": "integer",
						"description": "Maximum total results to return after deduplication (default: 15)"
					},
					"threshold": {
						"type": "number",
						"minimum": 0.0,
						"maximum": 1.0,
						"description": "Minimum similarity score 0.0-1.0 (default: 0.0)"
					},
					"profiles": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Profiles to search across (e.g., ['code', 'docs'])"
					},
					"no_boost": {
						"type": "boolean",
						"description": "Disable multi-match score boosting (default: false)"
					},
					"no_dedupe": {
						"type": "boolean",
						"description": "Disable result deduplication (default: false)"
					},
					"output": {
						"type": "string",
						"enum": ["blended", "by_query", "by_collection"],
						"description": "Output format: blended (flat list sorted by score), by_query (grouped by query), by_collection (grouped by profile)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON (default: true for MCP)"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - only essential fields"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Single profile to search (alternative to 'profiles' array; 'profiles' takes precedence if both specified)"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings"
					}
				},
				"required": ["queries"]
			}`),
		},

		// 1c. Search code - convenience wrapper with code profile pre-set
		{
			Name:        ToolPrefix + "search_code",
			Description: "Search code repository using natural language. Convenience wrapper for search with 'code' profile pre-set. Requires config file with semantic.code_collection and semantic.code_storage defined.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Natural language search query (e.g., 'authentication middleware' or 'database connection handling')"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					},
					"top_k": {
						"type": "integer",
						"description": "Maximum number of results to return (default: 10)"
					},
					"threshold": {
						"type": "number",
						"minimum": 0.0,
						"maximum": 1.0,
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
					"hybrid": {
						"type": "boolean",
						"description": "Enable hybrid search (dense + lexical with RRF fusion)"
					},
					"recency_boost": {
						"type": "boolean",
						"description": "Enable recency boost (recently modified files ranked higher)"
					}
				},
				"required": ["query"]
			}`),
		},

		// 1d. Search docs - convenience wrapper with docs profile pre-set
		{
			Name:        ToolPrefix + "search_docs",
			Description: "Search documentation using natural language. Convenience wrapper for search with 'docs' profile pre-set. Requires config file with semantic.docs_collection and semantic.docs_storage defined.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Natural language search query (e.g., 'API authentication' or 'configuration options')"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					},
					"top_k": {
						"type": "integer",
						"description": "Maximum number of results to return (default: 10)"
					},
					"threshold": {
						"type": "number",
						"minimum": 0.0,
						"maximum": 1.0,
						"description": "Minimum similarity score 0.0-1.0 (default: 0.0)"
					},
					"path": {
						"type": "string",
						"description": "Filter results by path prefix"
					},
					"hybrid": {
						"type": "boolean",
						"description": "Enable hybrid search (dense + lexical with RRF fusion)"
					}
				},
				"required": ["query"]
			}`),
		},

		// 1e. Search memory - convenience wrapper with memory profile pre-set
		{
			Name:        ToolPrefix + "search_memory",
			Description: "Search stored memories using natural language. Convenience wrapper for memory search with 'memory' profile pre-set. Requires config file with semantic.memory_collection and semantic.memory_storage defined.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Natural language search query (e.g., 'authentication decisions' or 'API design choices')"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					},
					"top_k": {
						"type": "integer",
						"description": "Maximum number of results to return (default: 10)"
					},
					"threshold": {
						"type": "number",
						"minimum": 0.0,
						"maximum": 1.0,
						"description": "Minimum similarity score 0.0-1.0 (default: 0.0)"
					},
					"tags": {
						"type": "string",
						"description": "Filter by tags (comma-separated)"
					},
					"status": {
						"type": "string",
						"enum": ["pending", "promoted"],
						"description": "Filter by status"
					}
				},
				"required": ["query"]
			}`),
		},

		// 2. Index status
		{
			Name:        ToolPrefix + "index_status",
			Description: "Show semantic index status including file count, chunk count, and last update time.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
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
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					},
					"recalibrate": {
						"type": "boolean",
						"description": "Force recalibration of score thresholds even if calibration exists"
					},
					"skip_calibration": {
						"type": "boolean",
						"description": "Skip the calibration step during indexing"
					}
				}
			}`),
		},

		// 4. Index update (incremental)
		{
			Name:        ToolPrefix + "index_update",
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
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					}
				}
			}`),
		},

		// 5. Memory store
		{
			Name:        ToolPrefix + "memory_store",
			Description: "Store a learned decision or clarification in semantic memory for future retrieval.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"question": {
						"type": "string",
						"description": "The question or decision being recorded"
					},
					"answer": {
						"type": "string",
						"description": "The answer or decision made"
					},
					"tags": {
						"type": "string",
						"description": "Comma-separated context tags (e.g., 'auth,security')"
					},
					"source": {
						"type": "string",
						"description": "Origin source (default: 'manual')"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output format"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					}
				},
				"required": ["question", "answer"]
			}`),
		},

		// 6. Memory search
		{
			Name:        ToolPrefix + "memory_search",
			Description: "Search stored memories using natural language. Returns semantically similar entries.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "Natural language search query"
					},
					"top_k": {
						"type": "integer",
						"description": "Maximum number of results to return (default: 10)"
					},
					"threshold": {
						"type": "number",
						"minimum": 0.0,
						"maximum": 1.0,
						"description": "Minimum similarity score 0.0-1.0 (default: 0.0)"
					},
					"tags": {
						"type": "string",
						"description": "Filter by tags (comma-separated)"
					},
					"status": {
						"type": "string",
						"enum": ["pending", "promoted"],
						"description": "Filter by status"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output format"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					}
				},
				"required": ["query"]
			}`),
		},

		// 7. Memory promote
		{
			Name:        ToolPrefix + "memory_promote",
			Description: "Promote a memory entry to CLAUDE.md for persistent project knowledge.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id": {
						"type": "string",
						"description": "Memory entry ID to promote"
					},
					"target": {
						"type": "string",
						"description": "Target CLAUDE.md file path"
					},
					"section": {
						"type": "string",
						"description": "Section header to append under (default: 'Learned Clarifications')"
					},
					"force": {
						"type": "boolean",
						"description": "Re-promote even if already promoted"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output format"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					}
				},
				"required": ["id", "target"]
			}`),
		},

		// 8. Memory list
		{
			Name:        ToolPrefix + "memory_list",
			Description: "List stored memories with optional filtering by status.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"limit": {
						"type": "integer",
						"description": "Maximum number of entries to return (default: 50)"
					},
					"status": {
						"type": "string",
						"enum": ["pending", "promoted"],
						"description": "Filter by status"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output format"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					}
				}
			}`),
		},

		// 9. Memory delete
		{
			Name:        ToolPrefix + "memory_delete",
			Description: "Delete a memory entry by ID.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"id": {
						"type": "string",
						"description": "Memory entry ID to delete"
					},
					"force": {
						"type": "boolean",
						"description": "Skip confirmation"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output format"
					},
					"storage": {
						"type": "string",
						"enum": ["sqlite", "qdrant"],
						"description": "Storage backend (default: sqlite)"
					},
					"collection": {
						"type": "string",
						"description": "Collection name for qdrant storage (default: llm_semantic)"
					},
					"profile": {
						"type": "string",
						"description": "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config"
					},
					"config": {
						"type": "string",
						"description": "Path to config.yaml file containing profile settings (e.g., '.planning/.config/config.yaml')"
					}
				},
				"required": ["id"]
			}`),
		},
	}
}
