package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// Optional --config support for the review fan-out commands. Reads the same
// keys the /execute-code-review skill historically plumbed by hand:
//
//	review.direct.agents | serial_agents | timeout_seconds
//	review.multi_agent.reviewers | serial_reviewers | openclaw_host |
//	                   timeout_seconds | per_reviewer_timeout_seconds
//
// Precedence: explicit CLI flag > config value > built-in default. Config is
// never discovered implicitly — no --config means current behavior.

// configCSV reads a key as a comma-separated string. YAML sequences are
// joined with ","; plain strings pass through.
func configCSV(cfg map[string]interface{}, key string) (string, bool, error) {
	val, ok := getValueAtPath(cfg, key)
	if !ok || val == nil {
		return "", false, nil
	}
	switch v := val.(type) {
	case string:
		return v, true, nil
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, strings.TrimSpace(fmt.Sprintf("%v", item)))
		}
		return strings.Join(parts, ","), true, nil
	default:
		return "", false, fmt.Errorf("config key %s: expected string or list, got %T", key, val)
	}
}

// configString reads a key as a scalar string.
func configString(cfg map[string]interface{}, key string) (string, bool, error) {
	val, ok := getValueAtPath(cfg, key)
	if !ok || val == nil {
		return "", false, nil
	}
	s, isStr := val.(string)
	if !isStr {
		return "", false, fmt.Errorf("config key %s: expected string, got %T", key, val)
	}
	return s, true, nil
}

// configInt reads a key as an int, accepting numeric strings (YAML authors
// quote values surprisingly often).
func configInt(cfg map[string]interface{}, key string) (int, bool, error) {
	val, ok := getValueAtPath(cfg, key)
	if !ok || val == nil {
		return 0, false, nil
	}
	switch v := val.(type) {
	case int:
		return v, true, nil
	case int64:
		return int(v), true, nil
	case uint64:
		return int(v), true, nil
	case float64:
		return int(v), true, nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false, fmt.Errorf("config key %s: %q is not an integer", key, v)
		}
		return n, true, nil
	default:
		return 0, false, fmt.Errorf("config key %s: expected integer, got %T", key, val)
	}
}

// loadReviewConfig reads an explicitly requested config file; a missing or
// unparseable file is a hard error (the flag was explicit).
func loadReviewConfig(path string) (map[string]interface{}, error) {
	cfg, err := readYAMLAsMap(path)
	if err != nil {
		return nil, fmt.Errorf("--config %s: %w", path, err)
	}
	return cfg, nil
}

// applyReviewDirectConfig merges review.direct.* config values into the
// review_direct flag variables for every flag the user did not set.
func applyReviewDirectConfig(cmd *cobra.Command, cfgPath string) error {
	cfg, err := loadReviewConfig(cfgPath)
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("reviewers") {
		if v, ok, err := configCSV(cfg, "review.direct.agents"); err != nil {
			return err
		} else if ok {
			rdReviewers = v
		}
	}
	if !cmd.Flags().Changed("serial-reviewers") {
		if v, ok, err := configCSV(cfg, "review.direct.serial_agents"); err != nil {
			return err
		} else if ok {
			rdSerialReviewers = v
		}
	}
	if !cmd.Flags().Changed("timeout-seconds") {
		if v, ok, err := configInt(cfg, "review.direct.timeout_seconds"); err != nil {
			return err
		} else if ok {
			rdTimeoutSeconds = v
		}
	}
	return nil
}

// applyMultiAgentConfig merges review.multi_agent.* config values into the
// multi_review flag variables for every flag the user did not set.
func applyMultiAgentConfig(cmd *cobra.Command, cfgPath string) error {
	cfg, err := loadReviewConfig(cfgPath)
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("reviewers") {
		if v, ok, err := configCSV(cfg, "review.multi_agent.reviewers"); err != nil {
			return err
		} else if ok {
			mrReviewers = v
		}
	}
	if !cmd.Flags().Changed("serial-reviewers") {
		if v, ok, err := configCSV(cfg, "review.multi_agent.serial_reviewers"); err != nil {
			return err
		} else if ok {
			mrSerialReviewers = v
		}
	}
	if !cmd.Flags().Changed("openclaw-host") {
		if v, ok, err := configString(cfg, "review.multi_agent.openclaw_host"); err != nil {
			return err
		} else if ok {
			mrOpenclawHost = v
		}
	}
	if !cmd.Flags().Changed("timeout-seconds") {
		if v, ok, err := configInt(cfg, "review.multi_agent.timeout_seconds"); err != nil {
			return err
		} else if ok {
			mrTimeoutSeconds = v
		}
	}
	if !cmd.Flags().Changed("per-reviewer-timeout-seconds") {
		if v, ok, err := configInt(cfg, "review.multi_agent.per_reviewer_timeout_seconds"); err != nil {
			return err
		} else if ok {
			mrPerReviewerTO = v
		}
	}
	return nil
}
