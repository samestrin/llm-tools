package commands

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/samestrin/llm-tools/pkg/output"
	"github.com/spf13/cobra"
)

var (
	tcPackages string
	tcConfig   string
	tcJSON     bool
	tcMin      bool
)

func newTierClassifierCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tier-classifier",
		Short: "Classify packages into documentation tiers (deterministic passes 1-3)",
		Long: `Assign each package a tier (critical/important/pattern/utility/skip) using the
deterministic passes from /init-documentation:
  Pass 1 explicit  — exact name in the config's "packages" map
  Pass 2 patterns  — first matching glob in "patterns" (path.Match)
  Pass 3 categories — first "categories" keyword the name contains
First match wins within a pass; earlier passes take precedence. Unmatched
packages are returned in "unassigned" for the model's Pass 4 (ecosystem
convention) and Pass 5 (default to utility).

Output is JSON: {assigned:{pkg:{tier,pass}}, unassigned:[...], counts:{...}}.`,
		RunE: runTierClassifier,
	}
	cmd.Flags().StringVar(&tcPackages, "packages", "", "Comma-separated package names (required)")
	cmd.Flags().StringVar(&tcConfig, "config", "", "Path to package-tiers.yaml (packages/patterns/categories)")
	cmd.Flags().BoolVar(&tcJSON, "json", true, "Output as JSON (default true)")
	cmd.Flags().BoolVar(&tcMin, "min", false, "Minimal output format")
	cmd.MarkFlagRequired("packages")
	return cmd
}

func runTierClassifier(cmd *cobra.Command, _ []string) error {
	pkgs := splitCSV(tcPackages)
	if len(pkgs) == 0 {
		return fmt.Errorf("--packages required (comma-separated names)")
	}
	cfg := TierConfig{Explicit: map[string]string{}}
	if tcConfig != "" {
		loaded, err := loadTierConfig(tcConfig)
		if err != nil {
			return err
		}
		cfg = loaded
	}
	result := classifyTiers(pkgs, cfg)
	formatter := output.New(tcJSON, tcMin, cmd.OutOrStdout())
	return formatter.Print(result, func(w io.Writer, data interface{}) {
		r := data.(*TierResult)
		fmt.Fprintf(w, "%d assigned, %d unassigned\n", len(r.Assigned), len(r.Unassigned))
	})
}

func init() {
	RootCmd.AddCommand(newTierClassifierCmd())
}

// loadTierConfig reads package-tiers.yaml into a TierConfig. `packages` is a
// name->tier map. `patterns` and `categories` may each be either an ordered
// YAML sequence (of {pattern|keyword, tier} maps) or a map (glob/keyword ->
// tier), in which case rules are ordered most-specific-first (longer key wins
// the tie) for deterministic first-match.
func loadTierConfig(path string) (TierConfig, error) {
	raw, err := readYAMLAsMap(path)
	if err != nil {
		return TierConfig{}, fmt.Errorf("--config %s: %w", path, err)
	}
	cfg := TierConfig{Explicit: map[string]string{}}
	if pm, ok := raw["packages"].(map[string]interface{}); ok {
		for name, tier := range pm {
			cfg.Explicit[name] = fmt.Sprintf("%v", tier)
		}
	}
	cfg.Patterns = toTierRules(raw["patterns"], "pattern")
	cfg.Categories = toTierRules(raw["categories"], "keyword")
	return cfg, nil
}

// toTierRules normalizes the patterns/categories config value into ordered
// rules. matchKey is the per-entry key holding the glob/keyword.
func toTierRules(val interface{}, matchKey string) []TierRule {
	var rules []TierRule
	switch v := val.(type) {
	case []interface{}:
		for _, item := range v {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			tier, _ := m["tier"].(string)
			// accept {pattern|keyword|match: x, tier: y} or a single {glob: tier} pair
			match := firstString(m, matchKey, "match", "glob", "pattern", "keyword")
			if match == "" && tier == "" && len(m) == 1 {
				for k, t := range m {
					match, tier = k, fmt.Sprintf("%v", t)
				}
			}
			if match != "" && tier != "" {
				rules = append(rules, TierRule{Match: match, Tier: tier})
			}
		}
	case map[string]interface{}:
		for k, t := range v {
			rules = append(rules, TierRule{Match: k, Tier: fmt.Sprintf("%v", t)})
		}
		// Deterministic order: longer (more specific) match first, then alpha.
		sort.SliceStable(rules, func(i, j int) bool {
			if len(rules[i].Match) != len(rules[j].Match) {
				return len(rules[i].Match) > len(rules[j].Match)
			}
			return rules[i].Match < rules[j].Match
		})
	}
	return rules
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// TierRule is an ordered glob/keyword → tier rule (patterns or categories).
type TierRule struct {
	Match string
	Tier  string
}

// TierConfig is the deterministic part of package-tiers.yaml.
type TierConfig struct {
	Explicit   map[string]string // exact name → tier
	Patterns   []TierRule        // ordered glob rules (path.Match)
	Categories []TierRule        // ordered keyword rules (substring)
}

// TierAssignment records the tier and which pass assigned it.
type TierAssignment struct {
	Tier string `json:"tier"`
	Pass int    `json:"pass"`
}

// TierCounts is the per-pass tally.
type TierCounts struct {
	Pass1      int `json:"pass1"`
	Pass2      int `json:"pass2"`
	Pass3      int `json:"pass3"`
	Unassigned int `json:"unassigned"`
}

// TierResult is the classifier output. Unassigned packages are left for the
// model's Pass 4 (ecosystem-convention judgment) and Pass 5 (default).
type TierResult struct {
	Assigned   map[string]TierAssignment `json:"assigned"`
	Unassigned []string                  `json:"unassigned"`
	Counts     TierCounts                `json:"counts"`
}

// classifyTiers runs Passes 1–3 deterministically: exact config match, then
// ordered glob patterns, then ordered keyword categories. First match wins
// within a pass; earlier passes take precedence (a package assigned in pass 1
// is never reconsidered). Unmatched packages go to Unassigned in input order.
func classifyTiers(packages []string, cfg TierConfig) *TierResult {
	res := &TierResult{
		Assigned:   map[string]TierAssignment{},
		Unassigned: []string{},
	}
	for _, pkg := range packages {
		switch {
		case cfg.Explicit[pkg] != "":
			res.Assigned[pkg] = TierAssignment{Tier: cfg.Explicit[pkg], Pass: 1}
			res.Counts.Pass1++
		default:
			if rule, ok := firstGlobMatch(pkg, cfg.Patterns); ok {
				res.Assigned[pkg] = TierAssignment{Tier: rule.Tier, Pass: 2}
				res.Counts.Pass2++
			} else if rule, ok := firstKeywordMatch(pkg, cfg.Categories); ok {
				res.Assigned[pkg] = TierAssignment{Tier: rule.Tier, Pass: 3}
				res.Counts.Pass3++
			} else {
				res.Unassigned = append(res.Unassigned, pkg)
			}
		}
	}
	res.Counts.Unassigned = len(res.Unassigned)
	return res
}

func firstGlobMatch(pkg string, rules []TierRule) (TierRule, bool) {
	for _, r := range rules {
		if ok, err := path.Match(r.Match, pkg); err == nil && ok {
			return r, true
		}
	}
	return TierRule{}, false
}

func firstKeywordMatch(pkg string, rules []TierRule) (TierRule, bool) {
	for _, r := range rules {
		if r.Match != "" && strings.Contains(pkg, r.Match) {
			return r, true
		}
	}
	return TierRule{}, false
}
