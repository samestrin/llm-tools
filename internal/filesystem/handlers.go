package filesystem

import (
	"fmt"
)

// ExecuteHandler routes tool calls to their implementations
func (s *Server) ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	case "llm_filesystem_list_allowed_directories":
		return s.handleListAllowedDirectories(args)
	case "llm_filesystem_read_file":
		return s.handleReadFile(args)
	case "llm_filesystem_read_multiple_files":
		return s.handleReadMultipleFiles(args)
	case "llm_filesystem_write_file":
		return s.handleWriteFile(args)
	case "llm_filesystem_large_write_file":
		return s.handleLargeWriteFile(args)
	case "llm_filesystem_list_directory":
		return s.handleListDirectory(args)
	case "llm_filesystem_get_file_info":
		return s.handleGetFileInfo(args)
	case "llm_filesystem_create_directory":
		return s.handleCreateDirectory(args)
	case "llm_filesystem_search_files":
		return s.handleSearchFiles(args)
	case "llm_filesystem_search_code":
		return s.handleSearchCode(args)
	case "llm_filesystem_get_directory_tree":
		return s.handleGetDirectoryTree(args)
	case "llm_filesystem_edit_block":
		return s.handleEditBlock(args)
	case "llm_filesystem_safe_edit":
		return s.handleSafeEdit(args)
	case "llm_filesystem_edit_multiple_blocks":
		return s.handleEditMultipleBlocks(args)
	case "llm_filesystem_edit_blocks":
		return s.handleEditBlocks(args)
	case "llm_filesystem_extract_lines":
		return s.handleExtractLines(args)
	case "llm_filesystem_copy_file":
		return s.handleCopyFile(args)
	case "llm_filesystem_move_file":
		return s.handleMoveFile(args)
	case "llm_filesystem_delete_file":
		return s.handleDeleteFile(args)
	case "llm_filesystem_batch_file_operations":
		return s.handleBatchFileOperations(args)
	case "llm_filesystem_get_disk_usage":
		return s.handleGetDiskUsage(args)
	case "llm_filesystem_find_large_files":
		return s.handleFindLargeFiles(args)
	case "llm_filesystem_compress_files":
		return s.handleCompressFiles(args)
	case "llm_filesystem_extract_archive":
		return s.handleExtractArchive(args)
	case "llm_filesystem_sync_directories":
		return s.handleSyncDirectories(args)
	case "llm_filesystem_edit_file":
		return s.handleEditFile(args)
	case "llm_filesystem_search_and_replace":
		return s.handleSearchAndReplace(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Stub handlers - to be implemented in separate files

func (s *Server) handleListAllowedDirectories(args map[string]interface{}) (string, error) {
	return fmt.Sprintf(`{"allowed_directories": %q}`, s.allowedDirs), nil
}

// handleReadFile - implemented in read.go
// handleReadMultipleFiles - implemented in read.go
// handleWriteFile - implemented in write.go
// handleLargeWriteFile - implemented in write.go
// handleGetFileInfo - implemented in write.go
// handleCreateDirectory - implemented in write.go
// handleListDirectory - implemented in directory.go
// handleGetDirectoryTree - implemented in directory.go
// handleSearchFiles - implemented in search.go
// handleSearchCode - implemented in search.go
// handleEditBlock - implemented in edit.go
// handleSafeEdit - implemented in edit.go
// handleEditMultipleBlocks - implemented in edit.go
// handleEditBlocks - implemented in edit.go
// handleExtractLines - implemented in read.go
// handleEditFile - implemented in edit.go
// handleSearchAndReplace - implemented in edit.go
// handleCopyFile - implemented in fileops.go
// handleMoveFile - implemented in fileops.go
// handleDeleteFile - implemented in fileops.go
// handleBatchFileOperations - implemented in fileops.go
// handleGetDiskUsage - implemented in advanced.go
// handleFindLargeFiles - implemented in advanced.go
// handleCompressFiles - implemented in advanced.go
// handleExtractArchive - implemented in advanced.go
// handleSyncDirectories - implemented in advanced.go
