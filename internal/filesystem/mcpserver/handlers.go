package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BinaryPath is the path to the llm-filesystem binary
var BinaryPath = "/usr/local/bin/llm-filesystem"

// AllowedDirs holds the allowed directories for path validation
var AllowedDirs []string

// CommandTimeout is the default timeout for command execution
var CommandTimeout = 60 * time.Second

// ExecuteHandler executes the appropriate command for a tool
func ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	// Strip prefix to get command name
	cmdName := strings.TrimPrefix(toolName, ToolPrefix)

	// Build command args
	cmdArgs, err := buildArgs(cmdName, args)
	if err != nil {
		return "", err
	}

	// Add --json flag for machine-parseable output
	cmdArgs = append(cmdArgs, "--json")

	// Add allowed-dirs if configured
	if len(AllowedDirs) > 0 {
		for _, dir := range AllowedDirs {
			cmdArgs = append(cmdArgs, "--allowed-dirs", dir)
		}
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, BinaryPath, cmdArgs...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command timed out after %v", CommandTimeout)
	}

	if err != nil {
		// Return output even on error (may contain useful error message)
		if len(output) > 0 {
			return string(output), nil
		}
		return "", fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// buildArgs builds CLI arguments for the given tool
func buildArgs(cmdName string, args map[string]interface{}) ([]string, error) {
	switch cmdName {
	case "read_file":
		return buildReadFileArgs(args), nil
	case "read_multiple_files":
		return buildReadMultipleFilesArgs(args), nil
	case "write_file":
		return buildWriteFileArgs(args), nil
	case "large_write_file":
		return buildLargeWriteFileArgs(args), nil
	case "get_file_info":
		return buildGetFileInfoArgs(args), nil
	case "create_directory":
		return buildCreateDirectoryArgs(args), nil
	case "list_directory":
		return buildListDirectoryArgs(args), nil
	case "get_directory_tree":
		return buildGetDirectoryTreeArgs(args), nil
	case "search_files":
		return buildSearchFilesArgs(args), nil
	case "search_code":
		return buildSearchCodeArgs(args), nil
	case "edit_block":
		return buildEditBlockArgs(args), nil
	case "edit_blocks":
		return buildEditBlocksArgs(args), nil
	case "safe_edit":
		return buildSafeEditArgs(args), nil
	case "edit_file":
		return buildEditFileArgs(args), nil
	case "search_and_replace":
		return buildSearchAndReplaceArgs(args), nil
	case "extract_lines":
		return buildExtractLinesArgs(args), nil
	case "copy_file":
		return buildCopyFileArgs(args), nil
	case "move_file":
		return buildMoveFileArgs(args), nil
	case "delete_file":
		return buildDeleteFileArgs(args), nil
	case "batch_file_operations":
		return buildBatchFileOperationsArgs(args), nil
	case "get_disk_usage":
		return buildGetDiskUsageArgs(args), nil
	case "find_large_files":
		return buildFindLargeFilesArgs(args), nil
	case "compress_files":
		return buildCompressFilesArgs(args), nil
	case "extract_archive":
		return buildExtractArchiveArgs(args), nil
	case "sync_directories":
		return buildSyncDirectoriesArgs(args), nil
	case "list_allowed_directories":
		return []string{"list-allowed-directories"}, nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdName)
	}
}

func buildReadFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"read-file"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if offset, ok := getInt(args, "start_offset"); ok {
		cmdArgs = append(cmdArgs, "--offset", strconv.Itoa(offset))
	}
	if maxSize, ok := getInt(args, "max_size"); ok {
		cmdArgs = append(cmdArgs, "--max-size", strconv.Itoa(maxSize))
	}
	if lineStart, ok := getInt(args, "line_start"); ok {
		cmdArgs = append(cmdArgs, "--line-start", strconv.Itoa(lineStart))
	}
	if lineCount, ok := getInt(args, "line_count"); ok {
		cmdArgs = append(cmdArgs, "--line-count", strconv.Itoa(lineCount))
	}
	return cmdArgs
}

func buildReadMultipleFilesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"read-multiple-files"}
	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				cmdArgs = append(cmdArgs, "--paths", s)
			}
		}
	}
	return cmdArgs
}

func buildWriteFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"write-file"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if content, ok := args["content"].(string); ok {
		cmdArgs = append(cmdArgs, "--content", content)
	}
	if getBool(args, "append") {
		cmdArgs = append(cmdArgs, "--append")
	}
	return cmdArgs
}

func buildLargeWriteFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"large-write-file"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if content, ok := args["content"].(string); ok {
		cmdArgs = append(cmdArgs, "--content", content)
	}
	if !getBoolDefault(args, "backup", true) {
		cmdArgs = append(cmdArgs, "--backup=false")
	}
	if !getBoolDefault(args, "verify", true) {
		cmdArgs = append(cmdArgs, "--verify=false")
	}
	return cmdArgs
}

func buildGetFileInfoArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"get-file-info"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	return cmdArgs
}

func buildCreateDirectoryArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"create-directory"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if !getBoolDefault(args, "recursive", true) {
		cmdArgs = append(cmdArgs, "--recursive=false")
	}
	return cmdArgs
}

func buildListDirectoryArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"list-directory"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBool(args, "show_hidden") {
		cmdArgs = append(cmdArgs, "--show-hidden")
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	if sortBy, ok := args["sort_by"].(string); ok {
		cmdArgs = append(cmdArgs, "--sort-by", sortBy)
	}
	if getBool(args, "reverse") {
		cmdArgs = append(cmdArgs, "--reverse")
	}
	if page, ok := getInt(args, "page"); ok {
		cmdArgs = append(cmdArgs, "--page", strconv.Itoa(page))
	}
	if pageSize, ok := getInt(args, "page_size"); ok {
		cmdArgs = append(cmdArgs, "--page-size", strconv.Itoa(pageSize))
	}
	return cmdArgs
}

func buildGetDirectoryTreeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"get-directory-tree"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if maxDepth, ok := getInt(args, "max_depth"); ok {
		cmdArgs = append(cmdArgs, "--depth", strconv.Itoa(maxDepth))
	}
	if getBool(args, "show_hidden") {
		cmdArgs = append(cmdArgs, "--show-hidden")
	}
	if getBool(args, "include_files") {
		cmdArgs = append(cmdArgs, "--include-files")
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	return cmdArgs
}

func buildSearchFilesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"search-files"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	if !getBoolDefault(args, "recursive", true) {
		cmdArgs = append(cmdArgs, "--recursive=false")
	}
	if getBool(args, "show_hidden") {
		cmdArgs = append(cmdArgs, "--show-hidden")
	}
	if maxResults, ok := getInt(args, "max_results"); ok {
		cmdArgs = append(cmdArgs, "--max-results", strconv.Itoa(maxResults))
	}
	return cmdArgs
}

func buildSearchCodeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"search-code"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	if getBool(args, "ignore_case") {
		cmdArgs = append(cmdArgs, "--ignore-case")
	}
	if getBool(args, "regex") {
		cmdArgs = append(cmdArgs, "--regex")
	}
	if context, ok := getInt(args, "context"); ok {
		cmdArgs = append(cmdArgs, "--context", strconv.Itoa(context))
	}
	if fileTypes, ok := args["file_types"].([]interface{}); ok {
		for _, ft := range fileTypes {
			if s, ok := ft.(string); ok {
				cmdArgs = append(cmdArgs, "--file-types", s)
			}
		}
	}
	if maxResults, ok := getInt(args, "max_results"); ok {
		cmdArgs = append(cmdArgs, "--max-results", strconv.Itoa(maxResults))
	}
	if getBool(args, "show_hidden") {
		cmdArgs = append(cmdArgs, "--show-hidden")
	}
	return cmdArgs
}

func buildEditBlockArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"edit-block"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if old, ok := args["old_string"].(string); ok {
		cmdArgs = append(cmdArgs, "--old", old)
	}
	if new, ok := args["new_string"].(string); ok {
		cmdArgs = append(cmdArgs, "--new", new)
	}
	return cmdArgs
}

func buildEditBlocksArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"edit-blocks"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if edits, ok := args["edits"].([]interface{}); ok {
		editsJSON, _ := json.Marshal(edits)
		cmdArgs = append(cmdArgs, "--edits", string(editsJSON))
	}
	return cmdArgs
}

func buildSafeEditArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"safe-edit"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if old, ok := args["old_string"].(string); ok {
		cmdArgs = append(cmdArgs, "--old", old)
	}
	if new, ok := args["new_string"].(string); ok {
		cmdArgs = append(cmdArgs, "--new", new)
	}
	if !getBoolDefault(args, "backup", true) {
		cmdArgs = append(cmdArgs, "--backup=false")
	}
	if getBool(args, "dry_run") {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	return cmdArgs
}

func buildEditFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"edit-file"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if op, ok := args["operation"].(string); ok {
		cmdArgs = append(cmdArgs, "--operation", op)
	}
	if line, ok := getInt(args, "line"); ok {
		cmdArgs = append(cmdArgs, "--line", strconv.Itoa(line))
	}
	if content, ok := args["content"].(string); ok {
		cmdArgs = append(cmdArgs, "--content", content)
	}
	return cmdArgs
}

func buildSearchAndReplaceArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"search-and-replace"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	if replacement, ok := args["replacement"].(string); ok {
		cmdArgs = append(cmdArgs, "--replacement", replacement)
	}
	if getBool(args, "regex") {
		cmdArgs = append(cmdArgs, "--regex")
	}
	if getBool(args, "dry_run") {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	if fileTypes, ok := args["file_types"].([]interface{}); ok {
		for _, ft := range fileTypes {
			if s, ok := ft.(string); ok {
				cmdArgs = append(cmdArgs, "--file-types", s)
			}
		}
	}
	return cmdArgs
}

func buildExtractLinesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"extract-lines"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if start, ok := getInt(args, "start"); ok {
		cmdArgs = append(cmdArgs, "--start", strconv.Itoa(start))
	}
	if end, ok := getInt(args, "end"); ok {
		cmdArgs = append(cmdArgs, "--end", strconv.Itoa(end))
	}
	if lines, ok := args["lines"].([]interface{}); ok {
		for _, l := range lines {
			if n, ok := l.(float64); ok {
				cmdArgs = append(cmdArgs, "--lines", strconv.Itoa(int(n)))
			}
		}
	}
	return cmdArgs
}

func buildCopyFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"copy-file"}
	if source, ok := args["source"].(string); ok {
		cmdArgs = append(cmdArgs, "--source", source)
	}
	if dest, ok := args["destination"].(string); ok {
		cmdArgs = append(cmdArgs, "--dest", dest)
	}
	return cmdArgs
}

func buildMoveFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"move-file"}
	if source, ok := args["source"].(string); ok {
		cmdArgs = append(cmdArgs, "--source", source)
	}
	if dest, ok := args["destination"].(string); ok {
		cmdArgs = append(cmdArgs, "--dest", dest)
	}
	return cmdArgs
}

func buildDeleteFileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"delete-file"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBool(args, "recursive") {
		cmdArgs = append(cmdArgs, "--recursive")
	}
	return cmdArgs
}

func buildBatchFileOperationsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"batch-file-operations"}
	if ops, ok := args["operations"].([]interface{}); ok {
		opsJSON, _ := json.Marshal(ops)
		cmdArgs = append(cmdArgs, "--operations", string(opsJSON))
	}
	return cmdArgs
}

func buildGetDiskUsageArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"get-disk-usage"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	return cmdArgs
}

func buildFindLargeFilesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"find-large-files"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if minSize, ok := getInt(args, "min_size"); ok {
		cmdArgs = append(cmdArgs, "--min-size", strconv.Itoa(minSize))
	}
	if limit, ok := getInt(args, "limit"); ok {
		cmdArgs = append(cmdArgs, "--limit", strconv.Itoa(limit))
	}
	return cmdArgs
}

func buildCompressFilesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"compress-files"}
	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				cmdArgs = append(cmdArgs, "--paths", s)
			}
		}
	}
	if output, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", output)
	}
	if format, ok := args["format"].(string); ok {
		cmdArgs = append(cmdArgs, "--format", format)
	}
	return cmdArgs
}

func buildExtractArchiveArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"extract-archive"}
	if archive, ok := args["archive"].(string); ok {
		cmdArgs = append(cmdArgs, "--archive", archive)
	}
	if dest, ok := args["destination"].(string); ok {
		cmdArgs = append(cmdArgs, "--dest", dest)
	}
	return cmdArgs
}

func buildSyncDirectoriesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"sync-directories"}
	if source, ok := args["source"].(string); ok {
		cmdArgs = append(cmdArgs, "--source", source)
	}
	if dest, ok := args["destination"].(string); ok {
		cmdArgs = append(cmdArgs, "--dest", dest)
	}
	return cmdArgs
}

// Helper functions

func getBool(args map[string]interface{}, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func getBoolDefault(args map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getInt(args map[string]interface{}, key string) (int, bool) {
	switch v := args[key].(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	}
	return 0, false
}
