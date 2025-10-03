# Addon System Limitations and Patterns

This document describes known limitations in the addon system and patterns for working around them or solving them in future iterations.

---

## Pattern: Multiple Instances of Same Addon

### Use Case

A Component may need multiple instances of the same addon with different configurations.

**Example:**
```yaml
# Want to mount TWO persistent volumes
# - Application data volume
# - Cache data volume

apiVersion: platform/v1alpha1
kind: Component
spec:
  componentType: web-app

  addons:
    - name: persistent-volume
      instanceId: app-data      # Unique identifier (always required)
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 100Gi

    - name: persistent-volume
      instanceId: cache-data    # Different instance
      config:
        volumeName: cache-data
        mountPath: /app/cache
        size: 50Gi
```

### Instance ID Requirements

**Always required:**
- `instanceId` is **required** for all addon instances in Component
- Ensures consistent EnvSettings override structure
- Prevents breaking changes when adding additional instances later

**Naming:**
- Must be unique within the same addon name
- Used as a key in EnvSettings overrides
- Included in generated resource names

**Example:**
```yaml
addons:
  - name: network-policy
    instanceId: default       # Always required
    config:
      denyAll: true
```

### EnvSettings Overrides

EnvSettings always uses instanceId as keys:

```yaml
apiVersion: platform/v1alpha1
kind: EnvSettings
spec:
  # Override addon envOverrides
  addonOverrides:
    persistent-volume:        # Addon name
      app-data:               # instanceId as key
        size: 200Gi           # Override for this instance
        storageClass: premium
      cache-data:             # Different instance
        size: 100Gi
        storageClass: fast

    network-policy:           # Single instance - still uses instanceId
      default:                # instanceId as key
        allowIngress:
          - from: "namespace:production-gateway"
```

**Consistent structure:**
- Always: `addonOverrides.<addonName>.<instanceId>.*`
- No special cases for single vs multiple instances

### Alternative: Specialized Addons

Instead of using instanceId, create separate addon definitions:

```yaml
# Create specialized addons
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: app-data-volume
spec:
  # ... specialized for app data

---
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: cache-volume
spec:
  # ... specialized for cache
```

**Trade-off:** More addon definitions, but simpler configuration and clearer intent.

---

## Limitation 1: Targeting Runtime-Generated Resources

### Problem

Some resources are generated at runtime from dynamic sources (workload.yaml, forEach loops) and don't exist in the ComponentTypeDefinition template.

**Example 1: forEach-generated resources**
```yaml
# ComponentTypeDefinition creates multiple Ingress resources via forEach
resources:
  - id: public-ingress
    forEach: ${workload.endpoints.filter(e, e.visibility == "public")}
    template:
      apiVersion: networking.k8s.io/v1
      kind: Ingress
      metadata:
        name: ${metadata.name}-${item.name}

# TLS addon wants to patch these Ingresses
# But PE doesn't know the endpoint names at composition time!
```

**Example 2: workload.yaml-driven resources**
```yaml
# Ingress resources are created based on workload.yaml endpoints
# Endpoints only known at runtime when developer commits workload.yaml
# PE can't use queryResources=Ingress to select them
```

### Solution Options

#### Option A: Pattern-Based Targeting (Recommended)

Allow addons to target resources by pattern or type without specific ID:

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: tls-certificate
spec:
  schema:
    parameters:
      issuer: string | default=letsencrypt-prod
      domains: "[]string" | required=true
      # NO ingressName parameter needed

  targets:
    - resourceType: Ingress  # Target ALL Ingress resources
      # Or with pattern matching:
      # resourceId: "*-ingress"  # Match any resource ending in -ingress

  patches:
    - target:
        resourceType: Ingress
      patch:
        op: add
        path: /spec/tls/-
        value:
          hosts: ${spec.domains}
          secretName: ${metadata.name}-tls-secret
```

**Behavior:** Addon patches are applied to ALL matching resources at runtime, including forEach-generated ones.

#### Option B: CEL Expressions in Targeting

Allow CEL expressions in target selectors:

```yaml
targets:
  - resourceType: Ingress
    resourceId: ${workload.endpoints.filter(e, e.visibility == "public").map(e, metadata.name + "-" + e.name)}
```

**Trade-off:** More powerful but harder to validate at composition time.

#### Option C: Deferred Validation (Current Approach)

Document that some addons can only be validated at runtime:

```yaml
spec:
  schema:
    parameters:
      ingressName: string  # PE must know the resourceId pattern

  targets:
    - resourceType: Ingress
      resourceId: ${spec.ingressName}
```

**Limitation:** PE must understand ComponentDefinition structure and naming conventions.

### Recommended Pattern

For runtime-generated resources, design addons to target by **type** rather than **specific ID**:

```yaml
# Good: Generic targeting
targets:
  - resourceType: Ingress

# Avoid: Specific ID targeting (only works for static resources)
targets:
  - resourceId: specific-ingress-name
```

If specific targeting is needed, use **labels** or **annotations**:

```yaml
# ComponentDefinition marks resources with labels
resources:
  - id: public-ingress
    forEach: ${workload.endpoints.filter(e, e.visibility == "public")}
    template:
      metadata:
        labels:
          ingress-type: public  # Marker for addons

# Addon targets by label
targets:
  - resourceType: Ingress
    selector:
      matchLabels:
        ingress-type: public
```

---

## Limitation 3: Cross-Addon Dependencies

### Problem

Addons may need to reference configuration or outputs from other addons.

**Example 1: Backup addon needs volume name**
```yaml
# persistent-volume addon creates a volume
addons:
  - name: persistent-volume
    instanceId: app-data
    config:
      volumeName: app-data
      mountPath: /app/data

# backup addon needs to know which volume to backup
# Currently must repeat the configuration
  - name: volume-backup
    instanceId: default
    config:
      volumeName: app-data  # Duplicate! Must match above
```

**Example 2: Monitoring addon needs sidecar port**
```yaml
# metrics-sidecar addon exposes metrics on port 9090
addons:
  - name: metrics-sidecar
    instanceId: default
    config:
      port: 9090

# service-monitor addon needs to scrape that port
  - name: service-monitor
    instanceId: default
    config:
      port: 9090  # Duplicate! Must match above
```

### Solution: Addon Output References

Allow addons to declare outputs and reference other addon configurations:

#### Addon declares outputs:

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume
spec:
  schema:
    parameters:
      volumeName: string | required=true
      mountPath: string | required=true

  # Declare what this addon exposes
  outputs:
    - name: volumeName
      value: ${spec.volumeName}
    - name: claimName
      value: ${metadata.name}-${spec.volumeName}
```

#### Other addons reference outputs:

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: volume-backup
spec:
  # Declare dependencies
  dependencies:
    requires:
      - addon: persistent-volume
        reason: "Needs volume to backup"

  schema:
    parameters:
      # Reference other addon's output
      volumeName: string | default="${addons.persistentVolume.outputs.volumeName}"
      schedule: string | default="0 2 * * *"

  creates:
    - apiVersion: batch/v1
      kind: CronJob
      metadata:
        name: ${metadata.name}-backup
      spec:
        schedule: ${spec.schedule}
        jobTemplate:
          spec:
            template:
              spec:
                containers:
                  - name: backup
                    image: backup-tool:latest
                    volumeMounts:
                      - name: ${addons.persistentVolume.outputs.volumeName}
                        mountPath: /backup-source
                volumes:
                  - name: ${addons.persistentVolume.outputs.volumeName}
                    persistentVolumeClaim:
                      claimName: ${addons.persistentVolume.outputs.claimName}
```

#### Usage in Component:

```yaml
addons:
  - name: persistent-volume
    instanceId: app-storage
    config:
      volumeName: app-data
      mountPath: /app/data
      size: 100Gi

  - name: volume-backup
    instanceId: default
    config:
      # volumeName automatically resolved from persistent-volume outputs
      schedule: "0 2 * * *"
```

### Alternative Pattern: Convention-Based References

Use naming conventions that both addons agree on:

```yaml
# persistent-volume always creates: ${metadata.name}-${spec.volumeName}
# backup addon uses same convention

apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: volume-backup
spec:
  schema:
    parameters:
      volumeName: string | required=true

  creates:
    - apiVersion: batch/v1
      kind: CronJob
      spec:
        template:
          spec:
            volumes:
              - name: ${spec.volumeName}
                persistentVolumeClaim:
                  # Convention: component-name + volume-name
                  claimName: ${metadata.name}-${spec.volumeName}
```

**Trade-off:** Less explicit but simpler. Requires documentation of naming conventions.

---

## Limitation 4: Conditional Addon Application

### Problem

Some addons should only be applied based on runtime conditions.

**Example:**
```yaml
# Only create Ingress if workload has public endpoints
# Only create PVC if workload needs persistence

# Current limitation: addons are always applied if included in ComponentType
```

### Solution: Conditional Directives

Allow addons to declare conditions:

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: public-ingress
spec:
  # Only apply if workload has public endpoints
  condition: ${workload.endpoints.exists(e, e.visibility == "public")}

  creates:
    - forEach: ${workload.endpoints.filter(e, e.visibility == "public")}
      resource:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        # ...
```

Or at ComponentType level:

```yaml
platformAddons:
  - name: persistent-volume
    condition: ${spec.persistence.enabled}  # Only if developer enables
    config:
      volumeName: app-data
```

---

## Limitation 5: Addon Ordering and Patch Conflicts

### Problem

Multiple addons may patch the same resource path, causing conflicts or unexpected behavior.

**Example:**
```yaml
# Addon 1 sets container resources
patches:
  - target: {resourceType: Deployment}
    patch:
      op: add
      path: /spec/template/spec/containers/[?(@.name=='app')]/resources
      value: {requests: {cpu: "100m"}}

# Addon 2 also sets container resources
patches:
  - target: {resourceType: Deployment}
    patch:
      op: add
      path: /spec/template/spec/containers/[?(@.name=='app')]/resources
      value: {limits: {memory: "512Mi"}}

# Which one wins? Are they merged?
```

### Solution: Merge Strategies

Define explicit merge behavior:

```yaml
patches:
  - target: {resourceType: Deployment}
    patch:
      op: merge  # Deep merge objects instead of replace
      path: /spec/template/spec/containers/[?(@.name=='app')]/resources
      value:
        requests:
          cpu: "100m"
```

Or declare ordering via dependencies:

```yaml
dependencies:
  loadOrder:
    after: [base-resources]  # Apply this addon after base-resources
```

**Current workaround:** Document that platform engineers should be careful about addon ordering in ComponentType.

---

## Limitation 6: Dynamic Schema Based on Other Addons

### Problem

An addon's schema may need to change based on what other addons are present.

**Example:**
```yaml
# If persistent-volume addon is included, backup addon should show volume selector
# If no volumes, backup addon might backup different things (database dumps, etc.)
```

### Solution: Context-Aware Schemas

This is complex and may be out of scope for v1. Document as a known limitation.

**Workaround:** Create separate addon variants:
- `volume-backup` (requires persistent-volume)
- `database-backup` (requires database addon)
- `generic-backup` (standalone)

---

## Best Practices

### 1. Design Addons for Composability

```yaml
# Good: Self-contained, minimal dependencies
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: logging-sidecar

# Avoid: Assumes specific structure of other addons
```

### 2. Use Labels for Cross-Addon Communication

```yaml
# Addon 1 adds label
patches:
  - target: {resourceType: Deployment}
    patch:
      op: add
      path: /metadata/labels/storage-enabled
      value: "true"

# Addon 2 targets by label
targets:
  - resourceType: Deployment
    selector:
      matchLabels:
        storage-enabled: "true"
```

### 3. Document Naming Conventions

If using convention-based references, document them clearly:

```
Resource naming convention:
- PVC: ${metadata.name}-${volumeName}
- ConfigMap: ${metadata.name}-${configName}
- Secret: ${metadata.name}-${secretName}
```

### 4. Prefer Type-Based Targeting Over ID-Based

```yaml
# Good: Works with forEach and runtime resources
targets:
  - resourceType: Ingress

# Avoid: Only works with static resources
targets:
  - resourceId: specific-ingress
```

### 5. Use Dependencies to Encode Requirements

```yaml
dependencies:
  requires:
    - addon: persistent-volume
      reason: "Backup addon requires a volume to backup"

  conflictsWith:
    - addon: ephemeral-storage
      reason: "Cannot use backup with ephemeral storage"

  loadOrder:
    after: [persistent-volume]  # Apply patches after volume addon
```

---

## Future Enhancements

These limitations suggest potential future enhancements:

1. **Addon Outputs and References**: Full support for cross-addon data flow
2. **Pattern Matching in Targets**: `resourceId: "*/public-*"`
3. **Conditional Application**: Runtime conditions for addon inclusion
4. **Merge Strategies**: Explicit control over patch conflicts
5. **Instance Management**: First-class support for multiple addon instances
6. **Runtime Validation**: Validate addon targets against actual generated resources

---

## Summary

| Limitation | Current Workaround | Future Solution |
|------------|-------------------|-----------------|
| Multiple addon instances | Create separate addon definitions | Support `instanceId` |
| Runtime-generated resources | Target by type, use labels | Pattern matching, CEL in targets |
| Cross-addon dependencies | Convention-based naming | Addon outputs and references |
| Conditional addons | Always apply, use forEach filters | Conditional directives |
| Patch conflicts | Careful ordering | Merge strategies, explicit ordering |
| Dynamic schemas | Create addon variants | Context-aware schemas (complex) |

Most limitations can be worked around with careful addon design and clear documentation. Future versions can add more sophisticated features as use cases emerge.
