package mcpserver

import (
	"encoding/json"
)

// ToolPrefix is the prefix for all llm-support tools
const ToolPrefix = "llm_support_"

// ToolDefinition defines a tool for the MCP SDK
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

// GetToolDefinitions returns tool definitions for the official MCP SDK
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		// 1. Directory tree structure
		{
			Name:        ToolPrefix + "tree",
			Description: "Display directory structure as a tree. Respects .gitignore by default.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory path (default: current directory)"
					},
					"depth": {
						"type": "integer",
						"description": "Maximum depth to display"
					},
					"sizes": {
						"type": "boolean",
						"description": "Show file sizes"
					},
					"no_gitignore": {
						"type": "boolean",
						"description": "Include gitignored files"
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

		// 2. Pattern search
		{
			Name:        ToolPrefix + "grep",
			Description: "Search for regex pattern in files. Supports case-insensitive search and line numbers.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"pattern": {
						"type": "string",
						"description": "Regular expression pattern to search for"
					},
					"paths": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Files or directories to search"
					},
					"ignore_case": {
						"type": "boolean",
						"description": "Case-insensitive search"
					},
					"line_numbers": {
						"type": "boolean",
						"description": "Show line numbers"
					},
					"files_only": {
						"type": "boolean",
						"description": "Only show filenames, not matches"
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
				"required": ["pattern", "paths"]
			}`),
		},

		// 3. Multi-file existence check
		{
			Name:        ToolPrefix + "multiexists",
			Description: "Check if multiple files or directories exist. Returns detailed status for each path.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Paths to check"
					},
					"verbose": {
						"type": "boolean",
						"description": "Show file type (file/directory/symlink)"
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
				"required": ["paths"]
			}`),
		},

		// 4. JSON query
		{
			Name:        ToolPrefix + "json_query",
			Description: "Query JSON file with dot notation (e.g., '.users[0].name').",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "JSON file to query"
					},
					"query": {
						"type": "string",
						"description": "Query path (e.g., .users[0].name)"
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
				"required": ["file", "query"]
			}`),
		},

		// 5. Markdown headers extraction
		{
			Name:        ToolPrefix + "markdown_headers",
			Description: "Extract headers from markdown file. Can filter by header level.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Markdown file"
					},
					"level": {
						"type": "string",
						"description": "Filter by level (e.g., '2,3' for h2 and h3)"
					},
					"plain": {
						"type": "boolean",
						"description": "Output text only (no markdown formatting)"
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

		// 6. Template substitution
		{
			Name:        ToolPrefix + "template",
			Description: "Variable substitution in template files. Supports [[var]] or {{var}} syntax.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Template file"
					},
					"vars": {
						"type": "object",
						"description": "Variables as key-value pairs"
					},
					"syntax": {
						"type": "string",
						"enum": ["brackets", "braces"],
						"description": "Variable syntax: brackets [[var]] or braces {{var}}"
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

		// 7. Discover test infrastructure
		{
			Name:        ToolPrefix + "discover_tests",
			Description: "Discover test patterns, runners, and infrastructure in a project. Returns PATTERN, FRAMEWORK, TEST_RUNNER, CONFIG_FILE, SOURCE_DIR, TEST_DIR, E2E_DIR, and test counts.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Project path to analyze (default: current directory)"
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

		// 8. Multi-keyword search
		{
			Name:        ToolPrefix + "multigrep",
			Description: "Search for multiple keywords in parallel with intelligent output. Prioritizes definitions over usages. Outputs keyword files with DEF: and USE: prefixes.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"keywords": {
						"type": "string",
						"description": "Comma-separated keywords to search"
					},
					"path": {
						"type": "string",
						"description": "Path to search (default: current directory)"
					},
					"extensions": {
						"type": "string",
						"description": "Comma-separated file extensions (e.g., 'ts,tsx,js')"
					},
					"max_per_keyword": {
						"type": "integer",
						"description": "Max matches per keyword (default: 20)"
					},
					"ignore_case": {
						"type": "boolean",
						"description": "Case-insensitive search"
					},
					"definitions_only": {
						"type": "boolean",
						"description": "Only show definition matches"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - token-optimized format"
					},
					"output_dir": {
						"type": "string",
						"description": "Write per-keyword results to directory"
					}
				},
				"required": ["keywords"]
			}`),
		},

		// 9. File dependency analysis
		{
			Name:        ToolPrefix + "analyze_deps",
			Description: "Analyze file dependencies from a user story or task markdown file. Returns FILES_READ, FILES_MODIFY, FILES_CREATE, DIRECTORIES.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Markdown file to analyze"
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

		// 10. Project detection
		{
			Name:        ToolPrefix + "detect",
			Description: "Detect project type and technology stack. Returns STACK, LANGUAGE, PACKAGE_MANAGER, FRAMEWORK, HAS_TESTS, PYTEST_AVAILABLE.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Project path to analyze"
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

		// 11. Count items
		{
			Name:        ToolPrefix + "count",
			Description: "Count checkboxes, lines, or files. For checkboxes: returns TOTAL, CHECKED, UNCHECKED, COMPLETION%.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"mode": {
						"type": "string",
						"enum": ["checkboxes", "lines", "files"],
						"description": "What to count"
					},
					"path": {
						"type": "string",
						"description": "File or directory to analyze"
					},
					"recursive": {
						"type": "boolean",
						"description": "Search recursively"
					},
					"pattern": {
						"type": "string",
						"description": "Glob pattern (for files mode)"
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
				"required": ["mode", "path"]
			}`),
		},

		// 12. Summarize directory
		{
			Name:        ToolPrefix + "summarize_dir",
			Description: "Summarize directory contents for LLM context. Formats: outline (default), headers, frontmatter, first-lines.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory to summarize"
					},
					"format": {
						"type": "string",
						"enum": ["outline", "headers", "frontmatter", "first-lines"],
						"description": "Output format"
					},
					"recursive": {
						"type": "boolean",
						"description": "Search recursively"
					},
					"glob": {
						"type": "string",
						"description": "Glob pattern for files"
					},
					"max_tokens": {
						"type": "integer",
						"description": "Maximum tokens in output"
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
				"required": ["path"]
			}`),
		},

		// 13. Extract dependencies from package manifests
		{
			Name:        ToolPrefix + "deps",
			Description: "Extract dependencies from package manifest files. Supports package.json, requirements.txt, pyproject.toml, go.mod, Cargo.toml, Gemfile. Returns TYPE, PRODUCTION_COUNT, DEV_COUNT, TOTAL_COUNT, and dependency lists.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"manifest": {
						"type": "string",
						"description": "Path to package manifest file"
					},
					"type": {
						"type": "string",
						"enum": ["all", "prod", "dev"],
						"description": "Dependency type to extract (default: all)"
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
				"required": ["manifest"]
			}`),
		},

		// 14. Git context gathering
		{
			Name:        ToolPrefix + "git_context",
			Description: "Gather git information for LLM context. Returns BRANCH, HAS_UNCOMMITTED, COMMIT_COUNT, and RECENT_COMMITS.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Repository path (default: current directory)"
					},
					"include_diff": {
						"type": "boolean",
						"description": "Include diff statistics"
					},
					"since": {
						"type": "string",
						"description": "Only include commits since this date"
					},
					"max_commits": {
						"type": "integer",
						"description": "Maximum commits to show (default: 10)"
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

		// 15. Validate plan structure
		{
			Name:        ToolPrefix + "validate_plan",
			Description: "Validate plan directory structure. Returns VALID status, counts of user stories, acceptance criteria, documentation, and tasks. Lists any warnings or errors.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to plan directory"
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
				"required": ["path"]
			}`),
		},

		// 16. Partition work for parallel execution
		{
			Name:        ToolPrefix + "partition_work",
			Description: "Partition work items into parallel execution groups using graph coloring. Ensures items in the same group don't touch the same files. Returns TOTAL_ITEMS, GROUP_COUNT, CONFLICTS_FOUND, RECOMMENDATION, and group details.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"stories": {
						"type": "string",
						"description": "Path to user stories directory"
					},
					"tasks": {
						"type": "string",
						"description": "Path to tasks directory"
					},
					"verbose": {
						"type": "boolean",
						"description": "Show detailed file lists"
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

		// 17. Repository root detection
		{
			Name:        ToolPrefix + "repo_root",
			Description: "Find git repository root for a path. Returns ROOT (absolute path) and optionally VALID (TRUE/FALSE). Use for anchoring all paths in LLM prompts.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Starting path (default: current directory)"
					},
					"validate": {
						"type": "boolean",
						"description": "Also verify .git directory exists"
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

		// 18. Extract relevant content using LLM
		{
			Name:        ToolPrefix + "extract_relevant",
			Description: "Extract only relevant content from files using LLM API. Filters large files/directories to just the parts relevant to your query context. Great for context window relief.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "File or directory path (default: current directory)"
					},
					"context": {
						"type": "string",
						"description": "What you're looking for - describes relevance criteria"
					},
					"concurrency": {
						"type": "integer",
						"description": "Number of concurrent API calls (default: 2)"
					},
					"output": {
						"type": "string",
						"description": "Output file path (optional)"
					},
					"timeout": {
						"type": "integer",
						"description": "Timeout in seconds (default: 60)"
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
				"required": ["context"]
			}`),
		},

		// 19. Find highest numbered directory/file
		{
			Name:        ToolPrefix + "highest",
			Description: "Find highest numbered directory or file in a path. Returns HIGHEST (version), NAME, FULL_PATH, NEXT (incremented), COUNT. Auto-detects pattern based on directory context (plans, sprints, user-stories, acceptance-criteria, tasks, technical-debt).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory to search in (default: current directory)"
					},
					"pattern": {
						"type": "string",
						"description": "Custom regex pattern with capture groups for version extraction"
					},
					"type": {
						"type": "string",
						"enum": ["dir", "file", "both"],
						"description": "Type to search: dir, file, both (default: both)"
					},
					"prefix": {
						"type": "string",
						"description": "Filter to items starting with this prefix (e.g., '01-' for ACs in story 1)"
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

		// 20. Plan type extraction
		{
			Name:        ToolPrefix + "plan_type",
			Description: "Extract plan type from planning metadata files with rich type information. Supports feature, bugfix, test-remediation, tech-debt, and infrastructure types.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Plan directory path (default: current directory)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output (type only)"
					}
				}
			}`),
		},

		// 21. Git changes detection
		{
			Name:        ToolPrefix + "git_changes",
			Description: "Count and list git working tree changes with optional path filtering. Returns count and file list with support for filtering by path prefix, staged-only, and untracked inclusion.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Filter to files matching this path prefix"
					},
					"include_untracked": {
						"type": "boolean",
						"description": "Include untracked files (default: true)"
					},
					"staged_only": {
						"type": "boolean",
						"description": "Only show staged changes"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output (count only)"
					}
				}
			}`),
		},

		// 22. Context variable management
		{
			Name:        ToolPrefix + "context",
			Description: "Manage persistent key-value storage for prompt variables. Supports init, set, get, list, dump, and clear operations. Solves the 'forgotten timestamp' problem by persisting values across prompt executions.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"operation": {
						"type": "string",
						"enum": ["init", "set", "get", "list", "dump", "clear"],
						"description": "Operation to perform: init (create context file), set (store value), get (retrieve value), list (show all), dump (shell-sourceable), clear (remove all)"
					},
					"dir": {
						"type": "string",
						"description": "Directory containing context.env file (required)"
					},
					"key": {
						"type": "string",
						"description": "Variable key (for set/get operations)"
					},
					"value": {
						"type": "string",
						"description": "Variable value (for set operation)"
					},
					"default": {
						"type": "string",
						"description": "Default value if key not found (for get operation)"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON (for get/list operations)"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - just the value (for get operation)"
					}
				},
				"required": ["operation", "dir"]
			}`),
		},
	}
}
