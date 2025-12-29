package filesystem

import (
	"fmt"
)

// ExecuteHandler routes tool calls to their implementations
func (s *Server) ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	switch toolName {
	case "fast_list_allowed_directories":
		return s.handleListAllowedDirectories(args)
	case "fast_read_file":
		return s.handleReadFile(args)
	case "fast_read_multiple_files":
		return s.handleReadMultipleFiles(args)
	case "fast_write_file":
		return s.handleWriteFile(args)
	case "fast_large_write_file":
		return s.handleLargeWriteFile(args)
	case "fast_list_directory":
		return s.handleListDirectory(args)
	case "fast_get_file_info":
		return s.handleGetFileInfo(args)
	case "fast_create_directory":
		return s.handleCreateDirectory(args)
	case "fast_search_files":
		return s.handleSearchFiles(args)
	case "fast_search_code":
		return s.handleSearchCode(args)
	case "fast_get_directory_tree":
		return s.handleGetDirectoryTree(args)
	case "fast_edit_block":
		return s.handleEditBlock(args)
	case "fast_safe_edit":
		return s.handleSafeEdit(args)
	case "fast_edit_multiple_blocks":
		return s.handleEditMultipleBlocks(args)
	case "fast_edit_blocks":
		return s.handleEditBlocks(args)
	case "fast_extract_lines":
		return s.handleExtractLines(args)
	case "fast_copy_file":
		return s.handleCopyFile(args)
	case "fast_move_file":
		return s.handleMoveFile(args)
	case "fast_delete_file":
		return s.handleDeleteFile(args)
	case "fast_batch_file_operations":
		return s.handleBatchFileOperations(args)
	case "fast_get_disk_usage":
		return s.handleGetDiskUsage(args)
	case "fast_find_large_files":
		return s.handleFindLargeFiles(args)
	case "fast_compress_files":
		return s.handleCompressFiles(args)
	case "fast_extract_archive":
		return s.handleExtractArchive(args)
	case "fast_sync_directories":
		return s.handleSyncDirectories(args)
	case "fast_edit_file":
		return s.handleEditFile(args)
	case "fast_search_and_replace":
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

func (s *Server) handleSearchFiles(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_search_files")
}

func (s *Server) handleSearchCode(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_search_code")
}

func (s *Server) handleEditBlock(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_edit_block")
}

func (s *Server) handleSafeEdit(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_safe_edit")
}

func (s *Server) handleEditMultipleBlocks(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_edit_multiple_blocks")
}

func (s *Server) handleEditBlocks(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_edit_blocks")
}

// handleExtractLines - implemented in read.go

func (s *Server) handleCopyFile(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_copy_file")
}

func (s *Server) handleMoveFile(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_move_file")
}

func (s *Server) handleDeleteFile(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_delete_file")
}

func (s *Server) handleBatchFileOperations(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_batch_file_operations")
}

func (s *Server) handleGetDiskUsage(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_get_disk_usage")
}

func (s *Server) handleFindLargeFiles(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_find_large_files")
}

func (s *Server) handleCompressFiles(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_compress_files")
}

func (s *Server) handleExtractArchive(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_extract_archive")
}

func (s *Server) handleSyncDirectories(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_sync_directories")
}

func (s *Server) handleEditFile(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_edit_file")
}

func (s *Server) handleSearchAndReplace(args map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not implemented: fast_search_and_replace")
}
