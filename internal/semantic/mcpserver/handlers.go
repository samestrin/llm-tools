package mcpserver

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// BinaryPath is the path to the llm-semantic binary
var BinaryPath = "/usr/local/bin/llm-semantic"

// CommandTimeout is the default timeout for command execution
// Increased to 120s for index/update operations which can be slow
var CommandTimeout = 120 * time.Second

// ExecuteHandler executes the appropriate command for a tool
func ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	// Strip prefix to get command name
	cmdName := stripPrefix(toolName)

	// Build command args
	cmdArgs, err := buildArgs(cmdName, args)
	if err != nil {
		return "", err
	}

	// Add --json flag for machine-parseable output
	cmdArgs = append(cmdArgs, "--json")

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

func stripPrefix(toolName string) string {
	if len(toolName) > len(ToolPrefix) {
		return toolName[len(ToolPrefix):]
	}
	return toolName
}

// buildArgs builds CLI arguments for the given tool
func buildArgs(cmdName string, args map[string]interface{}) ([]string, error) {
	switch cmdName {
	case "search":
		return buildSearchArgs(args), nil
	case "index":
		return buildIndexArgs(args), nil
	case "status":
		return buildStatusArgs(args), nil
	case "update":
		return buildUpdateArgs(args), nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdName)
	}
}

func buildSearchArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"search"}

	if query, ok := args["query"].(string); ok {
		cmdArgs = append(cmdArgs, query)
	}
	if topK, ok := getInt(args, "top_k"); ok {
		cmdArgs = append(cmdArgs, "--top", strconv.Itoa(topK))
	}
	if threshold, ok := getFloat(args, "threshold"); ok {
		cmdArgs = append(cmdArgs, "--threshold", fmt.Sprintf("%.4f", threshold))
	}
	if typeFilter, ok := args["type"].(string); ok && typeFilter != "" {
		cmdArgs = append(cmdArgs, "--type", typeFilter)
	}
	if pathFilter, ok := args["path"].(string); ok && pathFilter != "" {
		cmdArgs = append(cmdArgs, "--path", pathFilter)
	}
	if getBool(args, "min") {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildIndexArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"index"}

	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, path)
	}
	if includes, ok := args["include"].([]interface{}); ok {
		for _, inc := range includes {
			if s, ok := inc.(string); ok {
				cmdArgs = append(cmdArgs, "--include", s)
			}
		}
	}
	if excludes, ok := args["exclude"].([]interface{}); ok {
		for _, exc := range excludes {
			if s, ok := exc.(string); ok {
				cmdArgs = append(cmdArgs, "--exclude", s)
			}
		}
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "--force")
	}

	return cmdArgs
}

func buildStatusArgs(args map[string]interface{}) []string {
	return []string{"index-status"}
}

func buildUpdateArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"index-update"}

	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, path)
	}
	if includes, ok := args["include"].([]interface{}); ok {
		for _, inc := range includes {
			if s, ok := inc.(string); ok {
				cmdArgs = append(cmdArgs, "--include", s)
			}
		}
	}
	if excludes, ok := args["exclude"].([]interface{}); ok {
		for _, exc := range excludes {
			if s, ok := exc.(string); ok {
				cmdArgs = append(cmdArgs, "--exclude", s)
			}
		}
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

func getFloat(args map[string]interface{}, key string) (float64, bool) {
	switch v := args[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	}
	return 0, false
}
