# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/samestrin/llm-tools/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/samestrin/llm-tools/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/samestrin/llm-tools/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/samestrin/llm-tools/releases/tag/v1.0.0
