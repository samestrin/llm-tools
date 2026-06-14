package commands

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileDrift is the resolved state of one referenced code path.
type FileDrift struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	CommitsSince int    `json:"commits_since"`
}

// DriftReport captures how far one entry has drifted from the code it cites.
type DriftReport struct {
	Created      string      `json:"created"`
	AgeDays      int         `json:"age_days"`
	Files        []FileDrift `json:"files"`
	DanglingRefs []string    `json:"dangling_refs,omitempty"`
	Placeholders bool        `json:"placeholders"`
	Flags        []string    `json:"flags,omitempty"`
}

// gitFirstAddDate returns the YYYY-MM-DD a path was first added (the earliest
// add event under --follow), or "" if untracked / not a git repo.
func gitFirstAddDate(repoRoot, rel string) string {
	out, err := runGitOutput(repoRoot, "log", "--diff-filter=A", "--follow", "--format=%as", "--", rel)
	if err != nil {
		return ""
	}
	lines := nonEmptyLines(out)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1] // last = earliest add
}

// gitCommitsSince counts commits touching rel with a date after `since`
// (YYYY-MM-DD). Returns 0 on any git error.
func gitCommitsSince(repoRoot, rel, since string) int {
	if since == "" {
		return 0
	}
	out, err := runGitOutput(repoRoot, "log", "--since="+since, "--format=%H", "--", rel)
	if err != nil {
		return 0
	}
	return len(nonEmptyLines(out))
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// listValue returns a frontmatter list key's string elements.
func (e *Entry) listValue(key string) []string {
	v, ok := e.value(key)
	if !ok {
		return nil
	}
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, el := range t {
			if s := strings.TrimSpace(toStr(el)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if s := strings.TrimSpace(t); s != "" {
			return []string{s}
		}
	}
	return nil
}

// analyzeDrift resolves an entry's code references against the working tree and
// git history. entryRel is the entry file's repo-relative path (used to derive
// `created` from git when the frontmatter lacks it). It never panics on a
// non-git directory — git-derived fields are simply empty.
func analyzeDrift(repoRoot, entryRel string, e *Entry, now time.Time) DriftReport {
	d := DriftReport{}

	created := ""
	if v, ok := e.value("created"); ok {
		created = dateOnly(toStr(v))
	}
	if created == "" {
		if v, ok := e.value("date"); ok {
			created = dateOnly(toStr(v))
		}
	}
	if created == "" {
		created = gitFirstAddDate(repoRoot, entryRel)
	}
	d.Created = created

	if created != "" {
		if ct, err := time.Parse("2006-01-02", created); err == nil {
			days := int(now.Sub(ct).Hours() / 24)
			if days < 0 {
				days = 0
			}
			d.AgeDays = days
		}
	}

	flags := map[string]bool{}
	for _, rel := range e.listValue("files") {
		exists := fileExistsIn(repoRoot, rel)
		cs := 0
		if exists {
			cs = gitCommitsSince(repoRoot, rel, created)
		}
		d.Files = append(d.Files, FileDrift{Path: rel, Exists: exists, CommitsSince: cs})
		if !exists {
			d.DanglingRefs = append(d.DanglingRefs, rel)
			flags["dangling_ref"] = true
		}
		if cs > 0 {
			flags["code_changed_after_capture"] = true
		}
	}

	d.Placeholders = placeholderRe.MatchString(e.body)
	if d.Placeholders {
		flags["incomplete"] = true
	}

	// Stable flag order.
	for _, f := range []string{"dangling_ref", "code_changed_after_capture", "incomplete"} {
		if flags[f] {
			d.Flags = append(d.Flags, f)
		}
	}
	return d
}

func fileExistsIn(repoRoot, rel string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, rel))
	return err == nil
}

// dateOnly trims a value to its leading YYYY-MM-DD if present.
func dateOnly(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		if _, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return s[:10]
		}
	}
	return s
}
