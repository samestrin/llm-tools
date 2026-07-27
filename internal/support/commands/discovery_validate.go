package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var (
	discoveryValidatePath  string
	discoveryValidateRoot  string
	discoveryValidateWrite bool
	discoveryValidateJSON  bool
	discoveryValidateMin   bool
)

// DiscoveryValidateItem holds the result for a single checked or skipped path.
type DiscoveryValidateItem struct {
	Field  string `json:"field"`
	Index  int    `json:"index"`
	Path   string `json:"path"`
	Status string `json:"status"` // missing|deprecated|skipped
	Reason string `json:"reason,omitempty"`
}

// DiscoveryValidateSummary holds aggregate counts.
type DiscoveryValidateSummary struct {
	Checked              int `json:"checked"`
	Missing              int `json:"missing"`
	AlreadyDeprecated    int `json:"already_deprecated"`
	InformationalSkipped int `json:"informational_skipped"`
}

// DiscoveryValidateResult is the full JSON payload returned by discovery-validate.
type DiscoveryValidateResult struct {
	Items   []DiscoveryValidateItem  `json:"items"`
	Summary DiscoveryValidateSummary `json:"summary"`
	Written bool                     `json:"written"`
}

// discoveryValidateArraySpec describes one array field in codebase-discovery.json
// whose entries carry a path (or paths) that can be auto-deprecated when stale.
type discoveryValidateArraySpec struct {
	Field string
	Kind  string // "single" | "filesArray" | "location"
	Key   string // object key holding the path (or array of paths, or "file:symbol" location)
}

var discoveryValidateArraySpecs = []discoveryValidateArraySpec{
	{Field: "files_to_modify", Kind: "single", Key: "path"},
	{Field: "related_files", Kind: "single", Key: "path"},
	{Field: "semantic_matches", Kind: "single", Key: "file"},
	{Field: "reusable_components", Kind: "single", Key: "path"},
	{Field: "existing_patterns", Kind: "filesArray", Key: "files"},
	{Field: "integration_points", Kind: "location", Key: "location"},
}

// discoveryValidateInformationalSpec describes a single-object anchor field that is
// reported when stale but never auto-mutated (requires human/model judgement).
type discoveryValidateInformationalSpec struct {
	Field string
	Key   string
}

var discoveryValidateInformationalSpecs = []discoveryValidateInformationalSpec{
	{Field: "build_from", Key: "primary_file"},
	{Field: "test_patterns", Key: "test_location"},
	{Field: "test_patterns", Key: "example_test"},
}

func newDiscoveryValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discovery-validate",
		Short: "Verify paths cited in codebase-discovery.json still exist",
		Long: `Reads a codebase-discovery.json file and checks whether each cited file
path still exists under --root. Report-only by default; pass --write to mark
stale entries "status": "deprecated" with a "deprecated_reason" (never deletes
entries, and never overwrites an entry's existing "reason" field).

Checked fields: files_to_modify[*].path, related_files[*].path,
semantic_matches[*].file, reusable_components[*].path,
existing_patterns[*].files[], integration_points[*].location (file segment).

Reported but never auto-mutated (single-object anchors — a human/model call,
not a deterministic one): build_from.primary_file, test_patterns.test_location,
test_patterns.example_test. files_to_create[*].path is skipped entirely — those
files are expected not to exist yet.

Entries already carrying "status": "deprecated" are left untouched (idempotent).`,
		RunE: runDiscoveryValidate,
	}
	cmd.Flags().StringVar(&discoveryValidatePath, "path", "", "Path to codebase-discovery.json (required)")
	cmd.Flags().StringVar(&discoveryValidateRoot, "root", ".", "Root for resolving relative paths cited in the JSON")
	cmd.Flags().BoolVar(&discoveryValidateWrite, "write", false, "Mark stale entries deprecated in place (default: report-only)")
	cmd.Flags().BoolVar(&discoveryValidateJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&discoveryValidateMin, "min", false, "Minimal output")
	cmd.MarkFlagRequired("path")
	return cmd
}

func runDiscoveryValidate(cmd *cobra.Command, _ []string) error {
	raw, err := os.ReadFile(discoveryValidatePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	if !gjson.ValidBytes(raw) {
		return fmt.Errorf("invalid JSON: %s", discoveryValidatePath)
	}

	root := discoveryValidateRoot
	if root == "" {
		root = "."
	}

	result := &DiscoveryValidateResult{Items: make([]DiscoveryValidateItem, 0)}
	changed := false

	for _, spec := range discoveryValidateArraySpecs {
		arr := gjson.GetBytes(raw, spec.Field)
		if !arr.Exists() || !arr.IsArray() {
			continue
		}

		idx := -1
		arr.ForEach(func(_, entry gjson.Result) bool {
			idx++

			if entry.Get("status").String() == "deprecated" {
				result.Summary.AlreadyDeprecated++
				return true
			}

			switch spec.Kind {
			case "single":
				p := entry.Get(spec.Key).String()
				if p == "" {
					return true
				}
				result.Summary.Checked++
				if discoveryPathExists(root, p) {
					return true
				}
				result.Summary.Missing++
				reason := fmt.Sprintf("Path not found: %s (auto-validated by discovery-validate)", p)
				item := DiscoveryValidateItem{Field: spec.Field, Index: idx, Path: p, Status: "missing", Reason: reason}
				if discoveryValidateWrite {
					raw = discoveryMarkDeprecated(raw, spec.Field, idx, reason)
					item.Status = "deprecated"
					changed = true
				}
				result.Items = append(result.Items, item)

			case "filesArray":
				files := entry.Get(spec.Key)
				if !files.Exists() || !files.IsArray() {
					return true
				}
				var missing []string
				files.ForEach(func(_, fv gjson.Result) bool {
					f := fv.String()
					if f == "" {
						return true
					}
					result.Summary.Checked++
					if !discoveryPathExists(root, f) {
						missing = append(missing, f)
					}
					return true
				})
				if len(missing) == 0 {
					return true
				}
				result.Summary.Missing += len(missing)
				reason := fmt.Sprintf("Path(s) not found: %s (auto-validated by discovery-validate)", strings.Join(missing, ", "))
				item := DiscoveryValidateItem{Field: spec.Field, Index: idx, Path: strings.Join(missing, ", "), Status: "missing", Reason: reason}
				if discoveryValidateWrite {
					raw = discoveryMarkDeprecated(raw, spec.Field, idx, reason)
					item.Status = "deprecated"
					changed = true
				}
				result.Items = append(result.Items, item)

			case "location":
				loc := entry.Get(spec.Key).String()
				colonIdx := strings.IndexByte(loc, ':')
				if colonIdx < 0 {
					if loc != "" {
						result.Summary.InformationalSkipped++
						result.Items = append(result.Items, DiscoveryValidateItem{
							Field: spec.Field, Index: idx, Path: loc, Status: "skipped",
							Reason: "cannot parse file segment from location (expected file:symbol)",
						})
					}
					return true
				}
				p := loc[:colonIdx]
				if p == "" {
					return true
				}
				result.Summary.Checked++
				if discoveryPathExists(root, p) {
					return true
				}
				result.Summary.Missing++
				reason := fmt.Sprintf("Path not found: %s (auto-validated by discovery-validate)", p)
				item := DiscoveryValidateItem{Field: spec.Field, Index: idx, Path: p, Status: "missing", Reason: reason}
				if discoveryValidateWrite {
					raw = discoveryMarkDeprecated(raw, spec.Field, idx, reason)
					item.Status = "deprecated"
					changed = true
				}
				result.Items = append(result.Items, item)
			}

			return true
		})
	}

	for _, spec := range discoveryValidateInformationalSpecs {
		v := gjson.GetBytes(raw, spec.Field+"."+spec.Key).String()
		if v == "" || discoveryPathExists(root, v) {
			continue
		}
		result.Summary.InformationalSkipped++
		result.Items = append(result.Items, DiscoveryValidateItem{
			Field: spec.Field, Index: -1, Path: v, Status: "skipped",
			Reason: "informational only — requires human/model judgement, not auto-deprecated",
		})
	}

	if discoveryValidateWrite && changed {
		if err := os.WriteFile(discoveryValidatePath, raw, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		result.Written = true
	}

	formatter := output.New(discoveryValidateJSON, discoveryValidateMin, cmd.OutOrStdout())
	return formatter.Print(result, printDiscoveryValidateText)
}

// discoveryPathExists resolves p against root (unless already absolute) and stats it.
func discoveryPathExists(root, p string) bool {
	full := p
	if !filepath.IsAbs(p) {
		full = filepath.Join(root, p)
	}
	_, err := os.Stat(full)
	return err == nil
}

// discoveryMarkDeprecated sets status/deprecated_reason on raw[field][idx] via sjson,
// preserving the surrounding document's formatting and key order. The reason is
// written to "deprecated_reason" rather than "reason" because several schema
// entries (e.g. files_to_modify[*].reason) already use "reason" for unrelated,
// human-authored content that must not be clobbered. Errors are ignored (best
// effort) since sjson only fails on malformed paths, which cannot happen here.
func discoveryMarkDeprecated(raw []byte, field string, idx int, reason string) []byte {
	statusPath := fmt.Sprintf("%s.%d.status", field, idx)
	reasonPath := fmt.Sprintf("%s.%d.deprecated_reason", field, idx)
	if updated, err := sjson.SetBytes(raw, statusPath, "deprecated"); err == nil {
		raw = updated
	}
	if updated, err := sjson.SetBytes(raw, reasonPath, reason); err == nil {
		raw = updated
	}
	return raw
}

func printDiscoveryValidateText(w io.Writer, data interface{}) {
	r := data.(*DiscoveryValidateResult)
	fmt.Fprintf(w, "discovery_validate: %d path(s) checked — %d missing, %d already deprecated, %d informational-only\n",
		r.Summary.Checked, r.Summary.Missing, r.Summary.AlreadyDeprecated, r.Summary.InformationalSkipped)

	for _, it := range r.Items {
		if it.Index < 0 {
			fmt.Fprintf(w, "%s: %s %s — %s\n", strings.ToUpper(it.Status), it.Field, it.Path, it.Reason)
			continue
		}
		fmt.Fprintf(w, "%s: %s.%d %s — %s\n", strings.ToUpper(it.Status), it.Field, it.Index, it.Path, it.Reason)
	}

	if r.Written {
		fmt.Fprintln(w, "WRITTEN: true")
	}
}

func init() {
	RootCmd.AddCommand(newDiscoveryValidateCmd())
}
