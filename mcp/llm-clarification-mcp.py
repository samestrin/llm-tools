#!/usr/bin/env python3
"""
MCP Server for LLM Clarification Learning System

⚠️ DEPRECATED: This Python MCP server is deprecated.
Use the native Go implementation instead: llm-clarification-mcp

Build the Go version:
    go build -o llm-clarification-mcp ./cmd/llm-clarification-mcp/

See docs/MCP_SETUP.md for migration instructions.

Provides clarification learning tools as MCP tools for Claude Desktop.
All tools use the `llm_clarify_` prefix for clear identification.

Analysis Tools (require API):
- llm_clarify_match: Match a question against existing entries
- llm_clarify_cluster: Group similar questions into clusters
- llm_clarify_detect_conflicts: Find conflicting answers
- llm_clarify_validate: Check for stale entries

Management Tools (no API needed):
- llm_clarify_init: Initialize tracking file
- llm_clarify_add: Add or update a clarification entry
- llm_clarify_promote: Promote entry to CLAUDE.md
- llm_clarify_list: List entries with filtering

Version: 1.0.0
"""

import asyncio
import subprocess
import sys
from pathlib import Path

# MCP SDK
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import Tool, TextContent


# Path to the llm-clarification.py script
SCRIPT_PATH = Path(__file__).parent / "llm-clarification.py"

# Tool name prefix
PREFIX = "llm_clarify_"


class LLMClarificationMCPServer:
    """MCP Server wrapping llm-clarification.py commands"""

    def __init__(self):
        self.server = Server("llm-clarification")
        self._setup_handlers()

    def _setup_handlers(self):
        """Setup MCP request handlers"""

        @self.server.list_tools()
        async def list_tools() -> list[Tool]:
            """List all available tools"""
            return [
                # =============================================================
                # ANALYSIS TOOLS (Read-only, require API)
                # =============================================================

                # 1. Match clarification
                Tool(
                    name=f"{PREFIX}match",
                    description="Match a new question against existing clarification entries using LLM semantic matching. Returns match ID, confidence score (0-1), and reasoning. Use this to find if a question has been asked before. REQUIRES: OpenAI-compatible API configured via env vars or .planning/.config/openai_* files.",
                    inputSchema={
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
                    }
                ),

                # 2. Cluster clarifications
                Tool(
                    name=f"{PREFIX}cluster",
                    description="Group semantically similar questions into clusters. Useful for identifying duplicate or related clarifications across sprints. Returns clusters with labels and question lists. REQUIRES: OpenAI-compatible API configured.",
                    inputSchema={
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
                    }
                ),

                # 3. Detect conflicts
                Tool(
                    name=f"{PREFIX}detect_conflicts",
                    description="Find clarification entries with conflicting answers. Analyzes entries that may ask the same underlying question but have different answers. Returns conflicts with severity and resolution suggestions. REQUIRES: OpenAI-compatible API configured.",
                    inputSchema={
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
                    }
                ),

                # 4. Validate clarifications
                Tool(
                    name=f"{PREFIX}validate",
                    description="Validate clarifications against current project state. Flags entries that may be stale, outdated, or need review based on project context and last-seen dates. REQUIRES: OpenAI-compatible API configured.",
                    inputSchema={
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
                    }
                ),

                # =============================================================
                # MANAGEMENT TOOLS (Write operations, no API needed)
                # =============================================================

                # 5. Initialize tracking
                Tool(
                    name=f"{PREFIX}init",
                    description="Initialize a new clarification tracking file with proper schema. Creates the file at the specified path. Use before starting clarification tracking for a project.",
                    inputSchema={
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
                    }
                ),

                # 6. Add clarification
                Tool(
                    name=f"{PREFIX}add",
                    description="Add or update a clarification entry in the tracking file. If a matching entry exists (by ID or simple match), updates it with incremented occurrence count. Otherwise creates a new entry with auto-generated ID. Handles all YAML serialization internally.",
                    inputSchema={
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
                    }
                ),

                # 7. Promote clarification
                Tool(
                    name=f"{PREFIX}promote",
                    description="Promote a clarification entry to CLAUDE.md. Updates entry status to 'promoted' and appends the clarification to the target CLAUDE.md file under a 'Learned Clarifications' section, organized by category based on context_tags.",
                    inputSchema={
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
                    }
                ),

                # 8. List entries
                Tool(
                    name=f"{PREFIX}list",
                    description="List entries in the tracking file with optional filtering by status or minimum occurrence count. Useful for reviewing what clarifications exist and identifying promotion candidates.",
                    inputSchema={
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
                    }
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: dict) -> list[TextContent]:
            """Execute a tool by calling llm-clarification.py"""
            try:
                # Strip prefix and get the actual command name
                if name.startswith(PREFIX):
                    cmd_name = name[len(PREFIX):]
                else:
                    cmd_name = name

                # Build command
                cmd = [sys.executable, str(SCRIPT_PATH)]

                # Map to CLI command structure
                if cmd_name == "match":
                    cmd.append("match-clarification")
                    cmd.extend(self._build_match_args(arguments))
                elif cmd_name == "cluster":
                    cmd.append("cluster-clarifications")
                    cmd.extend(self._build_cluster_args(arguments))
                elif cmd_name == "detect_conflicts":
                    cmd.append("detect-conflicts")
                    cmd.extend(self._build_detect_conflicts_args(arguments))
                elif cmd_name == "validate":
                    cmd.append("validate-clarifications")
                    cmd.extend(self._build_validate_args(arguments))
                elif cmd_name == "init":
                    cmd.append("init-tracking")
                    cmd.extend(self._build_init_args(arguments))
                elif cmd_name == "add":
                    cmd.append("add-clarification")
                    cmd.extend(self._build_add_args(arguments))
                elif cmd_name == "promote":
                    cmd.append("promote-clarification")
                    cmd.extend(self._build_promote_args(arguments))
                elif cmd_name == "list":
                    cmd.append("list-entries")
                    cmd.extend(self._build_list_args(arguments))
                else:
                    return [TextContent(type="text", text=f"ERROR: Unknown tool '{name}'")]

                # Execute command
                result = subprocess.run(
                    cmd,
                    capture_output=True,
                    text=True,
                    timeout=120
                )

                # Return output
                output = result.stdout
                if result.stderr:
                    output += f"\n\nSTDERR:\n{result.stderr}"

                return [TextContent(type="text", text=output)]

            except subprocess.TimeoutExpired:
                return [TextContent(type="text", text="ERROR: Command timed out after 120 seconds")]
            except Exception as e:
                return [TextContent(type="text", text=f"ERROR: {str(e)}")]

    # =========================================================================
    # ANALYSIS COMMAND ARG BUILDERS
    # =========================================================================

    def _build_match_args(self, args: dict) -> list[str]:
        """Build args for match-clarification command"""
        cmd_args = []

        if "question" in args:
            cmd_args.extend(["--question", args["question"]])
        if "entries_file" in args:
            cmd_args.extend(["--entries-file", args["entries_file"]])
        if "entries_json" in args:
            cmd_args.extend(["--entries-json", args["entries_json"]])
        if "timeout" in args:
            cmd_args.extend(["--timeout", str(args["timeout"])])

        return cmd_args

    def _build_cluster_args(self, args: dict) -> list[str]:
        """Build args for cluster-clarifications command"""
        cmd_args = []

        if "questions_file" in args:
            cmd_args.extend(["--questions-file", args["questions_file"]])
        if "questions_json" in args:
            cmd_args.extend(["--questions-json", args["questions_json"]])
        if "timeout" in args:
            cmd_args.extend(["--timeout", str(args["timeout"])])

        return cmd_args

    def _build_detect_conflicts_args(self, args: dict) -> list[str]:
        """Build args for detect-conflicts command"""
        cmd_args = []

        if "tracking_file" in args:
            cmd_args.append(args["tracking_file"])
        if "timeout" in args:
            cmd_args.extend(["--timeout", str(args["timeout"])])

        return cmd_args

    def _build_validate_args(self, args: dict) -> list[str]:
        """Build args for validate-clarifications command"""
        cmd_args = []

        if "tracking_file" in args:
            cmd_args.append(args["tracking_file"])
        if "context" in args:
            cmd_args.extend(["--context", args["context"]])
        if "timeout" in args:
            cmd_args.extend(["--timeout", str(args["timeout"])])

        return cmd_args

    # =========================================================================
    # MANAGEMENT COMMAND ARG BUILDERS
    # =========================================================================

    def _build_init_args(self, args: dict) -> list[str]:
        """Build args for init-tracking command"""
        cmd_args = []

        if "output" in args:
            cmd_args.extend(["--output", args["output"]])
        if args.get("force"):
            cmd_args.append("--force")

        return cmd_args

    def _build_add_args(self, args: dict) -> list[str]:
        """Build args for add-clarification command"""
        cmd_args = []

        if "tracking_file" in args:
            cmd_args.extend(["--tracking-file", args["tracking_file"]])
        if "question" in args:
            cmd_args.extend(["--question", args["question"]])
        if "answer" in args:
            cmd_args.extend(["--answer", args["answer"]])
        if "id" in args:
            cmd_args.extend(["--id", args["id"]])
        if "sprint_id" in args:
            cmd_args.extend(["--sprint-id", args["sprint_id"]])
        if "context_tags" in args:
            cmd_args.extend(["--context-tags", args["context_tags"]])
        if args.get("check_match"):
            cmd_args.append("--check-match")

        return cmd_args

    def _build_promote_args(self, args: dict) -> list[str]:
        """Build args for promote-clarification command"""
        cmd_args = []

        if "tracking_file" in args:
            cmd_args.extend(["--tracking-file", args["tracking_file"]])
        if "id" in args:
            cmd_args.extend(["--id", args["id"]])
        if "target" in args:
            cmd_args.extend(["--target", args["target"]])
        if args.get("force"):
            cmd_args.append("--force")

        return cmd_args

    def _build_list_args(self, args: dict) -> list[str]:
        """Build args for list-entries command"""
        cmd_args = []

        if "tracking_file" in args:
            cmd_args.append(args["tracking_file"])
        if "status" in args:
            cmd_args.extend(["--status", args["status"]])
        if "min_occurrences" in args:
            cmd_args.extend(["--min-occurrences", str(args["min_occurrences"])])
        if args.get("json_output"):
            cmd_args.append("--json")

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
    # Check if llm-clarification.py exists
    if not SCRIPT_PATH.exists():
        print(f"ERROR: llm-clarification.py not found at {SCRIPT_PATH}", file=sys.stderr)
        print("Make sure llm-clarification.py is in the same directory as this script.", file=sys.stderr)
        sys.exit(1)

    # Run the server
    server = LLMClarificationMCPServer()
    await server.run()


if __name__ == "__main__":
    asyncio.run(main())
