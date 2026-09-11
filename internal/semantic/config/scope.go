package config

import "strings"

// splitScope turns a comma-separated scope setting into patterns.
//
// An unset or blank setting yields no patterns rather than one empty pattern,
// which would match nothing and silently empty a collection on the next
// rebuild.
func splitScope(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}
