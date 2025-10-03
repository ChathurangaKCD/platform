# Addon Composition with ComponentTypeDefinitions

This document describes how addons are composed with ComponentTypeDefinitions to create Component instances.

## Simplified Composition Model

The system uses a single-stage composition model at runtime:

### Component Creation (Runtime)

Developer creates Component instance (kind: Component) which references a ComponentTypeDefinition. The controller composes the final resources by applying addons from both the Component spec and the build context.

## Creating Components with Addons

### High-Level Flow

```
ComponentTypeDefinition (base template)
        +
Component (with addon instances)
        +
Build Context (platform-injected)
        ↓
Composition Engine
        ↓
Final K8s Resources (with all addons applied)
```

### Step-by-Step Process

#### 1. Developer Defines Component

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app  # References ComponentTypeDefinition

  # Merged parameters from ComponentTypeDefinition
  parameters:
    replicas: 3

  # Addon instances
  addons:
    - name: persistent-volume
      instanceId: app-data  # Required for all addon instances
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

  # Build field is platform-injected
  build:
    image: gcr.io/project/customer-portal:v1.2.3
```

**Note:** All addon instances require an `instanceId` to support multiple instances of the same addon:
```yaml
addons:
  - name: persistent-volume
    instanceId: app-data
    config:
      volumeName: app-data
      mountPath: /app/data
  - name: persistent-volume
    instanceId: cache-data
    config:
      volumeName: cache-data
      mountPath: /app/cache
```

#### 2. Load ComponentTypeDefinition

```yaml
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
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

#### 3. Load Addon Definitions

Load addons specified in Component's `addons[]` array:

- `persistent-volume`
- `config-files`

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

#### 5. Apply Addons to Resources

All addons from the Component's `addons[]` array are applied to resources at runtime:

```javascript
function applyAddons(componentTypeDef, component, addons) {
  let resources = cloneDeep(componentTypeDef.spec.resources);
  const newResources = [];

  addons.forEach((addonInstance) => {
    const addon = loadAddon(addonInstance.name);
    const config = addonInstance.config;

    // Create new resources
    addon.spec.creates?.forEach((createSpec) => {
      const resource = renderTemplate(createSpec, {
        metadata: component.metadata,
        spec: config,
        build: component.spec.build,
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

**Result**: Resources now include volume mounts, config files, etc.

#### 6. Render Final Resources

The controller:

1. Loads ComponentTypeDefinition
2. Starts with base resources from ComponentTypeDefinition
3. Applies all addon instances from Component's `addons[]` array
4. Merges parameters from Component spec
5. Injects build context from platform
6. Renders final K8s resources
7. Applies to cluster

---

## EnvSettings Overrides

When deploying to production environment:

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

  # Override addon envOverrides (unified under addonOverrides)
  addonOverrides:
    app-data:  # instanceId of the addon
      size: 200Gi
      storageClass: premium

    app-config:  # instanceId of the addon
      # config-specific overrides
```

The controller merges these overrides when rendering resources for the production environment.

---

## Summary

The simplified composition model provides:

1. **Runtime Composition**: ComponentTypeDefinition + Component (with addons) + EnvSettings
   - All addons specified in Component's `addons[]` array
   - No intermediate ComponentType resource
   - Addons applied at runtime when Component is created
   - EnvSettings overrides `envOverrides` per environment
   - Final resources deployed to cluster

This simplified approach ensures:

- **Unified addon model**: All addons work the same way
- **Developer control**: Developers add addons directly to Component
- **Environment awareness**: Different configs per environment via EnvSettings
- **Flexibility**: No pre-baked configurations, all composition at runtime

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

#### 6. Generate Final Resources

The composition engine creates final Kubernetes resources:

```yaml
# Deployment with addons applied
apiVersion: apps/v1
kind: Deployment
metadata:
  name: customer-portal
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: app
          image: gcr.io/project/customer-portal:v1.2.3
          volumeMounts: # ← Added by persistent-volume addon
            - name: app-data
              mountPath: /app/data
            - name: app-config
              mountPath: /etc/config
      volumes: # ← Added by addons
        - name: app-data
          persistentVolumeClaim:
            claimName: customer-portal-app-data
        - name: app-config
          configMap:
            name: customer-portal-app-config

# New resources created by addons
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: customer-portal-app-data
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: fast
  resources:
    requests:
      storage: 50Gi

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: customer-portal-app-config
data:
  # ... config data
```

---

## Runtime Composition

Addons are composed **when Component instance is created**.

**Benefits:**

- Developers control which addons to use
- Single Component CRD (kind: Component)
- Flexible - can add/remove addons per instance

**Process:**

```yaml
# Developer creates Component with addons
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: my-app
spec:
  componentType: web-app  # References ComponentTypeDefinition

  parameters:
    replicas: 3

  addons:
    - name: persistent-volume
      instanceId: data-vol
      config:
        volumeName: data
        size: 50Gi
        mountPath: /app/data

  build:
    image: gcr.io/project/my-app:v1.0.0
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

**ComponentTypeDefinition:**

```yaml
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
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
          replicas: ${spec.parameters.replicas}
          template:
            spec:
              containers:
                - name: app
                  image: ${spec.build.image}
```

**Component:**

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  parameters:
    replicas: 3

  addons:
    - name: persistent-volume
      instanceId: data-vol
      config:
        volumeName: data
        size: 50Gi
        mountPath: /app/data

    - name: logging-sidecar
      instanceId: logger
      config:
        logLevel: info

  build:
    image: gcr.io/project/customer-portal:v1.2.3
```

### Output

**Final Deployment Resource:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: customer-portal
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: app
          image: gcr.io/project/customer-portal:v1.2.3
          volumeMounts:
            - name: data
              mountPath: /app/data

        - name: fluent-bit # ← From logging-sidecar addon
          image: fluent/fluent-bit:2.1
          env:
            - name: LOG_LEVEL
              value: info

      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: customer-portal-data
        - name: varlog # ← From logging-sidecar addon
          emptyDir: {}

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: customer-portal-data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
```

---

## Summary

Addon composition follows these principles:

1. **Load** ComponentTypeDefinition and Component
2. **Validate** addon compatibility and dependencies
3. **Merge** parameters from ComponentTypeDefinition and Component
4. **Sort** addons by dependency order
5. **Apply** addon patches and create resources
6. **Generate** final Kubernetes resources

The composition engine handles:

- Parameter merging from ComponentTypeDefinition
- Patch application with JSONPath
- CEL expression evaluation
- Dependency resolution
- Conflict detection
- Build context injection

Result: Developers get a simple, unified model where they create Components with addons, and the platform handles all composition at runtime.
