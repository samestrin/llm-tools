# llm-tools

High-performance CLI tools for LLM-assisted development workflows.

## Overview

This repository contains two powerful CLI tools written in Go:

- **llm-support**: Multi-purpose CLI for file operations, search, and LLM integration
- **llm-clarification**: Clarification tracking and management for software projects

Both tools are designed to be 10-20x faster than their Python predecessors, with single-binary distribution and native concurrency.

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/samestrin/llm-tools/releases) page.

Available platforms:
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

### From Source

Requires Go 1.22 or later.

```bash
go install github.com/samestrin/llm-tools/cmd/llm-support@latest
go install github.com/samestrin/llm-tools/cmd/llm-clarification@latest
```

### Build from Source

```bash
git clone https://github.com/samestrin/llm-tools.git
cd llm-tools
make build
# Binaries will be in ./build/
```

## llm-support Commands

### File Operations
| Command | Description |
|---------|-------------|
| `listdir` | List directory contents with filtering |
| `tree` | Display directory tree structure |
| `catfiles` | Concatenate multiple files |
| `hash` | Calculate file checksums (SHA256, MD5, SHA1) |
| `stats` | Show directory/file statistics |

### Search
| Command | Description |
|---------|-------------|
| `grep` | Search file contents with regex |
| `multigrep` | Search multiple keywords in parallel |
| `multiexists` | Check if multiple files exist |

### Code Analysis
| Command | Description |
|---------|-------------|
| `detect` | Detect project type and stack |
| `discover-tests` | Find test frameworks and patterns |
| `analyze-deps` | Extract file dependencies from markdown |
| `partition-work` | Group tasks by file conflicts |

### Data Processing
| Command | Description |
|---------|-------------|
| `json` | Query JSON with JSONPath |
| `toml` | Query TOML files |
| `markdown` | Parse and query markdown |
| `extract` | Extract URLs, emails, IPs, etc. |
| `transform` | Text transformations (case, trim, etc.) |
| `count` | Count lines, words, checkboxes |
| `encode` | Base64/URL encoding/decoding |

### LLM Integration
| Command | Description |
|---------|-------------|
| `prompt` | Execute LLM prompts with templates |
| `foreach` | Batch process files with LLM |
| `extract-relevant` | Extract relevant content with LLM |
| `summarize-dir` | Generate directory summaries |

### Development
| Command | Description |
|---------|-------------|
| `validate` | Validate JSON/YAML/TOML files |
| `validate-plan` | Validate sprint plans |
| `template` | Process text templates |
| `diff` | Compare files |
| `report` | Generate reports from data |

## llm-clarification Commands

| Command | Description |
|---------|-------------|
| `init` | Initialize clarification tracking |
| `add` | Add new clarification entry |
| `list` | List all clarifications |
| `validate` | Validate clarification file |
| `conflicts` | Detect conflicting clarifications |
| `normalize` | Normalize clarification entries |
| `consolidate` | Merge duplicate entries |
| `candidates` | Find candidate clarifications |
| `cluster` | Group related clarifications |
| `match` | Match clarifications to context |
| `promote` | Promote clarification to spec |

## Performance

llm-tools achieves significant performance improvements over Python versions:

| Operation | llm-tools (Go) | Python | Improvement |
|-----------|---------------|--------|-------------|
| Startup | ~7ms | ~200ms | ~30x |
| listdir (1000 files) | 70us | 15ms | ~200x |
| tree (depth 5) | 88ms | 200ms | ~2x |
| hash (1MB) | 0.4ms | 5ms | ~12x |

## Development

### Running Tests

```bash
# Run all tests
make test

# Run with race detector
make test-race

# Run with coverage
make test-cover

# Run benchmarks
go test -bench=. ./...
```

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Clean build artifacts
make clean
```

### Code Quality

```bash
# Run linter
make lint

# Format code
make fmt
```

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `LLM_SUPPORT_LLM_BINARY` | Path to LLM CLI binary (default: auto-detect) |
| `OPENAI_API_KEY` | OpenAI API key for LLM commands |

### API Key Configuration

For LLM-powered commands, you can configure the API key in several ways:

1. Environment variable: `OPENAI_API_KEY`
2. File: `.planning/.config/openai_api_key`
3. Command-line: `--api-key` flag

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please ensure:

1. Tests pass: `make test-race`
2. Code is formatted: `make fmt`
3. Linter passes: `make lint`

## Acknowledgments

- Original Python implementation
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [ripgrep](https://github.com/BurntSushi/ripgrep) - Fast search (used by grep/multigrep)
