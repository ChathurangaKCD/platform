# Component Renderer

A Go-based renderer for Kubernetes component definitions with addon composition, inspired by [Kro](https://github.com/kubernetes-sigs/kro).

## Overview

This renderer implements a declarative component system that allows:
- **ComponentTypeDefinitions**: Base resource templates (e.g., Deployment)
- **Addons**: Composable modifications that create resources or patch existing ones
- **Components**: Instances that reference a ComponentTypeDefinition and apply addons
- **EnvSettings**: Environment-specific parameter overrides (dev, prod, etc.)
- **JSON Schema Generation**: Using Kro's simpleschema for validation

## Architecture

```
renderer/
├── main.go                          # CLI entry point with dynamic stage generation
├── go.mod                          # Go module dependencies (cel-go v0.26.1, kro, etc.)
├── pkg/
│   ├── types/
│   │   └── types.go                # CRD struct definitions (ResourceTemplate with forEach & var)
│   ├── parser/
│   │   ├── component.go            # Load ComponentTypeDefinition & Component
│   │   ├── addon.go                # Load Addon definitions
│   │   ├── envsettings.go          # Load EnvSettings
│   │   ├── additional_context.go   # Load platform-injected context (build, secrets, configs)
│   │   └── schema.go               # Generate JSON schemas using kro's simpleschema
│   └── renderer/
│       ├── cel.go                  # CEL expression evaluation with extensions & custom functions
│       ├── merger.go               # Merge component params with EnvSettings & additional context
│       ├── patcher.go              # JSONPatch with array filter support
│       └── composer.go             # Render base resources (with forEach) and apply addons
└── examples/
    ├── additional_context.json     # Platform-injected context (build, podSelectors, secrets, configs)
    ├── component-type-definitions/
    │   └── deployment-component.yaml
    ├── addons/
    │   ├── pvc-addon.yaml
    │   ├── sidecar-addon.yaml
    │   └── emptydir-addon.yaml
    ├── components/
    │   └── example-component.yaml
    ├── env-settings/
    │   ├── dev-env.yaml
    │   └── prod-env.yaml
    ├── schemas/                    # Generated JSON schemas (OpenAPI v3)
    │   ├── deployment-component-schema.json
    │   ├── persistent-volume-claim-schema.json
    │   ├── sidecar-container-schema.json
    │   └── emptydir-volume-schema.json
    └── expected-output/            # Generated YAML manifests (7 resources per stage)
        ├── no-env/
        ├── dev/
        └── prod/
            ├── stage-1-base.yaml        # Deployment + ConfigMaps + ExternalSecretStores
            ├── stage-2-with-pvc.yaml    # + PersistentVolumeClaim
            ├── stage-3-with-sidecar.yaml # + Sidecar container
            └── stage-4-with-emptydir.yaml # + EmptyDir volume
```

## Key Features

### 1. ForEach Support with Custom Variable Names
Render multiple resources from arrays using `forEach` with custom variable names:

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: deployment-component
spec:
  resources:
    # Create ConfigMap per configuration file
    - id: file-configs
      forEach: ${configurations.files}
      var: configFile  # Custom variable name instead of 'item'
      template:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: ${metadata.name}-config-${configFile.name}
        data:
          config: ${configFile.content}
```

**Result**: If `configurations.files` contains 2 files, this generates 2 ConfigMaps.

**Key Points**:
- **Custom variable names**: Use `var: configFile` instead of default `item`
- **Dynamic CEL variables**: All input keys are automatically registered as CEL variables
- **Semantic naming**: Use meaningful names like `service`, `config`, `mount` instead of `item`
- **Nested properties**: Access `${configFile.name}`, `${configFile.content}`, etc.
- **Works with any CEL expression** that returns an array
- **Combinable with `includeWhen`** for conditional rendering

**Default behavior**: If `var` is omitted, defaults to `item` for backward compatibility

### 2. Dynamic Stage Generation
Stages are generated from the Component's addon list (not hardcoded):
```go
// main.go:118-138
func generateStages(component *types.Component) []types.Stage {
    stages := []types.Stage{
        {Name: "stage-1-base", AddonCount: 0},
    }
    for i, addonInstance := range component.Spec.Addons {
        shortName := shortNames[addonInstance.Name]
        stageName := fmt.Sprintf("stage-%d-with-%s", i+2, shortName)
        stages = append(stages, types.Stage{
            Name:       stageName,
            AddonCount: i + 1,
        })
    }
    return stages
}
```

### 3. CEL Expression Evaluation with Standard Extensions
Supports CEL expressions with standard extensions and custom functions:

**Variables**:
- `metadata` - Component metadata (name, namespace, labels)
- `spec` - Component/addon parameters
- `build` - Build context (image) from additional_context.json
- `podSelectors` - Platform-injected pod selectors from additional_context.json
- `configurations` - Platform-injected configurations (envs, files) from additional_context.json
- `secrets` - Platform-injected secrets (envs, files) from additional_context.json
- `instanceId` - Addon instance ID
- **Dynamic forEach variables** - Custom variable names defined via `var` field

**Standard CEL Extensions** (via `github.com/google/cel-go/ext v0.26.1`):
- **Strings** (`ext.Strings()`): `charAt()`, `indexOf()`, `lastIndexOf()`, `lowerAscii()`, `upperAscii()`, `replace()`, `split()`, `substring()`, `trim()`, `join()`
- **Encoders** (`ext.Encoders()`): `base64.encode()`, `base64.decode()`
- **Math** (`ext.Math()`): `ceil()`, `floor()`, `round()`, `abs()`, `max()`, `min()`
- **Lists** (`ext.Lists()`): `flatten()`, `unique()`, etc.
- **Sets** (`ext.Sets()`): `contains()`, `intersects()`, etc.
- **TwoVarComprehensions** (`ext.TwoVarComprehensions()`): `transformMapEntry()` - Convert arrays to maps

**Custom Functions**:
- `omit()` - Omit fields from output
- `merge(map, map)` - Deep merge two maps (override takes precedence)

**Built-in Operators**:
- **Array concatenation**: `[1,2] + [3,4]` returns `[1,2,3,4]`
- **String concatenation**: `"hello" + " " + "world"` returns `"hello world"`

**Example Expressions**:
- `${metadata.name}` - Access metadata
- `${spec.replicas}` - Access parameters
- `${build.image}` - Access platform-injected build context
- `${configurations.files + secrets.files}` - Concatenate arrays
- `${has(spec.command) && spec.command.size() > 0 ? spec.command : omit()}` - Conditional with omit
- `${metadata.name.upperAscii()}` - String manipulation
- `${['a', 'b', 'c'].join('-')}` - List join
- `${merge({"app": metadata.name}, podSelectors)}` - Map merge
- `${configurations.envs.transformMapEntry(i, e, {e.name: e.value})}` - Array to map conversion

```go
// renderer/cel.go
func evaluateCELExpression(expression string, inputs map[string]interface{}) (interface{}, error) {
    env, err := cel.NewEnv(
        // Variables
        cel.Variable("metadata", cel.DynType),
        cel.Variable("spec", cel.DynType),
        cel.Variable("build", cel.DynType),
        cel.Variable("item", cel.DynType),
        cel.Variable("instanceId", cel.DynType),
        cel.Variable("podSelectors", cel.DynType),
        cel.Variable("configurations", cel.DynType),
        cel.Variable("secrets", cel.DynType),

        cel.OptionalTypes(),

        // Standard CEL extensions
        ext.Strings(),   // String functions
        ext.Encoders(),  // Base64 encode/decode
        ext.Math(),      // Math functions
        ext.Lists(),     // List operations
        ext.Sets(),      // Set operations

        // Custom functions
        cel.Function("omit", ...),
        cel.Function("merge", ...),
    )
    // Parse, compile, and evaluate
}
```

### 4. JSON Schema Generation (Kro's simpleschema)
Converts human-friendly schema syntax to OpenAPI v3:

**Input (YAML)**:
```yaml
schema:
  parameters:
    volumeName: string | required=true
    mountPath: string | required=true
    size: string | default=10Gi
```

**Output (JSON)**:
```json
{
  "type": "object",
  "required": ["mountPath", "volumeName"],
  "properties": {
    "volumeName": {"type": "string"},
    "mountPath": {"type": "string"},
    "size": {"type": "string", "default": "10Gi"}
  }
}
```

Key implementation:
```go
// parser/schema.go:15-42
func GenerateJSONSchema(ctd *types.ComponentTypeDefinition) (*extv1.JSONSchemaProps, error) {
    mergedSchema := make(map[string]interface{})
    // Merge parameters and envOverrides
    for k, v := range ctd.Spec.Schema.Parameters {
        mergedSchema[k] = v
    }
    // Use kro's simpleschema
    jsonSchema, err := simpleschema.ToOpenAPISpec(mergedSchema, ctd.Spec.Schema.Types)
    sortRequiredFields(jsonSchema) // Alphabetically sorted
    return jsonSchema, nil
}
```

### 5. JSONPatch with Array Filters
Supports complex JSONPath expressions with array filters:

**Path**: `/spec/template/spec/containers/[?(@.name=='app')]/volumeMounts/-`

This path:
1. Navigates to `/spec/template/spec/containers`
2. Filters array items where `name == 'app'`
3. Appends to the `volumeMounts` array

```go
// renderer/patcher.go:158-251
func applyPathWithArrayFilter(target map[string]interface{}, path string, value interface{}) error {
    // Split: /spec/template/spec/containers/[?(@.name=='app')]/volumeMounts/-
    // Into: prefix + filter + suffix
    filterStart := strings.Index(path, "[?(")
    filterEnd := strings.Index(path[filterStart:], ")]") + filterStart + 2

    prefixPath := path[:filterStart]              // /spec/template/spec/containers
    filterExpr := path[filterStart:filterEnd]     // [?(@.name=='app')]
    suffixPath := path[filterEnd:]                // /volumeMounts/-

    // Navigate, filter, and apply suffix
}
```

### 6. Addon Composition
Addons can:
- **Create new resources** (e.g., PVC, ConfigMap)
- **Patch existing resources** (e.g., add volumeMounts to containers)
- **Use forEach** for multiple patches

Example addon:
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Addon
metadata:
  name: persistent-volume-claim
spec:
  schema:
    parameters:
      volumeName: string | required=true
      mountPath: string | required=true

  creates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: ${metadata.name}-${instanceId}
      spec:
        accessModes: [${spec.accessMode}]
        resources:
          requests:
            storage: ${spec.size}

  patches:
    - target:
        resourceType: Deployment
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: ${spec.volumeName}
          persistentVolumeClaim:
            claimName: ${metadata.name}-${instanceId}

    - target:
        resourceType: Deployment
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name=='${spec.containerName}')]/volumeMounts/-
        value:
          name: ${spec.volumeName}
          mountPath: ${spec.mountPath}
```

### 7. Platform-Injected Context and Secrets
Platform-injected data is stored in `additional_context.json`, separate from Component YAML:

**additional_context.json**:
```json
{
  "podSelectors": {
    "openchoreo.io/component-id": "web-service-12345",
    "openchoreo.io/project-id": "my-project-67890"
  },
  "build": {
    "image": "gcr.io/my-project/web-service:v1.0.0"
  },
  "configurations": {
    "envs": [
      {"name": "APP_ENV", "value": "production"},
      {"name": "LOG_LEVEL", "value": "info"}
    ],
    "files": [
      {
        "name": "app-config",
        "mountPath": "/etc/app/config.yaml",
        "content": "database:\\nhost: localhost\\n"
      }
    ]
  },
  "secrets": {
    "envs": [
      {"name": "DATABASE_PASSWORD", "valueRef": "db-credentials/password"},
      {"name": "API_KEY", "valueRef": "api-secrets/key"}
    ],
    "files": [
      {
        "name": "tls-cert",
        "mountPath": "/etc/secrets/tls.crt",
        "valueRef": "r1234"
      }
    ]
  }
}
```

**Mounting Secrets and Configurations**:
```yaml
# Deployment with configurations and secrets
containers:
  - name: app
    # Mount secret env vars via envFrom.secretRef
    envFrom:
      - configMapRef:
          name: ${metadata.name}-env-config
      - secretRef:
          name: ${metadata.name}-secret-envs

    # Concatenate config files and secret files
    volumeMounts: |
      ${(configurations.files.map(f, {
        "name": f.name,
        "mountPath": f.mountPath,
        "subPath": "config"
      })) +
      (secrets.files.map(f, {
        "name": f.name,
        "mountPath": f.mountPath,
        "subPath": f.name
      }))}

volumes: |
  ${(configurations.files.map(f, {
    "name": f.name,
    "configMap": {"name": metadata.name + "-config-" + f.name}
  })) +
  (secrets.files.map(f, {
    "name": f.name,
    "secret": {"secretName": metadata.name + "-secret-" + f.name}
  }))}

# ExternalSecretStore for secret env vars (single resource)
- id: secret-envs
  includeWhen: ${has(secrets.envs) && secrets.envs.size() > 0}
  template:
    apiVersion: external-secrets.io/v1beta1
    kind: ExternalSecretStore
    metadata:
      name: ${metadata.name}-secret-envs
    spec:
      data: "${secrets.envs.map(e, {\"key\": e.name, \"valueRef\": e.valueRef})}"

# ExternalSecretStore for secret files (one per file)
- id: secret-files
  forEach: ${secrets.files}
  var: secretFile
  template:
    apiVersion: external-secrets.io/v1beta1
    kind: ExternalSecretStore
    metadata:
      name: ${metadata.name}-secret-${secretFile.name}
    spec:
      data:
        - key: ${secretFile.name}
          valueRef: ${secretFile.valueRef}
```

**Key Points**:
- **Separation of concerns**: Platform data in `additional_context.json`, component config in YAML
- **Array concatenation**: Use `+` operator to combine configuration and secret arrays
- **ExternalSecretStore resolution**: Secret resources are resolved to K8s Secrets with same name
- **Explicit naming**: Use `name` field for volume names instead of computing from `mountPath`
- **Environment variable consolidation**: All config env vars in single ConfigMap, all secret env vars in single Secret

### 8. Environment Settings
Override parameters per environment:

**Component**:
```yaml
spec:
  parameters:
    replicas: 2
```

**EnvSettings (prod)**:
```yaml
spec:
  componentOverrides:
    replicas: 5
  addonOverrides:
    persistent-volume-claim:
      size: 100Gi
```

Result: prod environment gets 5 replicas and 100Gi PVC.

## Usage

```bash
# Build
go build -o renderer

# Run (generates schemas + renders all stages for all environments)
./renderer

# Output directories are cleaned and recreated on each run:
# - examples/schemas/
# - examples/expected-output/
```

## Key Implementation Details

### Stage Rendering Pipeline
```go
// main.go:142-184
func renderStage(ctd, component, addons, addonCount, envSettings) {
    // 1. Merge component params + env overrides
    inputs := renderer.BuildInputs(component, envSettings)

    // 2. Render base resources
    resources := renderer.RenderBaseResources(ctd, inputs)

    // 3. Apply addons in order
    for i := 0; i < addonCount; i++ {
        addonInstance := component.Spec.Addons[i]
        addon := addons[addonInstance.Name]

        // Merge addon config + addon-specific env overrides
        addonInputs := renderer.BuildAddonInputs(...)

        // Apply addon (creates + patches)
        resources = renderer.ApplyAddon(addon, addonInputs, resources)
    }

    return resources
}
```

### Array Append Fix
Critical fix for nested array operations (renderer/patcher.go:43-99):
```go
func applyAdd(target map[string]interface{}, path string, value interface{}) error {
    isArrayAppend := parts[len(parts)-1] == "-"

    // Navigate count differs for array append
    navigateCount := len(parts) - 1
    if isArrayAppend {
        navigateCount = len(parts) - 2  // Don't navigate into array itself
    }

    // Navigate to parent
    for i := 0; i < navigateCount; i++ {
        current = current[parts[i]]
    }

    // Append to array
    if isArrayAppend {
        arrayKey := parts[len(parts)-2]
        current[arrayKey] = append(current[arrayKey], value)
    }
}
```

This prevents double-nesting like:
```yaml
# WRONG (before fix)
volumeMounts:
  volumeMounts:
    - mountPath: /app/data

# RIGHT (after fix)
volumeMounts:
  - mountPath: /app/data
```

## Example Workflow

**Input**: Component with 3 addons (PVC, Sidecar, EmptyDir)

**Output**: 4 stages × 3 environments = 12 YAML files

1. **Stage 1**: Base Deployment
2. **Stage 2**: + PVC (creates PersistentVolumeClaim, adds volume + volumeMount)
3. **Stage 3**: + Sidecar (adds second container to pod)
4. **Stage 4**: + EmptyDir (adds emptyDir volume + mounts to both containers)

Each stage shows the incremental result of applying addons.

## Dependencies

- **CEL**: `github.com/google/cel-go v0.26.1` - Expression language with optional types support and built-in array concatenation
- **CEL Extensions**: `github.com/google/cel-go/ext` - Standard CEL extensions:
  - `ext.Strings()` - String manipulation functions
  - `ext.Encoders()` - Base64 encoding/decoding
  - `ext.Math()` - Mathematical functions
  - `ext.Lists()` - List operations (flatten, unique, etc.)
  - `ext.Sets()` - Set operations
  - `ext.TwoVarComprehensions()` - transformMapEntry for array-to-map conversion
- **Kro**: `github.com/kubernetes-sigs/kro/pkg/simpleschema` - Human-friendly schema conversion
- **K8s**: `k8s.io/apiextensions-apiserver v0.31.0` - OpenAPI types
- **YAML**: `gopkg.in/yaml.v3 v3.0.1` - YAML parsing and encoding

## Testing

Run the renderer and verify:
1. All 12 output files generated in `examples/expected-output/`
2. All 4 schema files generated in `examples/schemas/`
3. Stage outputs show proper addon composition
4. Environment-specific overrides applied correctly
5. No double-nesting in arrays (volumeMounts, volumes, containers)

## Known Limitations

1. Only supports `add`, `replace`, `remove`, `merge` patch operations
2. Array filter only supports simple equality checks: `[?(@.name=='value')]`
3. No validation of generated resources against K8s OpenAPI schemas
4. CEL expressions must use `has()` for optional fields to avoid runtime errors
