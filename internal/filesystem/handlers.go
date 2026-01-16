package filesystem

import (
	"fmt"
)

// ExecuteHandler routes tool calls to their implementations
// NOTE: This legacy server exposes 15 tools. Single-file operations
// should use Claude's native Read, Write, and Edit tools for better performance.
func (s *Server) ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	// Batch Reading
	case "llm_filesystem_read_multiple_files":
		return s.handleReadMultipleFiles(args)
	case "llm_filesystem_extract_lines":
		return s.handleExtractLines(args)

	// Batch Editing
	case "llm_filesystem_edit_blocks":
		return s.handleEditBlocks(args)
	case "llm_filesystem_search_and_replace":
		return s.handleSearchAndReplace(args)

	// Directory Operations
	case "llm_filesystem_list_directory":
		return s.handleListDirectory(args)
	case "llm_filesystem_get_directory_tree":
		return s.handleGetDirectoryTree(args)
	case "llm_filesystem_create_directories":
		return s.handleCreateDirectories(args)

	// Search Operations
	case "llm_filesystem_search_files":
		return s.handleSearchFiles(args)
	case "llm_filesystem_search_code":
		return s.handleSearchCode(args)

	// File Operations
	case "llm_filesystem_copy_file":
		return s.handleCopyFile(args)
	case "llm_filesystem_move_file":
		return s.handleMoveFile(args)
	case "llm_filesystem_delete_file":
		return s.handleDeleteFile(args)
	case "llm_filesystem_batch_file_operations":
		return s.handleBatchFileOperations(args)

	// Archive Operations
	case "llm_filesystem_compress_files":
		return s.handleCompressFiles(args)
	case "llm_filesystem_extract_archive":
		return s.handleExtractArchive(args)

	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Handler implementations are in separate files:
// handleReadMultipleFiles - implemented in read.go
// handleExtractLines - implemented in read.go
// handleEditBlocks - implemented in edit.go
// handleSearchAndReplace - implemented in edit.go
// handleListDirectory - implemented in directory.go
// handleGetDirectoryTree - implemented in directory.go
// handleCreateDirectories - implemented in write.go
// handleSearchFiles - implemented in search.go
// handleSearchCode - implemented in search.go
// handleCopyFile - implemented in fileops.go
// handleMoveFile - implemented in fileops.go
// handleDeleteFile - implemented in fileops.go
// handleBatchFileOperations - implemented in fileops.go
// handleCompressFiles - implemented in advanced.go
// handleExtractArchive - implemented in advanced.go
