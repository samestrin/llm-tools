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
			Description: "Configuration profile name (e.g., 'code', 'docs', 'memory', 'sprints') - looks up {profile}_collection and {profile}_storage from config (default: .planning/.config/config.yaml)",
		},
		"config": {
			Type:        "string",
			Description: "Path to config.yaml file containing profile settings (default: .planning/.config/config.yaml)",
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
			Description: "Search code using natural language queries.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"query": {
								"type": "string",
								"description": "Natural language query"
							},
							"top_k": {
								"type": "integer",
								"description": "Max results (default: 10)"
							},
							"threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min score 0.0-1.0"
							},
							"type": {
								"type": "string",
								"enum": ["function", "method", "struct", "interface", "file"]
							},
							"path": {
								"type": "string"
							},
							"profile": {
								"type": "string",
								"description": "Profile: code, docs, memory, or sprints"
							},
							"config": {
								"type": "string",
								"description": "Config file path"
							},
							"hybrid": {
								"type": "boolean",
								"description": "Enable hybrid search"
							},
							"fusion_k": {
								"type": "integer",
								"description": "RRF fusion k (default: 60)"
							},
							"fusion_alpha": {
								"type": "number",
								"description": "Dense/lexical weight (default: 0.7)"
							},
							"recency_boost": {
								"type": "boolean",
								"description": "Boost recently modified files"
							},
							"recency_factor": {
								"type": "number",
								"description": "Boost factor (default: 0.5)"
							},
							"recency_decay": {
								"type": "integer",
								"description": "Half-life in days (default: 7)"
							},
							"profiles": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Profiles to search (e.g., ['code', 'docs'])"
							},
							"rerank": {
								"type": "boolean",
								"description": "Enable reranking"
							},
							"rerank_candidates": {
								"type": "integer",
								"description": "Rerank candidates (default: top_k*5)"
							},
							"rerank_threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min reranker score"
							},
							"no_rerank": {
								"type": "boolean",
								"description": "Disable reranking"
							},
							"prefilter": {
								"type": "boolean",
								"description": "Enable lexical prefiltering"
							},
							"prefilter_top": {
								"type": "integer",
								"description": "Prefilter candidates (default: top_k*10)"
							}
						},
						"required": ["query"]
					}`),
		},

		// 1b. Multisearch - batch semantic search with deduplication and boosting
		{
			Name:        ToolPrefix + "multisearch",
			Description: "Execute multiple semantic queries with deduplication and multi-match boosting.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"queries": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "1-10 search queries"
							},
							"top_k": {
								"type": "integer",
								"description": "Max results after dedup (default: 15)"
							},
							"threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min score 0.0-1.0"
							},
							"profiles": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Profiles to search (e.g., ['code', 'docs'])"
							},
							"no_boost": {
								"type": "boolean",
								"description": "Disable multi-match boosting"
							},
							"no_dedupe": {
								"type": "boolean",
								"description": "Disable deduplication"
							},
							"output": {
								"type": "string",
								"enum": ["blended", "by_query", "by_collection"],
								"description": "blended, by_query, or by_collection"
							},
							"profile": {
								"type": "string",
								"description": "Single profile (alternative to profiles[])"
							},
							"config": {
								"type": "string",
								"description": "Config file path"
							}
						},
						"required": ["queries"]
					}`),
		},

		// 1c. Search code - convenience wrapper with code profile pre-set
		{
			Name:        ToolPrefix + "search_code",
			Description: "Search code with 'code' profile pre-set.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"query": {
								"type": "string",
								"description": "Natural language query"
							},
							"top_k": {
								"type": "integer",
								"description": "Max results (default: 10)"
							},
							"threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min score 0.0-1.0"
							},
							"type": {
								"type": "string",
								"enum": ["function", "method", "struct", "interface", "file"]
							},
							"path": {
								"type": "string"
							},
							"hybrid": {
								"type": "boolean",
								"description": "Enable hybrid search"
							},
							"recency_boost": {
								"type": "boolean",
								"description": "Boost recently modified files"
							}
						},
						"required": ["query"]
					}`),
		},

		// 1d. Search docs - convenience wrapper with docs profile pre-set
		{
			Name:        ToolPrefix + "search_docs",
			Description: "Search docs with 'docs' profile pre-set.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"query": {
								"type": "string",
								"description": "Natural language query"
							},
							"top_k": {
								"type": "integer",
								"description": "Max results (default: 10)"
							},
							"threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min score 0.0-1.0"
							},
							"path": {
								"type": "string"
							},
							"hybrid": {
								"type": "boolean",
								"description": "Enable hybrid search"
							}
						},
						"required": ["query"]
					}`),
		},

		// 1e. Search memory - convenience wrapper with memory profile pre-set
		{
			Name:        ToolPrefix + "search_memory",
			Description: "Search memories with 'memory' profile pre-set.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"query": {
								"type": "string",
								"description": "Natural language query"
							},
							"top_k": {
								"type": "integer",
								"description": "Max results (default: 10)"
							},
							"threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min score 0.0-1.0"
							},
							"tags": {
								"type": "string"
							},
							"status": {
								"type": "string",
								"enum": ["pending", "promoted"]
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
							"profile": {
								"type": "string",
								"description": "Profile: code, docs, memory, or sprints"
							},
							"config": {
								"type": "string",
								"description": "Config file path"
							}
						}
					}`),
		},

		// 3. Index build
		{
			Name:        ToolPrefix + "index",
			Description: "Build or rebuild the semantic index for a directory.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "Directory to index (supports globs)"
							},
							"include": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Include patterns (e.g., ['*.go'])"
							},
							"exclude": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Exclude patterns"
							},
							"exclude_tests": {
								"type": "boolean",
								"description": "Exclude test files"
							},
							"force": {
								"type": "boolean",
								"description": "Force re-index all"
							},
							"batch_size": {
								"type": "integer",
								"description": "Vectors per batch (default: 0)"
							},
							"parallel": {
								"type": "integer",
								"description": "Parallel uploads"
							},
							"embed_batch_size": {
								"type": "integer",
								"description": "Chunks per embed call (default: 0)"
							},
							"profile": {
								"type": "string",
								"description": "Profile: code, docs, memory, or sprints"
							},
							"config": {
								"type": "string",
								"description": "Config file path"
							},
							"recalibrate": {
								"type": "boolean",
								"description": "Force recalibration"
							},
							"skip_calibration": {
								"type": "boolean",
								"description": "Skip calibration"
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
								"description": "Directory to update (supports globs)"
							},
							"include": {
								"type": "array",
								"items": {
									"type": "string"
								}
							},
							"exclude": {
								"type": "array",
								"items": {
									"type": "string"
								}
							},
							"profile": {
								"type": "string",
								"description": "Profile: code, docs, memory, or sprints"
							},
							"config": {
								"type": "string",
								"description": "Config file path"
							}
						}
					}`),
		},

		// 5. Memory store
		{
			Name:        ToolPrefix + "memory_store",
			Description: "Store a decision or clarification in semantic memory.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"question": {
								"type": "string"
							},
							"answer": {
								"type": "string"
							},
							"tags": {
								"type": "string"
							},
							"source": {
								"type": "string"
							},
							"file_path": {
								"type": "string",
								"description": "Write as markdown file"
							},
							"sprints": {
								"type": "string",
								"description": "Sprint refs (comma-separated)"
							},
							"files": {
								"type": "string",
								"description": "File refs (comma-separated)"
							}
						},
						"required": ["question", "answer"]
					}`),
		},

		// 6. Memory search
		{
			Name:        ToolPrefix + "memory_search",
			Description: "Search stored memories using natural language.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"query": {
								"type": "string"
							},
							"top_k": {
								"type": "integer",
								"description": "Max results (default: 10)"
							},
							"threshold": {
								"type": "number",
								"minimum": 0.0,
								"maximum": 1.0,
								"description": "Min score 0.0-1.0"
							},
							"tags": {
								"type": "string"
							},
							"status": {
								"type": "string",
								"enum": ["pending", "promoted"]
							},
							"decay": {
								"type": "boolean",
								"description": "Apply temporal decay (recent memories score higher)"
							},
							"decay_half_life": {
								"type": "number",
								"description": "Decay half-life in days (default: 90)"
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
								"type": "string"
							},
							"target": {
								"type": "string"
							},
							"section": {
								"type": "string",
								"description": "Section header"
							},
							"force": {
								"type": "boolean"
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
								"description": "Max entries (default: 50)"
							},
							"status": {
								"type": "string",
								"enum": ["pending", "promoted"]
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
								"type": "string"
							},
							"force": {
								"type": "boolean"
							}
						},
						"required": ["id"]
					}`),
		},

		// 10. Collection delete
		{
			Name:        ToolPrefix + "collection_delete",
			Description: "Delete all chunks for a specific profile from the index.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"profile": {
								"type": "string",
								"enum": ["code", "docs", "memory", "sprints"],
								"description": "Profile to delete"
							},
							"force": {
								"type": "boolean"
							},
							"config": {
								"type": "string",
								"description": "Config file path"
							}
						},
						"required": ["profile"]
					}`),
		},

		// 11. Memory stats
		{
			Name:        ToolPrefix + "memory_stats",
			Description: "Display retrieval statistics for stored memories.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"id": {
								"type": "string"
							},
							"min_retrievals": {
								"type": "integer",
								"description": "Min retrievals filter"
							},
							"status": {
								"type": "string",
								"enum": ["pending", "promoted"],
								"description": "pending or promoted"
							},
							"tags": {
								"type": "string",
								"description": "Tag filter"
							},
							"limit": {
								"type": "integer",
								"description": "Max results"
							},
							"history": {
								"type": "boolean",
								"description": "Show retrieval history (needs --id)"
							},
							"prune": {
								"type": "boolean",
								"description": "Prune old log entries"
							},
							"older_than": {
								"type": "integer",
								"description": "Days threshold for pruning"
							},
							"yes": {
								"type": "boolean",
								"description": "Skip prune confirmation"
							}
						}
					}`),
		},
	}
}
