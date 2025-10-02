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

#### allowedFor (optional, via label)
Who is allowed to use this addon.

```yaml
metadata:
  labels:
    allowedFor: platform-engineer  # Options: platform-engineer, developer, both
```

**Values:**
- `platform-engineer`: Only PEs can configure when creating ComponentTypes (baked-in)
- `developer`: Only developers can opt-into when creating Component instances
- `both`: Can be used by both PEs and developers

**Default:** `platform-engineer`

---

### 2. Schema (required)

The schema defines the addon's configuration parameters, split into two categories following the ComponentDefinition model:

```yaml
schema:
  # Static parameters - cannot be overridden per environment
  parameters:
    <parameter-name>:
      type: <json-schema-type>
      description: <description>
      default: <default-value>
      # ... other JSON Schema properties

  # Environment-specific overrides - can be overridden in EnvBinding
  envOverrides:
    <parameter-name>:
      type: <json-schema-type>
      description: <description>
      default: <default-value>
      # ... other JSON Schema properties
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

**Supported JSON Schema Features:**
- All standard JSON Schema types: `string`, `number`, `integer`, `boolean`, `object`, `array`
- Validation: `pattern`, `minimum`, `maximum`, `minLength`, `maxLength`, `enum`
- Defaults: `default` values
- Nested objects and arrays
- References: `$ref` for reusable schema components

**Custom Extensions:**
- `format: textarea` - Hints UI to render multiline text input
- `format: code` - Hints UI to render code editor
- `format: hostname` - Hints UI to validate hostname
- `queryContainers: true` - Instructs UI to populate dropdown from ComponentDefinition containers
- `queryResources: {type: <ResourceType>}` - Instructs UI to populate from ComponentDefinition resources

**Example:**
```yaml
schema:
  # Static configuration
  parameters:
    volumeName:
      type: string
      description: "Name of the volume"
      pattern: "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

    mountPath:
      type: string
      description: "Path where volume should be mounted"
      pattern: "^/.*"

    containerName:
      type: string
      description: "Container to mount volume to"
      default: "app"
      queryContainers: true  # UI extension

  # Environment-specific configuration
  envOverrides:
    size:
      type: string
      description: "Volume size (e.g., 10Gi, 1Ti)"
      pattern: "^[0-9]+[EPTGMK]i$"
      default: "10Gi"

    storageClass:
      type: string
      description: "Storage class name"
      default: "standard"
      enum: ["standard", "fast", "premium"]

    accessMode:
      type: string
      description: "Volume access mode"
      default: "ReadWriteOnce"
      enum: ["ReadWriteOnce", "ReadWriteMany", "ReadOnlyMany"]
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

Provides hints to the UI for better rendering and UX.

```yaml
ui:
  formLayout:
    - field: <field-name>
      width: <full|half|third>
      type: <input-type>
      label: <custom-label>
      helpText: <help-text>
      placeholder: <placeholder>
      queryContainers: <boolean>
      queryResources:
        type: <ResourceType>
```

**Form Layout Options:**

Basic field:
```yaml
formLayout:
  - field: volumeName
    width: half
    label: "Volume Name"
    helpText: "Must be a valid DNS name"
    placeholder: "my-volume"
```

Toggle/checkbox:
```yaml
formLayout:
  - field: enabled
    type: toggle
    width: full
```

Select/dropdown:
```yaml
formLayout:
  - field: storageClass
    type: select
    width: half
```

Code editor:
```yaml
formLayout:
  - field: script
    type: code-editor
    language: bash
    width: full
```

Dynamic container selection:
```yaml
formLayout:
  - field: containerName
    width: half
    queryContainers: true  # Populates from ComponentDefinition
```

Dynamic resource selection:
```yaml
formLayout:
  - field: ingressName
    width: full
    queryResources:
      type: Ingress
```

Array fields:
```yaml
formLayout:
  - field: configs
    type: array
    addButtonLabel: "Add Configuration"
    itemLayout:
      - field: name
        width: half
      - field: value
        width: half
```

Nested objects:
```yaml
formLayout:
  - field: resources
    type: object
    width: full
    fields:
      - field: cpu
        width: half
      - field: memory
        width: half
```

**Preview Hints:**

```yaml
ui:
  preview:
    creates:
      - type: PersistentVolumeClaim
        summary: "1 PVC (${spec.size})"

    modifies:
      - type: Deployment
        summary: "Adds volume mount to container '${spec.containerName}'"
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
    allowedFor: platform-engineer  # PE-only addon
spec:
  displayName: "Persistent Volume"
  description: "Adds persistent storage to workloads"
  icon: "storage"

  schema:
    # Static parameters
    parameters:
      volumeName:
        type: string
        pattern: "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
        description: "Name of the volume"
      mountPath:
        type: string
        pattern: "^/.*"
        description: "Mount path in container"
      containerName:
        type: string
        default: "app"
        description: "Container to mount to"

    # Environment-specific overrides
    envOverrides:
      size:
        type: string
        pattern: "^[0-9]+[EPTGMK]i$"
        default: "10Gi"
        description: "Volume size"
      storageClass:
        type: string
        default: "standard"
        enum: ["standard", "fast", "premium"]
        description: "Storage class"

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

  ui:
    formLayout:
      - field: volumeName
        width: half
      - field: size
        width: half
      - field: storageClass
        width: half
      - field: containerName
        width: half
        queryContainers: true
      - field: mountPath
        width: full
```

---

## Schema Merging

When addons are composed into a ComponentType, their schemas are merged into the generated CRD schema.

### PE-Only Addons (Baked-In)
PE-configured addons are applied at ComponentType creation and are NOT exposed in the developer CRD schema. The resources are modified, but developers don't see the addon parameters.

### Developer-Allowed Addons
Developer-allowed addon schemas are merged under namespaced keys in the CRD.

**ComponentDefinition:**
```yaml
schema:
  parameters:
    replicas: number | default=1
  envOverrides:
    maxReplicas: number | default=3
```

**Developer-Allowed Addon:**
```yaml
schema:
  parameters:
    volumeName: string
    mountPath: string
  envOverrides:
    size: string | default=10Gi
    storageClass: string | default=standard
```

**Generated CRD Schema (for developers):**
```json
{
  "spec": {
    "properties": {
      "replicas": {
        "type": "integer",
        "default": 1
      },
      "maxReplicas": {
        "type": "integer",
        "default": 3
      },
      "persistentVolume": {
        "type": "object",
        "properties": {
          "volumeName": {"type": "string"},
          "mountPath": {"type": "string"},
          "size": {"type": "string", "default": "10Gi"},
          "storageClass": {"type": "string", "default": "standard"}
        }
      }
    }
  }
}
```

### EnvBinding Overrides

In EnvBinding, developers can override `envOverrides` from both component and addons:

```yaml
apiVersion: platform/v1alpha1
kind: EnvBinding
metadata:
  name: my-app-prod
spec:
  environment: production

  # Override component envOverrides
  overrides:
    maxReplicas: 10

  # Override addon envOverrides (only fields from addon's envOverrides)
  addonOverrides:
    persistentVolume:
      size: 200Gi        # ✓ Allowed (in envOverrides)
      storageClass: premium  # ✓ Allowed (in envOverrides)
      # mountPath: /foo  # ✗ Not allowed (in parameters, not envOverrides)
```

The addon parameters are namespaced under the addon name to avoid conflicts.
