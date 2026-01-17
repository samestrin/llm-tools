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
			Description: "Display directory structure as a tree. Respects .gitignore and excludes common build directories (node_modules, vendor, target, __pycache__, etc.) by default. Use max_entries to limit output size.",
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
					"max_entries": {
						"type": "integer",
						"description": "Maximum entries to display (default: 500, 0 = unlimited)"
					},
					"sizes": {
						"type": "boolean",
						"description": "Show file sizes"
					},
					"exclude": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Regex patterns to exclude (e.g., [\"test\", \"fixtures\"])"
					},
					"no_gitignore": {
						"type": "boolean",
						"description": "Disable .gitignore filtering"
					},
					"no_default_excludes": {
						"type": "boolean",
						"description": "Disable default excludes (node_modules, vendor, target, etc.)"
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
			Description: "Extract only relevant content from files or URLs using LLM API. Filters large files/directories/web pages to just the parts relevant to your query context. HTML is automatically converted to clean text. Great for context window relief.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "File path, directory path, or URL (http/https). HTML content is auto-converted to text. Default: current directory"
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

		// 19. Extract links from URL
		{
			Name:        ToolPrefix + "extract_links",
			Description: "Extract and rank links from a URL. Two modes: (1) Heuristic (default): scores by HTML context (h1=100, h2=85, nav=30, footer=10). (2) LLM mode (when --context provided): uses AI to rank by semantic relevance to context. Returns links with href, text, context, score, section.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"url": {
						"type": "string",
						"description": "URL to extract links from (http/https)"
					},
					"context": {
						"type": "string",
						"description": "Context for LLM-based ranking (enables LLM mode). When provided, links are scored by semantic relevance instead of HTML position."
					},
					"timeout": {
						"type": "integer",
						"description": "HTTP timeout in seconds (default: 30)"
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
				"required": ["url"]
			}`),
		},

		// 20. Find highest numbered directory/file
		{
			Name:        ToolPrefix + "highest",
			Description: "Find highest numbered directory or file in a path. Returns HIGHEST (version), NAME, FULL_PATH, NEXT (incremented), COUNT. Auto-detects pattern based on directory context (plans, sprints, user-stories, acceptance-criteria, tasks, technical-debt). Use 'paths' to search multiple directories (e.g., active + completed) and find the global highest.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory to search in (default: current directory)"
					},
					"paths": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Multiple directories to search for global highest (e.g., ['plans/active', 'plans/completed'])"
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

		// 22. Context multiset - batch set multiple key-value pairs
		{
			Name:        ToolPrefix + "context_multiset",
			Description: "Set multiple key-value pairs in a single operation. Validates all keys before writing (atomic). More efficient than multiple set calls.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"dir": {
						"type": "string",
						"description": "Directory containing context.env file (required)"
					},
					"pairs": {
						"type": "object",
						"description": "Key-value pairs to set (e.g., {\"KEY1\": \"val1\", \"KEY2\": \"val2\"})"
					}
				},
				"required": ["dir", "pairs"]
			}`),
		},

		// 23. Context multiget - batch retrieve multiple values
		{
			Name:        ToolPrefix + "context_multiget",
			Description: "Retrieve multiple values in a single operation. More efficient than multiple get calls. Returns error if any key is missing.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"dir": {
						"type": "string",
						"description": "Directory containing context.env file (required)"
					},
					"keys": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Keys to retrieve"
					},
					"defaults": {
						"type": "object",
						"description": "Default values for keys not found (e.g., {\"KEY1\": \"default1\"})"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON object"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - values only, newline-separated"
					}
				},
				"required": ["dir", "keys"]
			}`),
		},

		// 24. Context variable management
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

		// 25. YAML get - retrieve value by dot-notation key
		{
			Name:        ToolPrefix + "yaml_get",
			Description: "Retrieve a value from YAML config file by dot-notation key. Supports nested keys (helper.llm) and array indices (items[0]).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Path to YAML config file"
					},
					"key": {
						"type": "string",
						"description": "Dot-notation key (e.g., helper.llm, items[0].name)"
					},
					"default": {
						"type": "string",
						"description": "Default value if key not found"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - just the value"
					}
				},
				"required": ["file", "key"]
			}`),
		},

		// 26. YAML set - store value at dot-notation key
		{
			Name:        ToolPrefix + "yaml_set",
			Description: "Store a value in YAML config file at the specified key. Creates intermediate keys if needed. Preserves comments.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Path to YAML config file"
					},
					"key": {
						"type": "string",
						"description": "Dot-notation key (e.g., helper.llm)"
					},
					"value": {
						"type": "string",
						"description": "Value to set"
					},
					"create": {
						"type": "boolean",
						"description": "Create file if it doesn't exist"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output"
					}
				},
				"required": ["file", "key", "value"]
			}`),
		},

		// 27. YAML multiget - retrieve multiple values
		{
			Name:        ToolPrefix + "yaml_multiget",
			Description: "Retrieve multiple values from YAML config file in a single operation. More efficient than multiple get calls.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Path to YAML config file"
					},
					"keys": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Dot-notation keys to retrieve"
					},
					"defaults": {
						"type": "object",
						"description": "Default values for keys (e.g., {\"helper.llm\": \"gemini\"})"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output - values only, newline-separated"
					}
				},
				"required": ["file", "keys"]
			}`),
		},

		// 28. YAML multiset - set multiple key-value pairs
		{
			Name:        ToolPrefix + "yaml_multiset",
			Description: "Set multiple key-value pairs in YAML config file atomically. Validates all keys before writing.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Path to YAML config file"
					},
					"pairs": {
						"type": "object",
						"description": "Key-value pairs to set (e.g., {\"helper.llm\": \"claude\", \"helper.max_lines\": \"2500\"})"
					},
					"create": {
						"type": "boolean",
						"description": "Create file if it doesn't exist"
					},
					"json": {
						"type": "boolean",
						"description": "Output as JSON"
					},
					"min": {
						"type": "boolean",
						"description": "Minimal output"
					}
				},
				"required": ["file", "pairs"]
			}`),
		},

		// 29. Parse arguments into structured format
		{
			Name:        ToolPrefix + "args",
			Description: "Parse command-line arguments into structured format. Separates positional arguments from flags and key-value pairs. Returns POSITIONAL, FLAG_NAME: value, BOOLEAN_FLAG: true.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"arguments": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Arguments to parse"
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
				"required": ["arguments"]
			}`),
		},

		// 30. Concatenate files with headers
		{
			Name:        ToolPrefix + "catfiles",
			Description: "Concatenate multiple files or directory contents with headers. Each file is prefixed with a header showing file path and size. Respects .gitignore by default.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Files or directories to concatenate"
					},
					"max_size": {
						"type": "integer",
						"description": "Maximum total size in MB (default: 10)"
					},
					"no_gitignore": {
						"type": "boolean",
						"description": "Disable .gitignore filtering"
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

		// 31. Decode text
		{
			Name:        ToolPrefix + "decode",
			Description: "Decode text using base64, base32, hex, or URL encoding.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {
						"type": "string",
						"description": "Text to decode"
					},
					"encoding": {
						"type": "string",
						"enum": ["base64", "base32", "hex", "url"],
						"description": "Encoding type (default: base64)"
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
				"required": ["text"]
			}`),
		},

		// 32. Compare files
		{
			Name:        ToolPrefix + "diff",
			Description: "Compare two files and show the differences.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file1": {
						"type": "string",
						"description": "First file path"
					},
					"file2": {
						"type": "string",
						"description": "Second file path"
					},
					"unified": {
						"type": "boolean",
						"description": "Use unified diff format"
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
				"required": ["file1", "file2"]
			}`),
		},

		// 33. Encode text
		{
			Name:        ToolPrefix + "encode",
			Description: "Encode text using base64, base32, hex, or URL encoding.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {
						"type": "string",
						"description": "Text to encode"
					},
					"encoding": {
						"type": "string",
						"enum": ["base64", "base32", "hex", "url"],
						"description": "Encoding type (default: base64)"
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
				"required": ["text"]
			}`),
		},

		// 34. Extract patterns from text
		{
			Name:        ToolPrefix + "extract",
			Description: "Extract patterns from text files. Types: urls, paths, variables, todos, emails, ips.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"enum": ["urls", "paths", "variables", "todos", "emails", "ips"],
						"description": "Type of pattern to extract"
					},
					"file": {
						"type": "string",
						"description": "File to extract from"
					},
					"count": {
						"type": "boolean",
						"description": "Show count only"
					},
					"unique": {
						"type": "boolean",
						"description": "Remove duplicates"
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
				"required": ["type", "file"]
			}`),
		},

		// 35. Batch process files with LLM
		{
			Name:        ToolPrefix + "foreach",
			Description: "Process multiple files through an LLM using a template. Template variables: [[CONTENT]], [[FILENAME]], [[FILEPATH]], [[EXTENSION]], [[DIRNAME]], [[INDEX]], [[TOTAL]].",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"files": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Files to process"
					},
					"glob": {
						"type": "string",
						"description": "Glob pattern to match files (alternative to files)"
					},
					"template": {
						"type": "string",
						"description": "Template file with [[var]] placeholders (required)"
					},
					"llm": {
						"type": "string",
						"description": "LLM binary to use (default: from config or 'gemini')"
					},
					"output_dir": {
						"type": "string",
						"description": "Output directory for processed files"
					},
					"output_pattern": {
						"type": "string",
						"description": "Output filename pattern (e.g., '{{name}}-processed.md')"
					},
					"parallel": {
						"type": "integer",
						"description": "Number of parallel processes (default: 1)"
					},
					"skip_existing": {
						"type": "boolean",
						"description": "Skip files where output already exists"
					},
					"timeout": {
						"type": "integer",
						"description": "Timeout per file in seconds (default: 120)"
					},
					"vars": {
						"type": "object",
						"description": "Template variables as key-value pairs"
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
				"required": ["template"]
			}`),
		},

		// 36. Generate hash checksums
		{
			Name:        ToolPrefix + "hash",
			Description: "Generate hash checksums for files. Algorithms: md5, sha1, sha256 (default), sha512.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"paths": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Files to hash"
					},
					"algorithm": {
						"type": "string",
						"enum": ["md5", "sha1", "sha256", "sha512"],
						"description": "Hash algorithm (default: sha256)"
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

		// 37. Initialize temp directory
		{
			Name:        ToolPrefix + "init_temp",
			Description: "Initialize temp directory at .planning/.temp/{name}/ with common variables. Returns TEMP_DIR, REPO_ROOT, TODAY, TIMESTAMP, EPOCH, STATUS, CONTEXT_FILE. Optionally includes BRANCH and COMMIT_SHORT with --with-git.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name for temp directory (required)"
					},
					"clean": {
						"type": "boolean",
						"description": "Remove existing files (default: true)"
					},
					"preserve": {
						"type": "boolean",
						"description": "Keep existing files"
					},
					"with_git": {
						"type": "boolean",
						"description": "Include BRANCH and COMMIT_SHORT in output"
					},
					"skip_context": {
						"type": "boolean",
						"description": "Don't create context.env file"
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
				"required": ["name"]
			}`),
		},

		// 38. Clean temp directory
		{
			Name:        ToolPrefix + "clean_temp",
			Description: "Clean up temp directories created by init_temp. Removes specific directories by name, all directories, or directories older than a specified duration.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {
						"type": "string",
						"description": "Name of temp directory to remove"
					},
					"all": {
						"type": "boolean",
						"description": "Remove all temp directories"
					},
					"older_than": {
						"type": "string",
						"description": "Remove directories older than duration (e.g., 7d, 24h, 1h30m)"
					},
					"dry_run": {
						"type": "boolean",
						"description": "Show what would be removed without removing"
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

		// 39. Evaluate math expressions
		{
			Name:        ToolPrefix + "math",
			Description: "Evaluate mathematical expressions safely. Supports basic arithmetic, functions (sin, cos, sqrt, etc.), and variables.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"expression": {
						"type": "string",
						"description": "Mathematical expression to evaluate"
					}
				},
				"required": ["expression"]
			}`),
		},

		// 39. Execute LLM prompt
		{
			Name:        ToolPrefix + "prompt",
			Description: "Execute an LLM prompt with template substitution, retry logic, and validation. Supports caching and response validation.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"prompt": {
						"type": "string",
						"description": "Direct prompt text"
					},
					"file": {
						"type": "string",
						"description": "Read prompt from file"
					},
					"template": {
						"type": "string",
						"description": "Template file with [[var]] placeholders"
					},
					"llm": {
						"type": "string",
						"description": "LLM binary to use (default: from config or 'gemini')"
					},
					"instruction": {
						"type": "string",
						"description": "System instruction for the LLM"
					},
					"vars": {
						"type": "object",
						"description": "Template variables as key-value pairs"
					},
					"retries": {
						"type": "integer",
						"description": "Number of retries on failure"
					},
					"retry_delay": {
						"type": "integer",
						"description": "Initial retry delay in seconds (default: 2)"
					},
					"timeout": {
						"type": "integer",
						"description": "Timeout in seconds (default: 120)"
					},
					"cache": {
						"type": "boolean",
						"description": "Enable response caching"
					},
					"cache_ttl": {
						"type": "integer",
						"description": "Cache TTL in seconds (default: 3600)"
					},
					"refresh": {
						"type": "boolean",
						"description": "Force refresh cached response"
					},
					"min_length": {
						"type": "integer",
						"description": "Minimum response length"
					},
					"must_contain": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Required text in response"
					},
					"no_error_check": {
						"type": "boolean",
						"description": "Skip error pattern checking"
					},
					"output": {
						"type": "string",
						"description": "Output file path"
					},
					"strip": {
						"type": "boolean",
						"description": "Strip whitespace from file variable values"
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

		// 40. Generate status report
		{
			Name:        ToolPrefix + "report",
			Description: "Generate a formatted markdown status report with title, statistics, and status.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"title": {
						"type": "string",
						"description": "Report title (required)"
					},
					"status": {
						"type": "string",
						"enum": ["success", "partial", "failed"],
						"description": "Report status (required)"
					},
					"stats": {
						"type": "object",
						"description": "Statistics as key-value pairs"
					},
					"output": {
						"type": "string",
						"description": "Output file path"
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
				"required": ["title", "status"]
			}`),
		},

		// 41. Directory statistics
		{
			Name:        ToolPrefix + "stats",
			Description: "Display statistics about a directory including file counts, total size, and breakdown by file extension.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Directory path to analyze (default: current directory)"
					},
					"no_gitignore": {
						"type": "boolean",
						"description": "Disable .gitignore filtering"
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

		// 42. TOML query
		{
			Name:        ToolPrefix + "toml_query",
			Description: "Query TOML file with dot-notation path.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "TOML file to query"
					},
					"path": {
						"type": "string",
						"description": "Dot-notation query path"
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
				"required": ["file", "path"]
			}`),
		},

		// 43. TOML validate
		{
			Name:        ToolPrefix + "toml_validate",
			Description: "Validate TOML file syntax.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "TOML file to validate"
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

		// 44. TOML parse
		{
			Name:        ToolPrefix + "toml_parse",
			Description: "Parse and display TOML file contents.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "TOML file to parse"
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

		// 45. Transform case
		{
			Name:        ToolPrefix + "transform_case",
			Description: "Transform text case. Formats: camelCase, PascalCase, snake_case, kebab-case, UPPERCASE, lowercase, Title Case.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"text": {
						"type": "string",
						"description": "Text to transform"
					},
					"to": {
						"type": "string",
						"enum": ["camelCase", "PascalCase", "snake_case", "kebab-case", "UPPERCASE", "lowercase", "Title Case"],
						"description": "Target case format"
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
				"required": ["text", "to"]
			}`),
		},

		// 46. Transform CSV to JSON
		{
			Name:        ToolPrefix + "transform_csv_to_json",
			Description: "Convert CSV file to JSON.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "CSV file to convert"
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

		// 47. Transform JSON to CSV
		{
			Name:        ToolPrefix + "transform_json_to_csv",
			Description: "Convert JSON array file to CSV.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "JSON file to convert"
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

		// 48. Transform filter lines
		{
			Name:        ToolPrefix + "transform_filter",
			Description: "Filter lines in a file by regex pattern.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "File to filter"
					},
					"pattern": {
						"type": "string",
						"description": "Regex pattern to match"
					},
					"invert": {
						"type": "boolean",
						"description": "Invert match (exclude matching lines)"
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
				"required": ["file", "pattern"]
			}`),
		},

		// 49. Transform sort lines
		{
			Name:        ToolPrefix + "transform_sort",
			Description: "Sort lines in a file.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "File to sort"
					},
					"reverse": {
						"type": "boolean",
						"description": "Sort in reverse order"
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

		// 50. Validate files
		{
			Name:        ToolPrefix + "validate",
			Description: "Validate files of various formats: JSON, TOML, YAML, CSV, Markdown.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"files": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Files to validate"
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
				"required": ["files"]
			}`),
		},

		// 51. Calculate runtime/elapsed time
		{
			Name:        ToolPrefix + "runtime",
			Description: "Calculate and format elapsed time between epoch timestamps. Returns formatted duration with configurable format (secs, mins, mins-secs, hms, human, compact) and precision.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"start": {
						"type": "integer",
						"description": "Start epoch timestamp (required)"
					},
					"end": {
						"type": "integer",
						"description": "End epoch timestamp (default: now)"
					},
					"format": {
						"type": "string",
						"enum": ["secs", "mins", "mins-secs", "hms", "human", "compact"],
						"description": "Output format (default: human)"
					},
					"precision": {
						"type": "integer",
						"description": "Decimal precision for output (default: 1)"
					},
					"label": {
						"type": "boolean",
						"description": "Include 'Runtime: ' prefix"
					},
					"raw": {
						"type": "boolean",
						"description": "Output raw number without unit suffix"
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
				"required": ["start"]
			}`),
		},

		// 52. Complete - Direct OpenAI API completion
		{
			Name:        ToolPrefix + "complete",
			Description: "Send prompt directly to OpenAI-compatible API. Uses OPENAI_API_KEY, OPENAI_BASE_URL, OPENAI_MODEL environment variables. Great for direct LLM calls without external CLI tools.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"prompt": {
						"type": "string",
						"description": "Direct prompt text"
					},
					"file": {
						"type": "string",
						"description": "Read prompt from file"
					},
					"template": {
						"type": "string",
						"description": "Template file with [[var]] placeholders"
					},
					"vars": {
						"type": "object",
						"description": "Template variables as key-value pairs"
					},
					"system": {
						"type": "string",
						"description": "System instruction"
					},
					"model": {
						"type": "string",
						"description": "Model to use (overrides OPENAI_MODEL)"
					},
					"temperature": {
						"type": "number",
						"description": "Temperature 0.0-2.0 (default: 0.7)"
					},
					"max_tokens": {
						"type": "integer",
						"description": "Maximum tokens in response (0 = no limit)"
					},
					"timeout": {
						"type": "integer",
						"description": "Request timeout in seconds (default: 120)"
					},
					"retries": {
						"type": "integer",
						"description": "Number of retries on failure (default: 3)"
					},
					"output": {
						"type": "string",
						"description": "Output file path"
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

		// 48. Route technical debt issues
		{
			Name:        ToolPrefix + "route_td",
			Description: "Route parsed technical debt issues to appropriate destinations based on EST_MINUTES thresholds. Routes to quick_wins (<30min), backlog (30-2879min), or td_files (>=2880min). Returns arrays for each destination plus routing_summary with counts. Ensures zero data loss.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Input JSON file path"
					},
					"content": {
						"type": "string",
						"description": "Direct JSON content input"
					},
					"quick_wins_max": {
						"type": "integer",
						"description": "Max minutes for quick_wins routing (default: 30)"
					},
					"backlog_max": {
						"type": "integer",
						"description": "Min minutes for td_files routing (default: 2880)"
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

		// 49. Parse structured data streams
		{
			Name:        ToolPrefix + "parse_stream",
			Description: "Parse structured data streams (pipe-delimited, markdown checklists) into JSON. Eliminates LLM parsing inconsistency and context compaction data loss. Returns format, headers, rows, row_count, and parse_errors.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Input file path"
					},
					"content": {
						"type": "string",
						"description": "Direct content input (alternative to file)"
					},
					"format": {
						"type": "string",
						"enum": ["auto", "pipe", "markdown-checklist"],
						"description": "Format: auto (detect), pipe (delimited), markdown-checklist"
					},
					"delimiter": {
						"type": "string",
						"description": "Delimiter for pipe format (default: |)"
					},
					"headers": {
						"type": "string",
						"description": "Comma-separated header names (overrides auto-detection)"
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

		// 50. Coverage report for requirements
		{
			Name:        ToolPrefix + "coverage_report",
			Description: "Calculate requirement coverage from user stories. Parses requirements file to extract IDs (REQ-#, R-#, REQUIREMENT-#), scans user story markdown files, and returns total, covered, uncovered requirements with coverage percentage and per-story mapping.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"requirements": {
						"type": "string",
						"description": "Path to requirements markdown file"
					},
					"stories": {
						"type": "string",
						"description": "Path to user stories directory"
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
				"required": ["requirements", "stories"]
			}`),
		},

		// 51. Validate risks coverage
		{
			Name:        ToolPrefix + "validate_risks",
			Description: "Cross-reference sprint-design.md risks with user stories, tasks, or acceptance criteria. Parses the Risk Analysis section and checks if each risk (R-1, R-2, etc.) is addressed in work items. Returns risks identified, addressed, unaddressed, coverage percentage, and per-risk details.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"design": {
						"type": "string",
						"description": "Path to sprint-design.md file"
					},
					"stories": {
						"type": "string",
						"description": "Path to user stories directory"
					},
					"tasks": {
						"type": "string",
						"description": "Path to tasks directory"
					},
					"acceptance_criteria": {
						"type": "string",
						"description": "Path to acceptance criteria directory"
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
				"required": ["design"]
			}`),
		},

		// 52. Alignment check for requirements
		{
			Name:        ToolPrefix + "alignment_check",
			Description: "Verify requirements alignment with delivered work. Compares requirements file against user stories, calculating alignment score with met/partial/unmet counts, gaps array, and scope creep detection.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"requirements": {
						"type": "string",
						"description": "Path to requirements markdown file"
					},
					"stories": {
						"type": "string",
						"description": "Path to user stories directory"
					},
					"tasks": {
						"type": "string",
						"description": "Path to tasks directory (optional, scanned for additional traceability)"
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
				"required": ["requirements", "stories"]
			}`),
		},

		// 53. Sprint status determination
		{
			Name:        ToolPrefix + "sprint_status",
			Description: "Determine sprint completion status (COMPLETED/PARTIAL/FAILED) from completion data. Evaluates tasks completed, tests passed, coverage percentage, and critical issues count. Default thresholds: 90% for COMPLETED, 50% for PARTIAL, 60% minimum coverage.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"tasks_total": {
						"type": "integer",
						"description": "Total number of tasks"
					},
					"tasks_completed": {
						"type": "integer",
						"description": "Number of completed tasks"
					},
					"tests_passed": {
						"type": "boolean",
						"description": "Whether tests passed"
					},
					"coverage": {
						"type": "number",
						"description": "Coverage percentage"
					},
					"critical_issues": {
						"type": "integer",
						"description": "Number of critical issues"
					},
					"completed_threshold": {
						"type": "number",
						"description": "Completion threshold for COMPLETED status (0.0-1.0, default: 0.90)"
					},
					"partial_threshold": {
						"type": "number",
						"description": "Completion threshold for PARTIAL status (0.0-1.0, default: 0.50)"
					},
					"coverage_threshold": {
						"type": "number",
						"description": "Minimum coverage to avoid FAILED (default: 60)"
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
				"required": ["tests_passed"]
			}`),
		},

		// 54. TDD compliance analysis
		{
			Name:        ToolPrefix + "tdd_compliance",
			Description: "Analyze git history for TDD compliance. Classifies commits as test-first, test-with, test-after, or no-test patterns. Calculates compliance score (0-100) with letter grade (A-F). Returns violations with remediation suggestions.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {
						"type": "string",
						"description": "Path to git repository"
					},
					"content": {
						"type": "string",
						"description": "Git log content (pipe-delimited: hash|author|date|message|files)"
					},
					"since": {
						"type": "string",
						"description": "Analyze commits since date (YYYY-MM-DD)"
					},
					"until": {
						"type": "string",
						"description": "Analyze commits until date (YYYY-MM-DD)"
					},
					"count": {
						"type": "integer",
						"description": "Maximum number of commits to analyze"
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

		// 55. Categorize git changes
		{
			Name:        ToolPrefix + "categorize_changes",
			Description: "Categorize git status output by file type. Parses git status --porcelain format and groups files into categories: source, test, config, docs, generated, other. Detects sensitive files (.env, credentials, keys) that should not be committed.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"file": {
						"type": "string",
						"description": "Path to file containing git status output"
					},
					"content": {
						"type": "string",
						"description": "Git status porcelain content directly"
					},
					"sensitive_patterns": {
						"type": "string",
						"description": "Additional sensitive file patterns (comma-separated globs)"
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
	}
}
