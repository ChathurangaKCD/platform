# Component Renderer Examples

This directory contains example manifests demonstrating incremental component rendering with addons and environment-specific overrides.

## Directory Structure

```
renderer/examples/
├── component-type-definitions/
│   └── deployment-component.yaml      # Base Deployment template
├── addons/
│   ├── pvc-addon.yaml                 # PVC + volume + mount
│   ├── emptydir-addon.yaml            # EmptyDir + mounts to multiple containers
│   └── sidecar-addon.yaml             # Adds sidecar container
├── components/
│   └── example-component.yaml         # Component instance with 3 addons
├── env-settings/
│   ├── dev-env.yaml                   # Development environment overrides
│   └── prod-env.yaml                  # Production environment overrides
└── expected-output/
    ├── no-env/                        # Without EnvSettings (Component defaults)
    │   ├── stage-1-base.yaml
    │   ├── stage-2-with-pvc.yaml
    │   ├── stage-3-with-sidecar.yaml
    │   └── stage-4-with-emptydir.yaml
    ├── dev/                           # With dev-env.yaml applied
    │   ├── stage-1-base.yaml
    │   ├── stage-2-with-pvc.yaml
    │   ├── stage-3-with-sidecar.yaml
    │   └── stage-4-with-emptydir.yaml
    └── prod/                          # With prod-env.yaml applied
        ├── stage-1-base.yaml
        ├── stage-2-with-pvc.yaml
        ├── stage-3-with-sidecar.yaml
        └── stage-4-with-emptydir.yaml
```

## Rendering Flow

Each stage shows incremental addon composition. Expected outputs are provided for three scenarios:

1. **no-env/** - Component defaults (no EnvSettings applied)
2. **dev/** - DEV environment overrides applied at every stage
3. **prod/** - PROD environment overrides applied at every stage

### Stage 1: Base Component

- **Input**: ComponentTypeDefinition + Component parameters
- **Output**: Basic Deployment with 1 container
- **EnvSettings Impact**:
  - `no-env`: cpu: 100m, memory: 256Mi
  - `dev`: cpu: 50m, memory: 128Mi
  - `prod`: cpu: 500m, memory: 1Gi

### Stage 2: Add PVC Addon

- **Addon**: `persistent-volume-claim` (instanceId: `app-data`)
- **Creates**: PersistentVolumeClaim resource
- **Patches**: Adds volume + volumeMount to `app` container
- **EnvSettings Impact**:
  - `no-env`: 10Gi standard storage
  - `dev`: 5Gi standard storage
  - `prod`: 100Gi premium storage

### Stage 3: Add Sidecar Addon

- **Addon**: `sidecar-container` (instanceId: `logger`)
- **Patches**: Adds `fluent-bit` container to Deployment
- **EnvSettings Impact on sidecar resources**:
  - `no-env`: cpu: 50m, memory: 64Mi
  - `dev`: cpu: 25m, memory: 32Mi
  - `prod`: cpu: 100m, memory: 128Mi

### Stage 4: Add EmptyDir Addon

- **Addon**: `emptydir-volume` (instanceId: `shared-logs`)
- **Patches**: Adds emptyDir volume mounted to **both** containers
  - `app`: `/var/log/app` (read-write)
  - `fluent-bit`: `/var/log/app` (read-only)
- **EnvSettings Impact on emptyDir**:
  - `no-env`: 1Gi disk-backed
  - `dev`: 512Mi disk-backed
  - `prod`: 5Gi memory-backed (tmpfs)

## Key Features Demonstrated

### 1. CEL Expression Evaluation

- Variable interpolation: `${metadata.name}`, `${build.image}`
- Resource references: `${spec.resources.requests.cpu}`
- Conditional rendering: `${spec.medium != "" ? spec.medium : omit()}`

### 2. Addon Composition

- **Creates**: New resources (PersistentVolumeClaim)
- **Patches**: Modify existing resources (add volumes, containers)
- **Multiple instances**: Same addon type with different instanceIds

### 3. Advanced Patching

- Array append: `/spec/template/spec/volumes/-`
- JSONPath filtering: `/containers/[?(@.name=='app')]/volumeMounts/-`
- ForEach loops: Apply same patch for each item in array

### 4. Environment Overrides

- Component-level: Override `envOverrides` fields
- Addon-level: Override addon configs by `instanceId`
- Merged at render time

### 5. Schema Validation (Simple Schema)

- Custom types: `MountConfig`, `EnvVar`
- Array types: `'[]string'`, `'[]MountConfig'`
- Validation markers: `required=true`, `default=value`

## Component Instance

The example component (`example-component.yaml`) declares:

```yaml
spec:
  componentType: deployment-component
  parameters:
    replicas: 2
    resources: { ... }
  addons:
    - name: persistent-volume-claim
      instanceId: app-data
      config: { ... }
    - name: sidecar-container
      instanceId: logger
      config: { ... }
    - name: emptydir-volume
      instanceId: shared-logs
      config:
        mounts:
          - containerName: app
            mountPath: /var/log/app
          - containerName: fluent-bit
            mountPath: /var/log/app
            readOnly: true
```

## Rendering with Go Code

To render these examples, the Go renderer will:

1. Load ComponentTypeDefinition
2. Load Component with addon references
3. Load EnvSettings for target environment
4. Merge parameters and overrides
5. Apply addons in order:
   - Evaluate addon templates with CEL
   - Create new resources
   - Apply patches to existing resources
6. Render final Kubernetes manifests

## Testing the Renderer

```bash
# Render for dev environment
go run renderer/main.go \
  --component=renderer/examples/components/example-component.yaml \
  --env=renderer/examples/env-settings/dev-env.yaml \
  --output=rendered-dev.yaml

# Render for prod environment
go run renderer/main.go \
  --component=renderer/examples/components/example-component.yaml \
  --env=renderer/examples/env-settings/prod-env.yaml \
  --output=rendered-prod.yaml
```

## Validation

Compare rendered output with expected output:

```bash
diff rendered-dev.yaml expected-output/stage-4-dev.yaml
diff rendered-prod.yaml expected-output/stage-4-prod.yaml
```
