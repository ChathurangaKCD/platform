# Addon Composition with ComponentDefinitions

This document describes how addons are composed with ComponentDefinitions to create ComponentTypes and Component instances.

## Two-Stage Composition

The system uses a two-stage composition model:

### Stage 1: ComponentType Creation (PE-time)

Platform Engineer composes ComponentDefinition + Platform Addons → ComponentType

### Stage 2: Component Instance Creation (Runtime)

Developer creates Component instance from ComponentType + Developer Addons

## Stage 1: Creating ComponentType

### High-Level Flow

```
ComponentDefinition
        +
Platform Addons (PE-configured)
        +
Developer Addon Allowlist
        ↓
Composition Engine
        ↓
  ┌─────┴─────┐
  │           │
  ▼           ▼
CRD Schema  Resource Templates (with platform addon patches applied)
        ↓
  ComponentType
```

### Step-by-Step Process

#### 1. PE Defines ComponentType

```yaml
apiVersion: platform/v1alpha1
kind: ComponentType
metadata:
  name: production-web-app
spec:
  componentDefinitionRef:
    name: web-app

  # PE-only addons (baked in)
  platformAddons:
    - name: persistent-volume
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 50Gi
        storageClass: fast

    - name: network-policy
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress"

  # Developer-allowed addons
  developerAddons:
    allowed:
      - name: config-files
      - name: logging-sidecar
        defaults:
          enabled: true
          logLevel: info
```

**Note:** When using the same addon multiple times, add `instanceId` to differentiate:
```yaml
platformAddons:
  - name: persistent-volume
    instanceId: app-data      # Required for multiple instances
    config:
      volumeName: app-data
      mountPath: /app/data
  - name: persistent-volume
    instanceId: cache-data    # Different instance
    config:
      volumeName: cache-data
      mountPath: /app/cache
```

#### 2. Load ComponentDefinition

```yaml
apiVersion: platform/v1alpha1
kind: ComponentDefinition
metadata:
  name: web-app
spec:
  schema:
    parameters:
      replicas: integer | default=1
    envOverrides:
      maxReplicas: integer | default=3
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        # ... deployment spec
```

#### 3. Load Platform Addons

Load addons specified in `platformAddons`:

- `persistent-volume` (PE-only, allowedFor: platform-engineer)
- `network-policy` (PE-only, allowedFor: platform-engineer)

#### 4. Validate Compatibility

Check addon dependencies and conflicts:

```javascript
function validateCompatibility(componentDef, addons) {
  const errors = [];

  // Check each addon's targets match available resources
  addons.forEach((addon) => {
    addon.spec.targets.forEach((target) => {
      const hasMatch = componentDef.spec.resources.some((r) =>
        matchesTarget(r, target)
      );

      if (!hasMatch) {
        errors.push({
          addon: addon.metadata.name,
          error: `No matching resource for target: ${target.resourceType}`,
        });
      }
    });
  });

  // Check addon dependencies
  addons.forEach((addon) => {
    addon.spec.dependencies?.requires?.forEach((dep) => {
      const hasDep = addons.some((a) => a.metadata.name === dep.addon);
      if (!hasDep) {
        errors.push({
          addon: addon.metadata.name,
          error: `Missing required addon: ${dep.addon}`,
        });
      }
    });
  });

  // Check addon conflicts
  addons.forEach((addon) => {
    addon.spec.dependencies?.conflictsWith?.forEach((conflict) => {
      const hasConflict = addons.some(
        (a) => a.metadata.name === conflict.addon
      );
      if (hasConflict) {
        errors.push({
          addon: addon.metadata.name,
          error: `Conflicts with addon: ${conflict.addon}`,
        });
      }
    });
  });

  return errors;
}
```

#### 5. Apply Platform Addons to Resources

Platform addons are applied to resources immediately:

```javascript
function applyPlatformAddons(componentDef, platformAddons, configs) {
  let resources = cloneDeep(componentDef.spec.resources);
  const newResources = [];

  platformAddons.forEach((addon) => {
    const config = configs[addon.name];

    // Create new resources
    addon.spec.creates?.forEach((createSpec) => {
      const resource = renderTemplate(createSpec, {
        metadata: config.metadata,
        spec: config,
      });
      newResources.push(resource);
    });

    // Apply patches
    addon.spec.patches?.forEach((patchSpec) => {
      applyPatchToTargets(resources, patchSpec, config);
    });
  });

  return {
    resources: [...resources, ...newResources],
  };
}
```

**Result**: Resources now include volume mounts, network policies, etc.

#### 6. Merge Developer Addon Schemas

Only developer-allowed addon schemas are merged into the CRD:

```javascript
function mergeDeveloperAddonSchemas(componentDef, developerAddons) {
  const baseSchema = componentDef.spec.schema;
  const mergedSchema = cloneDeep(baseSchema);

  // Platform addon schemas are NOT included (already applied to resources)

  // Add each developer-allowed addon's schema
  developerAddons.allowed.forEach((addonRef) => {
    const addon = loadAddon(addonRef.name);
    const addonKey = camelCase(addon.metadata.name);

    mergedSchema.properties[addonKey] = {
      type: "object",
      description: addon.spec.description,
      properties: {
        ...addon.spec.schema.parameters,
        ...addon.spec.schema.envOverrides,
      },
    };
  });

  return mergedSchema;
}
```

**Example Result (CRD schema exposed to developers):**

```json
{
  "type": "object",
  "properties": {
    "replicas": {
      "type": "number",
      "default": 1
    },
    "maxReplicas": {
      "type": "number",
      "default": 3
    },

    // Developer-allowed addons only
    "configFiles": {
      "type": "object",
      "description": "Mount ConfigMaps/Secrets",
      "properties": {
        "configs": { "type": "array" }
      }
    },
    "loggingSidecar": {
      "type": "object",
      "description": "Logging sidecar",
      "properties": {
        "enabled": { "type": "boolean", "default": true },
        "logLevel": { "type": "string", "default": "info" }
      }
    }
  }
}
```

Note: `persistentVolume` and `networkPolicy` (platform addons) are NOT in the schema.

#### 7. Generate ComponentType

The composition engine creates the ComponentType resource:

```yaml
apiVersion: platform/v1alpha1
kind: ComponentType
metadata:
  name: production-web-app
spec:
  # ... original spec ...

status:
  generatedCRD:
    apiVersion: platform/v1alpha1
    kind: ProductionWebApp
  appliedPlatformAddons:
    - name: persistent-volume
      version: "1.0"
    - name: network-policy
      version: "1.0"
  conditions:
    - type: Ready
      status: "True"
```

---

## Stage 2: Component Instance Creation

When developer creates a Component instance:

```yaml
apiVersion: platform/v1alpha1
kind: ProductionWebApp
metadata:
  name: customer-portal
spec:
  replicas: 3
  maxReplicas: 10

  # Developer-allowed addons
  configFiles:
    configs:
      - name: app-config
        type: configmap
        mountPath: /etc/config

  loggingSidecar:
    enabled: true
    logLevel: debug
```

The controller:

1. Loads ComponentType
2. Starts with resources that already have platform addons applied
3. Applies developer-configured addons (config-files, logging-sidecar)
4. Renders final K8s resources
5. Applies to cluster

---

## EnvBinding Overrides

When deploying to production environment:

```yaml
apiVersion: platform/v1alpha1
kind: EnvBinding
metadata:
  name: customer-portal-prod
spec:
  environment: production

  # Override component envOverrides
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
      outputDestination: elasticsearch.prod.svc:9200
```

The controller merges these overrides when rendering resources for the production environment.

---

## Summary

The two-stage composition model provides:

1. **Stage 1 (PE-time)**: ComponentDefinition + Platform Addons → ComponentType

   - Platform addons baked into resources
   - Only developer-allowed addons in CRD schema
   - Generates CRD for developers

2. **Stage 2 (Runtime)**: Component Instance + Developer Addons + EnvBinding
   - Developer configures allowed addons
   - EnvBinding overrides `envOverrides` per environment
   - Final resources deployed to cluster

This separation ensures:

- **Platform control**: PEs enforce infrastructure policies
- **Developer flexibility**: Devs configure application concerns
- **Environment awareness**: Different configs per environment
- **Clear boundaries**: Platform vs application responsibilities

---

## Implementation Details

### Apply Addon Patches

Apply addon modifications to resources in order:

```javascript
function applyAddons(componentDef, addons, addonConfigs) {
  // Clone base resources
  let resources = cloneDeep(componentDef.spec.resources);
  const newResources = [];

  // Sort addons by dependency order
  const sortedAddons = topologicalSort(addons);

  sortedAddons.forEach((addon) => {
    const config = addonConfigs[addon.metadata.name];

    // Create new resources
    addon.spec.creates?.forEach((createSpec) => {
      // Handle forEach loops
      if (createSpec.forEach) {
        const items = evaluateCEL(createSpec.forEach, {
          spec: config,
          metadata: componentDef.metadata,
        });

        items.forEach((item) => {
          const resource = renderTemplate(createSpec.resource, {
            metadata: componentDef.metadata,
            spec: config,
            item,
          });
          newResources.push(resource);
        });
      } else {
        const resource = renderTemplate(createSpec, {
          metadata: componentDef.metadata,
          spec: config,
        });
        newResources.push(resource);
      }
    });

    // Apply patches
    addon.spec.patches?.forEach((patchSpec) => {
      // Handle forEach loops
      if (patchSpec.forEach) {
        const items = evaluateCEL(patchSpec.forEach, {
          spec: config,
          metadata: componentDef.metadata,
        });

        items.forEach((item) => {
          applyPatchToTargets(resources, patchSpec, config, item);
        });
      } else {
        applyPatchToTargets(resources, patchSpec, config);
      }
    });
  });

  return {
    resources: [...resources, ...newResources],
  };
}

function applyPatchToTargets(resources, patchSpec, config, item = null) {
  const targets = findTargets(resources, patchSpec.target);

  targets.forEach((target) => {
    const patchValue = renderTemplate(patchSpec.patch.value, {
      spec: config,
      item,
      metadata: target.metadata,
    });

    const path = renderTemplate(patchSpec.patch.path, {
      spec: config,
      item,
    });

    applyJSONPatch(target, {
      op: patchSpec.patch.op,
      path,
      value: patchValue,
    });
  });
}
```

#### 6. Generate Final ComponentDefinition

Create a new ComponentDefinition with addons baked in:

```yaml
apiVersion: platform/v1alpha1
kind: ComponentDefinition
metadata:
  name: web-app-with-storage
  labels:
    composedFrom: web-app
    addons: persistent-volume,tls-certificate
spec:
  schema:
    # Merged schema from step 4
    parameters:
      replicas: integer | default=1
      persistentVolume:
        volumeName: string | required=true
        size: string | default=10Gi
      tlsCertificate:
        issuer: string | default=letsencrypt-prod
        domains: "[]string" | required=true

  resources:
    # Original resources from ComponentDefinition
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${metadata.name}
        spec:
          template:
            spec:
              containers:
                - name: app
                  # ... original container spec
                  volumeMounts: # ← Added by persistent-volume addon
                    - name: ${spec.persistentVolume.volumeName}
                      mountPath: ${spec.persistentVolume.mountPath}
              volumes: # ← Added by persistent-volume addon
                - name: ${spec.persistentVolume.volumeName}
                  persistentVolumeClaim:
                    claimName: ${metadata.name}-${spec.persistentVolume.volumeName}

    # New resources created by addons
    - id: data-pvc # ← Added by persistent-volume addon
      template:
        apiVersion: v1
        kind: PersistentVolumeClaim
        metadata:
          name: ${metadata.name}-${spec.persistentVolume.volumeName}
        spec:
          accessModes: [ReadWriteOnce]
          storageClassName: ${spec.persistentVolume.storageClass}
          resources:
            requests:
              storage: ${spec.persistentVolume.size}

    - id: tls-cert # ← Added by tls-certificate addon
      template:
        apiVersion: cert-manager.io/v1
        kind: Certificate
        # ... certificate spec
```

---

## Composition Strategies

### Strategy 1: Bake-In (Recommended for PE Templates)

Addons are composed at **component type creation time** and the result is saved as a new ComponentDefinition.

**Pros:**

- Simple for developers (no addon awareness)
- Stable, validated configuration
- Performance (no runtime composition)

**Cons:**

- Less flexible (can't toggle addons per instance)
- Proliferation of component types

**Use Case:** Platform Engineer creating reusable templates for developers

```yaml
# PE creates this once
apiVersion: platform/v1alpha1
kind: ComponentDefinition
metadata:
  name: production-web-app # ← New component type
  annotations:
    platform/composed-from: web-app
    platform/addons: persistent-volume,tls-certificate,monitoring
spec:
  # Fully baked schema + resources
```

---

### Strategy 2: Runtime Composition (Optional)

Addons are composed **when component instance is created**.

**Pros:**

- Developers can opt-in/out of addons per instance
- Fewer component types to maintain

**Cons:**

- More complex reconciliation
- Validation at runtime
- Performance overhead

**Use Case:** Advanced developers who want flexibility

```yaml
# Developer specifies base + addons
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: my-app
spec:
  componentDefinition: web-app
  addons:
    - name: persistent-volume
      config:
        volumeName: data
        size: 50Gi
        mountPath: /app/data
  # ... component parameters
```

---

## Addon Ordering & Dependencies

### Dependency Declaration

Addons can declare dependencies:

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: database-init
spec:
  dependencies:
    requires:
      - addon: persistent-volume
        reason: "Needs volume to store initialization scripts"

    loadOrder:
      after: [persistent-volume]
      before: [monitoring]
```

### Topological Sort

The composition engine sorts addons to respect dependencies:

```javascript
function topologicalSort(addons) {
  const graph = new Map();
  const sorted = [];
  const visited = new Set();

  // Build dependency graph
  addons.forEach((addon) => {
    graph.set(addon.metadata.name, {
      addon,
      deps: addon.spec.dependencies?.requires?.map((d) => d.addon) || [],
    });
  });

  function visit(name) {
    if (visited.has(name)) return;
    visited.add(name);

    const node = graph.get(name);
    node.deps.forEach((dep) => visit(dep));

    sorted.push(node.addon);
  }

  addons.forEach((addon) => visit(addon.metadata.name));

  return sorted;
}
```

**Example:**

Addons: `[tls-certificate, persistent-volume, database-init]`

Dependencies:

- `database-init` requires `persistent-volume`

Sorted order: `[persistent-volume, database-init, tls-certificate]`

---

## Patch Application Details

### Target Matching

Find resources that match addon's target spec:

```javascript
function findTargets(resources, targetSpec) {
  return resources.filter((resource) => {
    // Match by resource type
    if (targetSpec.resourceType) {
      const types = Array.isArray(targetSpec.resourceType)
        ? targetSpec.resourceType
        : [targetSpec.resourceType];

      if (!types.includes(resource.template.kind)) {
        return false;
      }
    }

    // Match by resource ID
    if (targetSpec.resourceId && resource.id !== targetSpec.resourceId) {
      return false;
    }

    // Match by selector
    if (targetSpec.selector) {
      const labels = resource.template.metadata?.labels || {};
      return matchesSelector(labels, targetSpec.selector);
    }

    return true;
  });
}
```

### JSONPath Resolution

Resolve JSONPath expressions for patch application:

```javascript
function applyJSONPatch(target, patch) {
  const path = patch.path;

  // Handle array append: /path/-
  if (path.endsWith("/-")) {
    const arrayPath = path.slice(0, -2);
    const array = getValueAtPath(target, arrayPath) || [];
    array.push(patch.value);
    setValueAtPath(target, arrayPath, array);
    return;
  }

  // Handle array filter: /path/[?(@.name=='foo')]
  if (path.includes("[?(@")) {
    const [arrayPath, filter] = parseFilter(path);
    const array = getValueAtPath(target, arrayPath);
    const index = array.findIndex((item) => evaluateFilter(item, filter));

    if (index >= 0) {
      const itemPath = path.replace(/\[.*\]/, `[${index}]`);
      applyJSONPatch(target, { ...patch, path: itemPath });
    }
    return;
  }

  // Standard path
  switch (patch.op) {
    case "add":
    case "replace":
      setValueAtPath(target, path, patch.value);
      break;

    case "remove":
      removeValueAtPath(target, path);
      break;

    case "merge":
      const existing = getValueAtPath(target, path) || {};
      setValueAtPath(target, path, deepMerge(existing, patch.value));
      break;
  }
}
```

### Merge Strategies

When multiple addons modify the same path:

```javascript
const MERGE_STRATEGIES = {
  // Arrays: append all values
  array: (existing, newValue) => {
    return [...(existing || []), newValue];
  },

  // Objects: deep merge
  object: (existing, newValue) => {
    return deepMerge(existing || {}, newValue);
  },

  // Primitives: last writer wins
  primitive: (existing, newValue) => {
    return newValue;
  },
};

function resolveMerge(path, patches) {
  const type = inferType(patches[0].value);
  const strategy = MERGE_STRATEGIES[type];

  return patches.reduce((result, patch) => {
    return strategy(result, patch.value);
  }, null);
}
```

---

## CEL Expression Evaluation

Addons use CEL expressions for dynamic values:

### Available Context

```javascript
const celContext = {
  // Component instance metadata
  metadata: {
    name: "my-app",
    namespace: "default",
    labels: {
      /* ... */
    },
  },

  // Addon configuration
  spec: {
    volumeName: "data",
    size: "50Gi",
    // ... all addon config
  },

  // Build context (injected by platform)
  build: {
    image: "gcr.io/project/my-app:v1.2.3",
    tag: "v1.2.3",
  },

  // Current item (in forEach loops)
  item: {
    /* ... */
  },

  // Access to other resources
  resources: [
    /* ... */
  ],
};
```

### Example Evaluations

```javascript
// Simple interpolation
"${metadata.name}-pvc";
// → "my-app-pvc"

// Object construction
"${spec.endpoints.map(e, {name: e.name, port: e.port})}";
// → [{name: 'api', port: 8080}, {name: 'admin', port: 9090}]

// Filtering
"${spec.endpoints.filter(e, e.visibility == 'public')}";
// → [{name: 'api', visibility: 'public', ...}]

// Conditional
"${spec.scaleToZero.enabled ? 0 : 1}";
// → 0 or 1

// Reduce
"${spec.files.reduce((acc, f) => {acc[f.name] = f.content; return acc;}, {})}";
// → {file1: 'content1', file2: 'content2'}
```

---

## Example: Complete Composition

### Input

**ComponentDefinition:**

```yaml
apiVersion: platform/v1alpha1
kind: ComponentDefinition
metadata:
  name: web-app
spec:
  schema:
    parameters:
      replicas: integer | default=1
  resources:
    - id: deployment
      template:
        kind: Deployment
        spec:
          replicas: ${spec.replicas}
          template:
            spec:
              containers:
                - name: app
                  image: ${build.image}
```

**Addons:**

1. `persistent-volume` configured with:

   - volumeName: data
   - size: 50Gi
   - mountPath: /app/data

2. `logging-sidecar` (default config)

### Output

**Generated ComponentDefinition:**

```yaml
apiVersion: platform/v1alpha1
kind: ComponentDefinition
metadata:
  name: web-app-with-storage-logging
spec:
  schema:
    parameters:
      replicas: integer | default=1
      persistentVolume:
        volumeName: string | required=true
        size: string | default=10Gi
        mountPath: string | required=true
      loggingSidecar:
        enabled: boolean | default=true
        logLevel: string | default=info enum="debug,info,warn,error"

  resources:
    - id: deployment
      template:
        kind: Deployment
        spec:
          replicas: ${spec.replicas}
          template:
            spec:
              containers:
                - name: app
                  image: ${build.image}
                  volumeMounts:
                    - name: ${spec.persistentVolume.volumeName}
                      mountPath: ${spec.persistentVolume.mountPath}

                - name: fluent-bit # ← From logging-sidecar
                  image: fluent/fluent-bit:2.1
                  env:
                    - name: LOG_LEVEL
                      value: ${spec.loggingSidecar.logLevel}

              volumes:
                - name: ${spec.persistentVolume.volumeName}
                  persistentVolumeClaim:
                    claimName: ${metadata.name}-${spec.persistentVolume.volumeName}
                - name: varlog # ← From logging-sidecar
                  emptyDir: {}

    - id: data-pvc # ← From persistent-volume
      template:
        apiVersion: v1
        kind: PersistentVolumeClaim
        metadata:
          name: ${metadata.name}-${spec.persistentVolume.volumeName}
        spec:
          accessModes: [ReadWriteOnce]
          resources:
            requests:
              storage: ${spec.persistentVolume.size}
```

---

## Summary

Addon composition follows these principles:

1. **Load** ComponentDefinition and Addons
2. **Validate** compatibility and dependencies
3. **Merge** schemas into single CRD schema
4. **Sort** addons by dependency order
5. **Apply** patches and create resources
6. **Generate** final ComponentDefinition or component instance

The composition engine handles:

- Schema merging with namespacing
- Patch application with JSONPath
- CEL expression evaluation
- Dependency resolution
- Conflict detection

Result: Platform Engineers get composable, reusable addons that integrate seamlessly with ComponentDefinitions without code changes.
