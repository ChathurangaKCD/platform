# Renderer Improvements Plan

This document outlines planned improvements for the component renderer, particularly focusing on pattern matching capabilities for array filters.

---

## Pattern Matching Requirements

Based on [`/Users/chathurangada/projects/cel_playground/addons/limitations-and-patterns.md`](../addons/limitations-and-patterns.md), there are several areas where pattern matching would be valuable:

### 1. Pattern-Based Resource Targeting
```yaml
targets:
  - resourceType: Ingress
    resourceId: "*-ingress"  # Match any resource ending in -ingress
```

### 2. Label/Selector-Based Targeting
```yaml
targets:
  - resourceType: Ingress
    selector:
      matchLabels:
        ingress-type: public
```

### 3. CEL Expressions in Targeting
```yaml
targets:
  - resourceType: Ingress
    resourceId: ${workload.endpoints.filter(e, e.visibility == "public").map(e, metadata.name + "-" + e.name)}
```

### 4. Array Filter Pattern Matching
**Current limitation**: Only supports simple equality `[?(@.name=='app')]`

**Need**: Support complex filters in JSONPath:
- `[?(@.replicas>5)]`
- `[?(@.name=='app' && @.ready==true)]`
- `[?(@.metadata.labels.env=='prod')]`
- `[?(has(@.ports) && @.ports.exists(p, p.containerPort==8080))]`

---

## Priority 1: CEL-Based Array Filters

**Effort**: Medium (6-8 hours)
**Priority**: High - Directly addresses current limitation and enables powerful filtering

### Current State

**File**: `renderer/pkg/renderer/patcher.go:234-241`

```go
// Only supports: [?(@.name=='app')]
filterParts := strings.Split(filterContent, "==")
if len(filterParts) != 2 {
    return fmt.Errorf("invalid filter expression")
}

fieldPath := strings.TrimPrefix(filterParts[0], "@.")
targetValue := strings.Trim(filterParts[1], "\"'")

// Simple equality check
if itemMap[fieldPath] == targetValue {
    // Apply patch
}
```

### Enhanced Implementation

#### 1. Update `patcher.go` Array Filter Logic

**Replace** lines 234-250 with CEL-based filtering:

```go
// Parse filter: [?(@.name=='app' && @.resources.requests.cpu > '100m')]
filterContent := filterExpr[3 : len(filterExpr)-2] // Extract expression

// Find matching items using CEL
matchingItems := filterArrayItemsByCEL(arr, filterContent)

// Apply suffix path to each matching item
for _, item := range matchingItems {
    itemMap := item.(map[string]interface{})
    suffixPath = strings.TrimPrefix(suffixPath, "/")
    if suffixPath == "" {
        // Direct modification
        // ...
    } else {
        // Navigate suffix path and apply
        return applyAdd(itemMap, suffixPath, value)
    }
}
```

#### 2. Add CEL Array Filter Function

**New function** in `patcher.go`:

```go
// filterArrayItemsByCEL filters array items using a CEL expression
// filterExpr: "@.name=='app' && has(@.resources)"
// Returns: items where CEL expression evaluates to true
func filterArrayItemsByCEL(arr []interface{}, filterExpr string) []interface{} {
    matches := []interface{}{}

    for _, item := range arr {
        // Create CEL environment with @ bound to current item
        inputs := map[string]interface{}{
            "@": item,
        }

        // Evaluate CEL expression
        result, err := evaluateCELExpression(filterExpr, inputs)
        if err != nil {
            // Log error but continue (item doesn't match)
            continue
        }

        // Check if result is boolean true
        if boolResult, ok := result.(bool); ok && boolResult {
            matches = append(matches, item)
        }
    }

    return matches
}
```

#### 3. Extend CEL Environment

**Update** `renderer/pkg/renderer/cel.go:153-182`:

```go
func evaluateCELExpression(expression string, inputs map[string]interface{}) (interface{}, error) {
    // Create CEL environment with custom functions
    env, err := cel.NewEnv(
        cel.Variable("metadata", cel.DynType),
        cel.Variable("spec", cel.DynType),
        cel.Variable("build", cel.DynType),
        cel.Variable("item", cel.DynType),
        cel.Variable("instanceId", cel.DynType),
        cel.Variable("@", cel.DynType),              // Add @ variable for array filters
        cel.OptionalTypes(),                         // Already added for has() support
        cel.Function("join", ...),
        cel.Function("omit", ...),
    )
    // ... rest of implementation
}
```

### Example Capabilities After Implementation

```yaml
# Multiple conditions with logical operators
patches:
  - target: {resourceType: Deployment}
    patch:
      op: add
      path: /spec/template/spec/containers/[?(@.name=='app' && has(@.resources))]/volumeMounts/-
      value:
        name: app-data
        mountPath: /app/data

# Numeric comparisons
patches:
  - target: {resourceType: Deployment}
    patch:
      op: add
      path: /spec/template/spec/containers/[?(@.resources.requests.cpu > '100m')]/env/-
      value:
        name: HIGH_CPU_MODE
        value: "true"

# Nested field access
patches:
  - target: {resourceType: Ingress}
    patch:
      op: add
      path: /spec/rules/[?(@.host.endsWith('.prod.example.com'))]/tls/-
      value:
        hosts: [${@.host}]
        secretName: prod-tls-cert

# Function calls and complex logic
patches:
  - target: {resourceType: Deployment}
    patch:
      op: add
      path: /spec/template/spec/containers/[?(has(@.ports) && @.ports.size() > 0 && @.ports.exists(p, p.containerPort==8080))]/env/-
      value:
        name: HTTP_ENABLED
        value: "true"

# Label-based filtering
patches:
  - target: {resourceType: Pod}
    patch:
      op: add
      path: /spec/containers/[?(has(@.metadata.labels) && @.metadata.labels.tier=='frontend')]/resources
      value:
        limits:
          memory: 512Mi
```

### Implementation Steps

1. **Add `filterArrayItemsByCEL()` function** (~2 hours)
   - Implement CEL evaluation for each array item
   - Handle errors gracefully
   - Return matching items

2. **Update `applyPathWithArrayFilter()` function** (~2 hours)
   - Replace simple equality check with CEL filtering
   - Update error messages
   - Test with complex expressions

3. **Extend CEL environment** (~1 hour)
   - Add `@` variable support
   - Verify `has()` works with `@.field` syntax
   - Test type coercion for comparisons

4. **Add test cases** (~2 hours)
   - Test multiple conditions (`&&`, `||`)
   - Test numeric comparisons (`>`, `<`, `>=`, `<=`)
   - Test nested field access
   - Test function calls (`has()`, `exists()`, `size()`)
   - Test error handling for invalid expressions

5. **Update documentation** (~1-2 hours)
   - Update `CLAUDE.md` with CEL filter capabilities
   - Add examples section showing complex patterns
   - Document supported CEL features in filters
   - Update addon examples to showcase new capabilities

### Testing Strategy

Create test addon that exercises all filter patterns:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Addon
metadata:
  name: advanced-filter-test
spec:
  patches:
    # Test 1: Multiple conditions
    - target: {resourceType: Deployment}
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name=='app' && has(@.resources))]/labels/managed
        value: "true"

    # Test 2: Numeric comparison
    - target: {resourceType: Deployment}
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.resources.requests.memory > '256Mi')]/labels/high-memory
        value: "true"

    # Test 3: Nested field
    - target: {resourceType: Service}
      patch:
        op: add
        path: /spec/ports/[?(@.protocol=='TCP' && @.port==80)]/name
        value: http

    # Test 4: Function calls
    - target: {resourceType: Deployment}
      patch:
        op: add
        path: /spec/template/spec/containers/[?(has(@.env) && @.env.size() > 0)]/labels/has-env
        value: "true"
```

---

## Priority 2: Wildcard Pattern Matching for Resource IDs

**Effort**: Low (2-3 hours)
**Priority**: Medium - Useful for targeting dynamic resources

### Implementation

**File**: `renderer/pkg/renderer/patcher.go`

```go
// Add to FindTargetResources function
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

        // Match by resource ID with pattern support
        if target.ResourceID != "" {
            resourceID := extractResourceID(resource) // metadata.name or custom id field

            // Check if pattern matching is needed
            if strings.Contains(target.ResourceID, "*") {
                // Use glob pattern matching
                matched, _ := filepath.Match(target.ResourceID, resourceID)
                if !matched {
                    continue
                }
            } else {
                // Exact match
                if resourceID != target.ResourceID {
                    continue
                }
            }
        }

        matches = append(matches, resource)
    }

    return matches
}

// Helper to extract resource ID from resource
func extractResourceID(resource map[string]interface{}) string {
    metadata, ok := resource["metadata"].(map[string]interface{})
    if !ok {
        return ""
    }

    name, ok := metadata["name"].(string)
    if !ok {
        return ""
    }

    return name
}
```

### Examples

```yaml
# Match all ingress resources ending with -ingress
targets:
  - resourceType: Ingress
    resourceId: "*-ingress"

# Match all public-prefixed resources
targets:
  - resourceType: Service
    resourceId: "public-*"

# Match any resource containing -prod-
targets:
  - resourceType: Deployment
    resourceId: "*-prod-*"
```

---

## Priority 3: Label Selector Support

**Effort**: Medium (4-5 hours)
**Priority**: Medium - Enables powerful resource targeting

### Implementation

#### 1. Update Types

**File**: `renderer/pkg/types/types.go`

```go
type TargetSpec struct {
    ResourceType string                 `yaml:"resourceType,omitempty"`
    ResourceID   string                 `yaml:"resourceId,omitempty"`
    Selector     *ResourceSelector      `yaml:"selector,omitempty"`     // NEW
}

type ResourceSelector struct {
    MatchLabels      map[string]string       `yaml:"matchLabels,omitempty"`
    MatchExpressions []SelectorExpression    `yaml:"matchExpressions,omitempty"`
}

type SelectorExpression struct {
    Key      string   `yaml:"key"`
    Operator string   `yaml:"operator"` // In, NotIn, Exists, DoesNotExist
    Values   []string `yaml:"values,omitempty"`
}
```

#### 2. Update FindTargetResources

```go
func FindTargetResources(resources []map[string]interface{}, target types.TargetSpec) []map[string]interface{} {
    var matches []map[string]interface{}

    for _, resource := range resources {
        // ... existing kind and ID matching ...

        // Match by selector
        if target.Selector != nil {
            if !matchesSelector(resource, target.Selector) {
                continue
            }
        }

        matches = append(matches, resource)
    }

    return matches
}

func matchesSelector(resource map[string]interface{}, selector *types.ResourceSelector) bool {
    labels := extractLabels(resource)

    // Check matchLabels
    if selector.MatchLabels != nil {
        for key, value := range selector.MatchLabels {
            if labels[key] != value {
                return false
            }
        }
    }

    // Check matchExpressions
    for _, expr := range selector.MatchExpressions {
        if !matchesExpression(labels, expr) {
            return false
        }
    }

    return true
}

func matchesExpression(labels map[string]string, expr types.SelectorExpression) bool {
    switch expr.Operator {
    case "In":
        labelValue, exists := labels[expr.Key]
        if !exists {
            return false
        }
        for _, value := range expr.Values {
            if labelValue == value {
                return true
            }
        }
        return false
    case "NotIn":
        labelValue, exists := labels[expr.Key]
        if !exists {
            return true
        }
        for _, value := range expr.Values {
            if labelValue == value {
                return false
            }
        }
        return true
    case "Exists":
        _, exists := labels[expr.Key]
        return exists
    case "DoesNotExist":
        _, exists := labels[expr.Key]
        return !exists
    default:
        return false
    }
}
```

### Examples

```yaml
# Match by labels
targets:
  - resourceType: Ingress
    selector:
      matchLabels:
        ingress-type: public
        env: prod

# Match with expressions
targets:
  - resourceType: Service
    selector:
      matchLabels:
        app: web
      matchExpressions:
        - key: tier
          operator: In
          values: [frontend, api]
        - key: deprecated
          operator: DoesNotExist
```

---

## Priority 4: CEL Expressions in Target ResourceID

**Effort**: Low-Medium (3-4 hours)
**Priority**: Low - Advanced use case, can be deferred

### Implementation

**File**: `renderer/pkg/renderer/patcher.go`

```go
func FindTargetResources(resources []map[string]interface{}, target types.TargetSpec, inputs map[string]interface{}) []map[string]interface{} {
    var matches []map[string]interface{}

    // Evaluate resourceId if it contains CEL
    targetResourceIDs := []string{}
    if target.ResourceID != "" {
        if strings.Contains(target.ResourceID, "${") {
            // Evaluate CEL expression
            result, err := EvaluateCELExpressions(target.ResourceID, inputs)
            if err != nil {
                // Log error and skip
                return matches
            }

            // Result could be string or array
            switch v := result.(type) {
            case string:
                targetResourceIDs = []string{v}
            case []interface{}:
                for _, item := range v {
                    if str, ok := item.(string); ok {
                        targetResourceIDs = append(targetResourceIDs, str)
                    }
                }
            }
        } else {
            targetResourceIDs = []string{target.ResourceID}
        }
    }

    for _, resource := range resources {
        // ... kind matching ...

        // Match by resourceId
        if len(targetResourceIDs) > 0 {
            resourceID := extractResourceID(resource)
            matched := false
            for _, targetID := range targetResourceIDs {
                if resourceID == targetID || matchesPattern(resourceID, targetID) {
                    matched = true
                    break
                }
            }
            if !matched {
                continue
            }
        }

        matches = append(matches, resource)
    }

    return matches
}
```

### Examples

```yaml
# Dynamic resource targeting based on workload
targets:
  - resourceType: Ingress
    resourceId: ${workload.endpoints.filter(e, e.visibility == "public").map(e, metadata.name + "-" + e.name)}

# Conditional targeting
targets:
  - resourceType: Service
    resourceId: ${spec.external ? metadata.name + "-external" : metadata.name + "-internal"}
```

---

## Implementation Roadmap

### Phase 1: Core Array Filter Enhancement (Recommended First)
- **Priority 1: CEL-Based Array Filters** (6-8 hours)
- Directly addresses current limitation
- Most impactful improvement
- Enables complex filtering logic

### Phase 2: Resource Targeting Improvements (Optional)
- **Priority 2: Wildcard Pattern Matching** (2-3 hours)
- **Priority 3: Label Selector Support** (4-5 hours)
- Enables better targeting of dynamic resources

### Phase 3: Advanced Features (Future)
- **Priority 4: CEL in ResourceID** (3-4 hours)
- Advanced use case, can be deferred

### Total Estimated Effort
- **Phase 1 only**: 6-8 hours
- **Phases 1+2**: 12-16 hours
- **All phases**: 15-20 hours

---

## Documentation Updates Needed

### 1. Update `CLAUDE.md`
- Add section on "Advanced Array Filtering"
- Document supported CEL features in filters
- Add examples of complex filter patterns
- Update "Known Limitations" section

### 2. Create `FILTERS.md`
- Comprehensive guide to filter syntax
- Examples for each pattern type
- Common patterns and best practices
- Troubleshooting section

### 3. Update Addon Examples
- Add `advanced-filter-addon.yaml` showing complex patterns
- Update existing addons to use improved filters
- Add comments explaining filter logic

---

## Testing Requirements

### Unit Tests
- Test CEL filter evaluation with various expressions
- Test pattern matching with wildcards
- Test label selector matching logic
- Test error handling for invalid expressions

### Integration Tests
- Test end-to-end rendering with complex filters
- Test multiple filter types in same addon
- Test filter performance with large arrays
- Test edge cases (empty arrays, missing fields, null values)

### Example-Based Tests
- Verify all example addons render correctly
- Compare output with expected output
- Test across all environments (dev, prod, no-env)

---

## Breaking Changes

None. All improvements are backwards compatible:
- Existing simple equality filters (`[?(@.name=='app')]`) continue to work
- New features are opt-in
- No changes to existing APIs or file formats

---

## Future Considerations

### Performance Optimization
If filtering becomes a bottleneck with large arrays:
- Cache compiled CEL expressions
- Add early-exit optimizations
- Consider parallel filtering for large resource sets

### Schema Validation
Add validation for filter expressions:
- Validate CEL syntax at addon load time
- Provide helpful error messages
- Suggest corrections for common mistakes

### IDE Support
- JSON schema for addon definitions with filter examples
- VSCode snippets for common filter patterns
- Syntax highlighting for CEL in YAML

---

## References

- [CEL Specification](https://github.com/google/cel-spec)
- [JSONPath Syntax](https://goessner.net/articles/JsonPath/)
- [Kubernetes Label Selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)
- [Project Limitations Doc](../addons/limitations-and-patterns.md)
