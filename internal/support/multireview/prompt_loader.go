// Package multireview ...
//
// Per-agent prompt loader. multi_review loads task messages from
// markdown files at runtime so per-model prompt overrides can be
// updated without rebuilding the binary. See the plan and the
// README in claude-prompts/.planning/.templates/multi-review-prompts/
// for the schema and authoring conventions.
package multireview

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// PromptVars is the set of variables a prompt template can reference.
// Any field added here MUST be safe to render in the default text/template
// pipeline (no nil dereferences, sensible zero values).
type PromptVars struct {
	// DiffPath is the absolute path to the pre-computed unified diff
	// inside the gateway container, e.g.
	// /tmp/multi-review-1778868610/diff.txt
	DiffPath string

	// DiffBytes is the byte count of the diff file (from wc -c).
	DiffBytes int64

	// DiffLines is the line count of the diff file (from wc -l).
	DiffLines int

	// DiffMB is DiffBytes / 1_000_000 pre-formatted as a float, for
	// display in the large-diff section. Pre-computing avoids needing
	// template math.
	DiffMB float64

	// LargeDiff is true when DiffBytes > 1_000_000. Templates branch on
	// this via {{if .LargeDiff}}...{{end}} for the directive >1MB
	// workflow block.
	LargeDiff bool

	// BaseRef and HeadRef bound the diff range.
	BaseRef string
	HeadRef string

	// RemoteRepo is the container-local clone path the reviewer can
	// inspect for context, e.g. /tmp/multi-review-1778868610/myrepo/.
	RemoteRepo string

	// AgentName is the reviewer name (bruce/greta/kai/mira/dax/otto).
	// Available to templates for self-reference.
	AgentName string
}

// ResolvePromptDir returns the directory the loader reads per-agent
// prompts from. Resolution order:
//  1. LLM_TOOLS_MULTI_REVIEW_PROMPTS env var, if set and non-empty
//  2. $HOME/.llm-tools/multi-review/prompts (default)
//
// The directory is NOT required to exist — a missing dir signals the
// caller to use its hardcoded fallback.
func ResolvePromptDir() string {
	if v := os.Getenv("LLM_TOOLS_MULTI_REVIEW_PROMPTS"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// On the rare host where UserHomeDir fails, surface a sentinel
		// path that won't accidentally match anything. Caller will see
		// "missing dir" and fall back to hardcoded.
		return "/dev/null/llm-tools-prompts"
	}
	return filepath.Join(home, ".llm-tools", "multi-review", "prompts")
}

// LoadAgentPrompt loads and renders the per-agent task message from
// disk. Fallback chain:
//
//  1. <dir>/<agent>.md (with template syntax validated)
//  2. <dir>/_base.md
//  3. "" (empty string — caller falls back to hardcoded)
//
// A bad template at any layer falls through to the next; LoadAgentPrompt
// never returns an error for a missing or malformed file. The only error
// returned is for read failures other than not-found (permission denied,
// I/O error). Missing files are not errors.
//
// agent names are sanitized to prevent path traversal: only the basename
// of the agent argument is used to construct the file path.
func LoadAgentPrompt(dir, agent string, vars PromptVars) (string, error) {
	if dir == "" {
		return "", nil
	}
	// Sanitize agent name. We only join filepath.Base(agent) so
	// "../etc/passwd" collapses to "passwd" — guaranteed to be looked
	// up only inside `dir`, never outside.
	safeAgent := filepath.Base(strings.TrimSpace(agent))
	if safeAgent == "." || safeAgent == "" || safeAgent == string(filepath.Separator) {
		safeAgent = ""
	}

	// Try agent-specific first.
	if safeAgent != "" {
		agentPath := filepath.Join(dir, safeAgent+".md")
		rendered, ok, err := tryRender(agentPath, vars)
		if err != nil {
			// Read error (permissions etc.) — propagate.
			return "", err
		}
		if ok {
			return rendered, nil
		}
	}

	// Fall back to _base.md.
	basePath := filepath.Join(dir, "_base.md")
	rendered, ok, err := tryRender(basePath, vars)
	if err != nil {
		return "", err
	}
	if ok {
		return rendered, nil
	}

	// Nothing usable — caller will fall back to hardcoded default.
	return "", nil
}

// tryRender reads and renders a single template file. Returns:
//
//	(rendered, true,  nil) when the file exists AND parses AND executes
//	("",       false, nil) when the file is missing OR template-broken
//	("",       false, err) only on a real I/O error (not os.IsNotExist)
//
// The "broken template" case is intentionally treated as "missing" so
// LoadAgentPrompt's fallback chain takes over. A single bad agent file
// must not crash the run.
func tryRender(path string, vars PromptVars) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	tmpl, err := template.New(filepath.Base(path)).Parse(string(data))
	if err != nil {
		// Parse error — treat as missing, fall through.
		return "", false, nil
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		// Execution error (missing field, bad function call) — treat as missing.
		return "", false, nil
	}
	return buf.String(), true, nil
}
