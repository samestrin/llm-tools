package mcpserver

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// paramAliases maps canonical parameter names to their accepted aliases.
// This makes the MCP tools more forgiving when LLMs use alternative parameter names.
var paramAliases = map[string][]string{
	"path":     {"target", "file", "input", "dir", "directory"},
	"file":     {"path", "input", "template"},
	"manifest": {"path", "file", "package"},
	"pattern":  {"regex", "search"},
	"context":  {"prompt", "description"},
}

// normalizeArgs converts aliased parameter names to their canonical forms.
// It returns a new map with normalized keys while preserving original values.
func normalizeArgs(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return args
	}

	// Build set of canonical names (these should never be remapped)
	canonicalNames := make(map[string]bool)
	for canonical := range paramAliases {
		canonicalNames[canonical] = true
	}

	// Build reverse lookup: alias -> canonical
	// Skip aliases that are themselves canonical names to avoid conflicts
	aliasToCanonical := make(map[string]string)
	for canonical, aliases := range paramAliases {
		for _, alias := range aliases {
			// Don't add if alias is itself a canonical name (avoid conflicts)
			if !canonicalNames[alias] {
				aliasToCanonical[alias] = canonical
			}
		}
	}

	normalized := make(map[string]interface{})
	for key, value := range args {
		// Check if this key should be normalized
		if canonical, isAlias := aliasToCanonical[key]; isAlias {
			// Only normalize if canonical key not already present in input
			if _, hasCanonical := args[canonical]; !hasCanonical {
				normalized[canonical] = value
				continue
			}
		}
		normalized[key] = value
	}

	return normalized
}

// BinaryPath is the path to the llm-support binary
var BinaryPath = "/usr/local/bin/llm-support"

// CommandTimeout is the default timeout for command execution
var CommandTimeout = 60 * time.Second

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
	// Normalize parameter aliases before processing
	args = normalizeArgs(args)

	switch cmdName {
	case "tree":
		return buildTreeArgs(args), nil
	case "grep":
		return buildGrepArgs(args), nil
	case "multiexists":
		return buildMultiexistsArgs(args), nil
	case "json_query":
		return buildJSONQueryArgs(args), nil
	case "markdown_headers":
		return buildMarkdownHeadersArgs(args), nil
	case "template":
		return buildTemplateArgs(args), nil
	case "discover_tests":
		return buildDiscoverTestsArgs(args), nil
	case "multigrep":
		return buildMultigrepArgs(args), nil
	case "analyze_deps":
		return buildAnalyzeDepsArgs(args), nil
	case "detect":
		return buildDetectArgs(args), nil
	case "count":
		return buildCountArgs(args), nil
	case "summarize_dir":
		return buildSummarizeDirArgs(args), nil
	case "deps":
		return buildDepsArgs(args), nil
	case "git_context":
		return buildGitContextArgs(args), nil
	case "validate_plan":
		return buildValidatePlanArgs(args), nil
	case "partition_work":
		return buildPartitionWorkArgs(args), nil
	case "repo_root":
		return buildRepoRootArgs(args), nil
	case "extract_relevant":
		return buildExtractRelevantArgs(args), nil
	case "highest":
		return buildHighestArgs(args), nil
	case "plan_type":
		return buildPlanTypeArgs(args), nil
	case "git_changes":
		return buildGitChangesArgs(args), nil
	case "context":
		return buildContextArgs(args), nil
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdName)
	}
}

func buildTreeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"tree"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if depth, ok := getInt(args, "depth"); ok {
		cmdArgs = append(cmdArgs, "--depth", strconv.Itoa(depth))
	}
	if getBool(args, "sizes") {
		cmdArgs = append(cmdArgs, "--sizes")
	}
	if getBool(args, "no_gitignore") {
		cmdArgs = append(cmdArgs, "--no-gitignore")
	}
	return cmdArgs
}

func buildGrepArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"grep"}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, pattern)
	}
	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}
	if getBool(args, "ignore_case") {
		cmdArgs = append(cmdArgs, "-i")
	}
	if getBool(args, "line_numbers") {
		cmdArgs = append(cmdArgs, "-n")
	}
	if getBool(args, "files_only") {
		cmdArgs = append(cmdArgs, "-l")
	}
	return cmdArgs
}

func buildMultiexistsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"multiexists"}
	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}
	if getBool(args, "verbose") {
		cmdArgs = append(cmdArgs, "--verbose")
	}
	cmdArgs = append(cmdArgs, "--no-fail")
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildJSONQueryArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"json", "query"}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if query, ok := args["query"].(string); ok {
		cmdArgs = append(cmdArgs, query)
	}
	return cmdArgs
}

func buildMarkdownHeadersArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"markdown", "headers"}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if level, ok := args["level"].(string); ok {
		cmdArgs = append(cmdArgs, "--level", level)
	}
	if getBool(args, "plain") {
		cmdArgs = append(cmdArgs, "--plain")
	}
	return cmdArgs
}

func buildTemplateArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"template"}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if vars, ok := args["vars"].(map[string]interface{}); ok {
		for k, v := range vars {
			cmdArgs = append(cmdArgs, "--var", fmt.Sprintf("%s=%v", k, v))
		}
	}
	if syntax, ok := args["syntax"].(string); ok {
		cmdArgs = append(cmdArgs, "--syntax", syntax)
	}
	return cmdArgs
}

func buildDiscoverTestsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"discover-tests"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildMultigrepArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"multigrep"}
	if keywords, ok := args["keywords"].(string); ok {
		cmdArgs = append(cmdArgs, "--keywords", keywords)
	}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if ext, ok := args["extensions"].(string); ok {
		cmdArgs = append(cmdArgs, "--extensions", ext)
	}
	if max, ok := getInt(args, "max_per_keyword"); ok {
		cmdArgs = append(cmdArgs, "--max-per-keyword", strconv.Itoa(max))
	}
	if getBool(args, "ignore_case") {
		cmdArgs = append(cmdArgs, "--ignore-case")
	}
	if getBool(args, "definitions_only") {
		cmdArgs = append(cmdArgs, "--definitions-only")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	if dir, ok := args["output_dir"].(string); ok {
		cmdArgs = append(cmdArgs, "--output-dir", dir)
	}
	return cmdArgs
}

func buildAnalyzeDepsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"analyze-deps"}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildDetectArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"detect"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildCountArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"count"}
	if mode, ok := args["mode"].(string); ok {
		cmdArgs = append(cmdArgs, "--mode", mode)
	}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBool(args, "recursive") {
		cmdArgs = append(cmdArgs, "--recursive")
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildSummarizeDirArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"summarize-dir"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if format, ok := args["format"].(string); ok {
		cmdArgs = append(cmdArgs, "--format", format)
	}
	if getBool(args, "recursive") {
		cmdArgs = append(cmdArgs, "--recursive")
	}
	if glob, ok := args["glob"].(string); ok {
		cmdArgs = append(cmdArgs, "--glob", glob)
	}
	if max, ok := getInt(args, "max_tokens"); ok {
		cmdArgs = append(cmdArgs, "--max-tokens", strconv.Itoa(max))
	}
	return cmdArgs
}

func buildDepsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"deps"}
	if manifest, ok := args["manifest"].(string); ok {
		cmdArgs = append(cmdArgs, manifest)
	}
	if t, ok := args["type"].(string); ok {
		cmdArgs = append(cmdArgs, "--type", t)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildGitContextArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"git-context"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBool(args, "include_diff") {
		cmdArgs = append(cmdArgs, "--include-diff")
	}
	if since, ok := args["since"].(string); ok {
		cmdArgs = append(cmdArgs, "--since", since)
	}
	if max, ok := getInt(args, "max_commits"); ok {
		cmdArgs = append(cmdArgs, "--max-commits", strconv.Itoa(max))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildValidatePlanArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"validate-plan"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildPartitionWorkArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"partition-work"}
	if stories, ok := args["stories"].(string); ok {
		cmdArgs = append(cmdArgs, "--stories", stories)
	}
	if tasks, ok := args["tasks"].(string); ok {
		cmdArgs = append(cmdArgs, "--tasks", tasks)
	}
	if getBool(args, "verbose") {
		cmdArgs = append(cmdArgs, "--verbose")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildRepoRootArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"repo-root"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBool(args, "validate") {
		cmdArgs = append(cmdArgs, "--validate")
	}
	return cmdArgs
}

func buildExtractRelevantArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"extract-relevant"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if ctx, ok := args["context"].(string); ok {
		cmdArgs = append(cmdArgs, "--context", ctx)
	}
	if conc, ok := getInt(args, "concurrency"); ok {
		cmdArgs = append(cmdArgs, "--concurrency", strconv.Itoa(conc))
	}
	if output, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", output)
	}
	if timeout, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(timeout))
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildHighestArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"highest"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--pattern", pattern)
	}
	if t, ok := args["type"].(string); ok {
		cmdArgs = append(cmdArgs, "--type", t)
	}
	if prefix, ok := args["prefix"].(string); ok {
		cmdArgs = append(cmdArgs, "--prefix", prefix)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildPlanTypeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"plan-type"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildGitChangesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"git-changes"}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	// Handle include_untracked - default is true, only add flag if explicitly false
	if v, ok := args["include_untracked"].(bool); ok && !v {
		cmdArgs = append(cmdArgs, "--include-untracked=false")
	}
	if getBool(args, "staged_only") {
		cmdArgs = append(cmdArgs, "--staged-only")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}
	return cmdArgs
}

func buildContextArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"context"}

	// Get the operation (required)
	operation, _ := args["operation"].(string)
	cmdArgs = append(cmdArgs, operation)

	// Add --dir flag (required)
	if dir, ok := args["dir"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	}

	// Operation-specific arguments
	switch operation {
	case "set":
		if key, ok := args["key"].(string); ok {
			cmdArgs = append(cmdArgs, key)
		}
		if value, ok := args["value"].(string); ok {
			cmdArgs = append(cmdArgs, value)
		}
	case "get":
		if key, ok := args["key"].(string); ok {
			cmdArgs = append(cmdArgs, key)
		}
		if defaultVal, ok := args["default"].(string); ok {
			cmdArgs = append(cmdArgs, "--default", defaultVal)
		}
		if getBoolDefault(args, "json", true) {
			cmdArgs = append(cmdArgs, "--json")
		}
		if getBoolDefault(args, "min", true) {
			cmdArgs = append(cmdArgs, "--min")
		}
	case "list":
		if getBoolDefault(args, "json", true) {
			cmdArgs = append(cmdArgs, "--json")
		}
		if getBoolDefault(args, "min", true) {
			cmdArgs = append(cmdArgs, "--min")
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
