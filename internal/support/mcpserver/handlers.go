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

// paramAliases maps canonical parameter names to their accepted aliases.
// This makes the MCP tools more forgiving when LLMs use alternative parameter names.
var paramAliases = map[string][]string{
	"path":     {"target", "file", "input", "dir", "directory", "file_path"},
	"file":     {"path", "input", "template"},
	"manifest": {"path", "file", "package"},
	"pattern":  {"regex", "search"},
	"context":  {"description"},
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

	// Add --json and --min flags for machine-parseable, token-optimized output
	cmdArgs = append(cmdArgs, "--json", "--min")

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
	case "extract_links":
		return buildExtractLinksArgs(args), nil
	case "highest":
		return buildHighestArgs(args), nil
	case "plan_type":
		return buildPlanTypeArgs(args), nil
	case "git_changes":
		return buildGitChangesArgs(args), nil
	case "context":
		return buildContextArgs(args), nil
	case "context_multiset":
		return buildContextMultiSetArgs(args), nil
	case "context_multiget":
		return buildContextMultiGetArgs(args), nil
	case "yaml_get":
		return buildYamlGetArgs(args), nil
	case "yaml_set":
		return buildYamlSetArgs(args), nil
	case "yaml_multiget":
		return buildYamlMultigetArgs(args), nil
	case "yaml_multiset":
		return buildYamlMultisetArgs(args), nil
	case "args":
		return buildArgsParserArgs(args), nil
	case "catfiles":
		return buildCatfilesArgs(args), nil
	case "decode":
		return buildDecodeArgs(args), nil
	case "diff":
		return buildDiffArgs(args), nil
	case "encode":
		return buildEncodeArgs(args), nil
	case "extract":
		return buildExtractArgs(args), nil
	case "foreach":
		return buildForeachArgs(args), nil
	case "hash":
		return buildHashArgs(args), nil
	case "init_temp":
		return buildInitTempArgs(args), nil
	case "clean_temp":
		return buildCleanTempArgs(args), nil
	case "math":
		return buildMathArgs(args), nil
	case "prompt":
		return buildPromptArgs(args), nil
	case "report":
		return buildReportArgs(args), nil
	case "stats":
		return buildStatsArgs(args), nil
	case "toml_query":
		return buildTomlQueryArgs(args), nil
	case "toml_validate":
		return buildTomlValidateArgs(args), nil
	case "toml_parse":
		return buildTomlParseArgs(args), nil
	case "transform_case":
		return buildTransformCaseArgs(args), nil
	case "transform_csv_to_json":
		return buildTransformCsvToJsonArgs(args), nil
	case "transform_json_to_csv":
		return buildTransformJsonToCsvArgs(args), nil
	case "transform_filter":
		return buildTransformFilterArgs(args), nil
	case "transform_sort":
		return buildTransformSortArgs(args), nil
	case "validate":
		return buildValidateArgs(args), nil
	case "runtime":
		return buildRuntimeArgs(args), nil
	case "complete":
		return buildCompleteArgs(args), nil
	case "parse_stream":
		return buildParseStreamArgs(args), nil
	case "route_td":
		return buildRouteTDArgs(args), nil
	case "coverage_report":
		return buildCoverageReportArgs(args), nil
	case "validate_risks":
		return buildValidateRisksArgs(args), nil
	case "sprint_status":
		return buildSprintStatusArgs(args), nil
	case "alignment_check":
		return buildAlignmentCheckArgs(args), nil
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
	if maxEntries, ok := getInt(args, "max_entries"); ok {
		cmdArgs = append(cmdArgs, "--max-entries", strconv.Itoa(maxEntries))
	}
	if getBool(args, "sizes") {
		cmdArgs = append(cmdArgs, "--sizes")
	}
	if excludes, ok := args["exclude"].([]interface{}); ok {
		for _, e := range excludes {
			if s, ok := e.(string); ok {
				cmdArgs = append(cmdArgs, "--exclude", s)
			}
		}
	}
	if getBool(args, "no_gitignore") {
		cmdArgs = append(cmdArgs, "--no-gitignore")
	}
	if getBool(args, "no_default_excludes") {
		cmdArgs = append(cmdArgs, "--no-default-excludes")
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
	// Default to brackets syntax ([[var]]) to avoid conflicts with LLM template syntaxes
	// (Claude uses $var, Qwen/others use {{var}})
	syntax := "brackets"
	if s, ok := args["syntax"].(string); ok {
		syntax = s
	}
	cmdArgs = append(cmdArgs, "--syntax", syntax)
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
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
	// NOTE: --min is intentionally NOT used here. This command documents specific
	// output fields (PATTERN, FRAMEWORK, TEST_RUNNER, etc.) that must always be
	// present in JSON output for reliable parsing. Using --min would omit empty fields.
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
	// NOTE: --min is intentionally NOT used here. This command documents specific
	// output fields (STACK, LANGUAGE, PACKAGE_MANAGER, etc.) that must always be
	// present in JSON output for reliable parsing. Using --min would omit empty fields.
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

func buildExtractLinksArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"extract-links"}
	if url, ok := args["url"].(string); ok {
		cmdArgs = append(cmdArgs, "--url", url)
	}
	if context, ok := args["context"].(string); ok && context != "" {
		cmdArgs = append(cmdArgs, "--context", context)
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
	// Handle paths array - takes precedence over path if both specified
	if paths, ok := args["paths"].([]interface{}); ok && len(paths) > 0 {
		var pathStrs []string
		for _, p := range paths {
			if ps, ok := p.(string); ok {
				pathStrs = append(pathStrs, ps)
			}
		}
		if len(pathStrs) > 0 {
			cmdArgs = append(cmdArgs, "--paths", strings.Join(pathStrs, ","))
		}
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
	// Note: "dir" normalizes to "path" via paramAliases, so check path first
	if dir, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	} else if dir, ok := args["dir"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	}

	// Operation-specific arguments
	switch operation {
	case "init":
		if getBoolDefault(args, "json", true) {
			cmdArgs = append(cmdArgs, "--json")
		}
		if getBoolDefault(args, "min", true) {
			cmdArgs = append(cmdArgs, "--min")
		}
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

func getInt64(args map[string]interface{}, key string) (int64, bool) {
	switch v := args[key].(type) {
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case int64:
		return v, true
	}
	return 0, false
}

func getFloat(args map[string]interface{}, key string) (float64, bool) {
	switch v := args[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

func buildContextMultiSetArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"context", "multiset"}

	// Add --dir flag (required)
	// Note: "dir" normalizes to "path" via paramAliases, so check path first
	if dir, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	} else if dir, ok := args["dir"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	}

	// Convert pairs object to KEY VALUE arguments
	if pairs, ok := args["pairs"].(map[string]interface{}); ok {
		for key, value := range pairs {
			cmdArgs = append(cmdArgs, key)
			if v, ok := value.(string); ok {
				cmdArgs = append(cmdArgs, v)
			} else {
				cmdArgs = append(cmdArgs, fmt.Sprintf("%v", value))
			}
		}
	}

	return cmdArgs
}

func buildContextMultiGetArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"context", "multiget"}

	// Add --dir flag (required)
	// Note: "dir" normalizes to "path" via paramAliases, so check path first
	if dir, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	} else if dir, ok := args["dir"].(string); ok {
		cmdArgs = append(cmdArgs, "--dir", dir)
	}

	// Add keys as positional arguments
	if keys, ok := args["keys"].([]interface{}); ok {
		for _, k := range keys {
			if s, ok := k.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	// Add --defaults if provided (JSON map of fallback values)
	if defaults, ok := args["defaults"].(map[string]interface{}); ok && len(defaults) > 0 {
		jsonBytes, err := json.Marshal(defaults)
		if err == nil {
			cmdArgs = append(cmdArgs, "--defaults", string(jsonBytes))
		}
	}

	// Output format flags - both default to true (matching llm-support pattern)
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildCompleteArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"complete"}

	if prompt, ok := args["prompt"].(string); ok {
		cmdArgs = append(cmdArgs, "--prompt", prompt)
	}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}
	if template, ok := args["template"].(string); ok {
		cmdArgs = append(cmdArgs, "--template", template)
	}
	// Handle vars as a map
	if vars, ok := args["vars"].(map[string]interface{}); ok {
		for key, val := range vars {
			if strVal, ok := val.(string); ok {
				cmdArgs = append(cmdArgs, "--var", key+"="+strVal)
			}
		}
	}
	if system, ok := args["system"].(string); ok {
		cmdArgs = append(cmdArgs, "--system", system)
	}
	if model, ok := args["model"].(string); ok {
		cmdArgs = append(cmdArgs, "--model", model)
	}
	if temp, ok := getFloat(args, "temperature"); ok {
		cmdArgs = append(cmdArgs, "--temperature", strconv.FormatFloat(temp, 'f', -1, 64))
	}
	if maxTokens, ok := getInt(args, "max_tokens"); ok {
		cmdArgs = append(cmdArgs, "--max-tokens", strconv.Itoa(maxTokens))
	}
	if timeout, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(timeout))
	}
	if retries, ok := getInt(args, "retries"); ok {
		cmdArgs = append(cmdArgs, "--retries", strconv.Itoa(retries))
	}
	if output, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", output)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildYamlGetArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"yaml", "get"}

	// Add --file flag (required)
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}

	// Add key as positional argument
	if key, ok := args["key"].(string); ok {
		cmdArgs = append(cmdArgs, key)
	}

	// Default value
	if def, ok := args["default"].(string); ok {
		cmdArgs = append(cmdArgs, "--default", def)
	}

	// Output format flags
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildYamlSetArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"yaml", "set"}

	// Add --file flag (required)
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}

	// Add key and value as positional arguments
	if key, ok := args["key"].(string); ok {
		cmdArgs = append(cmdArgs, key)
	}
	if value, ok := args["value"].(string); ok {
		cmdArgs = append(cmdArgs, value)
	}

	// Create flag
	if getBool(args, "create") {
		cmdArgs = append(cmdArgs, "--create")
	}

	// Output format flags
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildYamlMultigetArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"yaml", "multiget"}

	// Add --file flag (required)
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}

	// Add keys as positional arguments
	if keys, ok := args["keys"].([]interface{}); ok {
		for _, k := range keys {
			if s, ok := k.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	// Add defaults as JSON string to --defaults flag
	if defaults, ok := args["defaults"].(map[string]interface{}); ok && len(defaults) > 0 {
		// Convert map to JSON string for the CLI
		jsonBytes, err := json.Marshal(defaults)
		if err == nil {
			cmdArgs = append(cmdArgs, "--defaults", string(jsonBytes))
		}
	}

	// Output format flags
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildYamlMultisetArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"yaml", "multiset"}

	// Add --file flag (required)
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}

	// Convert pairs object to KEY VALUE arguments
	if pairs, ok := args["pairs"].(map[string]interface{}); ok {
		for key, value := range pairs {
			cmdArgs = append(cmdArgs, key)
			if v, ok := value.(string); ok {
				cmdArgs = append(cmdArgs, v)
			} else {
				cmdArgs = append(cmdArgs, fmt.Sprintf("%v", value))
			}
		}
	}

	// Create flag
	if getBool(args, "create") {
		cmdArgs = append(cmdArgs, "--create")
	}

	// Output format flags
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildArgsParserArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"args", "--json", "--min", "--"}

	// Add arguments as positional parameters after "--" separator
	if arguments, ok := args["arguments"].([]interface{}); ok {
		for _, arg := range arguments {
			if s, ok := arg.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}

	return cmdArgs
}

func buildCatfilesArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"catfiles"}

	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}
	if maxSize, ok := getInt(args, "max_size"); ok {
		cmdArgs = append(cmdArgs, "--max-size", strconv.Itoa(maxSize))
	}
	if getBool(args, "no_gitignore") {
		cmdArgs = append(cmdArgs, "--no-gitignore")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildDecodeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"decode"}

	if text, ok := args["text"].(string); ok {
		cmdArgs = append(cmdArgs, text)
	}
	if encoding, ok := args["encoding"].(string); ok {
		cmdArgs = append(cmdArgs, "--encoding", encoding)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildDiffArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"diff"}

	if file1, ok := args["file1"].(string); ok {
		cmdArgs = append(cmdArgs, file1)
	}
	if file2, ok := args["file2"].(string); ok {
		cmdArgs = append(cmdArgs, file2)
	}
	if getBool(args, "unified") {
		cmdArgs = append(cmdArgs, "--unified")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildEncodeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"encode"}

	if text, ok := args["text"].(string); ok {
		cmdArgs = append(cmdArgs, text)
	}
	if encoding, ok := args["encoding"].(string); ok {
		cmdArgs = append(cmdArgs, "--encoding", encoding)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildExtractArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"extract"}

	if t, ok := args["type"].(string); ok {
		cmdArgs = append(cmdArgs, t)
	}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if getBool(args, "count") {
		cmdArgs = append(cmdArgs, "--count")
	}
	if getBool(args, "unique") {
		cmdArgs = append(cmdArgs, "--unique")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildForeachArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"foreach"}

	if files, ok := args["files"].([]interface{}); ok {
		for _, f := range files {
			if s, ok := f.(string); ok {
				cmdArgs = append(cmdArgs, "--files", s)
			}
		}
	}
	if glob, ok := args["glob"].(string); ok {
		cmdArgs = append(cmdArgs, "--glob", glob)
	}
	if template, ok := args["template"].(string); ok {
		cmdArgs = append(cmdArgs, "--template", template)
	}
	if llm, ok := args["llm"].(string); ok {
		cmdArgs = append(cmdArgs, "--llm", llm)
	}
	if outputDir, ok := args["output_dir"].(string); ok {
		cmdArgs = append(cmdArgs, "--output-dir", outputDir)
	}
	if outputPattern, ok := args["output_pattern"].(string); ok {
		cmdArgs = append(cmdArgs, "--output-pattern", outputPattern)
	}
	if parallel, ok := getInt(args, "parallel"); ok {
		cmdArgs = append(cmdArgs, "--parallel", strconv.Itoa(parallel))
	}
	if getBool(args, "skip_existing") {
		cmdArgs = append(cmdArgs, "--skip-existing")
	}
	if timeout, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(timeout))
	}
	if vars, ok := args["vars"].(map[string]interface{}); ok {
		for k, v := range vars {
			cmdArgs = append(cmdArgs, "--var", fmt.Sprintf("%s=%v", k, v))
		}
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildHashArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"hash"}

	if paths, ok := args["paths"].([]interface{}); ok {
		for _, p := range paths {
			if s, ok := p.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}
	if algorithm, ok := args["algorithm"].(string); ok {
		cmdArgs = append(cmdArgs, "--algorithm", algorithm)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildInitTempArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"init-temp"}

	if name, ok := args["name"].(string); ok {
		cmdArgs = append(cmdArgs, "--name", name)
	}
	// clean defaults to true, only add flag if explicitly false
	if v, ok := args["clean"].(bool); ok && !v {
		cmdArgs = append(cmdArgs, "--clean=false")
	}
	if getBool(args, "preserve") {
		cmdArgs = append(cmdArgs, "--preserve")
	}
	if getBool(args, "with_git") {
		cmdArgs = append(cmdArgs, "--with-git")
	}
	if getBool(args, "skip_context") {
		cmdArgs = append(cmdArgs, "--skip-context")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildCleanTempArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"clean-temp"}

	if name, ok := args["name"].(string); ok {
		cmdArgs = append(cmdArgs, "--name", name)
	}
	if getBool(args, "all") {
		cmdArgs = append(cmdArgs, "--all")
	}
	if olderThan, ok := args["older_than"].(string); ok {
		cmdArgs = append(cmdArgs, "--older-than", olderThan)
	}
	if getBool(args, "dry_run") {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildMathArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"math"}

	if expr, ok := args["expression"].(string); ok {
		cmdArgs = append(cmdArgs, expr)
	}

	return cmdArgs
}

func buildPromptArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"prompt"}

	if prompt, ok := args["prompt"].(string); ok {
		cmdArgs = append(cmdArgs, "--prompt", prompt)
	}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}
	if template, ok := args["template"].(string); ok {
		cmdArgs = append(cmdArgs, "--template", template)
	}
	if llm, ok := args["llm"].(string); ok {
		cmdArgs = append(cmdArgs, "--llm", llm)
	}
	if instruction, ok := args["instruction"].(string); ok {
		cmdArgs = append(cmdArgs, "--instruction", instruction)
	}
	if vars, ok := args["vars"].(map[string]interface{}); ok {
		for k, v := range vars {
			cmdArgs = append(cmdArgs, "--var", fmt.Sprintf("%s=%v", k, v))
		}
	}
	if retries, ok := getInt(args, "retries"); ok {
		cmdArgs = append(cmdArgs, "--retries", strconv.Itoa(retries))
	}
	if retryDelay, ok := getInt(args, "retry_delay"); ok {
		cmdArgs = append(cmdArgs, "--retry-delay", strconv.Itoa(retryDelay))
	}
	if timeout, ok := getInt(args, "timeout"); ok {
		cmdArgs = append(cmdArgs, "--timeout", strconv.Itoa(timeout))
	}
	if getBool(args, "cache") {
		cmdArgs = append(cmdArgs, "--cache")
	}
	if cacheTtl, ok := getInt(args, "cache_ttl"); ok {
		cmdArgs = append(cmdArgs, "--cache-ttl", strconv.Itoa(cacheTtl))
	}
	if getBool(args, "refresh") {
		cmdArgs = append(cmdArgs, "--refresh")
	}
	if minLength, ok := getInt(args, "min_length"); ok {
		cmdArgs = append(cmdArgs, "--min-length", strconv.Itoa(minLength))
	}
	if mustContain, ok := args["must_contain"].([]interface{}); ok {
		for _, m := range mustContain {
			if s, ok := m.(string); ok {
				cmdArgs = append(cmdArgs, "--must-contain", s)
			}
		}
	}
	if getBool(args, "no_error_check") {
		cmdArgs = append(cmdArgs, "--no-error-check")
	}
	if output, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", output)
	}
	if getBool(args, "strip") {
		cmdArgs = append(cmdArgs, "--strip")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildReportArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"report"}

	if title, ok := args["title"].(string); ok {
		cmdArgs = append(cmdArgs, "--title", title)
	}
	if status, ok := args["status"].(string); ok {
		cmdArgs = append(cmdArgs, "--status", status)
	}
	if stats, ok := args["stats"].(map[string]interface{}); ok {
		for k, v := range stats {
			cmdArgs = append(cmdArgs, "--stat", fmt.Sprintf("%s=%v", k, v))
		}
	}
	if output, ok := args["output"].(string); ok {
		cmdArgs = append(cmdArgs, "--output", output)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildStatsArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"stats"}

	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, "--path", path)
	}
	if getBool(args, "no_gitignore") {
		cmdArgs = append(cmdArgs, "--no-gitignore")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildTomlQueryArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"toml", "query"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if path, ok := args["path"].(string); ok {
		cmdArgs = append(cmdArgs, path)
	}

	return cmdArgs
}

func buildTomlValidateArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"toml", "validate"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}

	return cmdArgs
}

func buildTomlParseArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"toml", "parse"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}

	return cmdArgs
}

func buildTransformCaseArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"transform", "case"}

	if text, ok := args["text"].(string); ok {
		cmdArgs = append(cmdArgs, text)
	}
	if to, ok := args["to"].(string); ok {
		cmdArgs = append(cmdArgs, "--to", to)
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildTransformCsvToJsonArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"transform", "csv-to-json"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}

	return cmdArgs
}

func buildTransformJsonToCsvArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"transform", "json-to-csv"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}

	return cmdArgs
}

func buildTransformFilterArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"transform", "filter"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if pattern, ok := args["pattern"].(string); ok {
		cmdArgs = append(cmdArgs, pattern)
	}
	if getBool(args, "invert") {
		cmdArgs = append(cmdArgs, "--invert")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildTransformSortArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"transform", "sort"}

	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, file)
	}
	if getBool(args, "reverse") {
		cmdArgs = append(cmdArgs, "--reverse")
	}

	return cmdArgs
}

func buildValidateArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"validate"}

	if files, ok := args["files"].([]interface{}); ok {
		for _, f := range files {
			if s, ok := f.(string); ok {
				cmdArgs = append(cmdArgs, s)
			}
		}
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildRuntimeArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"runtime"}

	if start, ok := getInt64(args, "start"); ok {
		cmdArgs = append(cmdArgs, "--start", strconv.FormatInt(start, 10))
	}
	if end, ok := getInt64(args, "end"); ok {
		cmdArgs = append(cmdArgs, "--end", strconv.FormatInt(end, 10))
	}
	if format, ok := args["format"].(string); ok {
		cmdArgs = append(cmdArgs, "--format", format)
	}
	if precision, ok := getInt(args, "precision"); ok {
		cmdArgs = append(cmdArgs, "--precision", strconv.Itoa(precision))
	}
	if getBool(args, "label") {
		cmdArgs = append(cmdArgs, "--label")
	}
	if getBool(args, "raw") {
		cmdArgs = append(cmdArgs, "--raw")
	}
	if getBoolDefault(args, "json", true) {
		cmdArgs = append(cmdArgs, "--json")
	}
	if getBoolDefault(args, "min", true) {
		cmdArgs = append(cmdArgs, "--min")
	}

	return cmdArgs
}

func buildParseStreamArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"parse-stream"}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}
	if content, ok := args["content"].(string); ok {
		cmdArgs = append(cmdArgs, "--content", content)
	}
	if format, ok := args["format"].(string); ok {
		cmdArgs = append(cmdArgs, "--format", format)
	}
	if delimiter, ok := args["delimiter"].(string); ok {
		cmdArgs = append(cmdArgs, "--delimiter", delimiter)
	}
	if headers, ok := args["headers"].(string); ok {
		cmdArgs = append(cmdArgs, "--headers", headers)
	}
	return cmdArgs
}

func buildRouteTDArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"route-td"}
	if file, ok := args["file"].(string); ok {
		cmdArgs = append(cmdArgs, "--file", file)
	}
	if content, ok := args["content"].(string); ok {
		cmdArgs = append(cmdArgs, "--content", content)
	}
	if quickWinsMax, ok := getInt(args, "quick_wins_max"); ok {
		cmdArgs = append(cmdArgs, "--quick-wins-max", strconv.Itoa(quickWinsMax))
	}
	if backlogMax, ok := getInt(args, "backlog_max"); ok {
		cmdArgs = append(cmdArgs, "--backlog-max", strconv.Itoa(backlogMax))
	}
	return cmdArgs
}

func buildCoverageReportArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"coverage-report"}
	if requirements, ok := args["requirements"].(string); ok {
		cmdArgs = append(cmdArgs, "--requirements", requirements)
	}
	if stories, ok := args["stories"].(string); ok {
		cmdArgs = append(cmdArgs, "--stories", stories)
	}
	return cmdArgs
}

func buildValidateRisksArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"validate-risks"}
	if design, ok := args["design"].(string); ok {
		cmdArgs = append(cmdArgs, "--design", design)
	}
	if stories, ok := args["stories"].(string); ok {
		cmdArgs = append(cmdArgs, "--stories", stories)
	}
	if tasks, ok := args["tasks"].(string); ok {
		cmdArgs = append(cmdArgs, "--tasks", tasks)
	}
	if ac, ok := args["acceptance_criteria"].(string); ok {
		cmdArgs = append(cmdArgs, "--acceptance-criteria", ac)
	}
	return cmdArgs
}

func buildSprintStatusArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"sprint-status"}
	if tasksTotal, ok := getInt(args, "tasks_total"); ok {
		cmdArgs = append(cmdArgs, "--tasks-total", strconv.Itoa(tasksTotal))
	}
	if tasksCompleted, ok := getInt(args, "tasks_completed"); ok {
		cmdArgs = append(cmdArgs, "--tasks-completed", strconv.Itoa(tasksCompleted))
	}
	if getBool(args, "tests_passed") {
		cmdArgs = append(cmdArgs, "--tests-passed")
	} else {
		cmdArgs = append(cmdArgs, "--tests-passed=false")
	}
	if coverage, ok := args["coverage"].(float64); ok {
		cmdArgs = append(cmdArgs, "--coverage", strconv.FormatFloat(coverage, 'f', 1, 64))
	}
	if issues, ok := getInt(args, "critical_issues"); ok {
		cmdArgs = append(cmdArgs, "--critical-issues", strconv.Itoa(issues))
	}
	if thresh, ok := args["completed_threshold"].(float64); ok {
		cmdArgs = append(cmdArgs, "--completed-threshold", strconv.FormatFloat(thresh, 'f', 2, 64))
	}
	if thresh, ok := args["partial_threshold"].(float64); ok {
		cmdArgs = append(cmdArgs, "--partial-threshold", strconv.FormatFloat(thresh, 'f', 2, 64))
	}
	if thresh, ok := args["coverage_threshold"].(float64); ok {
		cmdArgs = append(cmdArgs, "--coverage-threshold", strconv.FormatFloat(thresh, 'f', 1, 64))
	}
	return cmdArgs
}

func buildAlignmentCheckArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"alignment-check"}
	if requirements, ok := args["requirements"].(string); ok {
		cmdArgs = append(cmdArgs, "--requirements", requirements)
	}
	if stories, ok := args["stories"].(string); ok {
		cmdArgs = append(cmdArgs, "--stories", stories)
	}
	return cmdArgs
}
