package mcp

import (
	"strings"
)

// BoolPtr returns a pointer to the given bool value
func BoolPtr(b bool) *bool {
	return &b
}

// filterAnnotations filters annotations based on include/exclude lists
func filterAnnotations(annotations map[string]string, includeKeys, excludeKeys []string) map[string]string {
	if annotations == nil {
		return nil
	}

	result := make(map[string]string)

	// If includeKeys is specified, only include those keys
	if len(includeKeys) > 0 {
		for _, key := range includeKeys {
			if value, exists := annotations[key]; exists {
				result[key] = value
			}
		}
		return result
	}

	// Otherwise, include all except excluded keys
	for key, value := range annotations {
		excluded := false
		for _, excludeKey := range excludeKeys {
			if matchesPattern(key, excludeKey) {
				excluded = true
				break
			}
		}
		if !excluded {
			result[key] = value
		}
	}

	return result
}

// matchesPattern checks if a key matches a pattern (supports wildcards with *)
func matchesPattern(key, pattern string) bool {
	if pattern == key {
		return true
	}

	// Simple wildcard matching - only supports * at the end
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix)
	}

	return false
}
