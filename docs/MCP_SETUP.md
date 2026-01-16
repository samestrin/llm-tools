# LLM Tools MCP Servers - Setup Guide

## Overview

This repository includes four MCP (Model Context Protocol) servers that make llm-tools commands available as native tools in Claude Desktop and other MCP-compatible clients:

1. **llm-support-mcp** - 50+ tools for file operations, search, LLM integration, and project analysis
2. **llm-clarification-mcp** - 12 tools for the Clarification Learning System
3. **llm-filesystem-mcp** - 15 batch/specialized tools for filesystem operations (single-file operations use Claude's native tools)
4. **llm-semantic-mcp** - 4 tools for semantic code search with embeddings

All servers are native Go binaries with no runtime dependencies.

## What You Get

### llm-support-mcp (50+ tools)

**File & Directory Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_tree` | Directory structure visualization |
| `llm_support_grep` | Pattern search in files |
| `llm_support_multigrep` | Search multiple keywords in parallel |
| `llm_support_multiexists` | Check multiple file/directory existence |
| `llm_support_catfiles` | Concatenate files with headers |
| `llm_support_stats` | Directory statistics by file type |
| `llm_support_diff` | Compare two files |

**Data & Transform Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_json_query` | Query JSON files with dot notation |
| `llm_support_yaml_get` | Query YAML files with dot notation |
| `llm_support_yaml_set` | Set values in YAML files |
| `llm_support_toml_parse` | Parse TOML files |
| `llm_support_markdown_headers` | Extract markdown headers |
| `llm_support_template` | Variable substitution in templates |
| `llm_support_transform_case` | Transform text case (camelCase, snake_case, etc.) |
| `llm_support_encode` / `llm_support_decode` | Base64, hex, URL encoding |

**Project Analysis Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_detect` | Detect project type and technology stack |
| `llm_support_discover_tests` | Discover test infrastructure and patterns |
| `llm_support_deps` | Extract dependencies from package manifests |
| `llm_support_analyze_deps` | Analyze file dependencies from markdown |
| `llm_support_validate_plan` | Validate plan directory structure |
| `llm_support_plan_type` | Extract plan type from metadata |

**Git & Context Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_git_context` | Gather git information for LLM context |
| `llm_support_git_changes` | Count and list working tree changes |
| `llm_support_repo_root` | Find git repository root path |

**LLM Integration Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_complete` | Send prompts to OpenAI-compatible API |
| `llm_support_prompt` | Execute prompts with templates and retry |
| `llm_support_foreach` | Process multiple files through LLM |
| `llm_support_extract_relevant` | Extract relevant content using LLM API |
| `llm_support_extract_links` | Extract and rank links from URLs |

**Utility Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_count` | Count checkboxes, lines, or files |
| `llm_support_summarize_dir` | Summarize directory contents for LLM context |
| `llm_support_highest` | Find highest numbered directory/file |
| `llm_support_partition_work` | Partition work items for parallel execution |
| `llm_support_init_temp` | Initialize temp directory with variables |
| `llm_support_clean_temp` | Clean up temp directories |
| `llm_support_runtime` | Calculate elapsed time between timestamps |
| `llm_support_context` | Persistent key-value storage |
| `llm_support_hash` | Generate file checksums |
| `llm_support_math` | Evaluate mathematical expressions |
| `llm_support_extract` | Extract patterns (URLs, emails, TODOs) |
| `llm_support_validate` | Validate JSON, YAML, TOML, CSV files |
| `llm_support_report` | Generate formatted status reports |

### llm-clarification-mcp (12 tools)

**Analysis Tools (require API):**
| Tool | Description |
|------|-------------|
| `llm_clarification_match_clarification` | Match question against existing entries |
| `llm_clarification_cluster_clarifications` | Group similar questions into clusters |
| `llm_clarification_detect_conflicts` | Find conflicting answers |
| `llm_clarification_validate_clarifications` | Check for stale entries |

**Management Tools (no API needed):**
| Tool | Description |
|------|-------------|
| `llm_clarification_init_tracking` | Initialize tracking file |
| `llm_clarification_add_clarification` | Add or update a clarification entry |
| `llm_clarification_delete_clarification` | Delete a clarification entry |
| `llm_clarification_promote_clarification` | Promote entry to CLAUDE.md |
| `llm_clarification_list_entries` | List entries with filtering |
| `llm_clarification_import_memory` | Import clarifications from YAML |
| `llm_clarification_export_memory` | Export clarifications to YAML |
| `llm_clarification_optimize_memory` | Optimize storage (vacuum, prune) |
| `llm_clarification_reconcile_memory` | Find stale file references |

### llm-filesystem-mcp (15 batch/specialized tools)

Single-file operations (read, write, edit) should use Claude's native Read, Write, and Edit tools for better performance. The MCP server exposes batch and specialized operations only.

**Batch Reading:**
| Tool | Description |
|------|-------------|
| `llm_filesystem_read_multiple_files` | Read multiple files simultaneously |
| `llm_filesystem_extract_lines` | Extract specific line ranges |

**Batch Editing:**
| Tool | Description |
|------|-------------|
| `llm_filesystem_edit_blocks` | Apply multiple edits to a file |
| `llm_filesystem_search_and_replace` | Regex replacement across files |

**Directory Operations:**
| Tool | Description |
|------|-------------|
| `llm_filesystem_list_directory` | List directory with filtering/pagination |
| `llm_filesystem_get_directory_tree` | Get directory tree structure |
| `llm_filesystem_create_directories` | Create multiple directories |

**Search Operations:**
| Tool | Description |
|------|-------------|
| `llm_filesystem_search_files` | Search files by name pattern |
| `llm_filesystem_search_code` | Search patterns in file contents |

**File Management:**
| Tool | Description |
|------|-------------|
| `llm_filesystem_copy_file` | Copy file or directory |
| `llm_filesystem_move_file` | Move or rename file/directory |
| `llm_filesystem_delete_file` | Delete file or directory |
| `llm_filesystem_batch_file_operations` | Batch copy/move/delete operations |

**Archive Operations:**
| Tool | Description |
|------|-------------|
| `llm_filesystem_compress_files` | Compress files into archive |
| `llm_filesystem_extract_archive` | Extract an archive |

### llm-semantic-mcp (4 tools)

| Tool | Description |
|------|-------------|
| `llm_semantic_search` | Search code using natural language queries |
| `llm_semantic_index` | Build/rebuild the semantic index |
| `llm_semantic_index_status` | Check index status (files, chunks, last update) |
| `llm_semantic_index_update` | Incrementally update index with changed files |

**Note:** Requires an OpenAI-compatible embedding API (Ollama, vLLM, OpenAI). Default model: `nomic-embed-text`.

## Prerequisites

1. **Go CLI binaries installed** - `llm-support`, `llm-clarification`, `llm-filesystem`, `llm-semantic` in your PATH
2. **Claude Desktop** or Claude Code installed
3. **Go 1.21+** (only needed if building from source)
4. **Ollama** (optional, for llm-semantic embeddings)

## Installation

### Step 1: Install Go CLI Binaries

```bash
# Option A: From source
git clone https://github.com/samestrin/llm-tools.git
cd llm-tools
make build
sudo cp build/llm-support build/llm-clarification build/llm-filesystem build/llm-semantic /usr/local/bin/

# Option B: Using go install
go install github.com/samestrin/llm-tools/cmd/llm-support@latest
go install github.com/samestrin/llm-tools/cmd/llm-clarification@latest
go install github.com/samestrin/llm-tools/cmd/llm-filesystem@latest
go install github.com/samestrin/llm-tools/cmd/llm-semantic@latest
```

Verify installation:
```bash
llm-support --version
llm-clarification --version
llm-filesystem --version
llm-semantic --version
```

### Step 2: Build MCP Servers

Build the native Go MCP server binaries:

```bash
cd llm-tools

# Build all MCP server binaries
go build -o llm-support-mcp ./cmd/llm-support-mcp/
go build -o llm-clarification-mcp ./cmd/llm-clarification-mcp/
go build -o llm-filesystem-mcp ./cmd/llm-filesystem-mcp/
go build -o llm-semantic-mcp ./cmd/llm-semantic-mcp/

# Install to a location in PATH
sudo cp llm-support-mcp llm-clarification-mcp llm-filesystem-mcp llm-semantic-mcp /usr/local/bin/
```

Verify installation:
```bash
# Test MCP server responds to initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | llm-support-mcp
```

### Step 3: Configure Claude Desktop

Edit your Claude Desktop config file:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`
**Linux:** `~/.config/Claude/claude_desktop_config.json`

Add the MCP server configurations:

```json
{
  "mcpServers": {
    "llm-support": {
      "command": "/usr/local/bin/llm-support-mcp"
    },
    "llm-clarification": {
      "command": "/usr/local/bin/llm-clarification-mcp"
    },
    "llm-filesystem": {
      "command": "/usr/local/bin/llm-filesystem-mcp"
    },
    "llm-semantic": {
      "command": "/usr/local/bin/llm-semantic-mcp"
    }
  }
}
```

### Step 4: Configure API for Clarification Tools (Optional)

The `llm_clarification_match_clarification`, `llm_clarification_cluster_clarifications`, `llm_clarification_detect_conflicts`, and `llm_clarification_validate_clarifications` tools require an OpenAI-compatible API.

**Environment Variables:**
```bash
export OPENAI_API_KEY=your-api-key
export OPENAI_BASE_URL=https://openrouter.ai/api/v1  # optional
export OPENAI_MODEL=gpt-4o-mini                       # optional
```

### Step 5: Configure Embeddings for Semantic Tools (Optional)

The `llm_semantic_*` tools require an OpenAI-compatible embedding API. By default, they use local Ollama.

**Option A: Local Ollama (recommended)**
```bash
# Install Ollama and pull the embedding model
ollama pull nomic-embed-text
```

**Option B: Remote embedding server**
```bash
export LLM_SEMANTIC_API_KEY=your-api-key           # if required
export LLM_SEMANTIC_API_URL=http://localhost:11434 # embedding server URL
export LLM_SEMANTIC_MODEL=nomic-embed-text         # model name
```

### Step 6: Restart Claude Desktop

Completely quit and restart Claude Desktop for the changes to take effect.

## Verify Installation

1. Start a new conversation in Claude Desktop
2. Type: "What tools do you have available?"
3. Claude should list:
   - 50+ `llm_support_*` tools
   - 12 `llm_clarification_*` tools
   - 15 `llm_filesystem_*` tools (batch/specialized operations)
   - 4 `llm_semantic_*` tools

## Usage Examples

### llm-support Tools

**Check if files exist:**
```
Can you check if these files exist: plan.md, metadata.md, user-stories/
```
Claude will use `llm_support_multiexists`.

**Search for multiple keywords:**
```
Find all definitions of handleSubmit, validateForm, and useAuth in the src/ directory
```
Claude will use `llm_support_multigrep`.

**Detect project stack:**
```
What technology stack is this project using?
```
Claude will use `llm_support_detect`.

**Count completed tasks:**
```
How many tasks are completed in the sprint plan?
```
Claude will use `llm_support_count`.

**Show directory structure:**
```
Show me the structure of the src/ directory
```
Claude will use `llm_support_tree`.

### llm-clarification Tools

**Initialize tracking:**
```
Initialize a clarification tracking file for this project
```
Claude will use `llm_clarification_init_tracking`.

**Record a clarification:**
```
Record this clarification: Q: "Should we use Tailwind or CSS modules?" A: "Use Tailwind for this project"
```
Claude will use `llm_clarification_add_clarification`.

**List clarifications:**
```
Show me all clarifications that have been asked more than once
```
Claude will use `llm_clarification_list_entries` with `min_occurrences: 2`.

**Find duplicate questions:**
```
Are there any clarifications in the tracking file that might be asking the same thing?
```
Claude will use `llm_clarification_cluster_clarifications`.

### llm-filesystem Tools

**Read multiple files:**
```
Read the package.json and tsconfig.json files
```
Claude will use `llm_filesystem_read_multiple_files`.

**Batch edit a file:**
```
Replace all "console.log" with "logger.info" and "console.error" with "logger.error" in src/utils.ts
```
Claude will use `llm_filesystem_edit_blocks`.

**Search for code patterns:**
```
Find all files that use the deprecated API
```
Claude will use `llm_filesystem_search_code`.

**Get directory tree:**
```
Show me the structure of the src/components directory
```
Claude will use `llm_filesystem_get_directory_tree`.

### llm-semantic Tools

**Index the codebase:**
```
Build a semantic index for this Go project
```
Claude will use `llm_semantic_index`.

**Search code semantically:**
```
Find code that handles user authentication
```
Claude will use `llm_semantic_search`.

**Check index status:**
```
What's the status of the semantic index?
```
Claude will use `llm_semantic_index_status`.

## Tool Reference

### llm_support_tree

Display directory structure as a tree.

**Parameters:**
- `path` (string): Directory path (default: current directory)
- `depth` (integer): Maximum depth to display
- `sizes` (boolean): Show file sizes
- `no_gitignore` (boolean): Include gitignored files

### llm_support_grep

Search for regex pattern in files.

**Parameters:**
- `pattern` (string, required): Regular expression pattern
- `paths` (array, required): Files or directories to search
- `ignore_case` (boolean): Case-insensitive search
- `line_numbers` (boolean): Show line numbers
- `files_only` (boolean): Only show filenames

### llm_support_multigrep

Search for multiple keywords in parallel.

**Parameters:**
- `keywords` (string, required): Comma-separated keywords
- `path` (string): Path to search
- `extensions` (string): File extensions (e.g., `ts,tsx,js`)
- `max_per_keyword` (integer): Max matches per keyword (default: 10)
- `ignore_case` (boolean): Case-insensitive search
- `definitions_only` (boolean): Only definition matches
- `json` (boolean): Output as JSON

### llm_support_detect

Detect project type and technology stack.

**Parameters:**
- `path` (string): Project path
- `json` (boolean): Output as JSON

**Returns:** STACK, LANGUAGE, PACKAGE_MANAGER, FRAMEWORK, HAS_TESTS

### llm_support_count

Count checkboxes, lines, or files.

**Parameters:**
- `mode` (string, required): `checkboxes`, `lines`, or `files`
- `target` (string, required): File or directory
- `recursive` (boolean): Search recursively
- `pattern` (string): Glob pattern (for files mode)

**Returns for checkboxes:** TOTAL, CHECKED, UNCHECKED, PERCENT

### llm_support_summarize_dir

Summarize directory contents for LLM context.

**Parameters:**
- `path` (string, required): Directory to summarize
- `format` (string): `outline`, `headers`, `frontmatter`, or `first-lines`
- `recursive` (boolean): Search recursively
- `glob` (string): Glob pattern for files
- `max_tokens` (integer): Maximum tokens in output

### llm_clarify_add

Add or update a clarification entry.

**Parameters:**
- `tracking_file` (string, required): Path to tracking file
- `question` (string, required): The clarification question
- `answer` (string): The answer/decision
- `id` (string): Entry ID (auto-generated if not provided)
- `sprint_id` (string): Sprint ID
- `context_tags` (string): Comma-separated tags

### llm_clarify_list

List entries with optional filtering.

**Parameters:**
- `tracking_file` (string, required): Path to tracking file
- `status` (string): Filter by status (`pending`, `promoted`, `expired`, `rejected`)
- `min_occurrences` (integer): Minimum occurrences to show
- `json_output` (boolean): Output as JSON

## Troubleshooting

### MCP server not showing up

1. **Check config path:**
   - Verify the path to the MCP binary in claude_desktop_config.json
   - Use `which llm-support-mcp` to get the full path

2. **Check binaries exist:**
   ```bash
   ls -la /usr/local/bin/llm-support-mcp
   ls -la /usr/local/bin/llm-clarification-mcp
   ls -la /usr/local/bin/llm-filesystem-mcp
   ls -la /usr/local/bin/llm-semantic-mcp
   ```

3. **Check logs:**
   - macOS: `~/Library/Logs/Claude/`
   - Look for MCP-related errors

### Tools not working

1. **Check Go binaries are installed:**
   ```bash
   which llm-support llm-clarification llm-filesystem llm-semantic
   llm-support --version
   ```

2. **Test binaries directly:**
   ```bash
   llm-support tree --path .
   llm-filesystem read-file --path README.md
   llm-semantic index-status
   ```

3. **Test MCP server directly:**
   ```bash
   echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | llm-support-mcp
   ```

### Clarification API tools failing

1. **Check API configuration:**
   ```bash
   echo $OPENAI_API_KEY
   ```

2. **Test API connectivity:**
   ```bash
   llm-clarification match-clarification -q "test" --entries-json "[]"
   ```

### Semantic search not working

1. **Check Ollama is running:**
   ```bash
   ollama list
   ```

2. **Pull the embedding model:**
   ```bash
   ollama pull nomic-embed-text
   ```

3. **Test indexing:**
   ```bash
   llm-semantic index . --include "*.go"
   ```

## Performance

- **Cold start:** ~7ms (Go binary startup)
- **Tool registration:** <1ms
- **Request handling:** <10ms overhead
- **multigrep:** 10+ keywords in <500ms

## Security

- MCP servers run locally on your machine
- No network access required (except clarification API tools)
- Same security model as running binaries directly
- All file operations use same permissions as your user

## Uninstalling

1. Remove MCP server configs from claude_desktop_config.json
2. Restart Claude Desktop
3. Optionally remove binaries:
   ```bash
   sudo rm /usr/local/bin/llm-support-mcp /usr/local/bin/llm-clarification-mcp \
           /usr/local/bin/llm-filesystem-mcp /usr/local/bin/llm-semantic-mcp
   ```

## See Also

- [README.md](../README.md) - Main documentation
- [quick-reference.md](quick-reference.md) - Command cheat sheet
- [llm-support-commands.md](llm-support-commands.md) - llm-support command reference
- [llm-filesystem-commands.md](llm-filesystem-commands.md) - llm-filesystem command reference
- [llm-semantic-commands.md](llm-semantic-commands.md) - llm-semantic command reference
