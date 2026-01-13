# llm-semantic Commands

Semantic code search CLI with local embeddings for natural language code discovery.

## Installation

```bash
go install github.com/samestrin/llm-tools/cmd/llm-semantic@latest
```

## Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--api-url` | Embedding API URL (OpenAI-compatible) | `http://localhost:11434` |
| `--model` | Embedding model name | `nomic-embed-text` |
| `--api-key` | API key (or set `LLM_SEMANTIC_API_KEY` env var) | |
| `--embedder` | Embedding provider: `openai`, `cohere`, `huggingface`, `openrouter` | `openai` |
| `--storage` | Storage backend: `sqlite` or `qdrant` | `sqlite` |
| `--index-dir` | Directory for semantic index | `.llm-index` |
| `--collection` | Qdrant collection name (see resolution below) | derived |
| `--json` | Output as JSON (machine-parseable) | `false` |
| `--min` | Minimal output (reduced verbosity) | `false` |

**Environment Variables:**
| Variable | Description |
|----------|-------------|
| `LLM_SEMANTIC_API_URL` | Embedding API URL (overrides --api-url default) |
| `LLM_SEMANTIC_API_KEY` | API key for embedding service |
| `LLM_SEMANTIC_MODEL` | Embedding model name |
| `OPENAI_API_KEY` | Fallback API key |
| `COHERE_API_KEY` | API key for Cohere embedder |
| `HUGGINGFACE_API_KEY` | API key for HuggingFace embedder |
| `OPENROUTER_API_KEY` | API key for OpenRouter embedder |
| `QDRANT_API_URL` | Qdrant server URL (e.g., `http://db.lan:6334`) |
| `QDRANT_API_KEY` | Qdrant API key |
| `QDRANT_COLLECTION` | Default Qdrant collection name |

### Collection Name Resolution (Qdrant)

When using `--storage qdrant`, the collection name is resolved in this priority order:

1. **`--collection` flag** - Explicit collection name
2. **Derived from `--index-dir`** - If index-dir is non-default (e.g., `.llm-index/code` → `code`)
3. **`QDRANT_COLLECTION` env var** - Environment variable fallback
4. **Default** - `llm_semantic`

**Examples:**
```bash
# Uses collection "code" (derived from index-dir)
llm-semantic index . --storage qdrant --index-dir .llm-index/code

# Uses collection "docs" (derived from index-dir)  
llm-semantic index ./documentation --storage qdrant --index-dir .llm-index/docs

# Uses explicit collection "my_project"
llm-semantic index . --storage qdrant --collection my_project

# Uses QDRANT_COLLECTION env var or default "llm_semantic"
llm-semantic index . --storage qdrant
```

This allows maintaining separate indexes for code vs documentation in the same Qdrant instance.

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
  "path": "/Users/user/project/.llm-index/semantic.db",
  "files_indexed": 156,
  "chunks_indexed": 1247,
  "last_updated": "2025-12-29T10:30:00Z"
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

## Memory Commands

Store, search, and manage learned decisions and clarifications using semantic search.

### memory store

Store a question/answer pair in the semantic memory database.

```bash
llm-semantic memory store \
  --question "How should auth tokens be handled?" \
  --answer "Use JWT with 24h expiry" \
  --tags "auth,security"
```

| Flag | Description | Default |
|------|-------------|---------|
| `-q, --question` | Question or decision (required) | |
| `-a, --answer` | Answer or decision made (required) | |
| `-t, --tags` | Comma-separated context tags | |
| `-s, --source` | Origin source | `manual` |

**Example Output:**
```json
{
  "status": "stored",
  "id": "mem-e38b9cbb3044a9eb",
  "question": "How should auth tokens be handled?",
  "answer": "Use JWT with 24h expiry"
}
```

### memory search

Search stored memories using natural language queries.

```bash
llm-semantic memory search "token handling" --top 5 --threshold 0.7
```

| Flag | Description | Default |
|------|-------------|---------|
| `--top` | Number of results to return | 10 |
| `--threshold` | Minimum similarity threshold (0.0-1.0) | 0.0 |
| `--tags` | Filter by tags (comma-separated) | |
| `--status` | Filter by status (pending, promoted) | |

**Example Output:**
```json
[
  {
    "entry": {
      "id": "mem-e38b9cbb3044a9eb",
      "question": "How should auth tokens be handled?",
      "answer": "Use JWT with 24h expiry",
      "tags": ["auth", "security"],
      "status": "pending"
    },
    "score": 0.85
  }
]
```

### memory promote

Promote a memory entry to CLAUDE.md for persistent project knowledge.

```bash
llm-semantic memory promote mem-e38b9cbb3044a9eb --target ./CLAUDE.md
```

| Flag | Description | Default |
|------|-------------|---------|
| `--target` | Target CLAUDE.md file path (required) | |
| `--section` | Section header to append under | `Learned Clarifications` |
| `--force` | Re-promote even if already promoted | false |

### memory list

List stored memories with optional filtering.

```bash
llm-semantic memory list --status pending --limit 20
```

| Flag | Description | Default |
|------|-------------|---------|
| `--limit` | Maximum entries to return | 50 |
| `--status` | Filter by status (pending, promoted) | |

### memory delete

Delete a memory entry by ID.

```bash
llm-semantic memory delete mem-e38b9cbb3044a9eb --force
```

| Flag | Description | Default |
|------|-------------|---------|
| `--force` | Skip confirmation prompt | false |

### memory import

Import memories from a clarification-tracking.yaml file.

```bash
llm-semantic memory import --source ./clarification-tracking.yaml --dry-run
```

| Flag | Description | Default |
|------|-------------|---------|
| `--source` | Source YAML file path (required) | |
| `--dry-run` | Preview without importing | false |

## MCP Integration

The MCP wrapper (`llm-semantic-mcp`) exposes commands as MCP tools with the `llm_semantic_` prefix:

**Index Commands:**
- `llm_semantic_search` - Search the semantic index
- `llm_semantic_index` - Build/rebuild the semantic index
- `llm_semantic_index_status` - Check index status
- `llm_semantic_index_update` - Incrementally update the index

**Memory Commands:**
- `llm_semantic_memory_store` - Store a learned decision in semantic memory
- `llm_semantic_memory_search` - Search memories using natural language
- `llm_semantic_memory_promote` - Promote memory to CLAUDE.md
- `llm_semantic_memory_list` - List stored memories
- `llm_semantic_memory_delete` - Delete a memory entry

See [MCP Setup Guide](MCP_SETUP.md) for integration instructions.

## How It Works

1. **Indexing**: Parses source code to extract semantic chunks (functions, types, methods, structs)
2. **Embedding**: Generates vector embeddings using any OpenAI-compatible API (Ollama, vLLM, OpenAI, etc.)
3. **Storage**: Stores embeddings in SQLite (default) or Qdrant vector database
4. **Search**: Converts queries to embeddings and finds nearest neighbors by cosine similarity

## Supported Languages

**Language-specific chunkers** (understand code structure):
- Go (`.go`)
- TypeScript/JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`)
- Python (`.py`)
- PHP (`.php`)
- Rust (`.rs`)

**Generic chunker** (falls back for other file types):
- Any text file with recognized extensions

## Performance Notes

- Initial indexing can take 1-2 minutes for large codebases
- Incremental updates are typically sub-second
- Search queries return in <100ms
- Index is stored locally in `.llm-index/semantic.db` (SQLite default)

## Example Usage

```bash
# Index a Go project using local Ollama
llm-semantic index . --include "*.go"

# Index a Rust project
llm-semantic index . --include "*.rs"

# Index using a remote vLLM server
llm-semantic index . \
  --api-url "http://192.168.1.100:11434" \
  --model "nomic-ai/nomic-embed-text-v1.5" \
  --include "*.go"

# Search for authentication-related code
llm-semantic search "user authentication and session management" --top 10

# Check index status
llm-semantic index-status

# Incremental update after code changes
llm-semantic index-update .
```

## Alternative Embedding Servers

### Ollama (Default)

The default embedding server. Works on most platforms.

```bash
# Install Ollama
brew install ollama

# Start server and pull model
ollama serve &
ollama pull nomic-embed-text

# Use with llm-semantic
llm-semantic index . --include "*.go"
```

### qwen3-embeddings-mlx (Apple Silicon)

For Apple Silicon Macs (M1-M5), [qwen3-embeddings-mlx](https://github.com/jakedahn/qwen3-embeddings-mlx) provides native MLX acceleration with state-of-the-art Qwen3 embedding models. **Recommended when Ollama doesn't work** (e.g., cutting-edge Metal versions).

**Models available:**
| Model | Speed | Quality | Memory |
|-------|-------|---------|--------|
| small (0.6B) | 44K tok/s | Good | 900MB |
| medium (4B) | 18K tok/s | Better | 2.5GB |
| large (8B) | 11K tok/s | Best | 4.5GB |

**Setup:**
```bash
# Clone and install
git clone https://github.com/jakedahn/qwen3-embeddings-mlx.git
cd qwen3-embeddings-mlx
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

# Start server (uses small model by default)
python server.py
```

**Usage with llm-semantic:**
```bash
# Set environment variable (recommended)
export LLM_SEMANTIC_API_URL=http://localhost:8000

# Or specify per-command
llm-semantic index . --api-url http://localhost:8000 --model large --include "*.go"

# Search
llm-semantic search "authentication logic" --api-url http://localhost:8000 --model large
```

**Note:** The model used for indexing must match the model used for searching. The large (8B) model produces 4096-dimensional embeddings vs 1024 for small.
