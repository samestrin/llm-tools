# llm-filesystem Migration Guide

This guide helps you migrate from `fast-filesystem` (TypeScript MCP) to `llm-filesystem` (Go MCP).

## Overview

`llm-filesystem` is a drop-in replacement for `fast-filesystem` with 100% API compatibility. This document covers the changes made to achieve parity.

## Breaking Changes

### Parameter Renames

| Old Name | New Name | Tools Affected |
|----------|----------|----------------|
| `depth` | `max_depth` | `get_directory_tree` |
| `limit` | `max_results` | `find_large_files`, `search_files`, `search_code` |

### Parameter Type Changes

| Parameter | Old Type | New Type | Example |
|-----------|----------|----------|---------|
| `min_size` | number (bytes) | string | `"100MB"`, `"1GB"`, `"500KB"` |

The `min_size` parameter now accepts human-readable size strings:
- `"100MB"` = 100 megabytes
- `"1GB"` = 1 gigabyte
- `"500KB"` = 500 kilobytes
- `"1024"` = 1024 bytes (numeric strings still work)

### Output Key Renames

| Old Key | New Key | Response Type |
|---------|---------|---------------|
| `root` | `tree` | `get_directory_tree` |
| `entries` | `items` | `list_directory` |

### Example: list_directory

**Before (fast-filesystem):**
```json
{
  "path": "/tmp",
  "entries": [
    {"name": "file.txt", "size": 1024, "is_dir": false}
  ],
  "total": 1
}
```

**After (llm-filesystem):**
```json
{
  "path": "/tmp",
  "items": [
    {
      "name": "file.txt",
      "path": "/tmp/file.txt",
      "type": "file",
      "size": 1024,
      "size_readable": "1.0 KB",
      "is_dir": false,
      "mode": "-rw-r--r--",
      "permissions": 420,
      "modified": "2024-01-15T10:30:00Z",
      "extension": ".txt",
      "mime_type": "text/plain",
      "is_readable": true,
      "is_writable": true
    }
  ],
  "total": 1
}
```

### Example: get_directory_tree

**Before (fast-filesystem):**
```json
{
  "root": {"name": "src", "is_dir": true, "children": []},
  "total_dirs": 5,
  "total_files": 10
}
```

**After (llm-filesystem):**
```json
{
  "tree": {"name": "src", "path": "/path/to/src", "is_dir": true, "children": []},
  "total_dirs": 5,
  "total_files": 10,
  "total_size": 102400
}
```

## New Fields

### File/Directory Entries

All file and directory entries now include:

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"file"` or `"directory"` |
| `size_readable` | string | Human-readable size (e.g., `"6.70 KB"`) |
| `permissions` | number | Numeric Unix permissions (e.g., `420` = 0644) |
| `extension` | string | File extension including dot (e.g., `".go"`) |
| `mime_type` | string | MIME type (e.g., `"text/plain"`) |
| `is_readable` | boolean | Whether file is readable |
| `is_writable` | boolean | Whether file is writable |

### Search Results

Code search results now include:

| Field | Type | Description |
|-------|------|-------------|
| `context_before` | string[] | Lines before match |
| `context_after` | string[] | Lines after match |
| `ripgrep_used` | boolean | Whether ripgrep was used |
| `search_time_ms` | number | Search duration in milliseconds |

### Pagination

List operations now support pagination with:

| Field | Type | Description |
|-------|------|-------------|
| `page` | number | Current page number |
| `page_size` | number | Items per page |
| `total_pages` | number | Total number of pages |
| `has_more` | boolean | Whether more pages exist |
| `continuation_token` | string | Token for next page |

### Read File

File read results now include:

| Field | Type | Description |
|-------|------|-------------|
| `encoding` | string | File encoding (default: `"utf-8"`) |
| `auto_chunked` | boolean | Whether auto-chunking was applied |
| `chunk_index` | number | Current chunk index |
| `total_chunks` | number | Total number of chunks |

### Find Large Files

Large file results now include:

| Field | Type | Description |
|-------|------|-------------|
| `total_count` | number | Total files found (before limit) |
| `total_size` | number | Combined size of all found files |

## Feature Additions

### .gitignore Filtering

`find_large_files` now respects `.gitignore` patterns:
- Automatically skips `.git/` directories
- Loads `.gitignore` from project root
- Filters out ignored files from results

### Auto-Chunking

Large file reads are automatically chunked:
- Default chunk size: 1MB
- Use `continuation_token` to read next chunk
- `has_more` indicates if more data available

### Continuation Tokens

Stateless pagination using base64-encoded tokens:
- Works across server restarts
- Encodes path, offset, and page info
- Validated to prevent tampering

## Tool Prefix Change

MCP tools use `llm_filesystem_` prefix instead of `fast_`:

| fast-filesystem | llm-filesystem |
|-----------------|----------------|
| `fast_read_file` | `llm_filesystem_read_file` |
| `fast_write_file` | `llm_filesystem_write_file` |
| `fast_list_directory` | `llm_filesystem_list_directory` |
| `fast_get_directory_tree` | `llm_filesystem_get_directory_tree` |

## Backward Compatibility

For easier migration, some legacy parameter names are still accepted:
- `limit` works alongside `max_results`
- Numeric `min_size` values still work

## Need Help?

If you encounter issues migrating, please open an issue at:
https://github.com/samestrin/llm-tools/issues
