package commands

import (
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// canonicalKeys is the authoritative knowledge-entry frontmatter schema, in
// canonical order. New keys appended by repair are added in this order.
var canonicalKeys = []string{
	"id", "question", "created", "last_retrieved",
	"sprints", "files", "tags", "retrievals", "status", "type",
}

// softKeys are values repair will NEVER invent — only a human or the model
// (via the --audit report) can fill them. Repair leaves them empty + flagged.
var softKeys = map[string]bool{"question": true, "tags": true}

// listKeys render as YAML block sequences (or `[]` when empty).
var listKeys = map[string]bool{"sprints": true, "files": true, "tags": true}

// keyAliases maps a legacy frontmatter key to its canonical name. Repair renames
// the line in place (preserving the value) when the canonical key is absent.
var keyAliases = map[string]string{"date": "created"}

// fmBlockRe captures the leading YAML frontmatter block. Group 1 is the inner
// YAML (no delimiters). The body is everything after the closing `---`.
var fmBlockRe = regexp.MustCompile(`(?s)^---[ \t]*\r?\n(.*?)\r?\n---[ \t]*`)

// topKeyRe matches an indent-0 `key:` at the start of a frontmatter line.
// Hyphens are allowed so non-canonical keys are detected (and preserved), not
// silently missed.
var topKeyRe = regexp.MustCompile(`^([A-Za-z0-9_-]+):`)

// codeFileRe finds repo-relative code paths in a body (must contain a slash and
// a known code extension). An optional `:line` suffix is not captured.
var codeFileRe = regexp.MustCompile(`[A-Za-z0-9_.][A-Za-z0-9_./-]*\.[A-Za-z0-9]+`)

var codeExts = map[string]bool{
	"go": true, "rs": true, "ts": true, "tsx": true, "js": true, "jsx": true,
	"py": true, "rb": true, "java": true, "kt": true, "c": true, "cc": true,
	"cpp": true, "h": true, "hpp": true, "cs": true, "php": true, "swift": true,
	"scala": true, "sh": true, "bash": true, "sql": true, "yaml": true, "yml": true,
	"toml": true, "json": true, "proto": true, "vue": true, "svelte": true,
}

// titleRe pulls the first `# Heading` from a body.
var titleRe = regexp.MustCompile(`(?m)^#[ \t]+(.+?)[ \t]*$`)

// placeholderRe detects unfilled template stubs left by capture.
var placeholderRe = regexp.MustCompile(`\[(from context|conditions|Example code|conditions\.\.\.)\]`)

// Entry is one parsed knowledge-base markdown file.
type Entry struct {
	File           string        // basename
	HasFrontmatter bool          // a leading --- block was found
	ParseErr       string        // non-empty if the frontmatter YAML failed to parse
	front          yaml.MapSlice // parsed frontmatter (ordered)
	raw            string        // full original file content
	rawFM          string        // inner frontmatter text (no delimiters)
	body           string        // bytes after the closing --- (preserved verbatim)
	Title          string        // first body heading
}

// parseEntry splits a knowledge entry into frontmatter + body without mutating
// either. A missing or malformed frontmatter block is surfaced, never panicked.
func parseEntry(filename string, content []byte) *Entry {
	e := &Entry{File: filename, raw: string(content)}
	loc := fmBlockRe.FindStringSubmatchIndex(e.raw)
	if loc == nil {
		// No frontmatter; the whole file is body.
		e.body = e.raw
		e.Title = firstTitle(e.raw)
		return e
	}
	e.HasFrontmatter = true
	e.rawFM = e.raw[loc[2]:loc[3]]
	e.body = e.raw[loc[1]:] // everything after the closing ---
	e.Title = firstTitle(e.body)

	var ms yaml.MapSlice
	if err := yaml.Unmarshal([]byte(e.rawFM), &ms); err != nil {
		e.ParseErr = err.Error()
		return e
	}
	e.front = ms
	return e
}

func firstTitle(s string) string {
	if m := titleRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// topKeys returns the indent-0 frontmatter keys in order.
func (e *Entry) topKeys() []string {
	var keys []string
	for _, line := range strings.Split(e.rawFM, "\n") {
		if m := topKeyRe.FindStringSubmatch(line); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys
}

// SchemaReport is the per-entry frontmatter conformance result.
type SchemaReport struct {
	Conformant bool              `json:"conformant"`
	Missing    []string          `json:"missing,omitempty"`
	Unknown    []string          `json:"unknown,omitempty"`
	Aliased    map[string]string `json:"aliased,omitempty"`
	NeedsInput []string          `json:"needs_input,omitempty"`
}

func (e *Entry) schemaReport() SchemaReport {
	r := SchemaReport{}
	if !e.HasFrontmatter {
		r.Missing = append([]string{}, canonicalKeys...)
		return r
	}
	present := map[string]bool{}
	for _, k := range e.topKeys() {
		present[k] = true
	}
	// Resolve aliases into an effective-present set.
	effective := map[string]bool{}
	for k := range present {
		effective[k] = true
	}
	for alias, target := range keyAliases {
		if present[alias] {
			if !present[target] {
				if r.Aliased == nil {
					r.Aliased = map[string]string{}
				}
				r.Aliased[alias] = target
				effective[target] = true
			}
		}
	}
	canon := map[string]bool{}
	for _, k := range canonicalKeys {
		canon[k] = true
		if !effective[k] {
			r.Missing = append(r.Missing, k)
		}
	}
	for _, k := range e.topKeys() {
		if !canon[k] && keyAliases[k] == "" {
			r.Unknown = append(r.Unknown, k)
		}
	}
	// Soft fields that are missing or empty need human/model input.
	for _, k := range []string{"question", "tags"} {
		if !present[k] || e.isEmpty(k) {
			r.NeedsInput = append(r.NeedsInput, k)
		}
	}
	r.Conformant = e.ParseErr == "" &&
		len(r.Missing) == 0 && len(r.Unknown) == 0 &&
		len(r.Aliased) == 0 && len(r.NeedsInput) == 0
	return r
}

// isEmpty reports whether a frontmatter key's value is empty ("" or []).
func (e *Entry) isEmpty(key string) bool {
	for _, it := range e.front {
		if k, _ := it.Key.(string); k == key {
			switch v := it.Value.(type) {
			case nil:
				return true
			case string:
				return strings.TrimSpace(v) == ""
			case []interface{}:
				return len(v) == 0
			default:
				return false
			}
		}
	}
	return true // absent counts as empty
}

func (e *Entry) value(key string) (interface{}, bool) {
	for _, it := range e.front {
		if k, _ := it.Key.(string); k == key {
			return it.Value, true
		}
	}
	return nil, false
}

// deriveFilesFromBody extracts repo-relative code paths cited in a body. Bare
// basenames (no slash) and non-code extensions are ignored.
func deriveFilesFromBody(body string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range codeFileRe.FindAllString(body, -1) {
		m = strings.Trim(m, ".,;:)('\"`")
		if !strings.Contains(m, "/") {
			continue
		}
		dot := strings.LastIndex(m, ".")
		if dot < 0 {
			continue
		}
		if !codeExts[strings.ToLower(m[dot+1:])] {
			continue
		}
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// inferType guesses the entry type from its filename prefix.
func inferType(filename string) string {
	switch {
	case strings.HasPrefix(filename, "clarifications-"):
		return "clarifications"
	case strings.HasPrefix(filename, "sprint-"):
		return "sprint-learning"
	default:
		return "knowledge"
	}
}

func synthID(created, title string) string {
	h := fmt.Sprintf("%x", md5.Sum([]byte(title)))[:6]
	return "mem-" + created + "-" + h
}

// repair returns the entry's content with frontmatter normalized to the
// canonical schema: alias keys renamed in place and missing canonical keys
// appended (derived where possible, soft fields left empty). The body is
// preserved byte-for-byte; existing keys, their order, and unknown keys are
// untouched. gitDate ("" if untracked) supplies a missing `created`. The bool
// reports whether anything changed; when false the original bytes are returned.
func (e *Entry) repair(gitDate string) ([]byte, bool) {
	if !e.HasFrontmatter || e.ParseErr != "" {
		return []byte(e.raw), false
	}
	lines := strings.Split(e.rawFM, "\n")
	present := map[string]bool{}
	for _, k := range e.topKeys() {
		present[k] = true
	}
	changed := false

	// Rename alias lines in place (only when the canonical target is absent).
	for alias, target := range keyAliases {
		if present[alias] && !present[target] {
			for i, line := range lines {
				if m := topKeyRe.FindStringSubmatch(line); m != nil && m[1] == alias {
					lines[i] = target + line[len(alias):]
					present[target] = true
					delete(present, alias)
					changed = true
					break
				}
			}
		}
	}

	// Resolve the effective `created` for id synthesis + default.
	created := gitDate
	if v, ok := e.value("created"); ok && toStr(v) != "" {
		created = toStr(v)
	} else if v, ok := e.value("date"); ok && toStr(v) != "" {
		created = toStr(v)
	}

	var appends []string
	for _, k := range canonicalKeys {
		if present[k] {
			continue
		}
		appends = append(appends, e.renderMissing(k, created, gitDate)...)
		changed = true
	}

	if !changed {
		return []byte(e.raw), false
	}

	newFM := strings.Join(lines, "\n")
	if len(appends) > 0 {
		newFM += "\n" + strings.Join(appends, "\n")
	}
	return []byte("---\n" + newFM + "\n---" + e.body), true
}

// renderMissing produces the YAML line(s) for a canonical key absent from the
// frontmatter.
func (e *Entry) renderMissing(key, created, gitDate string) []string {
	switch key {
	case "id":
		return []string{"id: " + synthID(created, e.Title)}
	case "created":
		return []string{"created: " + quoteIfEmpty(gitDate)}
	case "files":
		files := deriveFilesFromBody(e.body)
		return renderList("files", files)
	case "retrievals":
		return []string{"retrievals: 0"}
	case "status":
		return []string{"status: active"}
	case "type":
		return []string{"type: " + inferType(e.File)}
	case "last_retrieved":
		return []string{`last_retrieved: ""`}
	default:
		// question (soft scalar), sprints/tags (lists). Never invent soft values.
		if listKeys[key] {
			return renderList(key, nil)
		}
		return []string{key + `: ""`}
	}
}

func renderList(key string, items []string) []string {
	if len(items) == 0 {
		return []string{key + ": []"}
	}
	out := []string{key + ":"}
	for _, it := range items {
		out = append(out, "- "+it)
	}
	return out
}

func quoteIfEmpty(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

func kbContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// toStr renders a scalar frontmatter value as a string.
func toStr(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}
