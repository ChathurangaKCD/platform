# CEL Playground

A Go-based tool for evaluating CEL (Common Expression Language) expressions in YAML templates with JSON inputs.

## Usage

```bash
go run main.go <directory_path>
```

The tool reads `template.yaml` and `inputs.json` from the specified directory and generates `output.yaml` with evaluated CEL expressions.

## Features

### CEL Expression Syntax

CEL expressions are embedded in templates using `${...}` syntax:

```yaml
name: ${metadata.name}
replicas: ${spec.maxReplicas}
```

### YAML Block Scalars for Complex Expressions

For complex CEL expressions with braces, use YAML block scalars (`|`) to avoid escaping:

```yaml
ports: |
  ${spec.endpoints.map(e, {"containerPort": e.port})}
```

This is cleaner than escaping quotes:
```yaml
ports: "${spec.endpoints.map(e, {\"containerPort\": e.port})}"
```

### Supported CEL Operations

- **Simple substitution**: `${metadata.name}`
- **Nested paths**: `${spec.scaleToZero.pendingRequests}`
- **Map operations**: `spec.endpoints.map(e, {"containerPort": e.port})`
- **Filter operations**: `spec.items.filter(i, i.enabled)`
- **Ternary operators**: `${spec.enableDebug ? "NodePort" : "ClusterIP"}`
- **Arithmetic**: `${spec.maxReplicas * 2}`
- **String concatenation**: `${metadata.name + "-svc"}`
- **Array concatenation**: `[{...}] + (condition ? [{...}] : [])`
- **Custom join function**: `spec.features.map(f, "prefix-" + f).join(",")`
- **Field existence checks**: `has(spec.container.resources)`

### Conditional Logic

#### If-Else with Ternary Operator

```yaml
type: "${spec.enableDebug ? \"NodePort\" : \"ClusterIP\"}"
```

#### Checking Field Existence

```yaml
memory-limit: "${has(spec.containers[0].resources) && has(spec.containers[0].resources.limits) ? spec.containers[0].resources.limits.memory : \"not-specified\"}"
```

#### Nested Conditionals

```yaml
ports: |
  ${c.name == "app" ? (spec.enableMetrics ? [{"containerPort": 8080}, {"containerPort": 9090}] : [{"containerPort": 8080}]) : []}
```

### Conditionally Including Resources/Fields

#### Pattern 1: Conditionally Include Entire Resources (Best Approach)

Use the `condition` field to completely omit resources when the condition is false:

```yaml
- id: hpa
  condition: ${spec.enableHPA}
  template:
    apiVersion: autoscaling/v2
    kind: HorizontalPodAutoscaler
    metadata:
      name: ${metadata.name}-hpa
    spec:
      minReplicas: ${spec.minReplicas}
      maxReplicas: ${spec.maxReplicas}
```

**Result**: When `enableHPA: false`, the HPA resource is completely omitted from the output.

#### Pattern 2: Conditionally Add Fields (Returns Empty Structures)

```yaml
annotations: |
  ${spec.enableServiceMonitor ? {"prometheus.io/scrape": "true"} : {}}
```

**Result**: If false, returns empty map `{}`. The field still appears in the output.

#### Pattern 3: Conditionally Add to Arrays

```yaml
ports: |
  ${[{"port": 80, "name": "http"}] + (spec.enableServiceMonitor ? [{"port": 9090, "name": "metrics"}] : [])}
```

**Result**: Always includes HTTP port, adds metrics port only if condition is true.

### Omitting Fields Entirely vs Empty Values

**Current behavior**: When using ternary with empty values, you get:
```yaml
resources: {}  # Empty map appears in output
ports: []      # Empty array appears in output
```

**To omit fields entirely** (so they don't appear in the output at all):

#### Option 1: Use Resource-Level Conditions
Works for entire resources - they're completely omitted when condition is false.

#### Option 2: Build Objects Conditionally in CEL
Instead of:
```yaml
resources: |
  ${spec.enableHPA ? {...} : {}}
```

Build the entire object with/without the field:
```yaml
containers: |
  ${spec.enableHPA ?
    [{"name": "app", "image": "nginx", "resources": {...}}] :
    [{"name": "app", "image": "nginx"}]
  }
```

**Downside**: Requires duplicating structure, which is messy.

#### Option 3: Post-Processing (Not Yet Implemented)
The ideal solution would be to enhance the Go code with a post-processing step that recursively removes fields with empty values (`{}`, `[]`) from the final output before writing to YAML. This would allow clean templates where `? {...} : {}` automatically strips empty structures.

## Examples

### example/ - Simple Substitution
Basic CEL expressions with simple field substitution.

### example2/ - Complex Map Operations
Demonstrates `.map()` operations with proper YAML formatting using block scalars.

### example3/ - Block Scalars
Shows YAML block scalar syntax to avoid escaping in complex CEL expressions.

### example4/ - Escaped Quotes
Alternative approach using quoted expressions with escaping.

### example5/ - Join Function
Demonstrates the custom `.join()` function for string arrays.

### example6/ - Arrays and Strings
Shows both YAML array output and comma-separated string output from the same data.

### example7/ - Mixed Outputs
Examples of generating both array structures and joined strings.

### example_conditionals/ - If-Else Logic
Comprehensive examples of:
- Ternary operators (`? :`)
- Field existence checks with `has()`
- Nested conditionals
- Conditional environment variables
- Conditional resources based on boolean flags

**Key patterns**:
- Checking nested fields: `has(spec.containers[0].resources) && has(spec.containers[0].resources.limits)`
- Boolean to string: `spec.enableMetrics ? "true" : "false"`
- Conditional arrays: `condition ? [item1, item2] : [item1]`
- Default values: `has(field) ? field : "default"`

### example_optional_resources/ - Conditional Resource Inclusion
Shows the "if condition add block, else nothing" pattern:
- **Resource-level conditions**: Entire resources omitted when condition is false
- **Conditional annotations**: Add metadata only when needed
- **Conditional ports**: Add ports to arrays based on feature flags
- **Conditional resource limits**: Add CPU/memory limits only when autoscaling enabled

**Patterns demonstrated**:
```yaml
# Entire resource omitted if enableHPA is false
- id: hpa
  condition: ${spec.enableHPA}
  template: {...}

# Empty map if condition false
annotations: |
  ${condition ? {"key": "value"} : {}}

# Array concatenation with conditional items
ports: |
  ${[{...}] + (condition ? [{...}] : [])}
```

### example_omit/ - Omitting Fields with omit()
Comprehensive demonstration of the `omit()` function for completely removing fields from output:
- **String fields**: Omit optional descriptions
- **Number fields**: Omit replica counts when not specified
- **Object fields**: Omit resource limits and annotations
- **Array fields**: Omit ports arrays
- **Empty string preservation**: Shows `""` is preserved while `omit()` removes the field

**Demonstrates**:
- Fields marked with `omit()` do not appear in output at all (not even as empty values)
- Works with all data types (strings, numbers, arrays, objects)
- Can be used in ternary expressions: `${condition ? value : omit()}`

## YAML Output Format

CEL expressions that return arrays or objects are properly formatted as native YAML structures:

**Input**:
```yaml
containers: |
  ${spec.containers.map(c, {"name": c.name, "image": c.image})}
```

**Output**:
```yaml
containers:
  - name: app
    image: nginx:latest
  - name: sidecar
    image: busybox:latest
```

Not as JSON strings:
```yaml
containers: '[{"name":"app","image":"nginx:latest"}]'  # Old behavior
```

## Custom Functions

### join(separator)
Joins an array of strings with a separator:

```yaml
featuresList: ${spec.features.map(f, "feature-" + f).join(",")}
```

Output: `"feature-logging,feature-monitoring,feature-tracing"`

### omit()
Returns a special sentinel value that causes the field to be completely omitted from the output.

**Works for all types**:

```yaml
# String field - omitted if condition is false
description: |
  ${spec.hasDescription ? "My description" : omit()}

# Number field - omitted if condition is false
replicas: |
  ${spec.hasReplicas ? spec.replicas : omit()}

# Object field - omitted if condition is false
resources: |
  ${spec.hasResources ? {"limits": {"cpu": "500m"}} : omit()}

# Array field - omitted if condition is false
ports: |
  ${spec.hasPorts ? [{"containerPort": 8080}] : omit()}
```

**Key differences from empty values**:
- `omit()` → Field **does not appear** in output YAML
- `""` → Field appears as: `fieldName: ""`
- `{}` → Field appears as: `fieldName: {}`
- `[]` → Field appears as: `fieldName: []`

**When to use**:
- ✅ Use `omit()` when you want the field to be absent (cleaner YAML, avoids validation errors for optional fields)
- ✅ Use empty values (`""`, `{}`, `[]`) when you intentionally want an empty value

**Example**: See `example_omit/` for comprehensive demonstrations of omitting fields of all types.

## Implementation Notes

- CEL expressions in block scalars (`|`) are detected and evaluated to their native types
- Nested braces in CEL expressions are properly handled
- The `condition` field on resources allows complete omission of resources based on boolean expressions
- Empty structures (`{}`, `[]`) from false conditions remain in output unless using resource-level conditions
