package patch

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

// ApplyPatch applies a single patch operation against a target resource.
func ApplyPatch(target map[string]interface{}, patch types.Patch, inputs map[string]interface{}, renderString func(interface{}, map[string]interface{}) (interface{}, error)) error {
	path, err := renderString(patch.Path, inputs)
	if err != nil {
		return fmt.Errorf("failed to evaluate patch path: %w", err)
	}

	pathStr, ok := path.(string)
	if !ok {
		return fmt.Errorf("patch path must evaluate to a string, got %T", path)
	}

	var value interface{}
	if patch.Op != "remove" {
		value, err = renderString(patch.Value, inputs)
		if err != nil {
			return fmt.Errorf("failed to evaluate patch value: %w", err)
		}
	}

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
	if strings.Contains(path, "[?(") {
		return applyPathWithArrayFilter(target, path, value, "add")
	}

	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	_, err := setValue(target, parts, value, "add", "")
	return err
}

func applyReplace(target map[string]interface{}, path string, value interface{}) error {
	if strings.Contains(path, "[?(") {
		return applyPathWithArrayFilter(target, path, value, "replace")
	}

	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	_, err := setValue(target, parts, value, "replace", "")
	return err
}

func applyRemove(target map[string]interface{}, path string) error {
	if strings.Contains(path, "[?(") {
		return applyPathWithArrayFilter(target, path, nil, "remove")
	}

	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	_, err := setValue(target, parts, nil, "remove", "")
	return err
}

func applyMerge(target map[string]interface{}, path string, value interface{}) error {
	if strings.Contains(path, "[?(") {
		return applyPathWithArrayFilter(target, path, value, "merge")
	}

	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("merge value must be an object")
	}

	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	_, err := setValue(target, parts, valueMap, "merge", "")
	return err
}

func applyPathWithArrayFilter(target map[string]interface{}, path string, value interface{}, op string) error {
	filterStart := strings.Index(path, "[?(")
	if filterStart == -1 {
		return fmt.Errorf("no array filter found in path: %s", path)
	}

	filterEnd := strings.Index(path[filterStart:], ")]")
	if filterEnd == -1 {
		return fmt.Errorf("unclosed array filter in path: %s", path)
	}
	filterEnd += filterStart + 2

	prefixPath := path[:filterStart]
	filterExpr := path[filterStart:filterEnd]
	suffixPath := path[filterEnd:]

	prefixPath = strings.TrimPrefix(prefixPath, "/")
	prefixPath = strings.TrimSuffix(prefixPath, "/")

	prefixParts := []string{}
	if prefixPath != "" {
		prefixParts = strings.Split(prefixPath, "/")
	}

	if len(prefixParts) == 0 {
		return fmt.Errorf("array filter path %s must include a parent key", path)
	}

	current := interface{}(target)
	for i := 0; i < len(prefixParts)-1; i++ {
		part := prefixParts[i]
		switch node := current.(type) {
		case map[string]interface{}:
			next, ok := node[part]
			if !ok {
				if op == "remove" {
					return nil
				}
				newChild := make(map[string]interface{})
				node[part] = newChild
				current = newChild
				continue
			}
			current = next
		case []interface{}:
			idx, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("path segment %s is not a valid index", part)
			}
			if idx < 0 || idx >= len(node) {
				return fmt.Errorf("index %d out of bounds for segment %s", idx, part)
			}
			current = node[idx]
		default:
			return fmt.Errorf("cannot traverse segment %s on type %T", part, current)
		}
	}

	var parentMap map[string]interface{}
	if len(prefixParts) > 0 {
		last := prefixParts[len(prefixParts)-1]
		if currentMap, ok := current.(map[string]interface{}); ok {
			parentMap = currentMap
			current = currentMap[last]
		} else {
			return fmt.Errorf("path element %s is not an object", last)
		}
	}

	arrVal := current
	if arrVal == nil {
		if op == "remove" {
			return nil
		}
		arrVal = []interface{}{}
		parentMap[prefixParts[len(prefixParts)-1]] = arrVal
	}

	arr, ok := arrVal.([]interface{})
	if !ok {
		return fmt.Errorf("path element is not an array, got %T", arrVal)
	}

	if !strings.HasPrefix(filterExpr, "[?(") || !strings.HasSuffix(filterExpr, ")]") {
		return fmt.Errorf("invalid filter expression: %s", filterExpr)
	}

	filterContent := filterExpr[3 : len(filterExpr)-2]
	filterParts := strings.SplitN(filterContent, "==", 2)
	if len(filterParts) != 2 {
		return fmt.Errorf("invalid filter expression: %s", filterContent)
	}

	fieldPath := strings.TrimSpace(strings.TrimPrefix(filterParts[0], "@."))
	targetValue := strings.Trim(strings.TrimSpace(filterParts[1]), "\"'")

	suffixPath = strings.TrimPrefix(suffixPath, "/")
	suffixParts := []string{}
	if suffixPath != "" {
		suffixParts = parsePath(suffixPath)
	}

	matched := false
	newArr := make([]interface{}, 0, len(arr))

	for _, item := range arr {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			newArr = append(newArr, item)
			continue
		}

	valueAtPath, exists := getNestedValue(itemMap, fieldPath)
	if !exists || fmt.Sprintf("%v", valueAtPath) != targetValue {
			newArr = append(newArr, item)
			continue
		}

		matched = true

		if len(suffixParts) == 0 {
			switch op {
			case "remove":
				continue
			case "merge":
				valueMap, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("merge value must be an object for %s", path)
				}
				itemMap = deepMerge(itemMap, valueMap)
				newArr = append(newArr, itemMap)
			case "add", "replace":
				valueMap, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("value must be an object for %s", path)
				}
				for k, v := range valueMap {
					itemMap[k] = v
				}
				newArr = append(newArr, itemMap)
			default:
				return fmt.Errorf("unsupported operation %s for %s", op, path)
			}
			continue
		}

		_, err := setValue(itemMap, suffixParts, value, op, "")
		if err != nil {
			return err
		}
		newArr = append(newArr, itemMap)
	}

	if !matched {
		return nil
	}

	if len(prefixParts) > 0 {
		parentMap[prefixParts[len(prefixParts)-1]] = newArr
	}

	return nil
}

func parsePath(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return []string{}
	}

	parts := strings.Split(path, "/")
	var result []string
	for _, part := range parts {
		if strings.Contains(part, "[") {
			idx := strings.Index(part, "[")
			if idx > 0 {
				result = append(result, part[:idx])
			}
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

// FindTargetResources locates resources that match the given target specification.
func FindTargetResources(resources []map[string]interface{}, target types.TargetSpec, selector Matcher) []map[string]interface{} {
	var matches []map[string]interface{}
	for _, resource := range resources {
		if target.ResourceType != "" {
			kind, ok := resource["kind"].(string)
			if !ok || kind != target.ResourceType {
				continue
			}
		}
		if target.ResourceID != "" {
			if id, ok := resource["id"].(string); !ok || id != target.ResourceID {
				continue
			}
		}
		if target.Selector != "" && selector != nil {
			if !selector(resource, target.Selector) {
				continue
			}
		}
		matches = append(matches, resource)
	}
	return matches
}

// Matcher evaluates if a resource satisfies a selector expression.
type Matcher func(resource map[string]interface{}, selector string) bool

func setValue(container interface{}, parts []string, value interface{}, op string, pathPrefix string) (interface{}, error) {
	if len(parts) == 0 {
		return container, nil
	}

	token := parts[0]
	currentPath := pathPrefix + "/" + token
	isLast := len(parts) == 1

	switch node := container.(type) {
	case map[string]interface{}:
		if isLast {
			return applyToMap(node, token, value, op, currentPath)
		}

		child, exists := node[token]
		if !exists || child == nil {
			if op == "replace" {
				return nil, fmt.Errorf("cannot replace missing path %s", currentPath)
			}
			if op == "remove" {
				return node, nil
			}
			child = createIntermediateContainer(parts[1:])
			node[token] = child
		}

		updatedChild, err := setValue(child, parts[1:], value, op, currentPath)
		if err != nil {
			return nil, err
		}
		node[token] = updatedChild
		return node, nil

	case []interface{}:
		if token == "-" && !isLast {
			return nil, fmt.Errorf("'-' can only appear at the end of the path (seen at %s)", currentPath)
		}

		if isLast {
			return applyToSlice(node, token, value, op, currentPath)
		}

		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %s at %s", token, currentPath)
		}
		if idx < 0 {
			return nil, fmt.Errorf("negative array index %d at %s", idx, currentPath)
		}

		if idx >= len(node) {
			if op == "remove" {
				return node, nil
			}
			for len(node) <= idx {
				node = append(node, nil)
			}
		}

		child := node[idx]
		if child == nil && op != "remove" {
			child = createIntermediateContainer(parts[1:])
		}

		updatedChild, err := setValue(child, parts[1:], value, op, currentPath)
		if err != nil {
			return nil, err
		}
		node[idx] = updatedChild
		return node, nil

	case nil:
		if op == "remove" {
			return nil, nil
		}
		newContainer := createContainerForToken(token, parts[1:])
		return setValue(newContainer, parts, value, op, pathPrefix)

	default:
		if op == "remove" {
			return container, nil
		}
		return nil, fmt.Errorf("cannot traverse through type %T at %s", container, currentPath)
	}
}

func applyToMap(node map[string]interface{}, key string, value interface{}, op string, path string) (interface{}, error) {
	switch op {
	case "add", "replace":
		node[key] = value
		return node, nil
	case "merge":
		valueMap, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("merge value at %s must be an object", path)
		}
		existing, _ := node[key].(map[string]interface{})
		node[key] = deepMerge(existing, valueMap)
		return node, nil
	case "remove":
		delete(node, key)
		return node, nil
	default:
		return nil, fmt.Errorf("unsupported operation %s at %s", op, path)
	}
}

func applyToSlice(node []interface{}, token string, value interface{}, op string, path string) (interface{}, error) {
	switch op {
	case "add":
		if token == "-" {
			node = append(node, value)
			return node, nil
		}
		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %s at %s", token, path)
		}
		if idx < 0 || idx > len(node) {
			return nil, fmt.Errorf("array index %d out of bounds at %s", idx, path)
		}
		if idx == len(node) {
			node = append(node, value)
			return node, nil
		}
		node = append(node[:idx], append([]interface{}{value}, node[idx:]...)...)
		return node, nil
	case "replace":
		if token == "-" {
			return nil, fmt.Errorf("replace does not support '-' at %s", path)
		}
		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %s at %s", token, path)
		}
		if idx < 0 || idx >= len(node) {
			return nil, fmt.Errorf("array index %d out of bounds at %s", idx, path)
		}
		node[idx] = value
		return node, nil
	case "remove":
		if token == "-" {
			if len(node) == 0 {
				return node, nil
			}
			return node[:len(node)-1], nil
		}
		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %s at %s", token, path)
		}
		if idx < 0 || idx >= len(node) {
			return nil, fmt.Errorf("array index %d out of bounds at %s", idx, path)
		}
		return append(node[:idx], node[idx+1:]...), nil
	case "merge":
		if token == "-" {
			return nil, fmt.Errorf("merge does not support '-' at %s", path)
		}
		idx, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid array index %s at %s", token, path)
		}
		if idx < 0 || idx >= len(node) {
			return nil, fmt.Errorf("array index %d out of bounds at %s", idx, path)
		}
		existing, _ := node[idx].(map[string]interface{})
		valueMap, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("merge value at %s must be an object", path)
		}
		node[idx] = deepMerge(existing, valueMap)
		return node, nil
	default:
		return nil, fmt.Errorf("unsupported operation %s at %s", op, path)
	}
}

func createIntermediateContainer(remaining []string) interface{} {
	if len(remaining) == 0 {
		return make(map[string]interface{})
	}
	next := remaining[0]
	if next == "-" || isIndexToken(next) {
		return []interface{}{}
	}
	return make(map[string]interface{})
}

func createContainerForToken(token string, remaining []string) interface{} {
	if token == "-" || isIndexToken(token) {
		return []interface{}{}
	}
	return createIntermediateContainer(remaining)
}

func isIndexToken(token string) bool {
	if token == "" {
		return false
	}
	for _, ch := range token {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func getNestedValue(data interface{}, path string) (interface{}, bool) {
	current := data
	for _, segment := range strings.Split(path, ".") {
		if segment == "" {
			continue
		}
		switch node := current.(type) {
		case map[string]interface{}:
			val, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}
	return current, true
}

func deepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(base))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if existing, ok := result[k].(map[string]interface{}); ok {
			if add, ok := v.(map[string]interface{}); ok {
				result[k] = deepMerge(existing, add)
				continue
			}
		}
		result[k] = v
	}
	return result
}
