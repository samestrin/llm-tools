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
| `LLM_SEMANTIC_RERANKER_API_URL` | Reranker API URL (enables reranking when set) |
| `LLM_SEMANTIC_RERANKER_MODEL` | Reranker model name (default: `Qwen/Qwen3-Reranker-0.6B`) |
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
| `--top, -n` | Number of results to return | 10 |
| `--threshold, -t` | Minimum similarity threshold (0.0-1.0) | 0.0 |
| `--type` | Filter by symbol type (function, method, struct, interface) | |
| `--path, -p` | Filter by file path prefix | |
| `--profiles` | Profiles to search across (comma-separated, e.g., `code,docs`) | |
| `--min` | Minimal output format | false |
| `--json` | JSON output format | false |

**Hybrid Search:**
| Flag | Description | Default |
|------|-------------|---------|
| `--hybrid` | Enable hybrid search (dense + lexical with RRF fusion) | false |
| `--fusion-k` | RRF fusion k parameter (higher = smoother ranking) | 60 |
| `--fusion-alpha` | Fusion weight: 1.0 = dense only, 0.0 = lexical only | 0.7 |

**Prefilter Search:**
| Flag | Description | Default |
|------|-------------|---------|
| `--prefilter` | Enable lexical prefiltering (narrow candidates with FTS5 before vector search) | false |
| `--prefilter-top` | Number of lexical candidates for prefiltering | max(topK*10, 100) |

Prefilter search uses lexical search (FTS5) to narrow down candidates before vector search. Unlike `--hybrid` which **fuses** lexical and vector results, `--prefilter` uses lexical search purely as a **filter** to reduce the vector search space. This is more efficient for large indexes.

**Recency Boost:**
| Flag | Description | Default |
|------|-------------|---------|
| `--recency-boost` | Enable recency boost (recently modified files ranked higher) | false |
| `--recency-factor` | Recency boost factor (max boost = 1+factor) | 0.5 |
| `--recency-decay` | Recency half-life in days (higher = slower decay) | 7 |

**Reranking:**

When `LLM_SEMANTIC_RERANKER_API_URL` is set, reranking is automatically enabled. Reranking uses a cross-encoder model to re-score search results for improved precision.

| Flag | Description | Default |
|------|-------------|---------|
| `--rerank` | Enable reranking (auto-enabled when reranker URL is set) | auto |
| `--rerank-candidates` | Number of candidates to fetch for reranking | max(topK*5, 50) |
| `--rerank-threshold` | Minimum reranker score (0.0-1.0) | 0.0 |
| `--no-rerank` | Disable reranking even when reranker is configured | false |

**Example with reranking:**
```bash
# Reranking enabled automatically when env var is set
export LLM_SEMANTIC_RERANKER_API_URL=http://ai.lan:5000

# Search with reranking (on by default)
llm-semantic search "authentication middleware" --top 10

# Disable reranking for this query
llm-semantic search "simple query" --no-rerank

# Custom reranking settings
llm-semantic search "complex query" --rerank-candidates 100 --rerank-threshold 0.5
```

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
| `--include` | Glob patterns to include (can be repeated) | |
| `--exclude` | Patterns to exclude - directories and files (can be repeated) | |
| `--exclude-tests` | Exclude common test files (`*_test.go`, `*.spec.ts`, `__tests__/`, etc.) | false |
| `--force` | Force full reindex even if index exists | false |
| `--json` | JSON output format | false |

**Performance Options:**
| Flag | Description | Default |
|------|-------------|---------|
| `--batch-size` | Number of vectors per upsert batch (0 = unlimited) | 0 |
| `--parallel` | Number of parallel batch uploads (requires batch-size > 0) | 0 |
| `--embed-batch-size` | Number of chunks to embed per API call across files | 0 |

**Calibration Options:**
| Flag | Description | Default |
|------|-------------|---------|
| `--recalibrate` | Force recalibration of score thresholds | false |
| `--skip-calibration` | Skip calibration step during indexing | false |

**Progress Display:**

When using `--embed-batch-size` with `--batch-size`, real-time progress is displayed during embedding and upload phases:

```
Embedding: [3/10 batches] 384/1280 chunks (30%) ETA: 2m 15s
Uploading: [5/16 batches] 320/1024 chunks (31%) ETA: 45s
```

- TTY terminals show single-line updates (overwrites previous line)
- Non-TTY environments log at 10% intervals
- ETA calculation starts after 2 batches for accurate estimation

**Example with performance tuning:**
```bash
# Index with batched uploads and parallel processing
llm-semantic index . --include "*.go" --batch-size 100 --parallel 4

# Index with cross-file embedding batches (faster for remote APIs)
llm-semantic index . --include "*.go" --embed-batch-size 32

# Full performance setup with progress display
llm-semantic index . --include "*.go" --embed-batch-size 64 --batch-size 100 --parallel 4

# Exclude test files
llm-semantic index . --include "*.go" --exclude-tests
```

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

### Profile-Based Configuration

All MCP tools support the `profile` and `config` parameters for simplified configuration. This allows you to define named profiles in a config.yaml file and reference them by name instead of specifying `storage` and `collection` on every call.

**Config File Structure (`.planning/.config/config.yaml`):**
```yaml
semantic:
  code_collection: llm-tools-code
  code_storage: qdrant
  docs_collection: llm-tools-docs
  docs_storage: sqlite
  memory_collection: llm-tools-memory
  memory_storage: qdrant
```

**Supported Profiles:**
| Profile | Config Keys Used |
|---------|------------------|
| `code` | `code_collection`, `code_storage` |
| `docs` | `docs_collection`, `docs_storage` |
| `memory` | `memory_collection`, `memory_storage` |

**MCP Tool Usage:**
```json
// Get status for the code index
{
  "name": "llm_semantic_index_status",
  "arguments": {
    "profile": "code",
    "config": ".planning/.config/config.yaml"
  }
}

// Search the code index with custom top_k
{
  "name": "llm_semantic_search",
  "arguments": {
    "query": "authentication",
    "profile": "code",
    "config": ".planning/.config/config.yaml",
    "top_k": 3
  }
}
```

**Override Behavior:**
- Explicit `storage` and `collection` parameters override profile values
- Profile values are only applied when the corresponding parameter is not already set
- If `profile` is provided but `config` is missing, no profile resolution occurs

See [MCP Setup Guide](MCP_SETUP.md) for integration instructions.

## How It Works

1. **Indexing**: Parses source code to extract semantic chunks (functions, types, methods, structs)
2. **Embedding**: Generates vector embeddings using any OpenAI-compatible API (Ollama, vLLM, OpenAI, etc.)
3. **Storage**: Stores embeddings in SQLite (default) or Qdrant vector database
4. **Search**: Converts queries to embeddings and finds nearest neighbors by cosine similarity
5. **Reranking** (optional): Uses a cross-encoder model to re-score top candidates for improved precision

### Two-Stage Retrieval with Reranking

When reranking is enabled, search uses a two-stage retrieval pipeline:

1. **Stage 1 - Fast Recall**: Embedding-based search retrieves a large candidate pool (default: max(topK*5, 50))
2. **Stage 2 - Precise Reranking**: A cross-encoder model scores each (query, document) pair for semantic relevance

This approach combines the speed of embedding search with the precision of cross-encoder scoring. The reranker sees the full query and document context, enabling better relevance judgments than embedding similarity alone.

**Recommended model pairing:**
- **Embedding**: Qwen/Qwen3-Embedding-0.6B (1024 dims, ~1.2GB VRAM)
- **Reranker**: Qwen/Qwen3-Reranker-0.6B (~1.0GB VRAM)
- **Total**: ~2.2GB VRAM for both models

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
- Search queries return in <100ms with proper configuration
- Index is stored locally in `.llm-index/semantic.db` (SQLite default)

### Storage Backend Recommendations

Choose your storage backend based on index size:

| Index Size | Recommended Storage | Search Flags | Expected Latency |
|------------|---------------------|--------------|------------------|
| < 10K chunks | SQLite | None | ~500ms |
| 10K-50K chunks | SQLite | `--prefilter` | ~500ms-1s |
| 50K-200K chunks | SQLite | `--prefilter` | ~1-2s |
| > 50K chunks | **Qdrant** | None | ~300ms |
| > 200K chunks | **Qdrant** | None | ~300-500ms |

**Why Qdrant is faster for large indexes:**
- SQLite uses brute-force O(n) cosine similarity (scans all chunks)
- Qdrant uses HNSW indexing for O(log n) approximate nearest neighbor search
- With 200K+ chunks, this difference is dramatic (seconds vs milliseconds)

**When to use search flags:**

| Flag | Use Case | Effect |
|------|----------|--------|
| `--prefilter` | Large SQLite indexes | Narrows search space with FTS5 before vector search |
| `--hybrid` | Better recall needed | Fuses lexical + vector results (slightly slower) |
| `--rerank` | Better precision needed | Cross-encoder re-scoring (adds ~1-2s) |
| `--no-rerank` | Speed over precision | Disables reranker even when configured |

**Typical search time breakdown (Qdrant):**
- ~140ms: Embedding API call (query → vector)
- ~10-50ms: Qdrant vector search
- ~1-2s: Reranker (if enabled)

**Tips for sub-second search:**
1. Use Qdrant for indexes > 50K chunks
2. Disable reranker with `--no-rerank` when speed matters
3. Ensure embedding API is responsive (local Ollama or dedicated GPU server)
4. Use `--prefilter` with SQLite to reduce vector comparisons

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

### Reranker Server Setup

To enable reranking, you need a Cohere-compatible reranker API. Here's an example using FastAPI and Hugging Face transformers:

**Python server (`librarian.py`):**
```python
import torch
from fastapi import FastAPI
from pydantic import BaseModel
from typing import List, Optional
from transformers import AutoModel, AutoTokenizer, AutoModelForCausalLM

app = FastAPI()

# Load models
EMBEDDING_MODEL = "Qwen/Qwen3-Embedding-0.6B"
RERANKER_MODEL = "Qwen/Qwen3-Reranker-0.6B"

embedding_tokenizer = AutoTokenizer.from_pretrained(EMBEDDING_MODEL)
embedding_model = AutoModel.from_pretrained(EMBEDDING_MODEL, torch_dtype=torch.float16).cuda().eval()

reranker_tokenizer = AutoTokenizer.from_pretrained(RERANKER_MODEL, padding_side='left')
reranker_model = AutoModelForCausalLM.from_pretrained(RERANKER_MODEL, torch_dtype=torch.float16).cuda().eval()

class RerankRequest(BaseModel):
    query: str
    documents: List[str]
    model: Optional[str] = "default"
    top_n: Optional[int] = None
    instruction: Optional[str] = None

@app.post("/v1/rerank")
async def rerank(request: RerankRequest):
    # Format prompt, compute yes/no logits, return scores
    # See Qwen3 reranker documentation for implementation details
    ...
```

**Usage with llm-semantic:**
```bash
# Set reranker URL (enables reranking automatically)
export LLM_SEMANTIC_RERANKER_API_URL=http://ai.lan:5000

# Index and search with reranking
llm-semantic index . --include "*.go"
llm-semantic search "authentication middleware" --top 10

# Disable reranking for a specific query
llm-semantic search "simple query" --no-rerank
```

**Docker deployment:**
```bash
# Run embedding + reranking server
docker run -d --gpus all -p 5000:5000 \
  -v /path/to/models:/models \
  your-registry/ai-embeddings-server
```
