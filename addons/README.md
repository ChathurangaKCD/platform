# Addon System Documentation

This directory contains the complete specification and design for the Platform addon system.

## Overview

The addon system provides a composable, plugin-based architecture that allows Platform Engineers (PEs) to create reusable component types by combining base ComponentDefinitions with Addons.

## Key Concepts

### Resource Hierarchy

```
ComponentDefinition (PE-authored base template)
        +
    Addons (PE-selected & configured)
        ↓
ComponentType (Intermediate resource with generated CRD)
        ↓
Component Instance (Developer-created)
        +
    EnvBinding (Environment-specific overrides)
        ↓
Final K8s Resources
```

### Addon Permissions

- **PE-only addons** (`allowedFor: platform-engineer`): Infrastructure concerns (storage, networking, security)
- **Developer-allowed addons** (`allowedFor: developer`): Application concerns (config, logging)
- **Both** (`allowedFor: both`): Shared concerns (resource limits)

### Parameters vs EnvOverrides

Following ComponentDefinition model:
- **parameters**: Static configuration, same across all environments
- **envOverrides**: Environment-specific, can be overridden in EnvBinding

## Documentation Files

### [architecture.md](./architecture.md)
**Core system design and architecture**

- Resource hierarchy (ComponentDefinition → ComponentType → Component)
- Design principles
- Addon types (modifiers, creators, hybrid)
- Two-stage composition process
- Benefits and use cases

### [schema-spec.md](./schema-spec.md)
**Formal Addon CRD specification**

- Complete Addon CRD structure
- Schema fields (metadata, schema, targets, creates, patches, dependencies, validation, ui)
- Parameters vs envOverrides distinction
- JSON Schema extensions for UI hints
- Schema merging rules
- EnvBinding override semantics

### [examples.md](./examples.md)
**Concrete addon examples**

1. Persistent Volume (PE-only) - Storage provisioning
2. Logging Sidecar (Developer-allowed) - Log forwarding
3. Config Files (Developer-allowed) - ConfigMaps/Secrets
4. Network Policy (PE-only) - Network isolation
5. Resource Limits (Both) - CPU/memory quotas
6. TLS Certificate (PE-only) - Certificate provisioning
7. Init Container (Developer-allowed) - Initialization tasks

Each example includes:
- Use case
- Complete addon definition with parameters/envOverrides
- How it modifies resources
- UI integration hints

### [component-type.md](./component-type.md)
**ComponentType intermediate resource specification**

- ComponentType CRD structure
- PE workflow for creating ComponentTypes
- Platform addon vs developer-allowed addon distinction
- Generated CRD schema
- Developer experience
- EnvBinding overrides
- Lifecycle management
- Advanced features (conditional addons, presets, cross-addon config)

### [composition.md](./composition.md)
**How addons compose with ComponentDefinitions**

- Two-stage composition model:
  1. PE-time: ComponentDefinition + Platform Addons → ComponentType
  2. Runtime: Component + Developer Addons + EnvBinding → K8s Resources
- Step-by-step composition process
- Schema merging algorithm
- Patch application logic
- Dependency resolution
- EnvBinding override merging
- CEL expression evaluation

### [ui-integration.md](./ui-integration.md)
**UI rendering and interaction design**

- Platform Engineer workflow (5 screens)
  - ComponentType builder
  - Addon selection (platform vs developer-allowed)
  - Addon configuration
  - Impact preview
  - CRD schema preview
- Developer workflow
- Generic form rendering from JSON Schema
- Smart queries (queryContainers, queryResources)
- Impact preview logic
- API contracts
- Validation and error handling

## Quick Start

### For Platform Engineers

1. **Create a ComponentType**:
   ```yaml
   apiVersion: platform/v1alpha1
   kind: ComponentType
   metadata:
     name: production-web-app
   spec:
     componentDefinitionRef:
       name: web-app

     platformAddons:
       - name: persistent-volume
         config:
           volumeName: app-data
           mountPath: /app/data
           size: 50Gi
           storageClass: fast

     developerAddons:
       allowed:
         - name: config-files
         - name: logging-sidecar
   ```

2. Controller generates a `ProductionWebApp` CRD for developers

### For Developers

1. **Create Component instance**:
   ```yaml
   apiVersion: platform/v1alpha1
   kind: ProductionWebApp
   metadata:
     name: customer-portal
   spec:
     replicas: 3

     # Opt into developer-allowed addons
     configFiles:
       configs:
         - name: app-config
           type: configmap
           mountPath: /etc/config
   ```

2. **Create EnvBinding for production**:
   ```yaml
   apiVersion: platform/v1alpha1
   kind: EnvBinding
   metadata:
     name: customer-portal-prod
   spec:
     environment: production

     overrides:
       maxReplicas: 20

     # Override platform addon envOverrides
     platformAddonOverrides:
       persistentVolume:
         size: 200Gi
         storageClass: premium

     # Override developer addon envOverrides
     addonOverrides:
       loggingSidecar:
         logLevel: warn
   ```

## Key Design Decisions

### 1. Two-Stage Composition

**Why**: Separate platform infrastructure concerns from application configuration
- Stage 1 (PE): Enforce policies, provision infrastructure
- Stage 2 (Dev): Configure application, override per environment

### 2. PE-Only vs Developer-Allowed Addons

**Why**: Clear separation of responsibilities
- PEs control security, networking, storage (infrastructure)
- Developers control config, logging, init containers (application)
- Prevents developers from bypassing platform policies

### 3. Parameters vs EnvOverrides

**Why**: Some config is structural, some is environmental
- `parameters`: Mount paths, container names (same everywhere)
- `envOverrides`: Sizes, classes, log levels (vary per env)
- Enforced at schema level, validated by controllers

### 4. Generic UI Rendering

**Why**: PEs can create custom addons without frontend changes
- All forms rendered from JSON Schema
- UI hints (queryContainers, queryResources) for smart dropdowns
- Impact preview computed from addon metadata

## Benefits

1. **Separation of Concerns**: Platform vs application responsibilities
2. **Reusability**: One ComponentDefinition → many ComponentTypes
3. **Governance**: PEs enforce policies via platform addons
4. **Developer Experience**: Simple CRDs, hidden complexity
5. **Environment Awareness**: envOverrides for dev→staging→prod
6. **Extensibility**: PEs create custom addons for org needs
7. **UI-Friendly**: Generic rendering, no special-casing
8. **Type Safety**: Generated CRDs with full validation

## Implementation Roadmap

### Phase 1: Core Framework
- [ ] Addon CRD definition
- [ ] ComponentType CRD definition
- [ ] Basic composition engine
- [ ] Schema merging logic
- [ ] Patch application (JSONPath + CEL)

### Phase 2: Platform Addons
- [ ] Persistent Volume addon
- [ ] Network Policy addon
- [ ] Resource Limits addon
- [ ] TLS Certificate addon

### Phase 3: Developer Addons
- [ ] Config Files addon
- [ ] Logging Sidecar addon
- [ ] Init Container addon

### Phase 4: UI Integration
- [ ] ComponentType builder
- [ ] Generic form renderer
- [ ] Impact preview
- [ ] CRD schema viewer

### Phase 5: EnvBinding
- [ ] EnvBinding CRD
- [ ] Override merging logic
- [ ] Environment promotion workflow

### Phase 6: Advanced Features
- [ ] Addon dependencies
- [ ] Conditional addons
- [ ] Addon presets
- [ ] Cross-addon configuration

## FAQ

**Q: Can developers bypass platform addons?**
A: No. Platform addons are baked into resources at ComponentType creation. Developers only see the final CRD schema, which doesn't include platform addon parameters.

**Q: Can addons conflict with each other?**
A: The composition engine validates addon dependencies and conflicts. PEs are warned if addons conflict, and must resolve before creating ComponentType.

**Q: How do environment-specific overrides work?**
A: Only fields in `envOverrides` can be overridden in EnvBinding. `parameters` are immutable across environments.

**Q: Can PEs create custom addons?**
A: Yes! Addons are just CRDs. PEs can define custom addons following the schema spec, and they'll automatically appear in the UI.

**Q: What templating language is used?**
A: CEL (Common Expression Language) for all dynamic values. It's type-safe, sandboxed, and supports complex expressions.

**Q: How are CRDs generated?**
A: ComponentType controller merges schemas from ComponentDefinition + developer-allowed addons, then generates and installs the CRD in the cluster.

## Related Documentation

- [ComponentDefinition Spec](../component_defs/spec.md)
- [ComponentDefinition Proposal](../component_defs/proposal.md)
- [Project Notes](../.local/notes.txt)

## Contributing

When adding new addon types or extending the system:

1. Update relevant docs (architecture, schema-spec, examples)
2. Add concrete examples with both parameters and envOverrides
3. Update ui-integration with new UI patterns
4. Consider backward compatibility for existing ComponentTypes
