# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.7.0] - 2026-01-23

### Added

#### llm-semantic

- **`collection delete` command** - Delete all chunks for a specific profile/domain:
  - `llm-semantic collection delete --profile memory` removes all memory embeddings
  - Supports profiles: code, docs, memory, sprints
  - Confirmation prompt (skipped with `--force`)
  - Works with both SQLite and Qdrant storage backends

#### llm-semantic-mcp

- **`llm_semantic_collection_delete` tool** - Delete all chunks for a specific profile from the semantic index:
  - `profile` parameter: code, docs, memory, sprints
  - `force` parameter: skip confirmation

#### llm-support

- **`--checkbox` flag for `format-td-table`** - Add checkbox column to technical debt tables for tracking completion:
  - Adds empty checkboxes `[ ]` to the first column of each table
  - Useful for tracking TD items in markdown documents with checklists

### Changed

#### llm-semantic-mcp

- **Improved documentation** - Default config path explicitly shown in tool descriptions:
  - Changed from "Requires config file with semantic.*_collection defined"
  - To: "Uses default config (.planning/.config/config.yaml) with semantic.*_collection defined"

### Fixed

#### llm-semantic-mcp

- **Memory tools default to SQLite instead of respecting config** - Memory MCP tools now automatically inject the "memory" profile to respect `config.yaml` settings like `semantic.memory_storage: qdrant`:
  - Previously: `memory_store`, `memory_list`, `memory_delete`, `memory_promote`, and `memory_stats` defaulted to SQLite even when config specified Qdrant
  - Now automatically applies `memory` profile defaults, matching the pattern of `search_code`, `search_docs`, and `search_memory` convenience wrappers

## [1.8.0] - Unreleased

### Added

#### llm-semantic

- **Reranking support** - Two-stage retrieval with cross-encoder reranking for improved precision:
  - Automatic reranking when `LLM_SEMANTIC_RERANKER_API_URL` environment variable is set
  - Uses Cohere-compatible `/v1/rerank` API endpoint
  - Over-fetches candidates (default: max(topK*5, 50)) then reranks to final topK
  - CLI flags: `--rerank`, `--rerank-candidates`, `--rerank-threshold`, `--no-rerank`
  - Environment variables: `LLM_SEMANTIC_RERANKER_API_URL`, `LLM_SEMANTIC_RERANKER_MODEL`
  - Recommended model: Qwen/Qwen3-Reranker-0.6B (~1GB VRAM)
  - Graceful fallback: logs warning and uses embedding scores if reranker fails
  - MCP tools updated with rerank parameters

- **Upload progress with ETA** - Real-time progress feedback during embedding and upload phases:
  - Displays batch progress for embedding phase: `Embedding: [X/Y batches] N/M chunks (P%) ETA: Xm Ys`
  - Displays batch progress for uploading phase: `Uploading: [X/Y batches] N/M chunks (P%) ETA: Xm Ys`
  - TTY-aware: single-line updates on terminals, periodic logging in non-TTY environments
  - ETA calculation starts after 2 batches for accurate estimation
  - Progress visible when using `--embed-batch-size` with `--batch-size`

- **Parallel batch uploads** - Speed up indexing with concurrent vector upserts:
  - `--parallel N` flag to enable N concurrent batch uploads during indexing
  - Requires `--batch-size` to be set (parallelism only applies to batched mode)
  - Uses errgroup for fail-fast error handling
  - Example: `llm-semantic index --batch-size 64 --parallel 4` uploads 4 batches concurrently

- **Cross-file embedding batching** - Dramatically reduce embedding API calls:
  - `--embed-batch-size N` flag to batch N chunks per embedding API call across multiple files
  - Without this flag: 100 files = 100 API calls (one per file)
  - With `--embed-batch-size 64`: 100 files × 5 chunks = 500 chunks / 64 = 8 API calls
  - Chunks all files first, then batches embedding requests for maximum throughput
  - Works with `--batch-size` and `--parallel` for storage batching
  - Example: `llm-semantic index --embed-batch-size 64 --batch-size 64 --parallel 4`

- **Enhanced `--exclude` flag** - Now supports file patterns in addition to directories:
  - Exclude specific file patterns: `--exclude "*_test.go"` or `--exclude "*.spec.ts"`
  - Still works for directories: `--exclude vendor`
  - Patterns use glob matching (same as `--include`)

- **`--exclude-tests` convenience flag** - Exclude common test files and directories:
  - Excludes test file patterns: `*_test.go`, `*.test.ts`, `*.spec.js`, `test_*.py`, etc.
  - Excludes test directories: `__tests__`, `test`, `tests`, `testdata`, `fixtures`, etc.
  - Covers Go, TypeScript, JavaScript, Python, Rust, PHP, Ruby conventions

- **Benchmark tests for SearchMemory** - `storage_sqlite_bench_test.go` with large index performance verification
- **HTML chunker fallback tests** - TestFallbackToText with comprehensive edge case coverage
- **Markdown chunker list tests** - TestMarkdownChunker_ListContextTracking with 5 test cases for nested lists



## [1.6.0] - 2026-01-17

### Added

#### llm-semantic

- **Hybrid search with RRF fusion** - Combine dense vector search with lexical BM25 for better results:
  - `--hybrid` flag enables combined search
  - `--fusion-alpha` controls dense vs lexical weighting (0.0-1.0, default 0.7)
  - `--fusion-k` controls RRF smoothing (default 60)

- **Recency boosting** - Recently modified files rank higher:
  - `--recency-boost` enables the feature
  - `--recency-factor` controls boost strength (default 0.5)
  - `--recency-decay` sets half-life in days (default 7)

- **Multi-profile search** - Search across multiple indexes in one query:
  - `--profiles code,docs` searches both code and documentation
  - Results merged and deduplicated by score

- **`multisearch` command** - Execute 1-10 queries in parallel with intelligent result merging:
  - Multi-match boosting: results matching multiple queries score higher
  - Automatic deduplication
  - Output modes: blended (flat), by_query (grouped), by_collection

- **Config file and profile support** - Project-specific semantic search configuration:
  - `--config .planning/.config/config.yaml` loads project settings
  - `--profile code|docs|memory|sprints` selects index profile
  - Profiles define collection names and storage backends per project

- **Sprints profile** - Index sprint planning documents for semantic search

- **Calibration infrastructure** - Automatic threshold calibration per embedding model:
  - Runs on first index, stores high/medium/low thresholds
  - `--recalibrate` forces recalibration
  - `--skip-calibration` bypasses for faster indexing

- **Memory commands** - Consolidated from llm-clarification:
  - `memory store` - Store Q&A pairs with tags
  - `memory search` - Semantic search through memories
  - `memory list` - List stored memories
  - `memory promote` - Promote to CLAUDE.md
  - `memory delete` - Remove entries

- **Markdown chunker** - Semantic chunking for `.md` files by headers and sections

- **HTML chunker** - Semantic chunking for `.html`/`.htm` files by document structure

- **Rust language support** - Native chunker for `.rs` files with support for:
  - Functions (`fn`, `pub fn`, `async fn`, `unsafe fn`)
  - Structs and enums
  - Traits and impl blocks
  - Modules and type aliases

- **Enhanced search output** - Relevance labels (high/medium/low/marginal) based on calibrated thresholds

- **`LLM_SEMANTIC_MODEL` environment variable** - Set default embedding model

#### llm-support

- **8 deterministic workflow tools** (Sprint 8.14):
  - `tdd-compliance` - Analyze git history for TDD compliance scoring
  - `sprint-status` - Determine sprint completion status from metrics
  - `route-td` - Route technical debt by time estimate thresholds
  - `parse-stream` - Parse pipe-delimited and markdown checklist formats
  - `coverage-report` - Calculate requirement coverage from user stories
  - `alignment-check` - Compare requirements against delivered work
  - `validate-risks` - Cross-reference sprint risks with work items
  - `plan-type` - Extract plan type from metadata files

- **YAML command enhancements** (Sprint 9.0):
  - Array bracket notation support: `items[0].name`, `items[-1]`
  - `--quiet` flag suppresses success messages
  - `--dry-run` previews changes without writing
  - `--create` creates file if missing
  - Better error messages for invalid paths

- **`headers` format for `summarize-dir`** - Extract just markdown headers

#### llm-filesystem-mcp

- **Simplified to 15 batch/specialized tools** - Removed redundant single-file operations (use native Claude tools instead)

### Changed

- **Default embedding model** changed from `mxbai-embed-large` to `nomic-embed-text` (8K context, faster, better for code)

### Fixed

- **llm-semantic-mcp**: Binary path now resolves to absolute path on startup - fixes "Disconnected" status when running from directories without local binary
- **llm-semantic**: `--force` flag now properly clears index before rebuilding
- **llm-semantic**: Suppressed Qdrant API key warning in insecure mode
- **llm-support-mcp**: Default template syntax now correctly uses brackets

## [1.5.0] - 2026-01-08

### Added

#### llm-support

- **`highest --paths` flag** - Search multiple directories to find the global highest version number:
  - Useful when items move between `active/`, `completed/`, `pending/` subdirectories
  - Example: `llm-support highest --paths .planning/plans/active,.planning/plans/completed`
  - Returns the highest across all specified directories with correct `FULL_PATH`

#### pkg/pathvalidation

- **Unresolved template variable detection** - New validation package catches common template interpolation failures before creating directories:
  - Detects `{{VAR}}`, `${{VAR}}`, `${VAR}`, `$VAR`, `[[VAR]]`, and `[VAR]` patterns
  - Single bracket `[VAR]` only triggers for uppercase (avoids false positives on `[0]`, `[optional]`)
  - Clear error message: `path contains unresolved template variable '{{NEXT}}' - check your variable substitution`

#### llm-filesystem

- **Path validation on directory creation** - `create_directory` and `create_directories` now validate paths for unresolved template variables before creating

#### llm-support

- **Path validation on init-temp** - `init-temp --name` now validates for unresolved template variables

## [1.4.0] - 2026-01-06

### Added

#### llm-support

- **`extract-links` command** - Extract and rank links from URLs with intelligent scoring:
  - Links scored by HTML context (h1=100, h2=85, nav=30, footer=10, etc.)
  - Modifier bonuses for bold (+15), emphasis (+10), button roles (+10), title attributes (+5)
  - Tracks parent section headings for context
  - Deduplicates links and resolves relative URLs
  - Returns href, text, context, score, and section for each link

- **URL support for `extract-relevant`** - Now accepts HTTP/HTTPS URLs in addition to file paths:
  - HTML content automatically converted to clean markdown-style text
  - Strips scripts, styles, nav, footer elements
  - Preserves document structure (headings, lists, code blocks)

- **`highest` command documentation** - Updated to show `active/pending/completed` subdirectory structure for plans (matching sprints pattern)

#### llm-support-mcp

- **`file_path` parameter alias** - MCP tools now accept `file_path` as an alias for `path`, improving compatibility with LLMs that assume this parameter name

## [1.3.0] - 2026-01-04

### Added

#### llm-filesystem

- `create_directories` - Batch create multiple directories in a single operation
- **Size limit protection for read operations** - Prevents tool output from exceeding Claude's context limits:
  - Smart JSON size estimation accounts for encoding overhead (escaping `\n`, `\t`, `"`, `\` characters)
  - Hybrid checking: pre-check (raw size) + post-read (estimated JSON size)
  - Default 70,000 character limit with `-1` to disable
  - Works for both `read_file` and `read_multiple_files`

#### llm-support

- `runtime` command - Calculate and format elapsed time between epoch timestamps with configurable format (secs, mins, mins-secs, hms, human, compact) and precision
- `clean-temp` command - Clean up temp directories created by `init-temp` with options for removing specific directories, all directories, or directories older than a duration
- Global `--json`/`--min` output modes added to: `math`, `context`, `yaml`, `count`, `args` commands

#### llm-semantic

- Global `--json`/`--min` output flags for all commands
- Improved error handling

#### llm-clarification

- Global `--json`/`--min` output flags for all commands
- Improved error handling

### Fixed

- **llm-clarification-mcp**: Fixed `--file` flag passing (was positional arg)
- **llm-clarification-mcp**: Fixed `sprint_id` and `context_tags` parameter mapping
- **llm-support-mcp**: Removed `--min` from `detect`/`discover_tests` handlers (not supported)
- **llm-support**: Fixed `omitempty` on documented result fields causing missing output
- **llm-support**: Resolved duplicate JSON tag errors in `prompt` and `runtime` commands
- **llm-support**: Removed `count` from key abbreviations for consistency
- **CI**: Fixed missing assets during release cleanup

### Changed

- Pre-commit hook now auto-formats Go files with `gofmt`

## [1.2.0] - 2026-01-01

### Added

#### New Tool: llm-filesystem (27 commands)
Full-featured filesystem operations as a drop-in replacement for fast-filesystem-mcp:

**Core Operations:**
- `read_file` - Read files with auto-chunking and continuation tokens
- `read_multiple_files` - Read multiple files simultaneously
- `write_file` - Write content to files
- `large_write_file` - Write large files with backup and verification
- `list_directory` - List directory with pagination and sorting
- `get_directory_tree` - Recursive tree with configurable depth
- `get_file_info` - Detailed file metadata
- `create_directory` - Create directories recursively

**Search:**
- `search_files` - Find files by name pattern
- `search_code` - Search file contents with context lines
- `search_and_replace` - Bulk find/replace across files

**Editing:**
- `edit_block` - Precise block replacement
- `edit_blocks` - Multiple block edits in one operation
- `edit_file` - Line-based editing (insert/replace/delete)
- `safe_edit` - Edit with backup and dry-run support
- `extract_lines` - Extract specific line ranges

**File Management:**
- `copy_file` - Copy files/directories
- `move_file` - Move/rename files
- `delete_file` - Delete with confirmation
- `batch_file_operations` - Bulk copy/move/delete
- `sync_directories` - Directory synchronization
- `compress_files` - Create zip/tar archives
- `extract_archive` - Extract archives

**Analysis:**
- `get_disk_usage` - Disk space analysis
- `find_large_files` - Find files by size threshold
- `list_allowed_directories` - Show accessible paths

#### New Tool: llm-semantic (4 commands)
Semantic code search using local embeddings:
- `index` - Build semantic index for codebase
- `search` - Natural language code search
- `update` - Incrementally update index
- `status` - Check index status

#### llm-support Enhancements

**New Commands:**
- `context multiset` - Set multiple context variables atomically
- `context multiget` - Get multiple context variables in one call
- `yaml-get` - Get value from YAML config by dot-notation key
- `yaml-set` - Set value in YAML config
- `yaml-multiget` - Get multiple YAML values
- `yaml-multiset` - Set multiple YAML values atomically
- `plan-type` - Detect plan type from metadata
- `git-changes` - Count and list git working tree changes

**Enhanced Commands:**
- `init-temp` - Now returns REPO_ROOT, TODAY, TIMESTAMP, EPOCH, auto-creates context.env
  - New `--with-git` flag adds BRANCH and COMMIT_SHORT
  - New `--skip-context` flag skips context.env creation
  - Reduces prompt setup from 7 operations to 1

**Output Formatting:**
- All commands now support `--min` flag for token-optimized output
- All commands now support `--json` flag for structured output

#### llm-clarification Enhancements
- SQLite storage backend with YAML backward compatibility
- Renamed MCP tools for consistency with CLI commands

### Changed

#### llm-filesystem Breaking Changes (API Parity with fast-filesystem)
Parameter renames for fast-filesystem compatibility:
- `depth` → `max_depth` (get_directory_tree)
- `limit` → `max_results` (find_large_files)
- `min_size` now accepts string format ("100MB" instead of bytes)

Output key renames:
- `root` → `tree` (get_directory_tree)
- `entries` → `items` (list_directory)
- `files` → `results` (search operations)
- `is_dir` → `type` ("file" or "directory")
- `mod_time` → `modified`

New output fields:
- `size_readable` - Human-readable size ("6.70 KB")
- `created`, `accessed` - Additional timestamps
- `extension`, `mime_type` - File type info
- `is_readable`, `is_writable` - Access checks
- `continuation_token`, `auto_chunked` - Pagination support
- `context_before`, `context_after` - Search context lines
- `permissions` - Numeric file permissions

See `docs/llm-filesystem-migration.md` for upgrade guide.

#### Architecture
- CLI + MCP wrapper pattern: All tools now have separate CLI and MCP binaries
- MCP servers wrap CLI binaries for consistent behavior

### Fixed
- llm-filesystem pagination now respects `page_size` parameter
- llm-filesystem tree recursion properly traverses to `max_depth`
- Context operations use file locking for concurrent safety

## [1.1.0] - 2025-12-27

### Added
- **Parameter alias normalization system** for MCP tools - LLMs can now use alternative parameter names and they'll be automatically normalized to canonical names:
  - `target`, `dir`, `directory`, `input` → `path`
  - `template` → `file`
  - `package` → `manifest`
  - `regex`, `search` → `pattern`
  - `prompt`, `description` → `context`
- Canonical parameters always take precedence over aliases when both are provided

### Changed
- **Aligned MCP schema parameter names with CLI flags** - `llm_support_count` now uses `path` parameter (was `target`) for consistency with other tools

### Fixed
- **`llm_support_count` MCP tool** now correctly passes `--path` flag to CLI (was passing as positional argument, causing "required flag 'path' not set" errors)

## [1.0.1] - 2025-12-27

### Changed
- **Migrated MCP servers to official Go SDK** ([github.com/modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) v1.2.0)
- Removed custom MCP transport implementation in favor of SDK's `StdioTransport`
- Updated Go version requirement to 1.23.0

### Added
- Full Gemini CLI support (tested and verified working)
- Proper MCP protocol compliance via official SDK

### Fixed
- MCP server compatibility with Gemini CLI (previously only worked with Claude)
- Protocol handshake issues with non-Claude MCP clients
- Capabilities response format (now uses object instead of boolean per MCP spec)
- Instructions field placement in initialize response
- Claude and Gemini CLI compatibility verified working

### Removed
- Python MCP wrappers (Go-only implementation)
- Custom MCP transport code (replaced by official SDK)

## [1.0.0] - 2025-12-24

### Added

#### llm-support (32+ commands)

**File Operations:**
- `listdir` - List directory contents with filtering
- `tree` - Display directory tree structure
- `catfiles` - Concatenate multiple files
- `hash` - Calculate file checksums (SHA256, MD5, SHA1)
- `stats` - Show directory/file statistics

**Search:**
- `grep` - Search file contents with regex
- `multigrep` - Search multiple keywords in parallel (10+ keywords in <500ms)
- `multiexists` - Check if multiple files exist

**Code Analysis:**
- `detect` - Detect project type and stack
- `discover-tests` - Find test frameworks and patterns
- `analyze-deps` - Extract file dependencies from markdown
- `partition-work` - Group tasks by file conflicts using graph coloring

**Data Processing:**
- `json` - Query JSON with JSONPath
- `toml` - Query TOML files
- `markdown` - Parse and query markdown
- `extract` - Extract URLs, emails, IPs, variables, todos
- `transform` - Text transformations (case, trim, slug, etc.)
- `count` - Count lines, words, checkboxes
- `encode` - Base64/URL encoding/decoding
- `math` - Evaluate mathematical expressions

**LLM Integration:**
- `prompt` - Execute LLM prompts with templates, caching, retries
- `foreach` - Batch process files with LLM
- `extract-relevant` - Extract relevant content with LLM assistance
- `summarize-dir` - Generate directory summaries with LLM

**Development:**
- `validate` - Validate JSON/YAML/TOML files
- `validate-plan` - Validate sprint plan structure
- `template` - Process Go text templates
- `diff` - Compare files with unified diff
- `report` - Generate reports from data
- `init-temp` - Initialize temporary workspace
- `deps` - Show project dependencies
- `git-context` - Get git context information

#### llm-clarification (12 commands)
- `init` - Initialize clarification tracking file
- `add` - Add new clarification entry
- `list` - List all clarification entries
- `validate` - Validate clarification file format
- `conflicts` - Detect conflicting clarifications
- `normalize` - Normalize clarification entries
- `consolidate` - Merge duplicate entries
- `candidates` - Find candidate clarifications
- `cluster` - Group related clarifications
- `match` - Match clarifications to context
- `promote` - Promote clarification to specification

#### Infrastructure
- GitHub Actions CI/CD pipeline
- Cross-platform builds (Linux, macOS, Windows; amd64, arm64)
- SHA256 checksums for releases
- Comprehensive test suite (76%+ coverage)
- Benchmark tests for performance verification
- Golden file testing infrastructure

### Performance

Measured on llm-interface (21,322 files, 459MB):

| Operation | What it did | Time |
|-----------|-------------|------|
| Startup | `--help` | 6ms |
| MCP Server | Initialize handshake | 4ms |
| detect | Identify project stack | 6ms |
| tree | 3 levels, 847 entries | 22ms |
| listdir | src/ directory (45 items) | 42ms |
| grep | "function" in 21k files (58,296 matches) | 581ms |
| multigrep | 5 keywords in 21k files (156,893 matches) | 1.47s |
| hash | SHA256 of package.json | 6ms |
| count | Lines in package.json | 6ms |

Binary size: 14-15MB per platform.

### Technical Details

- Written in Go 1.22+
- Uses Cobra CLI framework
- Integrates with ripgrep for fast search
- OpenAI-compatible LLM API with retry and caching
- Race-condition free (verified with `go test -race`)

[Unreleased]: https://github.com/samestrin/llm-tools/compare/v1.7.0...HEAD
[1.7.0]: https://github.com/samestrin/llm-tools/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/samestrin/llm-tools/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/samestrin/llm-tools/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/samestrin/llm-tools/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/samestrin/llm-tools/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/samestrin/llm-tools/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/samestrin/llm-tools/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/samestrin/llm-tools/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/samestrin/llm-tools/releases/tag/v1.0.0
