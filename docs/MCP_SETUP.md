# LLM Support & Clarification MCP Servers - Setup Guide

## Overview

This repository includes two MCP (Model Context Protocol) servers that make llm-tools commands available as native tools in Claude Desktop:

1. **llm-support-mcp** - 18 tools for file operations, search, and project analysis
2. **llm-clarification-mcp** - 8 tools for the Clarification Learning System

**Available Implementations:**
- **Go (Recommended)** - Native binaries, no runtime dependencies
- **Python (Legacy)** - Requires Python 3.10+ and MCP SDK

## What You Get

### llm-support-mcp (18 tools)

**Base Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_tree` | Directory structure visualization |
| `llm_support_grep` | Pattern search in files |
| `llm_support_multiexists` | Check multiple file/directory existence |
| `llm_support_json_query` | Query JSON files with dot notation |
| `llm_support_markdown_headers` | Extract markdown headers |
| `llm_support_template` | Variable substitution in templates |

**Advanced Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_discover_tests` | Discover test infrastructure and patterns |
| `llm_support_multigrep` | Search multiple keywords in parallel |
| `llm_support_analyze_deps` | Analyze file dependencies from markdown |
| `llm_support_detect` | Detect project type and technology stack |
| `llm_support_count` | Count checkboxes, lines, or files |
| `llm_support_summarize_dir` | Summarize directory contents for LLM context |

**Context & Analysis Tools:**
| Tool | Description |
|------|-------------|
| `llm_support_deps` | Extract dependencies from package manifests |
| `llm_support_git_context` | Gather git information for LLM context |
| `llm_support_repo_root` | Find git repository root path |
| `llm_support_validate_plan` | Validate plan directory structure |
| `llm_support_partition_work` | Partition work items for parallel execution |
| `llm_support_extract_relevant` | Extract relevant content using LLM API |

### llm-clarification-mcp (8 tools)

**Analysis Tools (require API):**
| Tool | Description |
|------|-------------|
| `llm_clarify_match` | Match question against existing entries |
| `llm_clarify_cluster` | Group similar questions into clusters |
| `llm_clarify_detect_conflicts` | Find conflicting answers |
| `llm_clarify_validate` | Check for stale entries |

**Management Tools (no API needed):**
| Tool | Description |
|------|-------------|
| `llm_clarify_init` | Initialize tracking file |
| `llm_clarify_add` | Add or update a clarification entry |
| `llm_clarify_promote` | Promote entry to CLAUDE.md |
| `llm_clarify_list` | List entries with filtering |

## Prerequisites

1. **Go binaries installed** - `llm-support` and `llm-clarification` in your PATH
2. **Claude Desktop** installed
3. **Go 1.21+** (for Go MCP servers) OR **Python 3.10+ with MCP SDK** (for Python MCP servers)

## Installation

### Step 1: Install Go CLI Binaries

```bash
# Option A: From source
git clone https://github.com/samestrin/llm-tools.git
cd llm-tools
make build
sudo cp build/llm-support build/llm-clarification /usr/local/bin/

# Option B: Using go install
go install github.com/samestrin/llm-tools/cmd/llm-support@latest
go install github.com/samestrin/llm-tools/cmd/llm-clarification@latest
```

Verify installation:
```bash
llm-support --version
llm-clarification --version
```

### Step 2: Build MCP Servers

#### Option A: Go MCP Servers (Recommended)

Build the native Go MCP server binaries:

```bash
cd llm-tools

# Build MCP server binaries
go build -o llm-support-mcp ./cmd/llm-support-mcp/
go build -o llm-clarification-mcp ./cmd/llm-clarification-mcp/

# Install to a location in PATH
sudo cp llm-support-mcp llm-clarification-mcp /usr/local/bin/
```

Verify installation:
```bash
# Test MCP server responds to initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | llm-support-mcp
```

#### Option B: Python MCP Servers (Legacy)

Install MCP SDK:
```bash
pip install mcp
```

The Python MCP servers are in the `mcp/` directory:
```
mcp/
├── llm-support-mcp.py       # Python MCP server for llm-support
└── llm-clarification-mcp.py # Python MCP server for clarification
```

Make them executable:
```bash
chmod +x mcp/llm-support-mcp.py
chmod +x mcp/llm-clarification-mcp.py
```

### Step 3: Configure Claude Desktop

Edit your Claude Desktop config file:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`
**Linux:** `~/.config/Claude/claude_desktop_config.json`

#### For Go MCP Servers (Recommended)

If you installed the Go binaries to `/usr/local/bin/`:

```json
{
  "mcpServers": {
    "llm-support": {
      "command": "/usr/local/bin/llm-support-mcp"
    },
    "llm-clarification": {
      "command": "/usr/local/bin/llm-clarification-mcp"
    }
  }
}
```

#### For Python MCP Servers (Legacy)

```json
{
  "mcpServers": {
    "llm-support": {
      "command": "python3",
      "args": ["/absolute/path/to/llm-tools/mcp/llm-support-mcp.py"]
    },
    "llm-clarification": {
      "command": "python3",
      "args": ["/absolute/path/to/llm-tools/mcp/llm-clarification-mcp.py"]
    }
  }
}
```

**Important:** Use absolute paths for Python servers!

Example (macOS with Python):
```json
{
  "mcpServers": {
    "llm-support": {
      "command": "python3",
      "args": ["/Users/yourname/projects/llm-tools/mcp/llm-support-mcp.py"]
    },
    "llm-clarification": {
      "command": "python3",
      "args": ["/Users/yourname/projects/llm-tools/mcp/llm-clarification-mcp.py"]
    }
  }
}
```

### Step 4: Configure API for Clarification Tools (Optional)

The `llm_clarify_match`, `llm_clarify_cluster`, `llm_clarify_detect_conflicts`, and `llm_clarify_validate` tools require an OpenAI-compatible API.

**Option A: Environment Variables**
```bash
export OPENAI_API_KEY=your-api-key
export OPENAI_BASE_URL=https://openrouter.ai/api/v1  # optional
export OPENAI_MODEL=gpt-4o-mini                       # optional
```

**Option B: Config Files**
```bash
mkdir -p .planning/.config
echo 'your-api-key' > .planning/.config/openai_api_key
echo 'https://openrouter.ai/api/v1' > .planning/.config/openai_base_url
echo 'gpt-4o-mini' > .planning/.config/openai_model
```

### Step 5: Restart Claude Desktop

Completely quit and restart Claude Desktop for the changes to take effect.

## Verify Installation

1. Start a new conversation in Claude Desktop
2. Type: "What tools do you have available?"
3. Claude should list:
   - 18 `llm_support_*` tools
   - 8 `llm_clarify_*` tools

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
Claude will use `llm_clarify_init`.

**Record a clarification:**
```
Record this clarification: Q: "Should we use Tailwind or CSS modules?" A: "Use Tailwind for this project"
```
Claude will use `llm_clarify_add`.

**List clarifications:**
```
Show me all clarifications that have been asked more than once
```
Claude will use `llm_clarify_list` with `min_occurrences: 2`.

**Find duplicate questions:**
```
Are there any clarifications in the tracking file that might be asking the same thing?
```
Claude will use `llm_clarify_cluster`.

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
   - Verify absolute paths in claude_desktop_config.json
   - Use `pwd` in the directory to get the full path

2. **Check Python path:**
   - Try `python3` or `python` depending on your system
   - Verify with: `which python3`

3. **Check MCP installed:**
   ```bash
   python3 -c "import mcp; print('MCP installed')"
   ```

4. **Check logs:**
   - macOS: `~/Library/Logs/Claude/`
   - Look for MCP-related errors

### Tools not working

1. **Check Go binaries are installed:**
   ```bash
   which llm-support
   which llm-clarification
   llm-support --version
   ```

2. **Check scripts are executable:**
   ```bash
   chmod +x mcp/llm-support-mcp.py
   chmod +x mcp/llm-clarification-mcp.py
   ```

3. **Test binaries directly:**
   ```bash
   llm-support tree --path .
   llm-clarification --help
   ```

### Clarification API tools failing

1. **Check API configuration:**
   ```bash
   # Either set environment variables
   echo $OPENAI_API_KEY

   # Or use config files
   cat .planning/.config/openai_api_key
   ```

2. **Test API connectivity:**
   ```bash
   llm-clarification match-clarification -q "test" --entries-json "[]"
   ```

## Performance

- **Cold start:** ~10-20ms (Go binary startup)
- **Execution:** Same as running binaries directly
- **Overhead:** Minimal (~10-20ms) for MCP wrapper
- **multigrep:** 10+ keywords in <500ms

## Security

- MCP servers run locally on your machine
- No network access required (except clarification API tools)
- Same security model as running binaries directly
- All file operations use same permissions as your user

## Uninstalling

1. Remove MCP server configs from claude_desktop_config.json
2. Restart Claude Desktop
3. Optionally remove files:
   ```bash
   rm mcp/llm-support-mcp.py mcp/llm-clarification-mcp.py
   ```

## Migration from Python to Go

If you're currently using the Python MCP servers, migrate to Go for better performance:

1. Build Go binaries:
   ```bash
   go build -o llm-support-mcp ./cmd/llm-support-mcp/
   go build -o llm-clarification-mcp ./cmd/llm-clarification-mcp/
   sudo cp llm-support-mcp llm-clarification-mcp /usr/local/bin/
   ```

2. Update Claude Desktop config:
   ```json
   {
     "mcpServers": {
       "llm-support": {
         "command": "/usr/local/bin/llm-support-mcp"
       },
       "llm-clarification": {
         "command": "/usr/local/bin/llm-clarification-mcp"
       }
     }
   }
   ```

3. Restart Claude Desktop

**Benefits of Go implementation:**
- No Python runtime dependency
- ~10x faster startup time
- Native binary - single file deployment
- Identical functionality to Python version

## Version History

| Server | Version | Tools | Notes |
|--------|---------|-------|-------|
| llm-support-mcp (Go) | 1.0.0 | 18 | Native Go implementation |
| llm-clarification-mcp (Go) | 1.0.0 | 8 | Native Go implementation |
| llm-support-mcp (Python) | 2.9.0 | 18 | Legacy Python, deprecated |
| llm-clarification-mcp (Python) | 1.0.0 | 8 | Legacy Python, deprecated |

## See Also

- [README.md](../README.md) - Main documentation
- [quick-reference.md](quick-reference.md) - Command cheat sheet
- [llm-support-commands.md](llm-support-commands.md) - Detailed command reference
