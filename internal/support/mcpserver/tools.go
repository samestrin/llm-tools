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
			Description: "Display directory structure as a tree. Respects .gitignore. Use max_entries to limit output.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
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
								"type": "boolean"
							},
							"exclude": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Regex patterns to exclude (e.g., [\"test\", \"fixtures\"])"
							},
							"no_gitignore": {
								"type": "boolean",
								"description": "Disable .gitignore filtering"
							},
							"no_default_excludes": {
								"type": "boolean",
								"description": "Disable default excludes (node_modules, vendor, target, etc.)"
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
								"type": "string"
							},
							"paths": {
								"type": "array",
								"items": {
									"type": "string"
								}
							},
							"ignore_case": {
								"type": "boolean"
							},
							"line_numbers": {
								"type": "boolean"
							},
							"files_only": {
								"type": "boolean"
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
								"items": {
									"type": "string"
								}
							},
							"verbose": {
								"type": "boolean"
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
								"type": "string"
							},
							"query": {
								"type": "string",
								"description": "Query (e.g., .users[0].name)"
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
								"type": "string"
							},
							"level": {
								"type": "string",
								"description": "Level filter (e.g., 2,3)"
							},
							"plain": {
								"type": "boolean",
								"description": "Plain text output"
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
								"type": "string"
							},
							"vars": {
								"type": "object"
							},
							"syntax": {
								"type": "string",
								"enum": ["brackets", "braces"],
								"description": "brackets or braces"
							}
						},
						"required": ["file"]
					}`),
		},

		// 7. Discover test infrastructure
		{
			Name:        ToolPrefix + "discover_tests",
			Description: "Discover test patterns, runners, and infrastructure in a project.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							}
						}
					}`),
		},

		// 8. Multi-keyword search
		{
			Name:        ToolPrefix + "multigrep",
			Description: "Search multiple keywords in parallel. Prioritizes definitions over usages.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"keywords": {
								"type": "string"
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
								"type": "boolean"
							},
							"definitions_only": {
								"type": "boolean",
								"description": "Only show definition matches"
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
			Description: "Analyze file dependencies from a markdown task file.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							}
						},
						"required": ["file"]
					}`),
		},

		// 10. Project detection
		{
			Name:        ToolPrefix + "detect",
			Description: "Detect project type and technology stack.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "Project path to analyze"
							},
							"dirs": {
								"type": "string",
								"description": "Subdirs for per-component detection (e.g., backend,frontend)"
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
								"type": "boolean"
							},
							"pattern": {
								"type": "string",
								"description": "Glob pattern (for files mode)"
							}
						},
						"required": ["mode", "path"]
					}`),
		},

		// 12. Summarize directory
		{
			Name:        ToolPrefix + "summarize_dir",
			Description: "Summarize directory contents for LLM context.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							},
							"format": {
								"type": "string",
								"enum": ["outline", "headers", "frontmatter", "first-lines"]
							},
							"recursive": {
								"type": "boolean"
							},
							"glob": {
								"type": "string",
								"description": "Glob pattern for files"
							},
							"max_tokens": {
								"type": "integer",
								"description": "Maximum tokens in output"
							}
						},
						"required": ["path"]
					}`),
		},

		// 13. Extract dependencies from package manifests
		{
			Name:        ToolPrefix + "deps",
			Description: "Extract dependencies from package manifests (package.json, go.mod, Cargo.toml, etc.).",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"manifest": {
								"type": "string"
							},
							"type": {
								"type": "string",
								"enum": ["all", "prod", "dev"],
								"description": "Dependency type to extract (default: all)"
							}
						},
						"required": ["manifest"]
					}`),
		},

		// 14. Git context gathering
		{
			Name:        ToolPrefix + "git_context",
			Description: "Gather git information for LLM context.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
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
							}
						}
					}`),
		},

		// 15. Validate plan structure
		{
			Name:        ToolPrefix + "validate_plan",
			Description: "Validate plan directory structure.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							}
						},
						"required": ["path"]
					}`),
		},

		// 16. Partition work for parallel execution
		{
			Name:        ToolPrefix + "partition_work",
			Description: "Partition work items into parallel execution groups using graph coloring.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"stories": {
								"type": "string"
							},
							"tasks": {
								"type": "string"
							},
							"verbose": {
								"type": "boolean"
							}
						}
					}`),
		},

		// 17. Repository root detection
		{
			Name:        ToolPrefix + "repo_root",
			Description: "Find git repository root for a path.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							},
							"validate": {
								"type": "boolean"
							}
						}
					}`),
		},

		// 18. Extract relevant content using LLM
		{
			Name:        ToolPrefix + "extract_relevant",
			Description: "Extract relevant content from files/URLs using LLM. Filters to query-relevant parts.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "File, directory, or URL"
							},
							"context": {
								"type": "string",
								"description": "Relevance criteria"
							},
							"concurrency": {
								"type": "integer",
								"description": "Concurrent API calls (default: 2)"
							},
							"output": {
								"type": "string"
							},
							"timeout": {
								"type": "integer",
								"description": "Timeout in seconds (default: 60)"
							}
						},
						"required": ["context"]
					}`),
		},

		// 19. Extract links from URL
		{
			Name:        ToolPrefix + "extract_links",
			Description: "Extract and rank links from a URL. Heuristic mode (default) or LLM-ranked (with context).",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"url": {
								"type": "string",
								"description": "URL"
							},
							"context": {
								"type": "string",
								"description": "Context for LLM-based ranking (enables LLM mode)"
							},
							"timeout": {
								"type": "integer",
								"description": "HTTP timeout in seconds (default: 30)"
							}
						},
						"required": ["url"]
					}`),
		},

		// 20. Find highest numbered directory/file
		{
			Name:        ToolPrefix + "highest",
			Description: "Find highest numbered directory or file. Returns HIGHEST, NAME, FULL_PATH, NEXT, COUNT.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "Directory to search in (default: current directory)"
							},
							"paths": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Multiple dirs to search"
							},
							"pattern": {
								"type": "string",
								"description": "Custom regex for version extraction"
							},
							"type": {
								"type": "string",
								"enum": ["dir", "file", "both"],
								"description": "Type to search: dir, file, both (default: both)"
							},
							"prefix": {
								"type": "string",
								"description": "Prefix filter (e.g., 01-)"
							}
						}
					}`),
		},

		// 20. Plan type extraction
		{
			Name:        ToolPrefix + "plan_type",
			Description: "Extract plan type from planning metadata.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "Plan directory path (default: current directory)"
							}
						}
					}`),
		},

		// 21. Git changes detection
		{
			Name:        ToolPrefix + "git_changes",
			Description: "Count and list git working tree changes with optional path filtering.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string",
								"description": "Path prefix filter"
							},
							"include_untracked": {
								"type": "boolean",
								"description": "Include untracked files (default: true)"
							},
							"staged_only": {
								"type": "boolean",
								"description": "Only show staged changes"
							}
						}
					}`),
		},

		// 22. Context multiset - batch set multiple key-value pairs
		{
			Name:        ToolPrefix + "context_multiset",
			Description: "Set multiple context key-value pairs atomically.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"dir": {
								"type": "string",
								"description": "Dir with context.env"
							},
							"pairs": {
								"type": "object"
							}
						},
						"required": ["dir", "pairs"]
					}`),
		},

		// 23. Context multiget - batch retrieve multiple values
		{
			Name:        ToolPrefix + "context_multiget",
			Description: "Retrieve multiple context values in a single call.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"dir": {
								"type": "string",
								"description": "Dir with context.env"
							},
							"keys": {
								"type": "array",
								"items": {
									"type": "string"
								}
							},
							"defaults": {
								"type": "object",
								"description": "Defaults for missing keys"
							}
						},
						"required": ["dir", "keys"]
					}`),
		},

		// 24. Context variable management
		{
			Name:        ToolPrefix + "context",
			Description: "Manage persistent key-value context storage. Supports init/set/get/list/dump/clear.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"operation": {
								"type": "string",
								"enum": ["init", "set", "get", "list", "dump", "clear"],
								"description": "init/set/get/list/dump/clear"
							},
							"dir": {
								"type": "string",
								"description": "Dir with context.env"
							},
							"key": {
								"type": "string",
								"description": "Key name"
							},
							"value": {
								"type": "string",
								"description": "Value"
							},
							"default": {
								"type": "string",
								"description": "Default if missing"
							}
						},
						"required": ["operation", "dir"]
					}`),
		},

		// 25. YAML get - retrieve value by dot-notation key
		{
			Name:        ToolPrefix + "yaml_get",
			Description: "Retrieve a value from YAML config by dot-notation key.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"key": {
								"type": "string",
								"description": "Dot-notation key"
							},
							"default": {
								"type": "string",
								"description": "Default if missing"
							}
						},
						"required": ["file", "key"]
					}`),
		},

		// 26. YAML set - store value at dot-notation key
		{
			Name:        ToolPrefix + "yaml_set",
			Description: "Store a value in YAML config at dot-notation key.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"key": {
								"type": "string",
								"description": "Dot-notation key"
							},
							"value": {
								"type": "string"
							},
							"create": {
								"type": "boolean",
								"description": "Create file if it doesn't exist"
							},
							"dry_run": {
								"type": "boolean",
								"description": "Preview only"
							},
							"quiet": {
								"type": "boolean",
								"description": "Suppress success messages"
							}
						},
						"required": ["file", "key", "value"]
					}`),
		},

		// 27. YAML multiget - retrieve multiple values
		{
			Name:        ToolPrefix + "yaml_multiget",
			Description: "Retrieve multiple YAML values in a single call.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"keys": {
								"type": "array",
								"items": {
									"type": "string"
								},
								"description": "Keys (dot-notation)"
							},
							"defaults": {
								"type": "object",
								"description": "Defaults for keys"
							},
							"required_file": {
								"type": "string",
								"description": "File with required keys (one per line)"
							}
						},
						"required": ["file"]
					}`),
		},

		// 28. YAML multiset - set multiple key-value pairs
		{
			Name:        ToolPrefix + "yaml_multiset",
			Description: "Set multiple YAML key-value pairs atomically.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"pairs": {
								"type": "object",
								"description": "Key-value pairs"
							},
							"create": {
								"type": "boolean",
								"description": "Create file if it doesn't exist"
							},
							"dry_run": {
								"type": "boolean",
								"description": "Preview only"
							},
							"quiet": {
								"type": "boolean",
								"description": "Suppress success messages"
							}
						},
						"required": ["file", "pairs"]
					}`),
		},

		// 29. Parse arguments into structured format
		{
			Name:        ToolPrefix + "args",
			Description: "Parse command-line arguments into structured format.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"arguments": {
								"type": "array",
								"items": {
									"type": "string"
								}
							}
						},
						"required": ["arguments"]
					}`),
		},

		// 30. Concatenate files with headers
		{
			Name:        ToolPrefix + "catfiles",
			Description: "Concatenate multiple files with path headers. Respects .gitignore.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"paths": {
								"type": "array",
								"items": {
									"type": "string"
								}
							},
							"max_size": {
								"type": "integer",
								"description": "Max size in MB (default: 10)"
							},
							"no_gitignore": {
								"type": "boolean",
								"description": "Disable .gitignore filtering"
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
								"type": "string"
							},
							"encoding": {
								"type": "string",
								"enum": ["base64", "base32", "hex", "url"],
								"description": "Encoding type (default: base64)"
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
								"type": "string"
							},
							"file2": {
								"type": "string"
							},
							"unified": {
								"type": "boolean",
								"description": "Use unified diff format"
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
								"type": "string"
							},
							"encoding": {
								"type": "string",
								"enum": ["base64", "base32", "hex", "url"],
								"description": "Encoding type (default: base64)"
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
								"type": "string"
							},
							"count": {
								"type": "boolean"
							},
							"unique": {
								"type": "boolean",
								"description": "Remove duplicates"
							}
						},
						"required": ["type", "file"]
					}`),
		},

		// 35. Batch process files with LLM
		{
			Name:        ToolPrefix + "foreach",
			Description: "Process multiple files through an LLM using a template with [[var]] placeholders.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"files": {
								"type": "array",
								"items": {
									"type": "string"
								}
							},
							"glob": {
								"type": "string",
								"description": "Glob pattern (alternative to files)"
							},
							"template": {
								"type": "string",
								"description": "Template file with [[var]] vars"
							},
							"llm": {
								"type": "string",
								"description": "LLM binary (default: gemini)"
							},
							"output_dir": {
								"type": "string",
								"description": "Output directory for processed files"
							},
							"output_pattern": {
								"type": "string",
								"description": "Output pattern (e.g., {{name}}-out.md)"
							},
							"parallel": {
								"type": "integer",
								"description": "Parallel count (default: 1)"
							},
							"skip_existing": {
								"type": "boolean",
								"description": "Skip existing"
							},
							"timeout": {
								"type": "integer",
								"description": "Timeout per file (default: 120s)"
							},
							"vars": {
								"type": "object",
								"description": "Template vars"
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
								"items": {
									"type": "string"
								}
							},
							"algorithm": {
								"type": "string",
								"enum": ["md5", "sha1", "sha256", "sha512"],
								"description": "Hash algorithm (default: sha256)"
							}
						},
						"required": ["paths"]
					}`),
		},

		// 37. Initialize temp directory
		{
			Name:        ToolPrefix + "init_temp",
			Description: "Initialize temp directory at .planning/.temp/{name}/.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"name": {
								"type": "string"
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
								"description": "Include git info"
							},
							"skip_context": {
								"type": "boolean",
								"description": "Don't create context.env file"
							}
						},
						"required": ["name"]
					}`),
		},

		// 38. Clean temp directory
		{
			Name:        ToolPrefix + "clean_temp",
			Description: "Clean up temp directories created by init_temp.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"name": {
								"type": "string"
							},
							"all": {
								"type": "boolean",
								"description": "Remove all temp directories"
							},
							"older_than": {
								"type": "string",
								"description": "Remove dirs older than (e.g., 7d, 24h)"
							},
							"dry_run": {
								"type": "boolean",
								"description": "Preview only"
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
								"type": "string"
							}
						},
						"required": ["expression"]
					}`),
		},

		// 39. Generate status report
		{
			Name:        ToolPrefix + "report",
			Description: "Generate a formatted markdown status report with title, statistics, and status.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"title": {
								"type": "string"
							},
							"status": {
								"type": "string",
								"enum": ["success", "partial", "failed"]
							},
							"stats": {
								"type": "object",
								"description": "Statistics as key-value pairs"
							},
							"output": {
								"type": "string"
							}
						},
						"required": ["title", "status"]
					}`),
		},

		// 41. Directory statistics
		{
			Name:        ToolPrefix + "stats",
			Description: "Display directory statistics (file counts, size, extension breakdown).",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							},
							"no_gitignore": {
								"type": "boolean",
								"description": "Disable .gitignore filtering"
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
								"type": "string"
							},
							"path": {
								"type": "string",
								"description": "Dot-notation path"
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
								"type": "string"
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
								"type": "string"
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
								"type": "string"
							},
							"to": {
								"type": "string",
								"enum": ["camelCase", "PascalCase", "snake_case", "kebab-case", "UPPERCASE", "lowercase", "Title Case"],
								"description": "Target case format"
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
								"type": "string"
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
								"type": "string"
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
								"type": "string"
							},
							"pattern": {
								"type": "string",
								"description": "Regex pattern to match"
							},
							"invert": {
								"type": "boolean",
								"description": "Invert match (exclude matching lines)"
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
								"type": "string"
							},
							"reverse": {
								"type": "boolean",
								"description": "Sort in reverse order"
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
								"items": {
									"type": "string"
								}
							}
						},
						"required": ["files"]
					}`),
		},

		// 51. Calculate runtime/elapsed time
		{
			Name:        ToolPrefix + "runtime",
			Description: "Calculate elapsed time between epoch timestamps.",
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
							}
						},
						"required": ["start"]
					}`),
		},

		// 52. Complete - Direct OpenAI API completion
		{
			Name:        ToolPrefix + "complete",
			Description: "Send prompt to OpenAI-compatible API. Uses OPENAI_API_KEY/BASE_URL/MODEL env vars.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"prompt": {
								"type": "string"
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
								"description": "Template vars"
							},
							"system": {
								"type": "string"
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
								"type": "string"
							}
						}
					}`),
		},

		// 48. Route technical debt issues
		{
			Name:        ToolPrefix + "route_td",
			Description: "Route technical debt items by time estimate (quick_wins/backlog/td_files).",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"content": {
								"type": "string"
							},
							"quick_wins_max": {
								"type": "integer",
								"description": "Quick wins max min (default: 30)"
							},
							"backlog_max": {
								"type": "integer",
								"description": "Backlog max min (default: 2880)"
							}
						}
					}`),
		},

		// 49. Parse structured data streams
		{
			Name:        ToolPrefix + "parse_stream",
			Description: "Parse structured data streams (pipe-delimited, markdown checklists) into JSON.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"content": {
								"type": "string"
							},
							"format": {
								"type": "string",
								"enum": ["auto", "pipe", "markdown-checklist"],
								"description": "auto/pipe/markdown-checklist"
							},
							"delimiter": {
								"type": "string",
								"description": "Delimiter for pipe format (default: |)"
							},
							"headers": {
								"type": "string",
								"description": "Header names (comma-separated)"
							}
						}
					}`),
		},

		// 50. Coverage report for requirements
		{
			Name:        ToolPrefix + "coverage_report",
			Description: "Calculate requirement coverage from user stories.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"requirements": {
								"type": "string"
							},
							"stories": {
								"type": "string"
							}
						},
						"required": ["requirements", "stories"]
					}`),
		},

		// 51. Validate risks coverage
		{
			Name:        ToolPrefix + "validate_risks",
			Description: "Cross-reference sprint risks with stories/tasks/ACs for coverage.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"design": {
								"type": "string"
							},
							"stories": {
								"type": "string"
							},
							"tasks": {
								"type": "string"
							},
							"acceptance_criteria": {
								"type": "string"
							}
						},
						"required": ["design"]
					}`),
		},

		// 52. Alignment check for requirements
		{
			Name:        ToolPrefix + "alignment_check",
			Description: "Verify requirements alignment with delivered stories.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"requirements": {
								"type": "string"
							},
							"stories": {
								"type": "string"
							},
							"tasks": {
								"type": "string",
								"description": "Path to tasks directory (optional, scanned for additional traceability)"
							}
						},
						"required": ["requirements", "stories"]
					}`),
		},

		// 53. Sprint status determination
		{
			Name:        ToolPrefix + "sprint_status",
			Description: "Determine sprint completion status (COMPLETED/PARTIAL/FAILED).",
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
								"type": "boolean"
							},
							"coverage": {
								"type": "number"
							},
							"critical_issues": {
								"type": "integer",
								"description": "Number of critical issues"
							},
							"completed_threshold": {
								"type": "number",
								"description": "Threshold for COMPLETED (default: 0.90)"
							},
							"partial_threshold": {
								"type": "number",
								"description": "Threshold for PARTIAL (default: 0.50)"
							},
							"coverage_threshold": {
								"type": "number",
								"description": "Min coverage (default: 60)"
							}
						},
						"required": ["tests_passed"]
					}`),
		},

		// 54. TDD compliance analysis
		{
			Name:        ToolPrefix + "tdd_compliance",
			Description: "Analyze git history for TDD compliance.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							},
							"content": {
								"type": "string",
								"description": "Git log (pipe-delimited)"
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
							}
						}
					}`),
		},

		// 55. Categorize git changes
		{
			Name:        ToolPrefix + "categorize_changes",
			Description: "Categorize git status output by file type (source, test, config, docs, generated, other). Detects sensitive files.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"content": {
								"type": "string"
							},
							"sensitive_patterns": {
								"type": "string",
								"description": "Extra sensitive patterns (comma-separated)"
							}
						}
					}`),
		},

		// 56. Format TD items as markdown tables
		{
			Name:        ToolPrefix + "format_td_table",
			Description: "Format technical debt items as markdown tables.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"content": {
								"type": "string"
							},
							"section": {
								"type": "string",
								"enum": ["quick_wins", "backlog", "td_files", "all"],
								"description": "Section: quick_wins/backlog/td_files/all"
							},
							"checkbox": {
								"type": "boolean",
								"description": "Add checkbox column"
							}
						}
					}`),
		},

		// 57. Group TD items by path/category
		{
			Name:        ToolPrefix + "group_td",
			Description: "Group technical debt items by path, category, or file.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							},
							"content": {
								"type": "string"
							},
							"group_by": {
								"type": "string",
								"enum": ["path", "category", "file"],
								"description": "Strategy: path, category, or file"
							},
							"path_depth": {
								"type": "integer",
								"description": "Number of path segments for theme (default: 2)"
							},
							"min_group_size": {
								"type": "integer",
								"description": "Min items per group (default: 3)"
							},
							"critical_override": {
								"type": "boolean",
								"description": "CRITICAL gets own group (default: true)"
							},
							"root_theme": {
								"type": "string",
								"description": "Root theme (default: misc)"
							},
							"assign_numbers": {
								"type": "boolean",
								"description": "Assign group numbers"
							},
							"output_file": {
								"type": "string",
								"description": "Output markdown file (appends)"
							},
							"checkbox": {
								"type": "boolean",
								"description": "Add checkbox column"
							},
							"sprint_label": {
								"type": "string",
								"description": "Sprint name for section header in output file"
							},
							"date_label": {
								"type": "string",
								"description": "Date for section header in output file"
							},
							"format": {
								"type": "string",
								"enum": ["json", "pipe"],
								"description": "Input format: json (default) or pipe (pipe-delimited text with # comments)"
							},
							"headers": {
								"type": "string",
								"description": "Column headers for pipe format"
							},
							"delimiter": {
								"type": "string",
								"description": "Field delimiter for pipe format (default: |)"
							}
						}
					}`),
		},
		// 58. Tech debt statistics
		{
			Name:        ToolPrefix + "td_stats",
			Description: "Generate tech debt statistics from a markdown README with checkbox/severity table.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"path": {
								"type": "string"
							}
						},
						"required": ["path"]
					}`),
		},

		// 59. Project components (monorepo support)
		{
			Name:        ToolPrefix + "project_components",
			Description: "Read project config and return normalized component list.",
			InputSchema: json.RawMessage(`{
						"type": "object",
						"properties": {
							"file": {
								"type": "string"
							}
						},
						"required": ["file"]
					}`),
		},
	}
}
