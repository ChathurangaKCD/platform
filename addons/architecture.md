# Addon System Architecture

## Overview

The addon system provides a composable, plugin-based architecture that allows Platform Engineers (PEs) to augment ComponentDefinitions with reusable, cross-cutting capabilities without having to define separate CRDs for every possible variation.

## Core Concepts

### Resource Hierarchy

The system has multiple layers of abstraction:

1. **ComponentDefinition** (PE-authored)
   - Base template with K8s resource definitions
   - Defines core component behavior
   - Reusable across multiple component types

2. **Addon** (PE-authored)
   - Reusable augmentation units
   - Can be PE-only or developer-allowed
   - Declares `parameters` (static) and `envOverrides` (environment-specific)

3. **ComponentType** (PE-composed)
   - Intermediate resource combining ComponentDefinition + Addons
   - Specifies which addons are baked-in (PE-only)
   - Specifies which addons are available to developers
   - Generates a CRD for developers to use

4. **Component Instance** (Developer-created)
   - Instance of a ComponentType
   - Developers configure component + allowed addon parameters
   - PE-baked addons are transparent to developers

5. **EnvBinding** (Developer/PE-created)
   - Environment-specific overrides
   - Can override `envOverrides` from both component and addons
   - Cannot override `parameters` (those are static)

### What is an Addon?

An **Addon** is a reusable, composable unit that:
- Modifies or augments existing Kubernetes resources in a ComponentDefinition
- Adds new Kubernetes resources to a ComponentDefinition
- Exposes a well-defined schema with `parameters` (static) and `envOverrides` (environment-specific)
- Declares its impact on resources (what it creates, what it modifies)
- Can be marked as PE-only or developer-allowed
- Can be applied to any compatible ComponentDefinition

### Design Principles

1. **Composability**: Addons should compose cleanly with ComponentDefinitions and other addons
2. **Declarative Impact**: Addons must explicitly declare what resources they affect
3. **Schema-Driven**: Every addon has a JSON schema defining its configuration parameters
4. **UI-Friendly**: Metadata and schemas enable generic UI rendering without special-casing
5. **Targeting**: Addons can target specific resources by type, label, or ID
6. **Reusability**: Both platform-provided and PE-defined addons follow the same model
7. **Non-Destructive**: Addons augment existing resources using patches/overlays

## Addon Types

### 1. Resource Modifiers
Modify existing resources in a ComponentDefinition:
- Add volumes to Deployments/StatefulSets
- Inject sidecars into containers
- Add init containers
- Modify security contexts
- Add environment variables or config mounts

### 2. Resource Creators
Create new resources alongside the ComponentDefinition:
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
│  1. Select ComponentDefinition (base template)               │
│  2. Select PE-only Addons (baked into ComponentType)        │
│  3. Configure PE-only addons                                 │
│  4. Select developer-allowed Addons (optional for devs)      │
│  5. Set defaults for developer-allowed addons                │
│  6. Preview rendered K8s resources                           │
│  7. Register as ComponentType for developers                 │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│               Composition Engine                             │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. Load ComponentDefinition                                 │
│  2. Load PE-only Addons + configurations                     │
│  3. Apply PE-only addons (baked in)                          │
│  4. Generate CRD schema with:                                │
│     - Component parameters + envOverrides                    │
│     - Developer-allowed addon parameters + envOverrides      │
│  5. Generate resource templates with PE addon patches        │
│  6. Create ComponentType resource                            │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                      ComponentType                           │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  • CRD Schema (for developers)                               │
│  • Resource Templates (with PE addons applied)               │
│  • Developer-allowed addons list                             │
│  • Validation Rules                                          │
│                                                               │
└─────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                  Developer Workflow                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. Select ComponentType (e.g., ProductionWebApp)            │
│  2. Configure component parameters                           │
│  3. Opt into developer-allowed addons                        │
│  4. Configure allowed addon parameters                       │
│  5. Create Component instance                                │
│  6. Create EnvBinding per environment                        │
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
│  2. Load ComponentType                                       │
│  3. Apply developer-configured addons                        │
│  4. Load EnvBinding for target environment                   │
│  5. Apply environment overrides                              │
│  6. Render final K8s resources                               │
│  7. Apply to cluster                                         │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## How Addons Work

### 1. Addon Definition

Each addon is defined as a CRD with:
- **Metadata**: Name, description, category, compatibility rules
- **Schema**: Configuration parameters (JSON Schema)
- **Resource Targets**: Which resources in the ComponentDefinition it affects
- **Patches**: How it modifies existing resources
- **Resources**: New resources it creates

### 2. Composition Process

When a PE selects a ComponentDefinition + Addons:

1. **Schema Merging**: Addon schemas are merged into the component CRD schema
2. **Resource Augmentation**: Addon patches are applied to existing resources
3. **Resource Addition**: New resources from addons are added to the template
4. **Validation**: Ensure no conflicts between addons
5. **Generation**: Output final ComponentDefinition with addons baked in

### 3. Developer Experience

Developers interact with ComponentTypes:
- See a single CRD generated from ComponentType
- CRD includes:
  - Component `parameters` and `envOverrides`
  - Developer-allowed addon `parameters` and `envOverrides`
- PE-baked addons are transparent (already applied to resources)
- Can opt into developer-allowed addons
- Can override `envOverrides` per environment in EnvBinding
- Cannot override `parameters` (those are static)

### 4. Platform Engineer Experience

PEs create ComponentTypes by composing ComponentDefinition + Addons:

**Via CLI:**
```bash
oc component-type create production-web-app \
  --definition web-app \
  --platform-addons persistent-volume,network-policy \
  --developer-addons config-files,logging-sidecar
```

**Via UI:**
- Visual builder with addon categorization
- Mark addons as "PE-only" or "Developer-allowed"
- Configure PE-only addons (baked into ComponentType)
- Set defaults for developer-allowed addons

**Via YAML:**
- Direct ComponentType resource definition
- Git-based workflow for version control

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

ComponentTypes may need to use the same addon multiple times with different configurations.

### Instance ID

When using an addon more than once, use `instanceId` to differentiate instances:

```yaml
# ComponentType with multiple volume instances
platformAddons:
  - name: persistent-volume
    instanceId: app-data        # Required for multiple instances
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
```

### EnvBinding with Instance IDs

EnvBinding overrides always use instanceId as keys:

```yaml
# EnvBinding
platformAddonOverrides:
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
- Ensures consistent EnvBinding override structure
- Prevents breaking changes when adding more instances later
- Resource names include instanceId: `${metadata.name}-${instanceId}-resource`

## Parameters vs EnvOverrides

Following the ComponentDefinition model, addons distinguish between:

### Parameters (Static)
- Set once at component creation
- Same across all environments
- Cannot be overridden in EnvBinding
- Examples: volume mount paths, container names, addon behavior toggles

### EnvOverrides (Environment-specific)
- Can differ per environment
- Overridable in EnvBinding
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

1. **Separation of Concerns**: PEs control infrastructure/security, devs control application config
2. **Reusability**: Define addons once, use across many component types
3. **Maintainability**: Update addon once, all components benefit
4. **Discoverability**: UI can list all available addons with permissions
5. **Flexibility**: PEs create custom addons, control who can use them
6. **Developer Simplicity**: Devs get simple CRD with only relevant addons
7. **Governance**: Platform team controls infrastructure addons, devs control app config
8. **Environment Awareness**: `envOverrides` allow environment-specific tuning
