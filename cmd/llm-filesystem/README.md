# llm-filesystem

High-performance MCP filesystem server for Claude Code. A Go port of [fast-filesystem-mcp](https://github.com/anthropics/fast-filesystem-mcp) with significant performance improvements.

## Features

- **27 MCP tools** for file operations, editing, searching, and archiving
- **10-30x faster cold start** compared to Node.js/TypeScript version
- **3-5x lower memory usage**
- **Drop-in replacement** for fast-filesystem-mcp

## Installation

### From Source

```bash
go install github.com/samestrin/llm-tools/cmd/llm-filesystem@latest
```

### From Releases

Download the appropriate binary for your platform from the [releases page](https://github.com/samestrin/llm-tools/releases).

## Usage

### Command Line

```bash
# Run with allowed directories
llm-filesystem --allowed-dirs ~/projects --allowed-dirs /tmp

# Multiple directories via comma-separated list
llm-filesystem --allowed-dirs ~/projects,/tmp,/var/log
```

### Claude Desktop Configuration

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/path/to/llm-filesystem",
      "args": ["--allowed-dirs", "~/projects"]
    }
  }
}
```

## Available Tools

### Core File Operations
| Tool | Description |
|------|-------------|
| `fast_read_file` | Read file with auto-chunking support |
| `fast_read_multiple_files` | Read multiple files simultaneously |
| `fast_write_file` | Write or modify files |
| `fast_large_write_file` | Reliable large file writes with streaming |

### Directory Operations
| Tool | Description |
|------|-------------|
| `fast_list_directory` | List directory contents with pagination |
| `fast_get_directory_tree` | Get directory tree structure |
| `fast_create_directory` | Create directories recursively |

### Search Operations
| Tool | Description |
|------|-------------|
| `fast_search_files` | Search for files by name pattern |
| `fast_search_code` | Search for patterns in file contents |

### Edit Operations
| Tool | Description |
|------|-------------|
| `fast_edit_block` | Replace text in a file |
| `fast_edit_blocks` | Multiple replacements in one file |
| `fast_edit_multiple_blocks` | Edit multiple files at once |
| `fast_safe_edit` | Safe edit with backup and dry-run |
| `fast_edit_file` | Advanced file editing with line operations |
| `fast_search_and_replace` | Regex-based search and replace across files |
| `fast_extract_lines` | Extract specific line ranges |

### File Management
| Tool | Description |
|------|-------------|
| `fast_copy_file` | Copy files or directories |
| `fast_move_file` | Move or rename files |
| `fast_delete_file` | Delete files or directories |
| `fast_batch_file_operations` | Execute multiple operations atomically |

### Advanced Operations
| Tool | Description |
|------|-------------|
| `fast_get_disk_usage` | Get disk usage statistics |
| `fast_find_large_files` | Find files over a size threshold |
| `fast_compress_files` | Create zip or tar.gz archives |
| `fast_extract_archive` | Extract archives |
| `fast_sync_directories` | Sync directories |

### Info
| Tool | Description |
|------|-------------|
| `fast_get_file_info` | Get detailed file information |
| `fast_list_allowed_directories` | List configured allowed directories |

## Performance

Benchmarks on Apple M4 Pro:

| Operation | Time |
|-----------|------|
| Cold start | ~186µs |
| Read file (10KB) | ~21µs |
| Write file (10KB) | ~42µs |
| List directory (100 files) | ~65µs |
| Search code (50 files) | ~512µs |
| Edit block | ~71µs |

## Security

- All paths are validated against allowed directories
- Path traversal attacks are prevented
- Symlinks are resolved and validated
- Binary files are detected and handled appropriately

## Testing

```bash
# Run all tests
go test ./internal/filesystem/...

# Run with coverage
go test -cover ./internal/filesystem/...

# Run benchmarks
go test -bench=. ./internal/filesystem/...
```

## License

MIT License - see [LICENSE](../../LICENSE) for details.
