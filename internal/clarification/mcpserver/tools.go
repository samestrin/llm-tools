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
					"json_output": {
						"type": "boolean",
						"description": "Output as JSON for parsing"
					}
				},
				"required": ["tracking_file"]
			}`),
		},
	}
}
