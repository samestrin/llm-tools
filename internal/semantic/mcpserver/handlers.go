package mcpserver

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// BinaryPath is the path to the llm-semantic binary
// Defaults to "llm-semantic" (PATH lookup), falls back to /usr/local/bin
var BinaryPath = "llm-semantic"

// CommandTimeout is the default timeout for command execution
// Increased to 120s for index/update operations which can be slow
// Can be overridden via LLM_SEMANTIC_TIMEOUT env var (in seconds)
var CommandTimeout = 120 * time.Second

// argBuilder is a function that builds CLI arguments from the args map
type argBuilder struct {
	build    func(args map[string]interface{}) []string
	validate func(args map[string]interface{}) error // optional validation, nil means no extra validation
}

// commandRegistry maps command names to their argument builders.
// This provides compile-time verification that all commands are handled
// and makes it easy to see which commands exist.
var commandRegistry = map[string]argBuilder{
	"search":         {build: buildSearchArgs, validate: validateSearchArgs},
	"index":          {build: buildIndexArgs, validate: nil},
	"index_status":   {build: buildStatusArgs, validate: nil},
	"index_update":   {build: buildUpdateArgs, validate: nil},
	"memory_store":   {build: buildMemoryStoreArgs, validate: nil},
	"memory_search":  {build: buildMemorySearchArgs, validate: validateMemorySearchArgs},
	"memory_promote": {build: buildMemoryPromoteArgs, validate: nil},
	"memory_list":    {build: buildMemoryListArgs, validate: nil},
	"memory_delete":  {build: buildMemoryDeleteArgs, validate: nil},
}

// RegisteredCommands returns a list of all registered command names.
// Useful for testing and documentation.
func RegisteredCommands() []string {
	cmds := make([]string, 0, len(commandRegistry))
	for cmd := range commandRegistry {
		cmds = append(cmds, cmd)
	}
	return cmds
}

func init() {
	if timeoutStr := os.Getenv("LLM_SEMANTIC_TIMEOUT"); timeoutStr != "" {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			CommandTimeout = time.Duration(seconds) * time.Second
		}
	}

	// Check if binary is in PATH
	if _, err := exec.LookPath(BinaryPath); err != nil {
		// Not in PATH, fallback to standard install location
		if _, err := os.Stat("/usr/local/bin/llm-semantic"); err == nil {
			BinaryPath = "/usr/local/bin/llm-semantic"
		}
	}
}

// SemanticConfig represents the semantic section of config.yaml
type SemanticConfig struct {
	CodeCollection   string `yaml:"code_collection"`
	CodeStorage      string `yaml:"code_storage"`
	DocsCollection   string `yaml:"docs_collection"`
	DocsStorage      string `yaml:"docs_storage"`
	MemoryCollection string `yaml:"memory_collection"`
	MemoryStorage    string `yaml:"memory_storage"`
}

// Config represents the root config.yaml structure
type Config struct {
	Semantic SemanticConfig `yaml:"semantic"`
}

// loadConfig loads configuration from a YAML file
func loadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %q: %w", configPath, err)
	}

	return &cfg, nil
}

// resolveProfileSettings applies profile-based defaults to args
// Profile values are used only if the corresponding arg is not already set
func resolveProfileSettings(args map[string]interface{}) error {
	profile, hasProfile := args["profile"].(string)
	configPath, hasConfig := args["config"].(string)

	// If no profile or no config, nothing to do
	if !hasProfile || profile == "" || !hasConfig || configPath == "" {
		return nil
	}

	// Load the config file
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}

	// Look up profile-specific values
	var collection, storage string

	switch profile {
	case "code":
		collection = cfg.Semantic.CodeCollection
		storage = cfg.Semantic.CodeStorage
	case "docs":
		collection = cfg.Semantic.DocsCollection
		storage = cfg.Semantic.DocsStorage
	case "memory":
		collection = cfg.Semantic.MemoryCollection
		storage = cfg.Semantic.MemoryStorage
	default:
		return fmt.Errorf("unknown profile: %s (valid profiles: code, docs, memory)", profile)
	}

	// Apply profile values as defaults (only if not explicitly set)
	if _, hasCollection := args["collection"].(string); !hasCollection || args["collection"] == "" {
		if collection != "" {
			args["collection"] = collection
		}
	}

	if _, hasStorage := args["storage"].(string); !hasStorage || args["storage"] == "" {
		if storage != "" {
			args["storage"] = storage
		}
	}

	return nil
}

// ExecuteHandler executes the appropriate command for a tool
func ExecuteHandler(toolName string, args map[string]interface{}) (string, error) {
	// Strip prefix to get command name
	cmdName := stripPrefix(toolName)

	// Resolve profile-based settings before building args
	if err := resolveProfileSettings(args); err != nil {
		return "", fmt.Errorf("failed to resolve profile settings: %w", err)
	}

	// Build command args
	cmdArgs, err := buildArgs(cmdName, args)
	if err != nil {
		return "", err
	}

	// Add --json and --min flags for machine-parseable, token-optimized output
	// Check for existing flags to avoid duplication
	hasJSON, hasMin := false, false
	for _, arg := range cmdArgs {
		if arg == "--json" {
			hasJSON = true
		} else if arg == "--min" {
			hasMin = true
		}
	}
	if !hasJSON {
		cmdArgs = append(cmdArgs, "--json")
	}
	if !hasMin {
		cmdArgs = append(cmdArgs, "--min")
	}

	// Execute command
	ctx, cancel := context.WithTimeout(context.Background(), CommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, BinaryPath, cmdArgs...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("command %q timed out after %v", cmdName, CommandTimeout)
	}

	if err != nil {
		// Include both the error and any output for better diagnostics
		if len(output) > 0 {
			return "", fmt.Errorf("command %q failed: %w\nOutput: %s", cmdName, err, string(output))
		}
		return "", fmt.Errorf("command %q failed: %w", cmdName, err)
	}

	return string(output), nil
}

func stripPrefix(toolName string) string {
	if len(toolName) > len(ToolPrefix) {
		return toolName[len(ToolPrefix):]
	}
	return toolName
}

// buildArgs builds CLI arguments for the given tool using the command registry
func buildArgs(cmdName string, args map[string]interface{}) ([]string, error) {
	builder, ok := commandRegistry[cmdName]
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", cmdName)
	}

	// Run validation if defined
	if builder.validate != nil {
		if err := builder.validate(args); err != nil {
			return nil, err
		}
	}

	return builder.build(args), nil
}

// validateSearchArgs validates search command arguments
func validateSearchArgs(args map[string]interface{}) error {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return fmt.Errorf("search requires a non-empty query")
	}
	return nil
}

// validateMemorySearchArgs validates memory_search command arguments
func validateMemorySearchArgs(args map[string]interface{}) error {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return fmt.Errorf("memory_search requires a non-empty query")
	}
	return nil
}

func buildSearchArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"search"}

	if query, ok := args["query"].(string); ok && query != "" {
		cmdArgs = append(cmdArgs, query)
	}
	if topK, ok := getInt(args, "top_k"); ok {
		cmdArgs = append(cmdArgs, "--top", strconv.Itoa(topK))
	}
	if threshold, ok := getThreshold(args); ok {
		cmdArgs = append(cmdArgs, "--threshold", fmt.Sprintf("%.4f", threshold))
	}
	if typeFilter, ok := args["type"].(string); ok && typeFilter != "" {
		cmdArgs = append(cmdArgs, "--type", typeFilter)
	}
	if pathFilter, ok := args["path"].(string); ok && pathFilter != "" {
		cmdArgs = append(cmdArgs, "--path", pathFilter)
	}
	// Note: --min is already added globally by ExecuteHandler, no need to add here
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	// Hybrid search parameters
	if getBool(args, "hybrid") {
		cmdArgs = append(cmdArgs, "--hybrid")
	}
	if fusionK, ok := getInt(args, "fusion_k"); ok {
		cmdArgs = append(cmdArgs, "--fusion-k", strconv.Itoa(fusionK))
	}
	if fusionAlpha, ok := getFloat(args, "fusion_alpha"); ok {
		cmdArgs = append(cmdArgs, "--fusion-alpha", fmt.Sprintf("%.4f", fusionAlpha))
	}

	// Recency boost parameters
	if getBool(args, "recency_boost") {
		cmdArgs = append(cmdArgs, "--recency-boost")
	}
	if recencyFactor, ok := getFloat(args, "recency_factor"); ok {
		cmdArgs = append(cmdArgs, "--recency-factor", fmt.Sprintf("%.4f", recencyFactor))
	}
	if recencyDecay, ok := getInt(args, "recency_decay"); ok {
		cmdArgs = append(cmdArgs, "--recency-decay", strconv.Itoa(recencyDecay))
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
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}
	if getBool(args, "recalibrate") {
		cmdArgs = append(cmdArgs, "--recalibrate")
	}
	if getBool(args, "skip_calibration") {
		cmdArgs = append(cmdArgs, "--skip-calibration")
	}

	return cmdArgs
}

func buildStatusArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"index-status"}

	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	return cmdArgs
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
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
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
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
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
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// getThreshold extracts and validates threshold from args (must be 0.0-1.0)
func getThreshold(args map[string]interface{}) (float64, bool) {
	if threshold, ok := getFloat(args, "threshold"); ok {
		// Clamp to valid range [0.0, 1.0]
		if threshold < 0.0 {
			threshold = 0.0
		} else if threshold > 1.0 {
			threshold = 1.0
		}
		return threshold, true
	}
	return 0, false
}

// Memory command builders

func buildMemoryStoreArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"memory", "store"}

	if question, ok := args["question"].(string); ok && question != "" {
		cmdArgs = append(cmdArgs, "--question", question)
	}
	if answer, ok := args["answer"].(string); ok && answer != "" {
		cmdArgs = append(cmdArgs, "--answer", answer)
	}
	if tags, ok := args["tags"].(string); ok && tags != "" {
		cmdArgs = append(cmdArgs, "--tags", tags)
	}
	if source, ok := args["source"].(string); ok && source != "" {
		cmdArgs = append(cmdArgs, "--source", source)
	}
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	return cmdArgs
}

func buildMemorySearchArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"memory", "search"}

	if query, ok := args["query"].(string); ok && query != "" {
		cmdArgs = append(cmdArgs, query)
	}
	if topK, ok := getInt(args, "top_k"); ok {
		cmdArgs = append(cmdArgs, "--top", strconv.Itoa(topK))
	}
	if threshold, ok := getThreshold(args); ok {
		cmdArgs = append(cmdArgs, "--threshold", fmt.Sprintf("%.4f", threshold))
	}
	if tags, ok := args["tags"].(string); ok && tags != "" {
		cmdArgs = append(cmdArgs, "--tags", tags)
	}
	if status, ok := args["status"].(string); ok && status != "" {
		cmdArgs = append(cmdArgs, "--status", status)
	}
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	return cmdArgs
}

func buildMemoryPromoteArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"memory", "promote"}

	if id, ok := args["id"].(string); ok && id != "" {
		cmdArgs = append(cmdArgs, id)
	}
	if target, ok := args["target"].(string); ok && target != "" {
		cmdArgs = append(cmdArgs, "--target", target)
	}
	if section, ok := args["section"].(string); ok && section != "" {
		cmdArgs = append(cmdArgs, "--section", section)
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "--force")
	}
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	return cmdArgs
}

func buildMemoryListArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"memory", "list"}

	if limit, ok := getInt(args, "limit"); ok {
		cmdArgs = append(cmdArgs, "--limit", strconv.Itoa(limit))
	}
	if status, ok := args["status"].(string); ok && status != "" {
		cmdArgs = append(cmdArgs, "--status", status)
	}
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	return cmdArgs
}

func buildMemoryDeleteArgs(args map[string]interface{}) []string {
	cmdArgs := []string{"memory", "delete"}

	if id, ok := args["id"].(string); ok && id != "" {
		cmdArgs = append(cmdArgs, id)
	}
	if getBool(args, "force") {
		cmdArgs = append(cmdArgs, "--force")
	}
	if storage, ok := args["storage"].(string); ok && storage != "" {
		cmdArgs = append(cmdArgs, "--storage", storage)
	}
	if collection, ok := args["collection"].(string); ok && collection != "" {
		cmdArgs = append(cmdArgs, "--collection", collection)
	}

	return cmdArgs
}
