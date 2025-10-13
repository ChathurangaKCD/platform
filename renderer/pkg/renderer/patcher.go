package renderer

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chathurangada/cel_playground/renderer/pkg/types"
)

// ApplyPatch applies a patch to a target resource
func ApplyPatch(target map[string]interface{}, patch types.Patch, inputs map[string]interface{}) error {
	// Evaluate path and value with CEL
	path, err := EvaluateCELExpressions(patch.Path, inputs)
	if err != nil {
		return fmt.Errorf("failed to evaluate patch path: %w", err)
	}
	pathStr, ok := path.(string)
	if !ok {
		return fmt.Errorf("patch path must be a string, got %T", path)
	}

	value, err := EvaluateCELExpressions(patch.Value, inputs)
	if err != nil {
		return fmt.Errorf("failed to evaluate patch value: %w", err)
	}

	// Apply the patch based on operation
	switch patch.Op {
	case "add":
		return applyAdd(target, pathStr, value)
	case "replace":
		return applyReplace(target, pathStr, value)
	case "remove":
		return applyRemove(target, pathStr)
	case "merge":
		return applyMerge(target, pathStr, value)
	default:
		return fmt.Errorf("unknown patch operation: %s", patch.Op)
	}
}

func applyAdd(target map[string]interface{}, path string, value interface{}) error {
	// Check if path contains array filter
	if strings.Contains(path, "[?(") {
		return applyPathWithArrayFilter(target, path, value)
	}

	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	// Check if this is an array append operation
	isArrayAppend := parts[len(parts)-1] == "-"

	// Determine how many parts to navigate
	navigateCount := len(parts) - 1
	if isArrayAppend {
		// For array append (e.g., "volumeMounts/-"), navigate all except last 2
		navigateCount = len(parts) - 2
	}

	// Navigate to parent
	current := target
	for i := 0; i < navigateCount; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			// Create intermediate objects
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		} else {
			current, ok = next.(map[string]interface{})
			if !ok {
				return fmt.Errorf("path element %s is not an object", part)
			}
		}
	}

	// Handle array append (path ends with "/-")
	if isArrayAppend {
		// Parent should be an array
		arrayKey := parts[len(parts)-2]
		arr, ok := current[arrayKey].([]interface{})
		if !ok {
			// Initialize array if it doesn't exist
			arr = []interface{}{}
		}
		current[arrayKey] = append(arr, value)
		return nil
	}

	// Simple field set
	lastPart := parts[len(parts)-1]
	current[lastPart] = value
	return nil
}

func applyReplace(target map[string]interface{}, path string, value interface{}) error {
	return applyAdd(target, path, value)
}

func applyRemove(target map[string]interface{}, path string) error {
	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	// Navigate to parent
	current := target
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			return nil // Path doesn't exist, nothing to remove
		}
		current, ok = next.(map[string]interface{})
		if !ok {
			return nil
		}
	}

	delete(current, parts[len(parts)-1])
	return nil
}

func applyMerge(target map[string]interface{}, path string, value interface{}) error {
	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	// Navigate to target
	current := target
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		} else {
			current, ok = next.(map[string]interface{})
			if !ok {
				return fmt.Errorf("path element %s is not an object", part)
			}
		}
	}

	lastPart := parts[len(parts)-1]
	existing, ok := current[lastPart].(map[string]interface{})
	if !ok {
		existing = make(map[string]interface{})
	}

	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("merge value must be an object")
	}

	current[lastPart] = DeepMerge(existing, valueMap)
	return nil
}

func applyPathWithArrayFilter(target map[string]interface{}, path string, value interface{}) error {
	// Parse path like: /spec/template/spec/containers/[?(@.name=='app')]/volumeMounts/-
	// Split into: prefix path + array filter section

	// Find the array filter part
	filterStart := strings.Index(path, "[?(")
	if filterStart == -1 {
		return fmt.Errorf("no array filter found in path: %s", path)
	}

	filterEnd := strings.Index(path[filterStart:], ")]")
	if filterEnd == -1 {
		return fmt.Errorf("unclosed array filter in path: %s", path)
	}
	filterEnd += filterStart + 2 // Adjust to absolute position and include )]

	// Extract parts
	// prefixPath: /spec/template/spec/containers/
	// filterExpr: [?(@.name=='app')]
	// suffixPath: /volumeMounts/-
	prefixPath := path[:filterStart]
	filterExpr := path[filterStart : filterEnd]
	suffixPath := path[filterEnd:]

	// Clean and split prefix path
	prefixPath = strings.TrimPrefix(prefixPath, "/")
	prefixPath = strings.TrimSuffix(prefixPath, "/")

	prefixParts := []string{}
	if prefixPath != "" {
		prefixParts = strings.Split(prefixPath, "/")
	}

	current := target
	arrayKey := ""

	// Navigate through all prefix parts
	// For /spec/template/spec/containers, we navigate: spec -> template -> spec
	// and arrayKey becomes "containers"
	for i := 0; i < len(prefixParts)-1; i++ {
		part := prefixParts[i]
		next, ok := current[part]
		if !ok {
			return fmt.Errorf("path element %s not found", part)
		}
		current, ok = next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("path element %s is not an object (got %T)", part, next)
		}
	}

	// Last part of prefix is the array key
	if len(prefixParts) > 0 {
		arrayKey = prefixParts[len(prefixParts)-1]
	}

	// Get the array
	arr, ok := current[arrayKey].([]interface{})
	if !ok {
		return fmt.Errorf("path element %s is not an array, got %T", arrayKey, current[arrayKey])
	}

	// Parse filter: [?(@.name=='app')]
	if !strings.HasPrefix(filterExpr, "[?(") || !strings.HasSuffix(filterExpr, ")]") {
		return fmt.Errorf("invalid filter expression: %s", filterExpr)
	}

	filterContent := filterExpr[3 : len(filterExpr)-2] // Extract @.name=='app'
	filterParts := strings.Split(filterContent, "==")
	if len(filterParts) != 2 {
		return fmt.Errorf("invalid filter expression: %s", filterContent)
	}

	fieldPath := strings.TrimPrefix(filterParts[0], "@.")
	targetValue := strings.Trim(filterParts[1], "\"'")

	// Find matching items and apply suffix path
	for _, item := range arr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		if itemMap[fieldPath] == targetValue {
			// Apply the suffix path operation
			suffixPath = strings.TrimPrefix(suffixPath, "/")
			if suffixPath == "" {
				// Direct modification
				valueMap, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("value must be an object for direct modification")
				}
				for k, v := range valueMap {
					itemMap[k] = v
				}
			} else {
				// Navigate suffix path and apply
				return applyAdd(itemMap, suffixPath, value)
			}
		}
	}

	return nil
}

func parsePath(path string) []string {
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return []string{}
	}

	// Split by slash
	parts := strings.Split(path, "/")

	// Handle array indices
	var result []string
	for _, part := range parts {
		// Handle array index [0] or array append [-]
		if strings.Contains(part, "[") {
			// Extract base and index
			idx := strings.Index(part, "[")
			if idx > 0 {
				result = append(result, part[:idx])
			}
			// Extract index value
			indexPart := part[idx+1:]
			indexPart = strings.TrimSuffix(indexPart, "]")
			if indexPart != "" {
				result = append(result, indexPart)
			}
		} else {
			result = append(result, part)
		}
	}

	return result
}

// FindTargetResources finds resources matching the target specification
func FindTargetResources(resources []map[string]interface{}, target types.TargetSpec) []map[string]interface{} {
	var matches []map[string]interface{}

	for _, resource := range resources {
		// Match by resource type (Kind)
		if target.ResourceType != "" {
			kind, ok := resource["kind"].(string)
			if !ok || kind != target.ResourceType {
				continue
			}
		}

		// Match by resource ID (not in standard k8s resources, but in our templates)
		if target.ResourceID != "" {
			// This would match our internal ID field if we tracked it
			// For now, skip this check
		}

		matches = append(matches, resource)
	}

	return matches
}

func parseInt(s string) (int, error) {
	return strconv.Atoi(s)
}
