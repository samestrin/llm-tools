package mcpserver

import (
	"encoding/json"
)

// ToolPrefix is the prefix for all llm-clarification tools
const ToolPrefix = "llm_clarify_"

// ToolDefinition defines a tool for the MCP SDK
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// GetToolDefinitions returns tool definitions for the official MCP SDK
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// 1. Match clarification (API required)
		{
			Name:        ToolPrefix + "match",
			Description: "Match a new question against existing clarification entries using LLM semantic matching. Returns match ID, confidence score (0-1), and reasoning. Use this to find if a question has been asked before. REQUIRES: OpenAI-compatible API configured via env vars or .planning/.config/openai_* files.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"question": {
						"type": "string",
						"description": "The new question to match against existing entries"
					},
					"entries_file": {
						"type": "string",
						"description": "Path to YAML file containing existing clarification entries"
					},
					"entries_json": {
						"type": "string",
						"description": "JSON string of existing entries (alternative to entries_file)"
					},
					"timeout": {
						"type": "integer",
						"description": "API timeout in seconds (default: 30)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["question"]
			}`),
		},

		// 2. Cluster clarifications (API required)
		{
			Name:        ToolPrefix + "cluster",
			Description: "Group semantically similar questions into clusters. Useful for identifying duplicate or related clarifications across sprints. Returns clusters with labels and question lists. REQUIRES: OpenAI-compatible API configured.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"questions_file": {
						"type": "string",
						"description": "File containing questions (YAML tracking file or plain text, one per line)"
					},
					"questions_json": {
						"type": "string",
						"description": "JSON array of questions (alternative to questions_file)"
					},
					"timeout": {
						"type": "integer",
						"description": "API timeout in seconds (default: 30)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				}
			}`),
		},

		// 3. Detect conflicts (API required)
		{
			Name:        ToolPrefix + "detect_conflicts",
			Description: "Find clarification entries with conflicting answers. Analyzes entries that may ask the same underlying question but have different answers. Returns conflicts with severity and resolution suggestions. REQUIRES: OpenAI-compatible API configured.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tracking_file": {
						"type": "string",
						"description": "Path to clarification-tracking.yaml file"
					},
					"timeout": {
						"type": "integer",
						"description": "API timeout in seconds (default: 30)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["tracking_file"]
			}`),
		},

		// 4. Validate clarifications (API required)
		{
			Name:        ToolPrefix + "validate",
			Description: "Validate clarifications against current project state. Flags entries that may be stale, outdated, or need review based on project context and last-seen dates. REQUIRES: OpenAI-compatible API configured.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tracking_file": {
						"type": "string",
						"description": "Path to clarification-tracking.yaml file"
					},
					"context": {
						"type": "string",
						"description": "Project context description (auto-detected from package.json etc. if not provided)"
					},
					"timeout": {
						"type": "integer",
						"description": "API timeout in seconds (default: 30)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["tracking_file"]
			}`),
		},

		// 5. Initialize tracking (no API)
		{
			Name:        ToolPrefix + "init",
			Description: "Initialize a new clarification tracking file with proper schema. Creates the file at the specified path. Use before starting clarification tracking for a project.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"output": {
						"type": "string",
						"description": "Output file path (e.g., .planning/.config/clarification-tracking.yaml)"
					},
					"force": {
						"type": "boolean",
						"description": "Overwrite if file already exists"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["output"]
			}`),
		},

		// 6. Add clarification (no API)
		{
			Name:        ToolPrefix + "add",
			Description: "Add or update a clarification entry in the tracking file. If a matching entry exists (by ID or simple match), updates it with incremented occurrence count. Otherwise creates a new entry with auto-generated ID. Handles all YAML serialization internally.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tracking_file": {
						"type": "string",
						"description": "Path to tracking YAML file"
					},
					"question": {
						"type": "string",
						"description": "The clarification question"
					},
					"answer": {
						"type": "string",
						"description": "The answer/decision"
					},
					"id": {
						"type": "string",
						"description": "Entry ID (auto-generated if not provided)"
					},
					"sprint_id": {
						"type": "string",
						"description": "Sprint ID where this was asked"
					},
					"context_tags": {
						"type": "string",
						"description": "Comma-separated context tags (e.g., 'frontend,testing')"
					},
					"check_match": {
						"type": "boolean",
						"description": "Check for existing match before creating new entry"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["tracking_file", "question"]
			}`),
		},

		// 7. Promote clarification (no API)
		{
			Name:        ToolPrefix + "promote",
			Description: "Promote a clarification entry to CLAUDE.md. Updates entry status to 'promoted' and appends the clarification to the target CLAUDE.md file under a 'Learned Clarifications' section, organized by category based on context_tags.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tracking_file": {
						"type": "string",
						"description": "Path to tracking YAML file"
					},
					"id": {
						"type": "string",
						"description": "Entry ID to promote"
					},
					"target": {
						"type": "string",
						"description": "Target CLAUDE.md file (e.g., CLAUDE.md, apps/web/CLAUDE.md)"
					},
					"force": {
						"type": "boolean",
						"description": "Re-promote if already promoted"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["tracking_file", "id", "target"]
			}`),
		},

		// 8. List entries (no API)
		{
			Name:        ToolPrefix + "list",
			Description: "List entries in the tracking file with optional filtering by status or minimum occurrence count. Useful for reviewing what clarifications exist and identifying promotion candidates.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tracking_file": {
						"type": "string",
						"description": "Path to tracking YAML file"
					},
					"status": {
						"type": "string",
						"enum": ["pending", "promoted", "expired", "rejected"],
						"description": "Filter by status"
					},
					"min_occurrences": {
						"type": "integer",
						"description": "Minimum occurrences to show (useful for finding promotion candidates)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["tracking_file"]
			}`),
		},

		// 9. Delete clarification (no API)
		{
			Name:        ToolPrefix + "delete",
			Description: "Delete a clarification entry by ID from the storage file. Supports both YAML and SQLite storage backends.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Path to storage file (YAML or SQLite)"
					},
					"id": {
						"type": "string",
						"description": "Entry ID to delete"
					},
					"force": {
						"type": "boolean",
						"description": "Skip confirmation prompt"
					},
					"quiet": {
						"type": "boolean",
						"description": "Suppress output"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["file", "id"]
			}`),
		},

		// 10. Export memory (no API)
		{
			Name:        ToolPrefix + "export",
			Description: "Export clarification data from any storage format (SQLite or YAML) to a human-readable YAML file for editing or backup.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {
						"type": "string",
						"description": "Source storage file path"
					},
					"output": {
						"type": "string",
						"description": "Output YAML file path"
					},
					"quiet": {
						"type": "boolean",
						"description": "Suppress output"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["source", "output"]
			}`),
		},

		// 11. Import memory (no API)
		{
			Name:        ToolPrefix + "import",
			Description: "Import clarification data from a YAML file to any supported storage format (SQLite or YAML). Supports append, overwrite, and merge modes.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"source": {
						"type": "string",
						"description": "Source YAML file path"
					},
					"target": {
						"type": "string",
						"description": "Target storage file path"
					},
					"mode": {
						"type": "string",
						"enum": ["append", "overwrite", "merge"],
						"description": "Import mode: append (skip existing), overwrite (replace all), merge (update existing)"
					},
					"quiet": {
						"type": "boolean",
						"description": "Suppress output"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["source", "target"]
			}`),
		},

		// 12. Optimize memory (no API)
		{
			Name:        ToolPrefix + "optimize",
			Description: "Optimize clarification storage for better performance. Supports vacuum (SQLite only), prune-stale, and stats operations.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Storage file path"
					},
					"vacuum": {
						"type": "boolean",
						"description": "Run SQLite VACUUM to reclaim space (SQLite only)"
					},
					"prune_stale": {
						"type": "string",
						"description": "Remove entries older than duration (e.g., 30d, 90d)"
					},
					"stats": {
						"type": "boolean",
						"description": "Show storage statistics"
					},
					"quiet": {
						"type": "boolean",
						"description": "Suppress output"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["file"]
			}`),
		},

		// 13. Reconcile memory (no API)
		{
			Name:        ToolPrefix + "reconcile",
			Description: "Scan clarification entries for file path references and identify references to files that no longer exist in the codebase. Marks stale entries for review.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Storage file path"
					},
					"project_root": {
						"type": "string",
						"description": "Project root directory to scan for file existence"
					},
					"dry_run": {
						"type": "boolean",
						"description": "Show changes without applying"
					},
					"quiet": {
						"type": "boolean",
						"description": "Suppress output"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					}
				},
				"required": ["file", "project_root"]
			}`),
		},
	}
}
