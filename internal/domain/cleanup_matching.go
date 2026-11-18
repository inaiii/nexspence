package domain

import (
	"fmt"
	"regexp"
)

// CleanupNameRegexp uses Go's RE2-style syntax and searches the stored asset
// path. Callers use ^ and $ when they want to match the entire path.
func CleanupNameRegexp(criteria map[string]any) (*regexp.Regexp, error) {
	value, exists := criteria["nameRegex"]
	if !exists {
		return nil, nil
	}
	pattern, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("criteria.nameRegex must be a string")
	}
	if len(pattern) > 4096 {
		return nil, fmt.Errorf("criteria.nameRegex must not exceed 4096 bytes")
	}
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid criteria.nameRegex (Go regexp): %w", err)
	}
	return re, nil
}
