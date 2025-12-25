# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

Initial release achieves significant performance improvements:

| Metric | Value |
|--------|-------|
| Cold start | ~7ms (vs Python ~200ms) |
| listdir (1000 files) | 70us |
| tree (depth 5) | 88ms |
| hash (1MB) | 0.4ms |
| Binary size | 14-15MB |

### Technical Details

- Written in Go 1.22+
- Uses Cobra CLI framework
- Integrates with ripgrep for fast search
- OpenAI-compatible LLM API with retry and caching
- Race-condition free (verified with `go test -race`)

[Unreleased]: https://github.com/samestrin/llm-tools/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/samestrin/llm-tools/releases/tag/v1.0.0
