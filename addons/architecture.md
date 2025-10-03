# Addon System Architecture

## Overview

The addon system provides a composable, plugin-based architecture that allows Platform Engineers (PEs) to augment ComponentTypeDefinitions with reusable, cross-cutting capabilities without having to define separate component types for every possible variation.

## Core Concepts

### Resource Hierarchy

The system has a simplified hierarchy:

1. **ComponentTypeDefinition** (PE-authored)
   - Base template with K8s resource definitions
   - Defines core component behavior
   - Reusable across multiple components

2. **Addon** (PE-authored)
   - Reusable augmentation units
   - Can be PE-only or developer-allowed
   - Declares `parameters` (static) and `envOverrides` (environment-specific)

3. **Component** (Developer-created, single CRD)
   - Developers directly create Component resources
   - Specifies `componentType` (which ComponentTypeDefinition to use)
   - Has `parameters` (merged from ComponentTypeDefinition)
   - Has `addons[]` array with addon instances and their configurations
   - Has `build` field (platform-injected)

4. **EnvSettings** (Developer/PE-created)
   - Environment-specific overrides
   - Can override `envOverrides` from both component and addons
   - Cannot override `parameters` (those are static)

### What is an Addon?

An **Addon** is a reusable, composable unit that:
- Modifies or augments existing Kubernetes resources in a ComponentTypeDefinition
- Adds new Kubernetes resources to a ComponentTypeDefinition
- Exposes a well-defined schema with `parameters` (static) and `envOverrides` (environment-specific)
- Declares its impact on resources (what it creates, what it modifies)
- Can be marked as PE-only or developer-allowed
- Can be applied to any compatible ComponentTypeDefinition

### Design Principles

1. **Composability**: Addons should compose cleanly with ComponentTypeDefinitions and other addons
2. **Declarative Impact**: Addons must explicitly declare what resources they affect
3. **Schema-Driven**: Every addon has a schema defining its configuration parameters
4. **UI-Friendly**: Metadata and schemas enable generic UI rendering without special-casing
5. **Targeting**: Addons can target specific resources by type, label, or ID
6. **Reusability**: Both platform-provided and PE-defined addons follow the same model
7. **Non-Destructive**: Addons augment existing resources using patches/overlays

## Addon Types

### 1. Resource Modifiers
Modify existing resources in a ComponentTypeDefinition:
- Add volumes to Deployments/StatefulSets
- Inject sidecars into containers
- Add init containers
- Modify security contexts
- Add environment variables or config mounts

### 2. Resource Creators
Create new resources alongside the ComponentTypeDefinition:
- PersistentVolumeClaims
- Secrets
- ConfigMaps
- NetworkPolicies
- ServiceAccounts
- RBAC rules

### 3. Hybrid Addons
Both create new resources AND modify existing ones:
- Volume addon: Creates PVC + mounts it to container
- TLS addon: Creates Certificate resource + adds volume mounts + updates Ingress
- Service Mesh addon: Creates ServiceEntry + modifies Deployment annotations

## Architecture Components

```
┌─────────────────────────────────────────────────────────────┐
│                  Platform Engineer Workflow                  │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. Define ComponentTypeDefinition (base template)           │
│  2. Define Addons (reusable units)                           │
│  3. Publish ComponentTypeDefinitions for developers          │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Developer Workflow                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. Create Component resource (kind: Component)              │
│  2. Specify componentType (which ComponentTypeDefinition)    │
│  3. Configure parameters (from ComponentTypeDefinition)      │
│  4. Add addon instances to addons[] array                    │
│  5. Configure each addon's parameters                        │
│  6. Provide build field with repository and template info    │
│  7. Create EnvSettings per environment                       │
│     - Override envOverrides from component                   │
│     - Override envOverrides from addons                      │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│              Runtime Reconciliation                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. Load Component instance                                  │
│  2. Load ComponentTypeDefinition                             │
│  3. Load workload metadata from source repo                  │
│  4. Apply addon instances from Component.spec.addons[]       │
│  5. Load EnvSettings for target environment                  │
│  6. Apply environment overrides                              │
│  7. Render final K8s resources                               │
│  8. Apply to cluster                                         │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## How Addons Work

### 1. Addon Definition

Each addon is defined as a CRD with:
- **Metadata**: Name, description, category, compatibility rules
- **Schema**: Configuration parameters using Kro's Simple Schema
- **Resource Targets**: Which resources in the ComponentTypeDefinition it affects
- **Patches**: How it modifies existing resources
- **Resources**: New resources it creates

### 2. Composition Process

When a developer creates a Component with addons:

1. **Load ComponentTypeDefinition**: Get base template with resource definitions
2. **Load Addons**: Get addon definitions specified in `spec.addons[]`
3. **Apply Addon Patches**: Addon patches are applied to existing resources
4. **Add Addon Resources**: New resources from addons are added to the template
5. **Validate**: Ensure no conflicts between addons
6. **Render**: Output final K8s resources

### 3. Developer Experience

Developers create Component resources directly:
- Specify `componentType` field (which ComponentTypeDefinition to use)
- Configure `parameters` from the ComponentTypeDefinition
- Add `addons[]` array with addon instances:
  - Each addon has `name`, `instanceId`, and `config`
  - `instanceId` required to differentiate multiple instances of same addon
- Provide `build` field with repository and template information
- Can override `envOverrides` per environment in EnvSettings
- Cannot override `parameters` (those are static)

### 4. Platform Engineer Experience

PEs define ComponentTypeDefinitions and Addons:

**ComponentTypeDefinition (via YAML):**
```yaml
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: web-app
spec:
  schema:
    parameters:
      appType: string | default=stateless
    envOverrides:
      maxReplicas: integer | default=3
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        # ... deployment spec
```

**Addons (via YAML):**
```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume
spec:
  displayName: "Persistent Volume"
  schema:
    parameters:
      volumeName: string | required=true
      mountPath: string | required=true
    envOverrides:
      size: string | default=10Gi
  creates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      # ... PVC spec
  patches:
    - target:
        resourceType: Deployment
      # ... patches
```

## Addon Targeting

Addons need to specify which resources they modify. Targeting mechanisms:

### 1. By Resource Type
```yaml
targets:
  - resourceType: Deployment
  - resourceType: StatefulSet
```

### 2. By Resource ID
```yaml
targets:
  - resourceId: deployment  # Matches resource with id: deployment
```

### 3. By Label Selector
```yaml
targets:
  - selector:
      matchLabels:
        app.platform.io/workload: "true"
```

### 4. By Container Name
```yaml
targets:
  - resourceType: Deployment
    containerName: app  # Modifies specific container
```

## Conflict Resolution

When multiple addons target the same resource:

1. **Explicit Ordering**: Addons declare dependencies
2. **Merge Strategies**: Arrays append, objects deep-merge
3. **Validation**: Detect and warn on conflicting patches
4. **PE Override**: Platform engineer has final say in composition

## Addon Permissions

Addons declare who can use them:

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume
  labels:
    allowedFor: platform-engineer  # Options: platform-engineer, developer, both
```

**platform-engineer:** Only PEs can configure this addon when creating ComponentTypes
**developer:** Only developers can opt into this addon when creating Component instances
**both:** Can be used by both PEs (baked-in) and developers (opt-in)

### Use Cases

- **PE-only addons**: Security policies, network policies, resource quotas, monitoring sidecars
- **Developer-allowed addons**: ConfigMaps, logging configuration, init containers
- **Both**: Resource limits (PE sets defaults, dev can adjust within bounds)

## Multiple Instances of Same Addon

Components may need to use the same addon multiple times with different configurations.

### Instance ID

`instanceId` is **always required** for all addon instances to ensure consistent structure:

```yaml
# Component with multiple volume instances
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: my-app
spec:
  componentType: web-app

  parameters:
    maxReplicas: 3

  addons:
    - name: persistent-volume
      instanceId: app-data        # Always required
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 100Gi

    - name: persistent-volume
      instanceId: cache-data      # Different instance
      config:
        volumeName: cache-data
        mountPath: /app/cache
        size: 50Gi

    - name: network-policy
      instanceId: default         # Always required, even for single instance
      config:
        denyAll: true

  build:
    # Platform-injected
```

### EnvSettings with Instance IDs

EnvSettings overrides always use instanceId as keys:

```yaml
# EnvSettings
apiVersion: platform/v1alpha1
kind: EnvSettings
metadata:
  name: my-app-prod
spec:
  owner:
    componentName: my-app
  environment: production

  # Override component envOverrides
  overrides:
    maxReplicas: 20

  # Override addon envOverrides
  addonOverrides:
    persistent-volume:            # Addon name
      app-data:                   # instanceId
        size: 200Gi
        storageClass: premium
      cache-data:                 # Different instance
        size: 100Gi
        storageClass: fast

    network-policy:               # Single instance - still uses instanceId
      default:                    # instanceId
        allowIngress:
          - from: "namespace:production"
```

**Rules:**
- `instanceId` is **always required** for all addon instances
- Ensures consistent EnvSettings override structure
- Prevents breaking changes when adding more instances later
- Resource names include instanceId: `${metadata.name}-${instanceId}-resource`

## Parameters vs EnvOverrides

Following the ComponentTypeDefinition model, addons distinguish between:

### Parameters (Static)
- Set once at component creation
- Same across all environments
- Cannot be overridden in EnvSettings
- Examples: volume mount paths, container names, addon behavior toggles

### EnvOverrides (Environment-specific)
- Can differ per environment
- Overridable in EnvSettings
- Examples: storage size, storage class, replica counts, resource limits

```yaml
schema:
  parameters:
    volumeName: string | required=true      # Static: same mount point everywhere
    mountPath: string | required=true       # Static: same path everywhere
    containerName: string | default=app     # Static: same container everywhere

  envOverrides:
    size: string | default=10Gi pattern="^[0-9]+[EPTGMK]i$"           # Env-specific: dev=10Gi, prod=200Gi
    storageClass: string | default=standard enum="standard,fast,premium"  # Env-specific: dev=standard, prod=premium
```

## Benefits

1. **Simplified Architecture**: Single Component CRD, no intermediate resources
2. **Separation of Concerns**: PEs define ComponentTypeDefinitions and Addons, devs compose them
3. **Reusability**: Define addons once, use across many components
4. **Maintainability**: Update addon once, all components using it benefit
5. **Discoverability**: UI can list all available addons and ComponentTypeDefinitions
6. **Flexibility**: PEs create custom ComponentTypeDefinitions and addons for org needs
7. **Developer Control**: Developers explicitly choose and configure addons per component
8. **Environment Awareness**: `envOverrides` allow environment-specific tuning via EnvSettings
