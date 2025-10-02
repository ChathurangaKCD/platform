# Development Notes: CEL Playground Design Decisions

This document captures the key design decisions and thinking process behind the CEL playground implementation.

## Core Problem

We needed a way to generate Kubernetes manifests from templates with dynamic values, similar to tools like Helm or Kustomize, but using CEL (Common Expression Language) for its standardization and type safety.

## Key Design Decisions

### 1. CEL Expression Syntax: `${...}`

**Decision**: Use `${expression}` syntax for embedding CEL expressions in YAML templates.

**Rationale**:
- Familiar syntax (similar to shell variables, JavaScript template literals)
- Clear visual distinction from regular YAML content
- Easy to parse with regex/string matching
- Doesn't conflict with YAML syntax when properly quoted

**Note**: This is one of several possible syntaxes/templating languages being evaluated. Other alternatives like Go templates (`{{...}}`), Jinja2, or other approaches may also be viable depending on requirements.

### 2. YAML Block Scalars for Complex Expressions

**Problem**: CEL expressions containing braces caused YAML parsing errors:
```yaml
ports: ${spec.endpoints.map(e, {"containerPort": e.port})}
# Error: "mapping values are not allowed in this context"
```

**Solution**: Use YAML block scalars (`|`):
```yaml
ports: |
  ${spec.endpoints.map(e, {"containerPort": e.port})}
```

**Why this works**:
- YAML treats everything after `|` as a literal string
- No need to escape quotes or braces
- Cleaner, more readable templates
- The CEL evaluator detects pure expressions and returns actual data structures, not strings

**Key insight**: We detect when a block scalar contains only a CEL expression (after trimming whitespace) and return the evaluated result directly, preserving arrays/maps as YAML structures instead of JSON strings.

### 3. Proper YAML Output vs JSON Strings

**Initial problem**: CEL expressions returning arrays/maps were being serialized as JSON strings:
```yaml
ports: '[{"containerPort":8080}]'  # Wrong - JSON string
```

**Solution**: Return native Go types from CEL evaluation and let YAML marshaller handle them:
```yaml
ports:
  - containerPort: 8080  # Correct - YAML array
```

**Implementation**:
1. Detect when string value is a pure CEL expression: `trimmed == "${...}"`
2. Return the evaluated result directly (not converted to string)
3. YAML marshaller properly formats arrays/maps as native YAML structures

**Critical fix**: Changed from:
```go
// Bad: Always converts arrays to JSON strings
if len(matches) == 1 && matches[0][0] == str {
    return evaluateCELExpression(...) // Returns []interface{}
}
result := str // But then treats it as string for replacement
```

To:
```go
// Good: Returns arrays as-is for pure expressions
trimmed := strings.TrimSpace(str)
if len(matches) == 1 && matches[0][0] == trimmed {
    return evaluateCELExpression(...) // Returns []interface{} directly
}
```

### 4. Conditional Resource Inclusion

**Requirement**: Support "if condition then include resource, else omit entirely" pattern.

**Solution 1**: Resource-level `condition` field:
```yaml
- id: hpa
  condition: ${spec.enableHPA}  # Entire resource omitted if false
  template: {...}
```

**Implementation**: Evaluate condition before processing template; skip resource if false.

**Why it works**: Clean separation between "should this resource exist" (condition) vs "what should this resource contain" (template).

**Solution 2**: The `omit()` function (more flexible):
```yaml
description: |
  ${spec.hasDescription ? "text" : omit()}
```

### 5. The `omit()` Function Design

**Problem**: How to distinguish between "field should be empty" vs "field should not exist"?

**Options considered**:

1. **Auto-remove empty structures** (`{}`, `[]`, `""`):
   - ❌ Ambiguous: Can't distinguish intentional empty values
   - ❌ Would break valid use cases where empty string/map is desired

2. **Special sentinel value like `"__OMIT__"`**:
   - ❌ Pollutes the value space
   - ❌ Could conflict with actual data
   - ❌ Not type-safe

3. **Custom CEL function returning special marker** ✅:
   - ✅ Explicit intent: `omit()` clearly means "remove this field"
   - ✅ Type-safe: Works for all types (string, number, array, object)
   - ✅ No ambiguity: Empty values are preserved, only `omit()` removes fields

**Implementation approach**:

1. **CEL function returns error**: `return types.NewErr("__OMIT_FIELD__")`
   - Why error? CEL doesn't have a "void" type, but errors can carry special meaning
   - The error message acts as our sentinel

2. **Convert error to Go sentinel**:
   ```go
   if err.Error() == "__OMIT_FIELD__" {
       return omitSentinel, nil  // Special *omitValue pointer
   }
   ```

3. **Filter during evaluation** (not after marshalling):
   ```go
   // In evaluateCELExpressions for maps:
   if evaluated == omitSentinel {
       continue  // Skip adding this field
   }
   ```

**Why filter early (not in post-processing)**:
- YAML marshaller converts unknown types (like our sentinel pointer) to `{}`
- By the time we marshal to YAML, it's too late to distinguish sentinel from empty map
- Filtering during evaluation preserves the sentinel before YAML sees it

**Key insight**: The sentinel must be removed BEFORE the data structure goes through YAML marshalling, not after.

### 6. Nested Brace Handling in CEL Expressions

**Problem**: Simple regex `\$\{([^}]+)\}` fails on nested braces:
```cel
${spec.endpoints.map(e, {"containerPort": e.port})}
                        ^                     ^ These confuse simple regex
```

**Solution**: Manual brace counting:
```go
braceCount := 1
for pos < len(str) && braceCount > 0 {
    if str[pos] == '{' {
        braceCount++
    } else if str[pos] == '}' {
        braceCount--
    }
    pos++
}
```

**Why manual parsing**: Regex doesn't support balanced bracket matching. We need to track nesting depth.

### 7. Custom CEL Functions

**join()**: Added because CEL doesn't have built-in string joining:
```go
cel.Function("join",
    cel.MemberOverload("list_join_string",
        []*cel.Type{cel.ListType(cel.StringType), cel.StringType},
        cel.StringType,
        cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
            // Implementation
        }),
    ),
)
```

**omit()**: Zero-argument function that returns sentinel:
```go
cel.Function("omit",
    cel.Overload("omit", []*cel.Type{}, cel.DynType,
        cel.FunctionBinding(func(values ...ref.Val) ref.Val {
            return types.NewErr("__OMIT_FIELD__")
        }),
    ),
)
```

### 8. Type Conversions: CEL ↔ Go

**Challenge**: CEL has its own type system (`ref.Val`), need to convert to Go types for YAML marshalling.

**Solution**: `convertCELValue()` function with type switching:
```go
switch val.Type() {
case types.StringType:
    return val.Value().(string)
case types.ListType:
    // Handle both CEL lists and Go slices
    switch list := val.Value().(type) {
    case []ref.Val:        // CEL native
    case []interface{}:    // Go native
    }
```

**Why handle both**: CEL operations sometimes return CEL native types, sometimes Go types depending on the operation.

## Lessons Learned

1. **YAML is tricky**: Block scalars are essential for complex expressions. Inline quoting + escaping is unmaintainable.

2. **Timing matters**: Data transformations must happen in the right order:
   - CEL evaluation → Go types → Filter omits → YAML marshal
   - Not: CEL evaluation → YAML marshal → Post-process

3. **Explicit is better**: The `omit()` function is cleaner than implicit empty-value removal.

4. **Type safety pays off**: Using CEL's type system caught many errors during development.

5. **Sentinel values need care**: Must be filtered before serialization, not after.

## Future Improvements

1. **Better error messages**: Currently CEL errors can be cryptic
2. **Line number tracking**: Show which template line caused evaluation errors
3. **Schema validation**: Validate inputs against expected structure
4. **More built-in functions**: `filter()`, `reduce()`, `groupBy()`, etc.
5. **Dry-run mode**: Show what would be generated without writing files
6. **Template includes**: Allow templates to reference other templates

## Anti-Patterns to Avoid

❌ **Don't use escaped quotes when block scalars work**:
```yaml
# Bad
ports: "${spec.endpoints.map(e, {\"containerPort\": e.port})}"

# Good
ports: |
  ${spec.endpoints.map(e, {"containerPort": e.port})}
```

❌ **Don't return empty values when field should be omitted**:
```yaml
# Bad - field appears as empty
description: ${spec.hasDescription ? spec.description : ""}

# Good - field doesn't appear at all
description: |
  ${spec.hasDescription ? spec.description : omit()}
```

❌ **Don't nest CEL expressions in string interpolation for complex types**:
```yaml
# Bad - becomes JSON string
message: "Ports: ${spec.ports}"

# Good - use separate field
ports: |
  ${spec.ports}
```

## Summary

The CEL playground successfully bridges the gap between static YAML and dynamic templating by:
- Using CEL for standardized, type-safe expressions
- Leveraging YAML block scalars for clean syntax
- Preserving native YAML structures (not JSON strings)
- Providing explicit control over field omission via `omit()`
- Supporting resource-level conditionals for entire resource inclusion/exclusion

The key insight throughout development was: **work with YAML's strengths (block scalars) and handle special cases early in the pipeline (before marshalling)**.
