# ForEach Implementation Summary

## Overview

Successfully implemented `forEach` support for ComponentTypeDefinition resources, enabling dynamic generation of multiple Kubernetes resources from arrays.

## Changes Made

### 1. Type Definition Update
**File**: `pkg/types/types.go`

Added `ForEach` field to `ResourceTemplate`:
```go
type ResourceTemplate struct {
    ID        string                 `yaml:"id"`
    Condition string                 `yaml:"condition,omitempty"`
    ForEach   string                 `yaml:"forEach,omitempty"`  // NEW
    Template  map[string]interface{} `yaml:"template"`
}
```

### 2. Rendering Logic Update
**File**: `pkg/renderer/composer.go`

Extended `RenderBaseResources()` to handle forEach:
```go
// Handle forEach - render template for each item in array
if resourceTemplate.ForEach != "" {
    // Evaluate forEach expression to get items
    itemsResult, err := EvaluateCELExpressions(resourceTemplate.ForEach, inputs)

    items, ok := itemsResult.([]interface{})

    // Render template for each item
    for _, item := range items {
        // Create new inputs with current item
        itemInputs := make(map[string]interface{})
        for k, v := range inputs {
            itemInputs[k] = v
        }
        itemInputs["item"] = item

        // Evaluate template with item context
        rendered, err := EvaluateCELExpressions(resourceTemplate.Template, itemInputs)
        // ... clean and append to resources
    }
}
```

### 3. Example Files Created

#### ComponentTypeDefinition: `multi-service-component.yaml`
Demonstrates forEach usage with custom types:
- Defines `Service` type with name, port, replicas, image
- Uses `forEach: ${spec.services}` to generate multiple Deployments
- Uses `forEach: ${spec.services}` to generate multiple Services
- Each iteration has access to `${item}` variable

#### Component: `multi-service-example.yaml`
Example instance with 3 services:
- frontend (replicas: 3, port: 8080)
- backend (replicas: 2, port: 8081)
- worker (replicas: 1, port: 8082)

#### Test Script: `test-foreach.go`
Standalone test that renders the multi-service example

## Feature Comparison

| Feature | ComponentTypeDefinition | Addons |
|---------|------------------------|--------|
| `forEach` support | ✅ Implemented | ✅ Already supported |
| Use case | Generate multiple base resources | Apply same patch multiple times |
| Item variable | `${item}` | `${item}` |
| Example | Create Deployment per service | Add volumeMount per mount config |
| Location | `resources[].forEach` | `patches[].forEach` |

## Example Usage

### Multi-Service Application

**Input**: Array of 3 services
```yaml
parameters:
  services:
    - name: frontend
      port: 8080
      replicas: 3
      image: gcr.io/my-project/frontend:v1.0.0
    - name: backend
      port: 8081
      replicas: 2
      image: gcr.io/my-project/backend:v1.0.0
    - name: worker
      port: 8082
      replicas: 1
      image: gcr.io/my-project/worker:v1.0.0
```

**Output**: 6 resources generated
- 3 Deployments (one per service)
- 3 Services (one per service)

Each resource uses `${item.name}`, `${item.port}`, `${item.replicas}`, and `${item.image}` to access the current service's properties.

## Testing Results

```bash
$ go run test-foreach.go
Loaded ComponentTypeDefinition: multi-service-component
Loaded Component: multi-service-app

=== Rendering Base Resources with forEach ===

Generated 6 resources:

--- Resource 1: Deployment/multi-service-app-frontend ---
--- Resource 2: Deployment/multi-service-app-backend ---
--- Resource 3: Deployment/multi-service-app-worker ---
--- Resource 4: Service/multi-service-app-frontend ---
--- Resource 5: Service/multi-service-app-backend ---
--- Resource 6: Service/multi-service-app-worker ---

✅ Written 6 resources to examples/expected-output/multi-service-test.yaml
```

## Verification

Existing examples continue to work:
```bash
$ ./renderer
✅ Rendering complete!
```

All 12 original output files (4 stages × 3 environments) rendered successfully with no changes to existing behavior.

## Key Benefits

1. **Dynamic Resource Generation**: Generate arbitrary number of resources from arrays
2. **Type Safety**: Works with Kro's simpleschema type system
3. **Flexible**: Any CEL expression that returns an array
4. **Composable**: Can combine with `condition` for conditional rendering
5. **Pod Selectors**: Works with platform-injected pod selectors
6. **Backwards Compatible**: Existing resources without forEach continue to work

## Advanced Use Cases

### Conditional ForEach
```yaml
resources:
  - id: public-ingresses
    condition: ${spec.enableIngress}
    forEach: ${spec.endpoints.filter(e, e.visibility == "public")}
    template:
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ${metadata.name}-${item.name}
```

### Nested Item Access
```yaml
forEach: ${spec.databases}
template:
  apiVersion: v1
  kind: Secret
  metadata:
    name: ${metadata.name}-${item.name}-credentials
  stringData:
    host: ${item.connection.host}
    port: ${item.connection.port}
    database: ${item.connection.database}
```

### Filtering Before ForEach
```yaml
forEach: ${spec.services.filter(s, s.enabled && s.replicas > 0)}
template:
  # Only create resources for enabled services with replicas > 0
```

## Implementation Notes

1. **Item Variable Scope**: The `${item}` variable is only available within forEach templates
2. **Order Preservation**: Resources are generated in array order
3. **Error Handling**: If forEach expression doesn't return an array, rendering fails with clear error
4. **CEL Evaluation**: forEach expression is evaluated once, then template is rendered for each item
5. **Input Context**: Each iteration gets full access to metadata, spec, build, podSelectors, plus item

## Future Enhancements

Potential improvements for future iterations:

1. **Index Variable**: Add `${index}` variable for current iteration index
2. **ForEach in Addons.Creates**: Extend forEach to addon create templates (currently only patches)
3. **Nested ForEach**: Support forEach within forEach (if needed)
4. **ForEach Validation**: Validate forEach expressions at schema generation time
5. **Performance**: Optimize for large arrays (100+ items)

## Documentation Updates

Updated files:
- `CLAUDE.md`: Added forEach as first key feature with examples
- `FOREACH_IMPLEMENTATION.md`: This file - comprehensive implementation guide
- Added example files demonstrating forEach usage

## Testing

To test forEach functionality:
```bash
# Run standalone forEach test
go run test-foreach.go

# Run full renderer (includes existing examples)
./renderer

# Check generated output
cat examples/expected-output/multi-service-test.yaml
```
