#!/usr/bin/env python3
"""
MCP Server for llm-support tool (V2 Edition)

Provides llm-support commands as MCP tools for Claude Desktop.
All tools use the `llm_support_` prefix for clear identification.

Base Tools (from llm-support.py v1.x):
- llm_support_tree: Directory structure visualization
- llm_support_grep: Pattern search in files
- llm_support_multiexists: Check multiple file/directory existence
- llm_support_json_query: Query JSON files with dot notation
- llm_support_markdown_headers: Extract markdown headers
- llm_support_template: Variable substitution in templates

V2.x Tools (advanced capabilities):
- llm_support_discover_tests: Discover test infrastructure
- llm_support_multigrep: Search for multiple keywords in parallel
- llm_support_analyze_deps: Analyze file dependencies from markdown
- llm_support_detect: Detect project type and technology stack
- llm_support_count: Count checkboxes, lines, or files
- llm_support_summarize_dir: Summarize directory contents for LLM context

V2.4 Tools (context & analysis):
- llm_support_deps: Extract dependencies from package manifests
- llm_support_git_context: Gather git information for LLM context
- llm_support_validate_plan: Validate plan directory structure
- llm_support_partition_work: Partition work items for parallel execution
- llm_support_repo_root: Find git repository root for path anchoring

NOTE: Clarification Learning System tools have been moved to llm-clarification-mcp.py

Version: 2.9.0 (Go Edition - uses llm-support-go binary)
"""

import asyncio
import subprocess
import sys
from pathlib import Path

# MCP SDK
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import Tool, TextContent


# Path to the llm-support binary (Go version)
# Falls back to Python script if Go binary not found
BINARY_PATH = "/usr/local/bin/llm-support"
SCRIPT_PATH = Path(__file__).parent / "llm-support.py"  # Fallback

# Tool name prefix
PREFIX = "llm_support_"


class LLMSupportMCPServer:
    """MCP Server wrapping llm-support.py commands"""

    def __init__(self):
        self.server = Server("llm-support")
        self._setup_handlers()

    def _setup_handlers(self):
        """Setup MCP request handlers"""

        @self.server.list_tools()
        async def list_tools() -> list[Tool]:
            """List all available tools"""
            return [
                # 1. Directory tree structure
                Tool(
                    name=f"{PREFIX}tree",
                    description="Display directory structure as a tree. Respects .gitignore by default.",
                    inputSchema={
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
                            }
                        }
                    }
                ),

                # 2. Pattern search
                Tool(
                    name=f"{PREFIX}grep",
                    description="Search for regex pattern in files. Supports case-insensitive search and line numbers.",
                    inputSchema={
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
                            }
                        },
                        "required": ["pattern", "paths"]
                    }
                ),

                # 3. Multi-file existence check
                Tool(
                    name=f"{PREFIX}multiexists",
                    description="Check if multiple files or directories exist. Returns detailed status for each path.",
                    inputSchema={
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
                            }
                        },
                        "required": ["paths"]
                    }
                ),

                # 4. JSON query
                Tool(
                    name=f"{PREFIX}json_query",
                    description="Query JSON file with dot notation (e.g., '.users[0].name').",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "file": {
                                "type": "string",
                                "description": "JSON file to query"
                            },
                            "query": {
                                "type": "string",
                                "description": "Query path (e.g., .users[0].name)"
                            }
                        },
                        "required": ["file", "query"]
                    }
                ),

                # 5. Markdown headers extraction
                Tool(
                    name=f"{PREFIX}markdown_headers",
                    description="Extract headers from markdown file. Can filter by header level.",
                    inputSchema={
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
                            }
                        },
                        "required": ["file"]
                    }
                ),

                # 6. Template substitution
                Tool(
                    name=f"{PREFIX}template",
                    description="Variable substitution in template files. Supports [[var]] or {{var}} syntax.",
                    inputSchema={
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
                            }
                        },
                        "required": ["file"]
                    }
                ),

                # =========================================================
                # V2.x TOOLS
                # =========================================================

                # 7. Discover test infrastructure
                Tool(
                    name=f"{PREFIX}discover_tests",
                    description="Discover test patterns, runners, and infrastructure in a project. Returns PATTERN, FRAMEWORK, TEST_RUNNER, CONFIG_FILE, SOURCE_DIR, TEST_DIR, E2E_DIR, and test counts.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Project path to analyze (default: current directory)"
                            },
                            "json": {
                                "type": "boolean",
                                "description": "Output as JSON"
                            }
                        }
                    }
                ),

                # 8. Multi-keyword search
                Tool(
                    name=f"{PREFIX}multigrep",
                    description="Search for multiple keywords in parallel with intelligent output. Prioritizes definitions over usages. Outputs keyword files with DEF: and USE: prefixes.",
                    inputSchema={
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
                            "output_dir": {
                                "type": "string",
                                "description": "Write per-keyword results to directory"
                            }
                        },
                        "required": ["keywords"]
                    }
                ),

                # 9. File dependency analysis
                Tool(
                    name=f"{PREFIX}analyze_deps",
                    description="Analyze file dependencies from a user story or task markdown file. Returns FILES_READ, FILES_MODIFY, FILES_CREATE, DIRECTORIES.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "file": {
                                "type": "string",
                                "description": "Markdown file to analyze"
                            },
                            "json": {
                                "type": "boolean",
                                "description": "Output as JSON"
                            }
                        },
                        "required": ["file"]
                    }
                ),

                # 10. Project detection
                Tool(
                    name=f"{PREFIX}detect",
                    description="Detect project type and technology stack. Returns STACK, LANGUAGE, PACKAGE_MANAGER, FRAMEWORK, HAS_TESTS, PYTEST_AVAILABLE.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Project path to analyze"
                            },
                            "json": {
                                "type": "boolean",
                                "description": "Output as JSON"
                            }
                        }
                    }
                ),

                # 11. Count items
                Tool(
                    name=f"{PREFIX}count",
                    description="Count checkboxes, lines, or files. For checkboxes: returns TOTAL, CHECKED, UNCHECKED, COMPLETION%.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "mode": {
                                "type": "string",
                                "enum": ["checkboxes", "lines", "files"],
                                "description": "What to count"
                            },
                            "target": {
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
                            }
                        },
                        "required": ["mode", "target"]
                    }
                ),

                # 12. Summarize directory
                Tool(
                    name=f"{PREFIX}summarize_dir",
                    description="Summarize directory contents for LLM context. Formats: outline (default), headers, frontmatter, first-lines.",
                    inputSchema={
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
                            }
                        },
                        "required": ["path"]
                    }
                ),

                # =========================================================
                # V2.4 TOOLS - Context & Analysis
                # =========================================================

                # 13. Extract dependencies from package manifests
                Tool(
                    name=f"{PREFIX}deps",
                    description="Extract dependencies from package manifest files. Supports package.json, requirements.txt, pyproject.toml, go.mod, Cargo.toml, Gemfile. Returns TYPE, PRODUCTION_COUNT, DEV_COUNT, TOTAL_COUNT, and dependency lists.",
                    inputSchema={
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
                            }
                        },
                        "required": ["manifest"]
                    }
                ),

                # 14. Git context gathering
                Tool(
                    name=f"{PREFIX}git_context",
                    description="Gather git information for LLM context. Returns BRANCH, HAS_UNCOMMITTED, COMMIT_COUNT, and RECENT_COMMITS.",
                    inputSchema={
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
                            }
                        }
                    }
                ),

                # 15. Validate plan structure
                Tool(
                    name=f"{PREFIX}validate_plan",
                    description="Validate plan directory structure. Returns VALID status, counts of user stories, acceptance criteria, documentation, and tasks. Lists any warnings or errors.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Path to plan directory"
                            },
                            "json": {
                                "type": "boolean",
                                "description": "Output as JSON"
                            }
                        },
                        "required": ["path"]
                    }
                ),

                # 16. Partition work for parallel execution
                Tool(
                    name=f"{PREFIX}partition_work",
                    description="Partition work items into parallel execution groups using graph coloring. Ensures items in the same group don't touch the same files. Returns TOTAL_ITEMS, GROUP_COUNT, CONFLICTS_FOUND, RECOMMENDATION, and group details.",
                    inputSchema={
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
                            }
                        }
                    }
                ),

                # 17. Repository root detection
                Tool(
                    name=f"{PREFIX}repo_root",
                    description="Find git repository root for a path. Returns ROOT (absolute path) and optionally VALID (TRUE/FALSE). Use for anchoring all paths in LLM prompts.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "path": {
                                "type": "string",
                                "description": "Starting path (default: current directory)"
                            },
                            "validate": {
                                "type": "boolean",
                                "description": "Also verify .git directory exists"
                            }
                        }
                    }
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: dict) -> list[TextContent]:
            """Execute a tool by calling llm-support.py"""
            try:
                # Strip prefix and get the actual command name
                if name.startswith(PREFIX):
                    cmd_name = name[len(PREFIX):]
                else:
                    cmd_name = name

                # Build command - use Go binary if available, else Python fallback
                if Path(BINARY_PATH).exists():
                    cmd = [BINARY_PATH]
                else:
                    cmd = [sys.executable, str(SCRIPT_PATH)]

                # Map to CLI command structure
                if cmd_name == "tree":
                    cmd.append("tree")
                    cmd.extend(self._build_tree_args(arguments))
                elif cmd_name == "grep":
                    cmd.append("grep")
                    cmd.extend(self._build_grep_args(arguments))
                elif cmd_name == "multiexists":
                    cmd.append("multiexists")
                    cmd.extend(self._build_multiexists_args(arguments))
                elif cmd_name == "json_query":
                    cmd.extend(["json", "query"])
                    cmd.extend(self._build_json_query_args(arguments))
                elif cmd_name == "markdown_headers":
                    cmd.extend(["markdown", "headers"])
                    cmd.extend(self._build_markdown_headers_args(arguments))
                elif cmd_name == "template":
                    cmd.append("template")
                    cmd.extend(self._build_template_args(arguments))
                # V2.x commands
                elif cmd_name == "discover_tests":
                    cmd.append("discover-tests")
                    cmd.extend(self._build_discover_tests_args(arguments))
                elif cmd_name == "multigrep":
                    cmd.append("multigrep")
                    cmd.extend(self._build_multigrep_args(arguments))
                elif cmd_name == "analyze_deps":
                    cmd.append("analyze-deps")
                    cmd.extend(self._build_analyze_deps_args(arguments))
                elif cmd_name == "detect":
                    cmd.append("detect")
                    cmd.extend(self._build_detect_args(arguments))
                elif cmd_name == "count":
                    cmd.append("count")
                    cmd.extend(self._build_count_args(arguments))
                elif cmd_name == "summarize_dir":
                    cmd.append("summarize-dir")
                    cmd.extend(self._build_summarize_dir_args(arguments))
                # V2.4 commands
                elif cmd_name == "deps":
                    cmd.append("deps")
                    cmd.extend(self._build_deps_args(arguments))
                elif cmd_name == "git_context":
                    cmd.append("git-context")
                    cmd.extend(self._build_git_context_args(arguments))
                elif cmd_name == "validate_plan":
                    cmd.append("validate-plan")
                    cmd.extend(self._build_validate_plan_args(arguments))
                elif cmd_name == "partition_work":
                    cmd.append("partition-work")
                    cmd.extend(self._build_partition_work_args(arguments))
                elif cmd_name == "repo_root":
                    cmd.append("repo-root")
                    cmd.extend(self._build_repo_root_args(arguments))
                else:
                    return [TextContent(type="text", text=f"ERROR: Unknown tool '{name}'")]

                # Execute command
                result = subprocess.run(
                    cmd,
                    capture_output=True,
                    text=True,
                    timeout=60
                )

                # Return output
                output = result.stdout
                if result.stderr:
                    output += f"\n\nSTDERR:\n{result.stderr}"

                return [TextContent(type="text", text=output)]

            except subprocess.TimeoutExpired:
                return [TextContent(type="text", text="ERROR: Command timed out after 60 seconds")]
            except Exception as e:
                return [TextContent(type="text", text=f"ERROR: {str(e)}")]

    def _build_tree_args(self, args: dict) -> list[str]:
        """Build args for tree command"""
        cmd_args = []
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if "depth" in args:
            cmd_args.extend(["--depth", str(args["depth"])])
        if args.get("sizes"):
            cmd_args.append("--sizes")
        if args.get("no_gitignore"):
            cmd_args.append("--no-gitignore")
        return cmd_args

    def _build_grep_args(self, args: dict) -> list[str]:
        """Build args for grep command"""
        cmd_args = []
        if "pattern" in args:
            cmd_args.append(args["pattern"])
        if "paths" in args:
            cmd_args.extend(args["paths"])
        if args.get("ignore_case"):
            cmd_args.append("-i")
        if args.get("line_numbers"):
            cmd_args.append("-n")
        if args.get("files_only"):
            cmd_args.append("-l")
        return cmd_args

    def _build_multiexists_args(self, args: dict) -> list[str]:
        """Build args for multiexists command"""
        cmd_args = []
        if "paths" in args:
            cmd_args.extend(args["paths"])
        if args.get("verbose"):
            cmd_args.append("--verbose")
        cmd_args.append("--no-fail")  # Don't exit with error
        return cmd_args

    def _build_json_query_args(self, args: dict) -> list[str]:
        """Build args for json query command"""
        cmd_args = []
        if "file" in args:
            cmd_args.append(args["file"])
        if "query" in args:
            cmd_args.append(args["query"])
        return cmd_args

    def _build_markdown_headers_args(self, args: dict) -> list[str]:
        """Build args for markdown headers command"""
        cmd_args = []
        if "file" in args:
            cmd_args.append(args["file"])
        if "level" in args:
            cmd_args.extend(["--level", args["level"]])
        if args.get("plain"):
            cmd_args.append("--plain")
        return cmd_args

    def _build_template_args(self, args: dict) -> list[str]:
        """Build args for template command"""
        cmd_args = []
        if "file" in args:
            cmd_args.append(args["file"])
        if "vars" in args:
            for key, value in args["vars"].items():
                cmd_args.extend(["--var", f"{key}={value}"])
        if "syntax" in args:
            cmd_args.extend(["--syntax", args["syntax"]])
        return cmd_args

    # =========================================================================
    # V2.x argument builders
    # =========================================================================

    def _build_discover_tests_args(self, args: dict) -> list[str]:
        """Build args for discover-tests command"""
        cmd_args = []
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_multigrep_args(self, args: dict) -> list[str]:
        """Build args for multigrep command"""
        cmd_args = []
        if "keywords" in args:
            cmd_args.extend(["--keywords", args["keywords"]])
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if "extensions" in args:
            cmd_args.extend(["--extensions", args["extensions"]])
        if "max_per_keyword" in args:
            cmd_args.extend(["--max-per-keyword", str(args["max_per_keyword"])])
        if args.get("ignore_case"):
            cmd_args.append("--ignore-case")
        if args.get("definitions_only"):
            cmd_args.append("--definitions-only")
        if args.get("json"):
            cmd_args.append("--json")
        if "output_dir" in args:
            cmd_args.extend(["--output-dir", args["output_dir"]])
        return cmd_args

    def _build_analyze_deps_args(self, args: dict) -> list[str]:
        """Build args for analyze-deps command"""
        cmd_args = []
        if "file" in args:
            cmd_args.append(args["file"])
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_detect_args(self, args: dict) -> list[str]:
        """Build args for detect command"""
        cmd_args = []
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_count_args(self, args: dict) -> list[str]:
        """Build args for count command"""
        cmd_args = []
        if "mode" in args:
            cmd_args.extend(["--mode", args["mode"]])
        if "target" in args:
            cmd_args.append(args["target"])
        if args.get("recursive"):
            cmd_args.append("--recursive")
        if "pattern" in args:
            cmd_args.extend(["--pattern", args["pattern"]])
        return cmd_args

    def _build_summarize_dir_args(self, args: dict) -> list[str]:
        """Build args for summarize-dir command"""
        cmd_args = []
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if "format" in args:
            cmd_args.extend(["--format", args["format"]])
        if args.get("recursive"):
            cmd_args.append("--recursive")
        if "glob" in args:
            cmd_args.extend(["--glob", args["glob"]])
        if "max_tokens" in args:
            cmd_args.extend(["--max-tokens", str(args["max_tokens"])])
        return cmd_args

    # =========================================================================
    # V2.4 argument builders
    # =========================================================================

    def _build_deps_args(self, args: dict) -> list[str]:
        """Build args for deps command"""
        cmd_args = []
        if "manifest" in args:
            cmd_args.append(args["manifest"])
        if "type" in args:
            cmd_args.extend(["--type", args["type"]])
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_git_context_args(self, args: dict) -> list[str]:
        """Build args for git-context command"""
        cmd_args = []
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if args.get("include_diff"):
            cmd_args.append("--include-diff")
        if "since" in args:
            cmd_args.extend(["--since", args["since"]])
        if "max_commits" in args:
            cmd_args.extend(["--max-commits", str(args["max_commits"])])
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_validate_plan_args(self, args: dict) -> list[str]:
        """Build args for validate-plan command"""
        cmd_args = []
        # Support both "path" and legacy "plan_path" for backwards compatibility
        path = args.get("path") or args.get("plan_path")
        if path:
            cmd_args.extend(["--path", path])
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_partition_work_args(self, args: dict) -> list[str]:
        """Build args for partition-work command"""
        cmd_args = []
        if "stories" in args:
            cmd_args.extend(["--stories", args["stories"]])
        if "tasks" in args:
            cmd_args.extend(["--tasks", args["tasks"]])
        if args.get("verbose"):
            cmd_args.append("--verbose")
        if args.get("json"):
            cmd_args.append("--json")
        return cmd_args

    def _build_repo_root_args(self, args: dict) -> list[str]:
        """Build args for repo-root command"""
        cmd_args = []
        if "path" in args:
            cmd_args.extend(["--path", args["path"]])
        if args.get("validate"):
            cmd_args.append("--validate")
        return cmd_args

    async def run(self):
        """Run the MCP server"""
        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options()
            )


async def main():
    """Main entry point"""
    # Check if llm-support binary or script exists
    go_binary_exists = Path(BINARY_PATH).exists()
    py_script_exists = SCRIPT_PATH.exists()

    if not go_binary_exists and not py_script_exists:
        print(f"ERROR: llm-support not found", file=sys.stderr)
        print(f"  Go binary: {BINARY_PATH} (not found)", file=sys.stderr)
        print(f"  Python script: {SCRIPT_PATH} (not found)", file=sys.stderr)
        sys.exit(1)

    if go_binary_exists:
        print(f"Using Go binary: {BINARY_PATH}", file=sys.stderr)
    else:
        print(f"Using Python fallback: {SCRIPT_PATH}", file=sys.stderr)

    # Run the server
    server = LLMSupportMCPServer()
    await server.run()


if __name__ == "__main__":
    asyncio.run(main())
