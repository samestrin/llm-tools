package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	kaDir        string
	kaRepair     bool
	kaAllEntries bool
	kaJSON       bool
	kaMin        bool
)

func newKnowledgeAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge-audit",
		Short: "Audit a .knowledge/ directory for frontmatter drift and stale code references",
		Long: `Scan every .md entry in a knowledge directory and report, per entry:
schema conformance against the canonical frontmatter (missing/unknown/aliased
keys, soft fields needing input), and drift against the codebase (git-derived
created date, age, whether cited files still exist and how many commits touched
them since capture, and unfilled template placeholders).

Read-only by default. With --repair-schema it deterministically normalizes
frontmatter (rename aliases, fill derivable + bookkeeping keys, synthesize a
missing id, derive files from body citations) — preserving the body byte for
byte, the filename, and any existing id. It never invents question/tags. Repair
is idempotent.

Output is JSON: {entries:[...], summary:{...}}. Each entry carries a flags array
(code_changed_after_capture, dangling_ref, incomplete, needs_input) that tells a
caller which entries warrant a semantic (model) review.`,
		RunE: runKnowledgeAudit,
	}
	cmd.Flags().StringVar(&kaDir, "dir", "", "Path to the .knowledge directory (required)")
	cmd.Flags().BoolVar(&kaRepair, "repair-schema", false, "Write deterministic frontmatter normalization (default off = read-only report)")
	cmd.Flags().BoolVar(&kaAllEntries, "all-entries", false, "Enumerate every entry (default: only non-clean entries appear in `entries`; all are counted in `summary`)")
	cmd.Flags().BoolVar(&kaJSON, "json", true, "Output as JSON (default true)")
	cmd.Flags().BoolVar(&kaMin, "min", false, "Minimal output format")
	cmd.MarkFlagRequired("dir")
	return cmd
}

func init() {
	RootCmd.AddCommand(newKnowledgeAuditCmd())
}

func runKnowledgeAudit(cmd *cobra.Command, _ []string) error {
	if kaDir == "" {
		return fmt.Errorf("--dir required")
	}
	info, err := os.Stat(kaDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("--dir %q is not a readable directory: %v", kaDir, err)
	}
	result, err := auditDir(kaDir, kaRepair, kaAllEntries, time.Now())
	if err != nil {
		return err
	}
	formatter := output.New(kaJSON, kaMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(*AuditResult)
		fmt.Fprintf(w, "%d entries: %d nonconformant, %d stale, %d dangling, %d incomplete, %d repaired\n",
			r.Summary.Total, r.Summary.Nonconformant, r.Summary.Stale, r.Summary.Dangling, r.Summary.Incomplete, r.Summary.Repaired)
	})
}

// EntryAudit is the per-entry audit result.
type EntryAudit struct {
	File     string       `json:"file"`
	ID       string       `json:"id,omitempty"`
	Title    string       `json:"title,omitempty"`
	Schema   SchemaReport `json:"schema"`
	Drift    DriftReport  `json:"drift"`
	Repaired bool         `json:"repaired,omitempty"`
	Flags    []string     `json:"flags,omitempty"`
}

// AuditSummary aggregates the KB's health. Counts cover every entry; `emitted`
// is how many appear in the result's entries list (non-clean only, unless
// --all-entries).
type AuditSummary struct {
	Total          int `json:"total"`
	Emitted        int `json:"emitted"`
	Nonconformant  int `json:"nonconformant"`
	MissingCreated int `json:"missing_created"`
	MissingFiles   int `json:"missing_files"`
	Dangling       int `json:"dangling"`
	Stale          int `json:"stale"`
	Incomplete     int `json:"incomplete"`
	Repaired       int `json:"repaired"`
}

// AuditResult is the full audit output.
type AuditResult struct {
	Entries []EntryAudit `json:"entries"`
	Summary AuditSummary `json:"summary"`
}

// auditDir audits every .md entry in dir. When repair is true it normalizes
// frontmatter in place (git-backed; the caller should run on a clean tree). By
// default only non-clean entries are enumerated in the result (all are counted
// in the summary); allEntries includes the clean ones too.
func auditDir(dir string, repair, allEntries bool, now time.Time) (*AuditResult, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	repoRoot := gitTopLevel(dir)

	var names []string
	for _, de := range des {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		names = append(names, de.Name())
	}
	sort.Strings(names)

	res := &AuditResult{Entries: []EntryAudit{}}
	for _, name := range names {
		full := filepath.Join(dir, name)
		content, rerr := os.ReadFile(full)
		if rerr != nil {
			continue
		}
		e := parseEntry(name, content)
		entryRel := name
		if repoRoot != "" {
			if rel, rerr := filepath.Rel(repoRoot, full); rerr == nil {
				entryRel = rel
			}
		}
		origFilesEmpty := len(e.listValue("files")) == 0

		schema := e.schemaReport()
		drift := analyzeDrift(repoRoot, entryRel, e, now)

		ea := EntryAudit{File: name, ID: idOf(e), Title: e.Title, Schema: schema, Drift: drift}

		if repair && e.HasFrontmatter && e.ParseErr == "" {
			newContent, changed := e.repair(drift.Created)
			if changed {
				// A failed write must NOT be silent — surface it so the caller
				// knows the KB was not fully repaired (e.g. read-only dir).
				if werr := atomicWrite(full, newContent); werr != nil {
					return nil, fmt.Errorf("repair %s: %w", name, werr)
				}
				ea.Repaired = true
				res.Summary.Repaired++
				// Re-evaluate post-repair so the report reflects the new state.
				e2 := parseEntry(name, newContent)
				ea.Schema = e2.schemaReport()
				ea.Drift = analyzeDrift(repoRoot, entryRel, e2, now)
				ea.ID = idOf(e2)
			}
		}

		ea.Flags = mergeAttentionFlags(ea.Schema, ea.Drift)

		res.Summary.Total++
		if !ea.Schema.Conformant {
			res.Summary.Nonconformant++
		}
		if ea.Drift.Created == "" {
			res.Summary.MissingCreated++
		}
		if origFilesEmpty {
			res.Summary.MissingFiles++
		}
		if hasStr(ea.Drift.Flags, "dangling_ref") {
			res.Summary.Dangling++
		}
		if hasStr(ea.Drift.Flags, "code_changed_after_capture") {
			res.Summary.Stale++
		}
		if ea.Drift.Placeholders {
			res.Summary.Incomplete++
		}
		// Enumerate only entries that warrant attention (or were repaired this
		// run), so a healthy KB returns a tiny payload and the model isn't fed
		// every clean entry. --all-entries overrides.
		if allEntries || !ea.Schema.Conformant || len(ea.Flags) > 0 || ea.Repaired {
			res.Entries = append(res.Entries, ea)
		}
	}
	res.Summary.Emitted = len(res.Entries)
	return res, nil
}

// mergeAttentionFlags is the set of signals that warrant a semantic (model)
// review of an entry: the drift flags plus needs_input when soft fields are
// blank.
func mergeAttentionFlags(s SchemaReport, d DriftReport) []string {
	flags := append([]string{}, d.Flags...)
	if len(s.NeedsInput) > 0 {
		flags = append(flags, "needs_input")
	}
	return flags
}

func idOf(e *Entry) string {
	if v, ok := e.value("id"); ok {
		return toStr(v)
	}
	return ""
}

func gitTopLevel(dir string) string {
	out, err := runGitOutput(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// atomicWrite writes content to path via a temp file + rename, preserving the
// existing file mode when present.
func atomicWrite(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ka-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func hasStr(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
