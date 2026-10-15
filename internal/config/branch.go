package config

import (
	"strings"
)

// ParseBranchTargets normalizes branch mapping values into a slice of allowed branch prefixes.
// Supports:
// - Single string ("issue")
// - Comma-separated string ("issue, feature")
// - Slice of strings ([]string{"issue", "feature"})
// - Slice of interfaces ([]any{"issue", "feature"})
func ParseBranchTargets(val any) []string {
	if val == nil {
		return nil
	}
	seen := make(map[string]bool)
	var targets []string

	add := func(target string) {
		trimmed := strings.ToLower(strings.TrimSpace(target))
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			targets = append(targets, trimmed)
		}
	}

	switch v := val.(type) {
	case string:
		for _, item := range strings.Split(v, ",") {
			add(item)
		}
	case []string:
		for _, item := range v {
			add(item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}
	return targets
}
