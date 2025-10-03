# Addon System Documentation

This directory contains the complete specification and design for the Platform addon system.

## Overview

The addon system provides a composable, plugin-based architecture that allows developers to enhance their components with reusable addons.

## Key Concepts

### Resource Hierarchy

```
ComponentTypeDefinition (base template)
        +
Component (with addon instances)
        +
Build Context (platform-injected)
        ↓
Composition Engine
        ↓
Final K8s Resources
        +
EnvSettings (Environment-specific overrides)
```

### Unified Addon Model

All addons work the same way - developers add them directly to Component instances via the `addons[]` array. There is no distinction between platform-controlled and developer-controlled addons.

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

### [component-type.md](./component-type.md) [DEPRECATED]
**This document describes the old ComponentType intermediate resource which has been removed in the simplified design.**

### [composition.md](./composition.md)
**How addons compose with ComponentTypeDefinitions**

- Simplified single-stage composition model:
  - Runtime: ComponentTypeDefinition + Component (with addons) + EnvSettings → K8s Resources
- Step-by-step composition process
- Parameter merging from ComponentTypeDefinition
- Patch application logic
- Dependency resolution
- EnvSettings override merging
- CEL expression evaluation
- Build context injection

### [ui-integration.md](./ui-integration.md) [NEEDS UPDATE]
**UI rendering and interaction design**

- Developer workflow for adding addons to Components
- Generic form rendering from addon schemas
- Smart queries (queryContainers, queryResources)
- Impact preview logic
- API contracts
- Validation and error handling

## Quick Start

### For Developers

1. **Create Component with addons**:
   ```yaml
   apiVersion: platform/v1alpha1
   kind: Component
   metadata:
     name: customer-portal
   spec:
     componentType: web-app  # References ComponentTypeDefinition

     # Parameters from ComponentTypeDefinition
     parameters:
       replicas: 3

     # Addon instances
     addons:
       - name: persistent-volume
         instanceId: app-data  # Required for all addons
         config:
           volumeName: app-data
           mountPath: /app/data
           size: 50Gi
           storageClass: fast

       - name: config-files
         instanceId: app-config
         config:
           configs:
             - name: app-config
               type: configmap
               mountPath: /etc/config

     # Build context (platform-injected)
     build:
       image: gcr.io/project/customer-portal:v1.2.3
   ```

2. **Create EnvSettings for production**:
   ```yaml
   apiVersion: platform/v1alpha1
   kind: EnvSettings
   metadata:
     name: customer-portal-prod
   spec:
     componentRef:
       name: customer-portal
     environment: production

     # Override component envOverrides
     overrides:
       maxReplicas: 20

     # Override addon envOverrides (keyed by instanceId)
     addonOverrides:
       app-data:  # instanceId of persistent-volume addon
         size: 200Gi
         storageClass: premium
   ```

## Key Design Decisions

### 1. Single-Stage Runtime Composition

**Why**: Simplified model with unified addon handling
- No intermediate ComponentType resource
- All addons specified directly in Component
- Composition happens at runtime when Component is created

### 2. Unified Addon Model

**Why**: Simpler mental model for developers
- All addons work the same way
- No distinction between platform-controlled and developer-controlled
- Developers have full control over which addons to use

### 3. Parameters vs EnvOverrides

**Why**: Some config is structural, some is environmental
- `parameters`: Mount paths, container names (same everywhere)
- `envOverrides`: Sizes, classes, log levels (vary per env)
- Enforced at schema level, validated by controllers

### 4. Required instanceId for All Addons

**Why**: Support multiple instances of the same addon
- Enables using the same addon multiple times (e.g., multiple volumes)
- Used as key in EnvSettings addonOverrides
- Provides clear identity for each addon instance

## Benefits

1. **Simplicity**: Single Component CRD, no intermediate resources
2. **Flexibility**: Developers control which addons to use
3. **Reusability**: Addons can be used across any ComponentTypeDefinition
4. **Developer Experience**: Clear, direct addon specification
5. **Environment Awareness**: EnvSettings for dev→staging→prod overrides
6. **Extensibility**: Anyone can create custom addons for their needs
7. **Multiple Instances**: Support for multiple instances of the same addon
8. **Runtime Composition**: All composition at runtime, no pre-baking

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

**Q: How do I use multiple instances of the same addon?**
A: Use different `instanceId` values for each addon instance in the Component's `addons[]` array.

**Q: Can addons conflict with each other?**
A: The composition engine validates addon dependencies and conflicts at runtime when the Component is created.

**Q: How do environment-specific overrides work?**
A: Only fields in `envOverrides` can be overridden in EnvSettings. `parameters` are immutable across environments. Use the addon's `instanceId` as the key in `addonOverrides`.

**Q: Can I create custom addons?**
A: Yes! Addons are just CRDs. You can define custom addons following the schema spec.

**Q: What templating language is used?**
A: CEL (Common Expression Language) for all dynamic values. It's type-safe, sandboxed, and supports complex expressions.

**Q: What happened to ComponentType?**
A: The ComponentType intermediate resource has been removed in favor of a simpler design where developers directly create Component resources with addons.

## Related Documentation

- [ComponentTypeDefinition Proposal](../component_defs/proposal.md)
- [Platform Spec](../platform-spec.md)

## Contributing

When adding new addon types or extending the system:

1. Update relevant docs (architecture, schema-spec, examples)
2. Add concrete examples with both parameters and envOverrides
3. Update ui-integration with new UI patterns
4. Consider backward compatibility for existing ComponentTypes
