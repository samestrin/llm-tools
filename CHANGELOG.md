# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### llm-semantic

- **Rust language support** - Native chunker for `.rs` files with support for:
  - Functions (`fn`, `pub fn`, `async fn`, `unsafe fn`)
  - Structs and enums
  - Traits and impl blocks
  - Modules and type aliases

### Changed

- **Default embedding model** changed from `mxbai-embed-large` to `nomic-embed-text` (8K context, faster, better for code)

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

[Unreleased]: https://github.com/samestrin/llm-tools/compare/v1.5.0...HEAD
[1.5.0]: https://github.com/samestrin/llm-tools/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/samestrin/llm-tools/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/samestrin/llm-tools/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/samestrin/llm-tools/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/samestrin/llm-tools/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/samestrin/llm-tools/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/samestrin/llm-tools/releases/tag/v1.0.0
