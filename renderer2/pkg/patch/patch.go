package patch

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

var filterExpr = regexp.MustCompile(`^@\.([A-Za-z0-9_.-]+)\s*==\s*['"](.*)['"]$`)

// ApplyPatch applies a single patch operation against a target resource.
func ApplyPatch(target map[string]interface{}, patch types.Patch, inputs map[string]interface{}, render func(interface{}, map[string]interface{}) (interface{}, error)) error {
	pathValue, err := render(patch.Path, inputs)
	if err != nil {
		return fmt.Errorf("failed to evaluate patch path: %w", err)
	}

	pathStr, ok := pathValue.(string)
	if !ok {
		return fmt.Errorf("patch path must evaluate to a string, got %T", pathValue)
	}

	var value interface{}
	if patch.Op != "remove" {
		value, err = render(patch.Value, inputs)
		if err != nil {
			return fmt.Errorf("failed to evaluate patch value: %w", err)
		}
	}

	op := strings.ToLower(patch.Op)
	switch op {
	case "add", "replace", "remove":
		return applyRFC6902(target, op, pathStr, value)
	case "merge":
		return applyMerge(target, pathStr, value)
	default:
		return fmt.Errorf("unknown patch operation: %s", patch.Op)
	}
}

func applyRFC6902(target map[string]interface{}, op, rawPath string, value interface{}) error {
	resolved, err := expandPaths(target, rawPath)
	if err != nil {
		return err
	}
	if len(resolved) == 0 {
		// No matches (e.g., filter didn't match anything); treat as no-op.
		return nil
	}

	for _, pointer := range resolved {
		if op == "add" {
			if err := ensureParentExists(target, pointer); err != nil {
				return err
			}
		}
		if err := applyJSONPatch(target, op, pointer, value); err != nil {
			return err
		}
	}
	return nil
}

func applyMerge(target map[string]interface{}, rawPath string, value interface{}) error {
	valueMap, ok := value.(map[string]interface{})
	if !ok {
		return fmt.Errorf("merge value must be an object")
	}

	resolved, err := expandPaths(target, rawPath)
	if err != nil {
		return err
	}
	if len(resolved) == 0 {
		// Nothing to merge into.
		return nil
	}

	for _, pointer := range resolved {
		if err := mergeAtPointer(target, pointer, valueMap); err != nil {
			return err
		}
	}
	return nil
}

// --- Path expansion --------------------------------------------------------

type pathState struct {
	pointer []string
	value   interface{}
}

func expandPaths(root map[string]interface{}, rawPath string) ([]string, error) {
	if rawPath == "" {
		return []string{""}, nil
	}

	segments := splitRawPath(rawPath)
	states := []pathState{{pointer: []string{}, value: root}}

	for _, segment := range segments {
		if segment == "-" {
			states = applyDash(states)
			continue
		}
		nextStates := make([]pathState, 0, len(states))
		for _, st := range states {
			expanded, err := applySegment(st, segment)
			if err != nil {
				return nil, err
			}
			nextStates = append(nextStates, expanded...)
		}
		states = nextStates
		if len(states) == 0 {
			break
		}
	}

	pointers := make([]string, 0, len(states))
	for _, st := range states {
		pointers = append(pointers, buildJSONPointer(st.pointer))
	}
	return pointers, nil
}

func applySegment(state pathState, segment string) ([]pathState, error) {
	current := []pathState{state}
	remaining := segment

	for len(remaining) > 0 {
		if strings.HasPrefix(remaining, "[") {
			closeIdx := strings.Index(remaining, "]")
			if closeIdx == -1 {
				return nil, fmt.Errorf("unclosed bracket segment in %q", segment)
			}
			content := remaining[1:closeIdx]
			remaining = remaining[closeIdx+1:]

			var err error
			switch {
			case strings.HasPrefix(content, "?(") && strings.HasSuffix(content, ")"):
				expr := content[2 : len(content)-1]
				current, err = applyFilter(current, expr)
			case content == "-":
				current = applyDash(current)
			default:
				index, parseErr := strconv.Atoi(content)
				if parseErr != nil {
					return nil, fmt.Errorf("unsupported array index %q", content)
				}
				current, err = applyIndex(current, index)
			}
			if err != nil {
				return nil, err
			}
		} else {
			nextBracket := strings.Index(remaining, "[")
			var token string
			if nextBracket == -1 {
				token = remaining
				remaining = ""
			} else {
				token = remaining[:nextBracket]
				remaining = remaining[nextBracket:]
			}
			if token == "" {
				continue
			}
			if idx, err := strconv.Atoi(token); err == nil {
				current, err = applyIndex(current, idx)
				if err != nil {
					return nil, err
				}
			} else {
				var err error
				current, err = applyKey(current, token)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return current, nil
}

func applyKey(states []pathState, key string) ([]pathState, error) {
	if key == "" {
		return states, nil
	}

	next := make([]pathState, 0, len(states))
	for _, st := range states {
		var child interface{}
		switch current := st.value.(type) {
		case map[string]interface{}:
			child = current[key]
		case nil:
			child = nil
		default:
			return nil, fmt.Errorf("path segment %q expects an object, got %T", key, st.value)
		}
		next = append(next, pathState{
			pointer: appendPointer(st.pointer, key),
			value:   child,
		})
	}
	return next, nil
}

func applyIndex(states []pathState, index int) ([]pathState, error) {
	next := make([]pathState, 0, len(states))
	for _, st := range states {
		arr, ok := st.value.([]interface{})
		if !ok {
			return nil, fmt.Errorf("path segment expects an array, got %T", st.value)
		}
		if index < 0 || index >= len(arr) {
			return nil, fmt.Errorf("array index %d out of bounds", index)
		}
		next = append(next, pathState{
			pointer: appendPointer(st.pointer, strconv.Itoa(index)),
			value:   arr[index],
		})
	}
	return next, nil
}

func applyDash(states []pathState) []pathState {
	next := make([]pathState, len(states))
	for i, st := range states {
		next[i] = pathState{
			pointer: appendPointer(st.pointer, "-"),
			value:   nil,
		}
	}
	return next
}

func applyFilter(states []pathState, expr string) ([]pathState, error) {
	next := []pathState{}
	for _, st := range states {
		arr, ok := st.value.([]interface{})
		if !ok || len(arr) == 0 {
			continue
		}
		for idx, item := range arr {
			match, err := matchesFilter(item, expr)
			if err != nil {
				return nil, err
			}
			if match {
				next = append(next, pathState{
					pointer: appendPointer(st.pointer, strconv.Itoa(idx)),
					value:   item,
				})
			}
		}
	}
	return next, nil
}

func matchesFilter(item interface{}, expr string) (bool, error) {
	matches := filterExpr.FindStringSubmatch(strings.TrimSpace(expr))
	if len(matches) != 3 {
		return false, fmt.Errorf("unsupported filter expression: %s", expr)
	}

	fieldPath := strings.Split(matches[1], ".")
	expected := matches[2]

	current := item
	for _, segment := range fieldPath {
		m, ok := current.(map[string]interface{})
		if !ok {
			return false, nil
		}
		current, ok = m[segment]
		if !ok {
			return false, nil
		}
	}

	if current == nil {
		return expected == "", nil
	}
	return fmt.Sprintf("%v", current) == expected, nil
}

func splitRawPath(path string) []string {
	if path == "" {
		return []string{}
	}
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return []string{""}
	}
	return strings.Split(trimmed, "/")
}

func appendPointer(base []string, segment string) []string {
	next := make([]string, len(base)+1)
	copy(next, base)
	next[len(base)] = segment
	return next
}

func buildJSONPointer(segments []string) string {
	if len(segments) == 0 {
		return ""
	}
	var b strings.Builder
	for _, seg := range segments {
		b.WriteByte('/')
		if seg == "-" {
			b.WriteString(seg)
		} else {
			b.WriteString(escapePointerSegment(seg))
		}
	}
	return b.String()
}

// --- RFC6902 execution -----------------------------------------------------

func applyJSONPatch(target map[string]interface{}, op, pointer string, value interface{}) error {
	ops := []map[string]interface{}{
		{
			"op":   op,
			"path": pointer,
		},
	}
	if op != "remove" {
		ops[0]["value"] = value
	}

	patchBytes, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	docBytes, err := json.Marshal(target)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	patch, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return fmt.Errorf("failed to decode JSON patch: %w", err)
	}

	patched, err := patch.Apply(docBytes)
	if err != nil {
		return fmt.Errorf("failed to apply JSON patch: %w", err)
	}

	var updated map[string]interface{}
	if err := json.Unmarshal(patched, &updated); err != nil {
		return fmt.Errorf("failed to unmarshal patched document: %w", err)
	}

	for k := range target {
		delete(target, k)
	}
	for k, v := range updated {
		target[k] = v
	}
	return nil
}

func ensureParentExists(root map[string]interface{}, pointer string) error {
	segments := splitPointer(pointer)
	if len(segments) == 0 {
		return nil
	}

	current := interface{}(root)
	for i := 0; i < len(segments)-1; i++ {
		seg := segments[i]

		switch node := current.(type) {
		case map[string]interface{}:
			child, exists := node[seg]
			if !exists || child == nil {
				next := segments[i+1]
				if next == "-" {
					node[seg] = []interface{}{}
				} else if _, err := strconv.Atoi(next); err == nil {
					return fmt.Errorf("array index %s out of bounds at segment %s", next, seg)
				} else {
					node[seg] = map[string]interface{}{}
				}
				child = node[seg]
			}
			current = child
		case []interface{}:
			index, err := strconv.Atoi(seg)
			if err != nil {
				return fmt.Errorf("expected array index at segment %s", seg)
			}
			if index < 0 || index >= len(node) {
				return fmt.Errorf("array index %d out of bounds at segment %s", index, seg)
			}
			current = node[index]
		default:
			return fmt.Errorf("cannot traverse segment %s on type %T", seg, current)
		}
	}
	return nil
}

// --- Merge -----------------------------------------------------------------

func mergeAtPointer(root map[string]interface{}, pointer string, value map[string]interface{}) error {
	parent, last, err := navigateToParent(root, pointer, true)
	if err != nil {
		return err
	}

	switch container := parent.(type) {
	case map[string]interface{}:
		existing, _ := container[last].(map[string]interface{})
		if existing == nil {
			container[last] = deepCopyMap(value)
			return nil
		}
		container[last] = DeepMerge(existing, value)
	case []interface{}:
		if last == "-" {
			return fmt.Errorf("merge operation cannot target append position '-'")
		}
		index, err := strconv.Atoi(last)
		if err != nil {
			return fmt.Errorf("invalid array index %q for merge", last)
		}
		if index < 0 || index >= len(container) {
			return fmt.Errorf("array index %d out of bounds for merge", index)
		}
		existing, _ := container[index].(map[string]interface{})
		if existing == nil {
			container[index] = deepCopyMap(value)
			return nil
		}
		container[index] = DeepMerge(existing, value)
	default:
		return fmt.Errorf("merge parent must be object or array, got %T", parent)
	}
	return nil
}

func navigateToParent(root map[string]interface{}, pointer string, create bool) (interface{}, string, error) {
	segments := splitPointer(pointer)
	if len(segments) == 0 {
		return root, "", nil
	}
	parentSegs := segments[:len(segments)-1]
	last := segments[len(segments)-1]

	current := interface{}(root)
	for i, seg := range parentSegs {
		switch node := current.(type) {
		case map[string]interface{}:
			child, exists := node[seg]
			if !exists || child == nil {
				if !create {
					return nil, "", fmt.Errorf("missing path at segment %s", seg)
				}
				next := determineNextContainerType(parentSegs, i, last)
				node[seg] = next
				child = node[seg]
			}
			current = child
		case []interface{}:
			index, err := strconv.Atoi(seg)
			if err != nil {
				return nil, "", fmt.Errorf("expected array index at segment %s", seg)
			}
			if index < 0 || index >= len(node) {
				return nil, "", fmt.Errorf("array index %d out of bounds at segment %s", index, seg)
			}
			current = node[index]
		default:
			return nil, "", fmt.Errorf("cannot traverse segment %s on type %T", seg, node)
		}
	}
	return current, last, nil
}

func determineNextContainerType(segments []string, index int, last string) interface{} {
	nextSeg := last
	if index+1 < len(segments) {
		nextSeg = segments[index+1]
	}
	if nextSeg == "-" {
		return []interface{}{}
	}
	if _, err := strconv.Atoi(nextSeg); err == nil {
		return []interface{}{}
	}
	return map[string]interface{}{}
}

// --- Helpers ----------------------------------------------------------------

func splitPointer(pointer string) []string {
	if pointer == "" {
		return []string{}
	}
	trimmed := strings.TrimPrefix(pointer, "/")
	if trimmed == "" {
		return []string{""}
	}
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		if part != "-" {
			parts[i] = unescapePointerSegment(part)
		}
	}
	return parts
}

func escapePointerSegment(seg string) string {
	seg = strings.ReplaceAll(seg, "~", "~0")
	seg = strings.ReplaceAll(seg, "/", "~1")
	return seg
}

func unescapePointerSegment(seg string) string {
	seg = strings.ReplaceAll(seg, "~1", "/")
	seg = strings.ReplaceAll(seg, "~0", "~")
	return seg
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	result := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch typed := v.(type) {
		case map[string]interface{}:
			result[k] = deepCopyMap(typed)
		case []interface{}:
			result[k] = deepCopySlice(typed)
		default:
			result[k] = typed
		}
	}
	return result
}

func deepCopySlice(src []interface{}) []interface{} {
	if src == nil {
		return nil
	}
	result := make([]interface{}, len(src))
	for i, v := range src {
		switch typed := v.(type) {
		case map[string]interface{}:
			result[i] = deepCopyMap(typed)
		case []interface{}:
			result[i] = deepCopySlice(typed)
		default:
			result[i] = typed
		}
	}
	return result
}

// --- Existing helpers retained ---------------------------------------------

// DeepMerge deeply merges two maps, with values from 'override' taking precedence.
func DeepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range base {
		result[k] = v
	}

	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overrideMap, ok := v.(map[string]interface{}); ok {
					result[k] = DeepMerge(baseMap, overrideMap)
					continue
				}
			}
		}
		result[k] = v
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
