package mcpserver

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BinaryPath is the path to the llm-clarification binary
var BinaryPath = "/usr/local/bin/llm-clarification"

// CommandTimeout is the default timeout for command execution
var CommandTimeout = 120 * time.Second

// ExecuteHandler executes the appropriate command for a tool
func ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	// Strip prefix
	cmdName := strings.TrimPrefix(toolName, ToolPrefix)

	// Build command args
	cmdArgs, err := buildArgs(cmdName, args)
	if err != nil {
		return "", err
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
	case "match":
		return buildMatchArgs(args), nil
	case "cluster":
		return buildClusterArgs(args), nil
	case "detect_conflicts":
		return buildDetectConflictsArgs(args), nil
	case "validate":
		return buildValidateArgs(args), nil
	case "init":
		return buildInitArgs(args), nil
	case "add":
		return buildAddArgs(args), nil
	case "promote":
		return buildPromoteArgs(args), nil
	case "list":
		return buildListArgs(args), nil
	case "delete":
		return buildDeleteArgs(args), nil
	case "export":
		return buildExportArgs(args), nil
	case "import":
		return buildImportArgs(args), nil
	case "optimize":
		return buildOptimizeArgs(args), nil
	case "reconcile":
		return buildReconcileArgs(args), nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdName)
	}
}

func buildMatchArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"match-clarification"}
	if q, ok := args["question"].(string); ok {
		cmdArgs = append(cmdArgs, "--question", q)
	}
	if f, ok := args["entries_file"].(string); ok {
		cmdArgs = append(cmdArgs, "--entries-file", f)
	}
	if j, ok := args["entries_json"].(string); ok {
		cmdArgs = append(cmdArgs, "--entries-json", j)
	}
	if t, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(t))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildClusterArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"cluster-clarifications"}
	if f, ok := args["questions_file"].(string); ok {
		cmdArgs = append(cmdArgs, "--questions-file", f)
	}
	if j, ok := args["questions_json"].(string); ok {
		cmdArgs = append(cmdArgs, "--questions-json", j)
	}
	if t, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(t))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildDetectConflictsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"detect-conflicts"}
	if f, ok := args["tracking_file"].(string); ok {
		cmdArgs = append(cmdArgs, f)
	}
	if t, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(t))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildValidateArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"validate-clarifications"}
	if f, ok := args["tracking_file"].(string); ok {
		cmdArgs = append(cmdArgs, f)
	}
	if c, ok := args["context"].(string); ok {
		cmdArgs = append(cmdArgs, "--context", c)
	}
	if t, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(t))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildInitArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"init-tracking"}
	if o, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", o)
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "--force")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildAddArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"add-clarification"}
	if f, ok := args["tracking_file"].(string); ok {
		cmdArgs = append(cmdArgs, "--tracking-file", f)
	}
	if q, ok := args["question"].(string); ok {
		cmdArgs = append(cmdArgs, "--question", q)
	}
	if a, ok := args["answer"].(string); ok {
		cmdArgs = append(cmdArgs, "--answer", a)
	}
	if id, ok := args["id"].(string); ok {
		cmdArgs = append(cmdArgs, "--id", id)
	}
	if s, ok := args["sprint_id"].(string); ok {
		cmdArgs = append(cmdArgs, "--sprint-id", s)
	}
	if t, ok := args["context_tags"].(string); ok {
		cmdArgs = append(cmdArgs, "--context-tags", t)
	}
	if getBool(args, "check_match") {
		cmdArgs = append(cmdArgs, "--check-match")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildPromoteArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"promote-clarification"}
	if f, ok := args["tracking_file"].(string); ok {
		cmdArgs = append(cmdArgs, "--tracking-file", f)
	}
	if id, ok := args["id"].(string); ok {
		cmdArgs = append(cmdArgs, "--id", id)
	}
	if t, ok := args["target"].(string); ok {
		cmdArgs = append(cmdArgs, "--target", t)
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "--force")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildListArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"list-entries"}
	if f, ok := args["tracking_file"].(string); ok {
		cmdArgs = append(cmdArgs, f)
	}
	if s, ok := args["status"].(string); ok {
		cmdArgs = append(cmdArgs, "--status", s)
	}
	if m, ok := getInt(args, "min_occurrences"); ok {
		cmdArgs = append(cmdArgs, "--min-occurrences", strconv.Itoa(m))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildDeleteArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"delete-clarification"}
	if f, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", f)
	}
	if id, ok := args["id"].(string); ok {
		cmdArgs = append(cmdArgs, "--id", id)
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "--force")
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildExportArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"export-memory"}
	if s, ok := args["source"].(string); ok {
		cmdArgs = append(cmdArgs, "--source", s)
	}
	if o, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", o)
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildImportArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"import-memory"}
	if s, ok := args["source"].(string); ok {
		cmdArgs = append(cmdArgs, "--source", s)
	}
	if t, ok := args["target"].(string); ok {
		cmdArgs = append(cmdArgs, "--target", t)
	}
	if m, ok := args["mode"].(string); ok {
		cmdArgs = append(cmdArgs, "--mode", m)
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildOptimizeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"optimize-memory"}
	if f, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", f)
	}
	if getBool(args, "vacuum") {
		cmdArgs = append(cmdArgs, "--vacuum")
	}
	if ps, ok := args["prune_stale"].(string); ok {
		cmdArgs = append(cmdArgs, "--prune-stale", ps)
	}
	if getBool(args, "stats") {
		cmdArgs = append(cmdArgs, "--stats")
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildReconcileArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"reconcile-memory"}
	if f, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", f)
	}
	if pr, ok := args["project_root"].(string); ok {
		cmdArgs = append(cmdArgs, "--project-root", pr)
	}
	if getBool(args, "dry_run") {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	if getBool(args, "quiet") {
		cmdArgs = append(cmdArgs, "--quiet")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
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

// getBoolDefault returns the bool value for key, or defaultVal if not set.
// Used for json/min flags that should default to true in MCP context.
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
