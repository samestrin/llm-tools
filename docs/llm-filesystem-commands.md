# llm-filesystem Commands

High-performance filesystem operations CLI with 26 commands for reading, writing, editing, and managing files.

## Installation

```bash
go install github.com/samestrin/llm-tools/cmd/llm-filesystem@latest
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON (machine-parseable) |
| `--allowed-dirs` | Directories the tool is allowed to access |

## Commands

### Reading Files

| Command | Description | Example |
|---------|-------------|---------|
| `read-file` | Read a file | `llm-filesystem read-file --path /tmp/test.txt --json` |
| `read-multiple-files` | Read multiple files | `llm-filesystem read-multiple-files --paths file1.txt --paths file2.txt` |
| `extract-lines` | Extract specific lines | `llm-filesystem extract-lines --path file.txt --start 10 --end 20` |

### Writing Files

| Command | Description | Example |
|---------|-------------|---------|
| `write-file` | Write content to file | `llm-filesystem write-file --path /tmp/out.txt --content "hello"` |
| `large-write-file` | Write large files with backup | `llm-filesystem large-write-file --path large.txt --content "..."` |
| `get-file-info` | Get file metadata | `llm-filesystem get-file-info --path /tmp/test.txt` |
| `create-directory` | Create a directory | `llm-filesystem create-directory --path /tmp/newdir` |

### Editing Files

| Command | Description | Example |
|---------|-------------|---------|
| `edit-block` | Replace text block | `llm-filesystem edit-block --path file.txt --old "foo" --new "bar"` |
| `edit-blocks` | Multiple replacements | `llm-filesystem edit-blocks --path file.txt --edits '[{"old":"a","new":"b"}]'` |
| `safe-edit` | Edit with backup | `llm-filesystem safe-edit --path file.txt --old "x" --new "y" --backup` |
| `edit-file` | Line-based editing | `llm-filesystem edit-file --path file.txt --operation insert --line 5 --content "new line"` |
| `search-and-replace` | Regex replace across files | `llm-filesystem search-and-replace --path ./src --pattern "old" --replacement "new"` |

### Directory Operations

| Command | Description | Example |
|---------|-------------|---------|
| `list-directory` | List directory contents | `llm-filesystem list-directory --path /tmp --json` |
| `get-directory-tree` | Get directory tree | `llm-filesystem get-directory-tree --path /tmp --max-depth 3` |

### Search Operations

| Command | Description | Example |
|---------|-------------|---------|
| `search-files` | Search files by name | `llm-filesystem search-files --path ./src --pattern "*.go"` |
| `search-code` | Search file contents | `llm-filesystem search-code --path ./src --pattern "TODO" --json` |

### File Operations

| Command | Description | Example |
|---------|-------------|---------|
| `copy-file` | Copy file/directory | `llm-filesystem copy-file --source a.txt --dest b.txt` |
| `move-file` | Move/rename file | `llm-filesystem move-file --source old.txt --dest new.txt` |
| `delete-file` | Delete file/directory | `llm-filesystem delete-file --path /tmp/old --recursive` |
| `batch-file-operations` | Batch operations | `llm-filesystem batch-file-operations --operations '[...]'` |

### Advanced Operations

| Command | Description | Example |
|---------|-------------|---------|
| `get-disk-usage` | Get disk usage stats | `llm-filesystem get-disk-usage --path /tmp` |
| `find-large-files` | Find files over size | `llm-filesystem find-large-files --path ./src --min-size "1MB"` |
| `compress-files` | Create archive | `llm-filesystem compress-files --paths file1.txt --paths file2.txt --output archive.zip` |
| `extract-archive` | Extract archive | `llm-filesystem extract-archive --archive file.zip --dest /tmp/extracted` |
| `sync-directories` | Sync directories | `llm-filesystem sync-directories --source /src --dest /backup` |
| `list-allowed-directories` | Show allowed dirs | `llm-filesystem list-allowed-directories` |

## MCP Integration

The MCP wrapper (`llm-filesystem-mcp`) exposes **15 batch/specialized tools** with the `llm_filesystem_` prefix. Single-file operations should use Claude's native Read, Write, and Edit tools for better performance.

**Batch Reading:**
- `llm_filesystem_read_multiple_files` - Read multiple files simultaneously
- `llm_filesystem_extract_lines` - Extract specific line ranges

**Batch Editing:**
- `llm_filesystem_edit_blocks` - Multiple replacements in one file
- `llm_filesystem_search_and_replace` - Regex replace across files

**Directory Operations:**
- `llm_filesystem_list_directory` - List with filtering/pagination
- `llm_filesystem_get_directory_tree` - Get directory tree structure
- `llm_filesystem_create_directories` - Create multiple directories

**Search Operations:**
- `llm_filesystem_search_files` - Search files by name pattern
- `llm_filesystem_search_code` - Search patterns in file contents

**File Management:**
- `llm_filesystem_copy_file` - Copy file or directory
- `llm_filesystem_move_file` - Move or rename file/directory
- `llm_filesystem_delete_file` - Delete file or directory
- `llm_filesystem_batch_file_operations` - Batch copy/move/delete

**Archive Operations:**
- `llm_filesystem_compress_files` - Compress files into archive
- `llm_filesystem_extract_archive` - Extract an archive

**Note:** The CLI still exposes all 26 commands. The MCP reduction removes tools that duplicate Claude's native capabilities.

See [MCP Setup Guide](MCP_SETUP.md) for integration instructions.

## API Parity with fast-filesystem

llm-filesystem is designed as a drop-in replacement for fast-filesystem MCP. Key features:

- **Output Structure**: Uses `items` instead of `entries`, `tree` instead of `root`
- **File Info**: Includes `type` ("file"/"directory"), `size_readable`, `permissions`, `extension`, `mime_type`
- **Access Checks**: Provides `is_readable` and `is_writable` fields
- **Search Results**: Includes `context_before`, `context_after`, `ripgrep_used`, `search_time_ms`
- **Pagination**: Supports `continuation_token` for large results
- **Filtering**: Respects `.gitignore` patterns in `find-large-files`

See [Migration Guide](llm-filesystem-migration.md) for details on migrating from fast-filesystem.
