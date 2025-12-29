# llm-semantic Commands

Semantic code search CLI with local embeddings for natural language code discovery.

## Installation

```bash
go install github.com/samestrin/llm-tools/cmd/llm-semantic@latest
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (machine-parseable) |
| `--min` | Minimal output (reduced verbosity) |

## Commands

### search

Search the semantic index using natural language queries.

```bash
llm-semantic search "authentication logic" --top 10 --threshold 0.7
```

| Flag | Description | Default |
|------|-------------|---------|
| `--top` | Number of results to return | 10 |
| `--threshold` | Minimum similarity threshold (0.0-1.0) | 0.0 |
| `--type` | Filter by symbol type (function, type, method, etc.) | |
| `--path` | Filter by file path pattern | |
| `--min` | Minimal output format | false |
| `--json` | JSON output format | false |

**Example Output:**
```json
{
  "results": [
    {
      "file": "internal/auth/handler.go",
      "name": "ValidateToken",
      "type": "function",
      "line": 42,
      "score": 0.89,
      "snippet": "func ValidateToken(token string) (*Claims, error) {"
    }
  ],
  "query": "authentication logic",
  "total": 1
}
```

### index

Build or rebuild the semantic index for a codebase.

```bash
llm-semantic index . --include "*.go" --exclude "vendor/*"
```

| Flag | Description | Default |
|------|-------------|---------|
| `--include` | File patterns to include (can be repeated) | |
| `--exclude` | File patterns to exclude (can be repeated) | |
| `--force` | Force full reindex even if index exists | false |
| `--json` | JSON output format | false |

**Example Output:**
```json
{
  "status": "completed",
  "files_indexed": 156,
  "symbols_indexed": 1247,
  "duration_ms": 8432
}
```

### index-status

Check the status of the semantic index.

```bash
llm-semantic index-status --json
```

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | JSON output format | false |

**Example Output:**
```json
{
  "exists": true,
  "path": "/Users/user/project/.semantic-index",
  "files_indexed": 156,
  "symbols_indexed": 1247,
  "last_updated": "2025-12-29T10:30:00Z",
  "model": "all-MiniLM-L6-v2"
}
```

### index-update

Incrementally update the semantic index with changed files.

```bash
llm-semantic index-update . --include "*.go"
```

| Flag | Description | Default |
|------|-------------|---------|
| `--include` | File patterns to include (can be repeated) | |
| `--exclude` | File patterns to exclude (can be repeated) | |
| `--json` | JSON output format | false |

**Example Output:**
```json
{
  "status": "completed",
  "files_added": 3,
  "files_updated": 7,
  "files_removed": 1,
  "duration_ms": 1256
}
```

## MCP Integration

The MCP wrapper (`llm-semantic-mcp`) exposes all 4 commands as MCP tools with the `llm_semantic_` prefix:

- `llm_semantic_search`
- `llm_semantic_index`
- `llm_semantic_status`
- `llm_semantic_update`

See [MCP Setup Guide](MCP_SETUP.md) for integration instructions.

## How It Works

1. **Indexing**: Parses source code to extract symbols (functions, types, methods, variables)
2. **Embedding**: Generates vector embeddings using a local model (all-MiniLM-L6-v2)
3. **Storage**: Stores embeddings in a local vector database
4. **Search**: Converts queries to embeddings and finds nearest neighbors

## Supported Languages

- Go (`.go`)
- TypeScript/JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`)
- Python (`.py`)
- Rust (`.rs`)
- Java (`.java`)

## Performance Notes

- Initial indexing can take 1-2 minutes for large codebases
- Incremental updates are typically sub-second
- Search queries return in <100ms
- Index is stored locally in `.semantic-index/` directory
