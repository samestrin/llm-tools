package commands

import (
	"path"
	"strings"
)

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
