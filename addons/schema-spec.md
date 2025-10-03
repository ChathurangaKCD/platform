# Addon Schema Specification

This document defines the formal specification for Addon CRDs.

## Addon CRD Structure

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: <addon-name>
  labels:
    category: <category>
    version: <version>
    compatibility: <k8s-version-range>
spec:
  # ... addon specification
```

## Spec Fields

### 1. Metadata Fields

#### displayName (required)
Human-readable name shown in UI.

```yaml
displayName: "Persistent Volume"
```

#### description (required)
Description of what the addon does.

```yaml
description: "Adds a persistent volume to your workload with configurable size, storage class, and mount path"
```

#### icon (optional)
Icon identifier for UI rendering.

```yaml
icon: "storage"  # Can reference Heroicons, Lucide, or custom icon set
```

#### category (optional, via label)
Categorization for UI grouping.

```yaml
metadata:
  labels:
    category: storage  # storage, security, observability, networking, configuration, lifecycle
```

#### version (optional, via label)
Addon version for compatibility tracking.

```yaml
metadata:
  labels:
    version: "1.0"
```

#### allowedFor (deprecated)
This label is no longer used. All addons can be used by developers when creating Component instances.

---

### 2. Schema (required)

The schema defines the addon's configuration parameters using Kro's [simple schema syntax](https://kro.run/docs/concepts/simple-schema/). Parameters are split into two categories:

```yaml
schema:
  # Static parameters - cannot be overridden per environment
  parameters:
    <parameter-name>: <type> | <constraint>=<value>

  # Environment-specific overrides - can be overridden in EnvBinding
  envOverrides:
    <parameter-name>: <type> | <constraint>=<value>
```

**Parameters vs EnvOverrides:**

- **parameters**: Configuration that remains static across all environments
  - Examples: mount paths, container names, feature toggles, structural config
  - Cannot be overridden in EnvBinding
  - Set once when Component is created

- **envOverrides**: Configuration that varies per environment
  - Examples: resource sizes, storage classes, replica counts, log levels
  - Can be overridden in EnvBinding per environment
  - Allows dev→staging→prod progression with different values

**Example:**
```yaml
schema:
  parameters:
    volumeName: string | required=true pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
    mountPath: string | required=true pattern="^/.*"
    containerName: string | default=app queryContainers=true

  envOverrides:
    size: string | default=10Gi pattern="^[0-9]+[EPTGMK]i$"
    storageClass: string | default=standard enum="standard,fast,premium"
```

---

### 3. Targets (required)

Declares which resources in the ComponentDefinition this addon affects.

```yaml
targets:
  - resourceType: <K8sResourceKind>
    resourceId: <optional-resource-id>
    containerName: <optional-container-name>
    selector:
      matchLabels: <optional-label-selector>
```

**Fields:**
- `resourceType` (array or string): Kubernetes resource kind(s) to target (e.g., `Deployment`, `StatefulSet`)
- `resourceId` (optional): Specific resource ID within ComponentDefinition (matches `resources[].id`)
- `containerName` (optional): Target specific container within pod spec
- `selector` (optional): Label-based selector for matching resources

**Examples:**

Target all Deployments:
```yaml
targets:
  - resourceType: Deployment
```

Target multiple resource types:
```yaml
targets:
  - resourceType: [Deployment, StatefulSet]
```

Target specific resource by ID:
```yaml
targets:
  - resourceId: main-deployment
```

Target specific container:
```yaml
targets:
  - resourceType: Deployment
    containerName: app
```

Target by labels:
```yaml
targets:
  - selector:
      matchLabels:
        app.platform.io/workload: "true"
```

---

### 4. Creates (optional)

Defines new Kubernetes resources that this addon creates.

```yaml
creates:
  - apiVersion: <api-version>
    kind: <kind>
    metadata:
      name: <name-with-interpolation>
      labels: <labels>
    spec:
      # ... resource spec with CEL interpolation
```

**Single Resource:**
```yaml
creates:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: "${metadata.name}-${spec.volumeName}"
    spec:
      accessModes:
        - "${spec.accessMode}"
      storageClassName: "${spec.storageClass}"
      resources:
        requests:
          storage: "${spec.size}"
```

**Multiple Resources with forEach:**
```yaml
creates:
  - forEach: "${spec.configs}"
    resource:
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: "${metadata.name}-${item.name}"
      data: "${item.files.reduce((acc, f) => {acc[f.fileName] = f.content; return acc;}, {})}"
```

**Available Context Variables:**
- `${metadata.*}` - Component instance metadata (name, namespace, labels)
- `${spec.*}` - Addon configuration parameters
- `${item}` - Current item in forEach loop
- `${resources.*}` - References to other resources in ComponentDefinition
- `${build.*}` - Build-related context (image, tag, etc.)

---

### 5. Patches (optional)

Defines how this addon modifies existing resources in the ComponentDefinition.

```yaml
patches:
  - target:
      resourceType: <type>
      resourceId: <id>
      containerName: <name>
    patch:
      op: <operation>
      path: <json-path>
      value: <value-with-interpolation>
```

**Patch Operations:**
- `add` - Add new field/array element
- `replace` - Replace existing field
- `remove` - Remove field
- `merge` - Deep merge objects

**JSONPath Syntax:**
- `/spec/template/spec/volumes/-` - Append to array
- `/spec/template/spec/containers/[?(@.name=='app')]/volumeMounts/-` - Append to specific container's volumeMounts
- `/metadata/labels/my-label` - Set specific label

**Single Patch:**
```yaml
patches:
  - target:
      resourceType: Deployment
    patch:
      op: add
      path: /spec/template/spec/volumes/-
      value:
        name: "${spec.volumeName}"
        persistentVolumeClaim:
          claimName: "${metadata.name}-${spec.volumeName}"
```

**Multiple Patches with forEach:**
```yaml
patches:
  - forEach: "${spec.configs}"
    target:
      resourceType: [Deployment, StatefulSet]
      containerName: "${item.containerName}"
    patch:
      op: add
      path: /spec/template/spec/containers/[?(@.name=='${item.containerName}')]/volumeMounts/-
      value:
        name: "${item.name}"
        mountPath: "${item.mountPath}"
```

**Conditional Patches:**
```yaml
patches:
  - condition: "${spec.enabled == true}"
    target:
      resourceType: Deployment
    patch:
      op: add
      path: /metadata/annotations/logging
      value: "enabled"
```

---

### 6. Dependencies (optional)

Declares dependencies on other addons and ordering constraints.

```yaml
dependencies:
  requires:
    - addon: persistent-volume
      reason: "ConfigMap addon requires volume addon to mount config files"

  conflictsWith:
    - addon: embedded-config
      reason: "Cannot use both ConfigMap and embedded config"

  loadOrder:
    after: ["persistent-volume"]
    before: ["tls-certificate"]
```

**Fields:**
- `requires` - Other addons that must be present
- `conflictsWith` - Addons that cannot be used together
- `loadOrder` - Controls the order patches are applied

---

### 7. Validation (optional)

Custom validation rules beyond JSON Schema.

```yaml
validation:
  rules:
    - name: volume-size-limit
      expression: "int(spec.size.replace('Gi', '')) <= 1000"
      message: "Volume size cannot exceed 1000Gi"

    - name: container-exists
      expression: "resources.exists(r, r.kind == 'Deployment' && r.spec.template.spec.containers.exists(c, c.name == spec.containerName))"
      message: "Container '${spec.containerName}' not found in any Deployment"
```

**Fields:**
- `name` - Rule identifier
- `expression` - CEL expression that must evaluate to `true`
- `message` - Error message shown when validation fails

---

### 8. UI Hints (optional)

Optional hints for UI tooling. The simple schema syntax supports `queryContainers=true` and `queryResources=<ResourceType>` constraints to help UI populate dropdowns from ComponentDefinition resources.

```yaml
schema:
  parameters:
    containerName: string | default=app queryContainers=true
    ingressName: string | queryResources=Ingress
```

---

## Complete Example

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume
  labels:
    category: storage
    version: "1.0"
spec:
  displayName: "Persistent Volume"
  description: "Adds persistent storage to workloads"
  icon: "storage"

  schema:
    # Static parameters
    parameters:
      volumeName: string | required=true pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
      mountPath: string | required=true pattern="^/.*"
      containerName: string | default=app

    # Environment-specific overrides
    envOverrides:
      size: string | default=10Gi pattern="^[0-9]+[EPTGMK]i$"
      storageClass: string | default=standard enum="standard,fast,premium"

  targets:
    - resourceType: [Deployment, StatefulSet]
      containerName: "${spec.containerName}"

  creates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: "${metadata.name}-${spec.volumeName}"
      spec:
        accessModes: ["ReadWriteOnce"]
        storageClassName: "${spec.storageClass}"
        resources:
          requests:
            storage: "${spec.size}"

  patches:
    - target:
        resourceType: [Deployment, StatefulSet]
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: "${spec.volumeName}"
          persistentVolumeClaim:
            claimName: "${metadata.name}-${spec.volumeName}"

    - target:
        resourceType: [Deployment, StatefulSet]
        containerName: "${spec.containerName}"
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name=='${spec.containerName}')]/volumeMounts/-
        value:
          name: "${spec.volumeName}"
          mountPath: "${spec.mountPath}"

  validation:
    rules:
      - name: volume-size-limit
        expression: "int(spec.size.replace('Gi', '')) <= 1000"
        message: "Volume size cannot exceed 1000Gi"

```

---

## Component Structure with Addons

When developers create Components, they specify addons in the `addons[]` array:

**ComponentTypeDefinition:**
```yaml
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: web-app
spec:
  schema:
    parameters:
      replicas: number | default=1
    envOverrides:
      maxReplicas: number | default=3
  resources:
    - id: deployment
      template:
        # ... deployment spec
```

**Component with Addons:**
```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  # Parameters from ComponentTypeDefinition
  parameters:
    replicas: 3

  # Addon instances
  addons:
    - name: persistent-volume
      instanceId: app-data  # Required for all addon instances
      config:
        volumeName: data
        mountPath: /app/data
        size: 50Gi
        storageClass: fast

  # Build context (platform-injected)
  build:
    image: gcr.io/project/customer-portal:v1.2.3
```

### EnvSettings Overrides

In EnvSettings, developers can override `envOverrides` from both component parameters and addon configs:

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
    maxReplicas: 10

  # Override addon envOverrides (keyed by instanceId)
  addonOverrides:
    app-data:  # instanceId of the addon
      size: 200Gi        # ✓ Allowed (in envOverrides)
      storageClass: premium  # ✓ Allowed (in envOverrides)
      # mountPath: /foo  # ✗ Not allowed (in parameters, not envOverrides)
```

The addon overrides are keyed by `instanceId` to support multiple instances of the same addon.
