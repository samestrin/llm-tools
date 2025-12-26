# llm-tools

High-performance CLI tools for LLM-assisted development workflows.

## Overview

This repository contains two powerful CLI tools written in Go:

- **llm-support**: 32+ commands for file operations, search, code analysis, and LLM integration
- **llm-clarification**: Clarification tracking and management for software projects

Both tools feature single-binary distribution, native concurrency, and fast startup times.

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

| Command | Description | Example |
|---------|-------------|---------|
| `listdir` | List directory contents with filtering | `llm-support listdir --path src/ --sizes --dates` |
| `tree` | Display directory tree structure | `llm-support tree --path src/ --depth 3` |
| `catfiles` | Concatenate multiple files | `llm-support catfiles src/ --max-size 5` |
| `hash` | Calculate file checksums | `llm-support hash file.txt -a sha256` |
| `stats` | Show directory/file statistics | `llm-support stats --path ./project` |

### Search

| Command | Description | Example |
|---------|-------------|---------|
| `grep` | Search file contents with regex | `llm-support grep "TODO" src/ -i -n` |
| `multigrep` | Search multiple keywords in parallel | `llm-support multigrep --path src/ --keywords "fn1,fn2"` |
| `multiexists` | Check if multiple files exist | `llm-support multiexists config.json README.md` |

### Code Analysis

| Command | Description | Example |
|---------|-------------|---------|
| `detect` | Detect project type and stack | `llm-support detect --path ./project` |
| `discover-tests` | Find test frameworks and patterns | `llm-support discover-tests --path ./project` |
| `analyze-deps` | Extract file dependencies from markdown | `llm-support analyze-deps story.md` |
| `partition-work` | Group tasks by file conflicts | `llm-support partition-work --stories ./user-stories/` |

### Data Processing

| Command | Description | Example |
|---------|-------------|---------|
| `json` | Query JSON with JSONPath | `llm-support json query data.json ".users[0].name"` |
| `toml` | Query TOML files | `llm-support toml get config.toml database.host` |
| `markdown` | Parse and query markdown | `llm-support markdown toc README.md` |
| `extract` | Extract URLs, emails, IPs, etc. | `llm-support extract urls file.txt` |
| `transform` | Text transformations | `llm-support transform case "myText" --to snake_case` |
| `count` | Count lines, words, checkboxes | `llm-support count --mode checkboxes --path plan.md` |
| `encode` | Base64/URL encoding/decoding | `llm-support encode "hello" -e base64` |
| `math` | Evaluate mathematical expressions | `llm-support math "2**10 + 5"` |

### LLM Integration

| Command | Description | Example |
|---------|-------------|---------|
| `prompt` | Execute LLM prompts with templates | `llm-support prompt --prompt "Explain this code" --llm gemini` |
| `foreach` | Batch process files with LLM | `llm-support foreach --glob "src/*.go" --template review.md --output-dir ./out` |
| `extract-relevant` | Extract relevant content with LLM | `llm-support extract-relevant docs/ --context "API endpoints"` |
| `summarize-dir` | Generate directory summaries | `llm-support summarize-dir src/ --format outline` |

### Development

| Command | Description | Example |
|---------|-------------|---------|
| `validate` | Validate JSON/YAML/TOML files | `llm-support validate config.json` |
| `validate-plan` | Validate sprint plans | `llm-support validate-plan --path ./sprint-01/` |
| `template` | Process text templates | `llm-support template file.txt --var name=John` |
| `diff` | Compare files | `llm-support diff file1.txt file2.txt` |
| `report` | Generate status reports | `llm-support report --title "Build" --status success` |
| `git-context` | Get git context information | `llm-support git-context` |
| `repo-root` | Find git repository root | `llm-support repo-root --validate` |

## llm-clarification Commands

| Command | Description | Example |
|---------|-------------|---------|
| `init-tracking` | Initialize clarification tracking | `llm-clarification init-tracking -o clarifications.yaml` |
| `add-clarification` | Add new clarification entry | `llm-clarification add-clarification -f tracking.yaml -q "Question?" -a "Answer"` |
| `list-entries` | List all clarifications | `llm-clarification list-entries -f tracking.yaml` |
| `validate-clarifications` | Validate clarification file | `llm-clarification validate-clarifications -f tracking.yaml` |
| `detect-conflicts` | Detect conflicting clarifications | `llm-clarification detect-conflicts -f tracking.yaml` |
| `normalize-clarification` | Normalize clarification entries | `llm-clarification normalize-clarification -f tracking.yaml` |
| `suggest-consolidation` | Merge duplicate entries | `llm-clarification suggest-consolidation -f tracking.yaml` |
| `identify-candidates` | Find candidate clarifications | `llm-clarification identify-candidates -f tracking.yaml` |
| `cluster-clarifications` | Group related clarifications | `llm-clarification cluster-clarifications -f tracking.yaml` |
| `match-clarification` | Match clarifications to context | `llm-clarification match-clarification -f tracking.yaml -q "Question?"` |
| `promote-clarification` | Promote clarification to spec | `llm-clarification promote-clarification -f tracking.yaml -i ID` |

## Common One-Liners

```bash
# Find all TODOs and FIXMEs
llm-support grep "TODO|FIXME" . -i -n

# Show project structure (3 levels deep)
llm-support tree --path . --depth 3

# Search for multiple function definitions
llm-support multigrep --path src/ --keywords "handleSubmit,validateForm,useAuth" -d

# Get first user from API response
llm-support json query response.json ".users[0]"

# Calculate percentage
llm-support math "round(42/100 * 75, 2)"

# Generate from template
llm-support template config.tpl --var domain=example.com --var port=8080

# Hash all Go files
llm-support hash internal/**/*.go -a sha256

# Count completed tasks in a sprint plan
llm-support count --mode checkboxes --path sprint/plan.md -r

# Detect project stack
llm-support detect --path .

# Validate all config files
llm-support validate config.json settings.yaml

# Compare two files
llm-support diff old-config.json new-config.json

# Find git repository root
llm-support repo-root --validate
```

## MCP Integration

Both tools include MCP (Model Context Protocol) servers for integration with Claude Desktop and other MCP-compatible clients.

See [docs/MCP_SETUP.md](docs/MCP_SETUP.md) for setup instructions.

**Available MCP Tools:**
- `llm-support-mcp`: 18 tools for file operations, search, and analysis
- `llm-clarification-mcp`: 8 tools for clarification tracking

## Performance

Measured on llm-interface (21k files, 459MB):

| Operation | Time |
|-----------|------|
| Startup | 6ms |
| MCP Server Startup | 4ms |
| detect | 6ms |
| tree (depth 3) | 22ms |
| listdir | 42ms |
| grep | 13ms |
| multigrep (5 keywords) | 13ms |
| hash | 6ms |
| count | 6ms |

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `LLM_SUPPORT_LLM_BINARY` | Path to LLM CLI binary (default: auto-detect) |
| `OPENAI_API_KEY` | OpenAI API key for LLM commands |
| `OPENAI_BASE_URL` | Custom API base URL (e.g., OpenRouter) |
| `OPENAI_MODEL` | Model to use (default: gpt-4o-mini) |

### API Key Configuration

For LLM-powered commands, you can configure the API key in several ways:

1. Environment variable: `OPENAI_API_KEY`
2. File: `.planning/.config/openai_api_key`
3. Command-line: `--api-key` flag

## Documentation

- [Quick Reference](docs/quick-reference.md) - Command cheat sheet
- [MCP Setup Guide](docs/MCP_SETUP.md) - Claude Desktop integration
- [llm-support Commands](docs/llm-support-commands.md) - Detailed command reference
- [llm-clarification Commands](docs/llm-clarification-commands.md) - Clarification system guide
- [CHANGELOG](CHANGELOG.md) - Version history

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

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please ensure:

1. Tests pass: `make test-race`
2. Code is formatted: `make fmt`
3. Linter passes: `make lint`

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [ripgrep](https://github.com/BurntSushi/ripgrep) - Fast search (used by grep/multigrep)
