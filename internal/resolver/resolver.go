package resolver

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// refPattern matches ${VAR_NAME} references
	refPattern = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)
	// maxDepth prevents infinite recursion in circular references
	maxDepth = 10
)

// ErrCircularReference is returned when secrets reference each other in a cycle
type ErrCircularReference struct {
	Key  string
	Path []string
}

func (e *ErrCircularReference) Error() string {
	return fmt.Sprintf("circular reference detected for '%s': %s", e.Key, strings.Join(e.Path, " -> "))
}

// ErrUnresolvedReference is returned when a referenced secret doesn't exist
type ErrUnresolvedReference struct {
	Key       string
	Reference string
}

func (e *ErrUnresolvedReference) Error() string {
	return fmt.Sprintf("unresolved reference in '%s': ${%s} not found", e.Key, e.Reference)
}

// Resolve resolves all ${VAR} references in the secrets map
// Returns a new map with all references replaced with actual values
func Resolve(secrets map[string]string) (map[string]string, error) {
	resolved := make(map[string]string)

	for key := range secrets {
		value, err := resolveValue(key, secrets, nil, 0)
		if err != nil {
			return nil, err
		}
		resolved[key] = value
	}

	return resolved, nil
}

// resolveValue resolves a single secret's value, following references recursively
func resolveValue(key string, secrets map[string]string, path []string, depth int) (string, error) {
	if depth > maxDepth {
		return "", &ErrCircularReference{Key: key, Path: append(path, key)}
	}

	// Check for circular reference
	for _, p := range path {
		if p == key {
			return "", &ErrCircularReference{Key: key, Path: append(path, key)}
		}
	}

	value, exists := secrets[key]
	if !exists {
		// This shouldn't happen if called correctly, but handle it
		return "", &ErrUnresolvedReference{Key: path[len(path)-1], Reference: key}
	}

	// Find all references in this value
	matches := refPattern.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		// No references, return as-is
		return value, nil
	}

	// Resolve each reference
	result := value
	newPath := append(path, key)

	for _, match := range matches {
		fullMatch := match[0] // ${VAR_NAME}
		refKey := match[1]    // VAR_NAME

		// Check if referenced key exists
		if _, exists := secrets[refKey]; !exists {
			return "", &ErrUnresolvedReference{Key: key, Reference: refKey}
		}

		// Recursively resolve the referenced value
		resolvedRef, err := resolveValue(refKey, secrets, newPath, depth+1)
		if err != nil {
			return "", err
		}

		// Replace the reference with the resolved value
		result = strings.Replace(result, fullMatch, resolvedRef, 1)
	}

	return result, nil
}

// HasReferences checks if a value contains any ${VAR} references
func HasReferences(value string) bool {
	return refPattern.MatchString(value)
}

// GetReferences extracts all referenced variable names from a value
func GetReferences(value string) []string {
	matches := refPattern.FindAllStringSubmatch(value, -1)
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		refs = append(refs, match[1])
	}
	return refs
}
