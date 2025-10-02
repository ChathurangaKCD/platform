# UI Integration Guide

This document describes how the UI integrates with the addon system to provide seamless experiences for both Platform Engineers and Developers.

## Overview

The system provides two distinct UI workflows:

### Platform Engineer Workflow
PEs create ComponentTypes by:
1. Selecting a base ComponentDefinition
2. Adding/configuring platform addons (baked into resources)
3. Selecting developer-allowed addons
4. Previewing resulting K8s resources and generated CRD
5. Registering ComponentType for developers

### Developer Workflow
Developers create Component instances by:
1. Selecting a ComponentType (generated CRD)
2. Configuring component parameters
3. Opting into developer-allowed addons
4. Creating EnvBinding for environment-specific overrides

All UI rendering is **generic** and driven by addon schemas and metadata—no special-casing per addon.

---

## Platform Engineer UI Workflow

### 1. ComponentType Builder - Initial Screen

```
┌─────────────────────────────────────────────────────────────┐
│  Create Component Type                                       │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Base Component Definition:                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ [Select ComponentDefinition ▼]                      │    │
│  │                                                       │    │
│  │ Available:                                            │    │
│  │  • web-app                                            │    │
│  │  • worker                                             │    │
│  │  • scheduled-task                                     │    │
│  │  • custom-deployment                                  │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                               │
│  [Continue →]                                                 │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

**Logic:**
- Fetch all available ComponentDefinitions from cluster/repository
- Display as searchable dropdown
- Show description/metadata for each

---

### 2. Addon Selection Screen

After selecting a ComponentDefinition, PE selects addons in two categories:

```
┌─────────────────────────────────────────────────────────────┐
│  Create Component Type: web-app                              │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─ Platform Addons (Baked Into Resources) ──────────────┐  │
│  │ Selected (2):                                          │  │
│  │ • [🗄️ Persistent Volume]          [Configure] [×]     │  │
│  │ • [🔒 Network Policy]             [Configure] [×]     │  │
│  │                                                         │  │
│  │ Available:                                             │  │
│  │ ┌─ Storage ──────────────────────────────────────┐    │  │
│  │ │ [+ Persistent Volume] 🔒 PE-only               │    │  │
│  │ └────────────────────────────────────────────────┘    │  │
│  │ ┌─ Security ─────────────────────────────────────┐    │  │
│  │ │ [+ Network Policy] 🔒 PE-only                  │    │  │
│  │ │ [+ TLS Certificate] 🔒 PE-only                 │    │  │
│  │ └────────────────────────────────────────────────┘    │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                               │
│  ┌─ Developer-Allowed Addons ──────────────────────────────┐  │
│  │ Selected (2):                                           │  │
│  │ • [📁 Config Files]                [Defaults] [×]      │  │
│  │ • [📊 Logging Sidecar]             [Defaults] [×]      │  │
│  │                                                         │  │
│  │ Available:                                             │  │
│  │ ┌─ Configuration ────────────────────────────────┐    │  │
│  │ │ [+ Config Files] 👤 Developer-allowed          │    │  │
│  │ │ [+ Environment Vars] 👤 Developer-allowed      │    │  │
│  │ └────────────────────────────────────────────────┘    │  │
│  │ ┌─ Observability ────────────────────────────────┐    │  │
│  │ │ [+ Logging Sidecar] 👤 Developer-allowed       │    │  │
│  │ │ [+ Init Container] 👤 Developer-allowed        │    │  │
│  │ └────────────────────────────────────────────────┘    │  │
│  │ ┌─ Resources ────────────────────────────────────┐    │  │
│  │ │ [+ Resource Limits] ⚡ Both                     │    │  │
│  │ └────────────────────────────────────────────────┘    │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                               │
│  [← Back]                              [Preview Resources →] │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

**Logic:**
- Fetch all available Addons from cluster/repository
- Filter by `metadata.labels.allowedFor`:
  - `platform-engineer` → Platform Addons section
  - `developer` → Developer-Allowed Addons section
  - `both` → Show in both sections
- Group by `metadata.labels.category`
- Display with icon, name, description, permission badge (🔒/👤/⚡)
- Validate dependencies/conflicts as addons are added

**Data Source:**
```graphql
query GetAddons {
  addons {
    name
    displayName
    description
    icon
    category
    version
    allowedFor  # NEW: platform-engineer | developer | both
  }
}
```

---

### 3. Addon Configuration Screen

When user clicks "Configure" on an addon:

```
┌─────────────────────────────────────────────────────────────┐
│  Configure Addon: Persistent Volume                          │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Volume Name *                    Size *                     │
│  ┌─────────────────────────┐     ┌─────────────────────┐    │
│  │ data                    │     │ 50Gi                │    │
│  └─────────────────────────┘     └─────────────────────┘    │
│  Must be a valid DNS name         e.g., 10Gi, 1Ti           │
│                                                               │
│  Storage Class                   Access Mode                 │
│  ┌─────────────────────────┐     ┌─────────────────────┐    │
│  │ fast               ▼    │     │ ReadWriteOnce  ▼    │    │
│  └─────────────────────────┘     └─────────────────────┘    │
│                                                               │
│  Container Name                                               │
│  ┌─────────────────────────┐                                 │
│  │ app                ▼    │     ← Populated from ComponentDefinition
│  └─────────────────────────┘                                 │
│  Available: app, sidecar                                      │
│                                                               │
│  Mount Path *                                                 │
│  ┌──────────────────────────────────────────────────┐        │
│  │ /app/data                                        │        │
│  └──────────────────────────────────────────────────┘        │
│  Path where volume will be mounted                           │
│                                                               │
│  Mount Permissions           Sub Path                        │
│  ┌─────────────────────────┐     ┌─────────────────────┐    │
│  │ 0755                    │     │                     │    │
│  └─────────────────────────┘     └─────────────────────┘    │
│                                                               │
│  ┌────────────────────────────────────────────────────┐      │
│  │ ℹ️ Impact Preview:                                  │      │
│  │                                                      │      │
│  │ Will create:                                         │      │
│  │  • 1 PersistentVolumeClaim (50Gi, fast)             │      │
│  │                                                      │      │
│  │ Will modify:                                         │      │
│  │  • Deployment "deployment" - add volume mount        │      │
│  │    to container "app" at /app/data                   │      │
│  └────────────────────────────────────────────────────┘      │
│                                                               │
│  [Cancel]                                    [Save & Close]  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

**Logic:**
- Render form fields from addon's `schema` (JSON Schema)
- Use `ui.formLayout` hints for field arrangement and types
- For fields with `queryContainers: true`:
  - Parse ComponentDefinition resources
  - Extract all container names from Deployment/StatefulSet specs
  - Populate dropdown
- For fields with `queryResources: {type: X}`:
  - Parse ComponentDefinition resources
  - Find all resources of type X
  - Populate dropdown with resource IDs/names
- Show real-time impact preview based on addon's `creates` and `patches`
- Validate inputs against JSON Schema

**Generic Rendering Algorithm:**

```javascript
function renderAddonForm(addon, componentDefinition) {
  const schema = addon.spec.schema;
  const layout = addon.spec.ui?.formLayout || generateDefaultLayout(schema);

  return layout.map(fieldConfig => {
    const fieldSchema = getFieldSchema(schema, fieldConfig.field);

    // Handle special query hints
    if (fieldConfig.queryContainers) {
      const containers = extractContainers(componentDefinition);
      return renderDropdown(fieldConfig, containers);
    }

    if (fieldConfig.queryResources) {
      const resources = extractResources(
        componentDefinition,
        fieldConfig.queryResources.type
      );
      return renderDropdown(fieldConfig, resources);
    }

    // Standard rendering based on schema type
    switch (fieldSchema.type) {
      case 'string':
        if (fieldSchema.enum) return renderSelect(fieldConfig, fieldSchema);
        if (fieldSchema.format === 'textarea') return renderTextarea(fieldConfig);
        if (fieldSchema.format === 'code') return renderCodeEditor(fieldConfig);
        return renderTextInput(fieldConfig, fieldSchema);

      case 'boolean':
        return renderToggle(fieldConfig, fieldSchema);

      case 'number':
      case 'integer':
        return renderNumberInput(fieldConfig, fieldSchema);

      case 'array':
        return renderArrayField(fieldConfig, fieldSchema, componentDefinition);

      case 'object':
        return renderObjectField(fieldConfig, fieldSchema, componentDefinition);
    }
  });
}

function extractContainers(componentDefinition) {
  const containers = [];

  componentDefinition.spec.resources.forEach(resource => {
    if (['Deployment', 'StatefulSet', 'Job'].includes(resource.template.kind)) {
      const podSpec = resource.template.spec.template.spec;
      podSpec.containers?.forEach(c => containers.push(c.name));
      podSpec.initContainers?.forEach(c => containers.push(c.name));
    }
  });

  return [...new Set(containers)];
}

function extractResources(componentDefinition, resourceType) {
  return componentDefinition.spec.resources
    .filter(r => r.template.kind === resourceType)
    .map(r => ({
      id: r.id,
      name: r.template.metadata.name
    }));
}
```

---

### 4. Impact Preview Screen

Shows what resources will be created/modified:

```
┌─────────────────────────────────────────────────────────────┐
│  Preview: web-app-with-storage                               │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Base Resources (from ComponentDefinition):                  │
│  ✓ Deployment            deployment                          │
│  ✓ Service               service                             │
│  ✓ Ingress               public-ingress                      │
│  ✓ HTTPScaledObject      http-scaler                         │
│                                                               │
│  ┌────────────────────────────────────────────────────┐      │
│  │ Added by Persistent Volume Addon:                   │      │
│  │ ➕ PersistentVolumeClaim   data-pvc                 │      │
│  │ 📝 Modified: Deployment                              │      │
│  │    • Added volume: data                              │      │
│  │    • Added volume mount to container "app"           │      │
│  └────────────────────────────────────────────────────┘      │
│                                                               │
│  ┌────────────────────────────────────────────────────┐      │
│  │ Added by TLS Certificate Addon:                      │      │
│  │ ➕ Certificate          customer-portal-tls          │      │
│  │ 📝 Modified: Ingress                                 │      │
│  │    • Added TLS configuration                         │      │
│  └────────────────────────────────────────────────────┘      │
│                                                               │
│  [View YAML]  [View Diagram]                                 │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐     │
│  │ [×] Deployment (deployment)                          │     │
│  │                                                       │     │
│  │ ```yaml                                               │     │
│  │ apiVersion: apps/v1                                   │     │
│  │ kind: Deployment                                      │     │
│  │ metadata:                                             │     │
│  │   name: ${metadata.name}                             │     │
│  │ spec:                                                 │     │
│  │   template:                                           │     │
│  │     spec:                                             │     │
│  │       containers:                                     │     │
│  │         - name: app                                   │     │
│  │           image: ${build.image}                      │     │
│  │           volumeMounts:              ← Added by addon│     │
│  │             - name: data             ← Added by addon│     │
│  │               mountPath: /app/data   ← Added by addon│     │
│  │       volumes:                       ← Added by addon│     │
│  │         - name: data                 ← Added by addon│     │
│  │           persistentVolumeClaim:     ← Added by addon│     │
│  │             claimName: ${metadata.name}-data         │     │
│  │ ```                                                   │     │
│  └─────────────────────────────────────────────────────┘     │
│                                                               │
│  [← Back]         [Export YAML]          [Save Component ✓]  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

**Logic:**
- Merge ComponentDefinition with all addon configurations
- Apply addon patches in dependency order
- Highlight additions/modifications from each addon (color-coded)
- Show both high-level summary and detailed YAML

**Rendering Algorithm:**

```javascript
function generatePreview(componentDefinition, addons, addonConfigs) {
  // Start with base resources
  let resources = cloneDeep(componentDefinition.spec.resources);
  const newResources = [];
  const modifications = [];

  // Apply each addon
  addons.forEach(addon => {
    const config = addonConfigs[addon.metadata.name];

    // Track new resources
    addon.spec.creates?.forEach(createSpec => {
      const rendered = renderTemplate(createSpec, {
        metadata: config.metadata,
        spec: config.spec
      });

      newResources.push({
        resource: rendered,
        addedBy: addon.metadata.name
      });
    });

    // Apply patches
    addon.spec.patches?.forEach(patchSpec => {
      const targets = findTargets(resources, patchSpec.target);

      targets.forEach(target => {
        const patch = renderPatch(patchSpec.patch, config);
        applyPatch(target, patch);

        modifications.push({
          resourceId: target.id,
          patch: patch,
          addedBy: addon.metadata.name
        });
      });
    });
  });

  return {
    baseResources: resources,
    newResources,
    modifications
  };
}
```

---

### 5. Generated CRD Schema Preview

Show developers what the final CRD will look like:

```
┌─────────────────────────────────────────────────────────────┐
│  Developer Experience Preview                                │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Developers will use this CRD:                               │
│                                                               │
│  ```yaml                                                      │
│  apiVersion: platform/v1alpha1                             │
│  kind: WebAppWithStorage                                     │
│  metadata:                                                    │
│    name: my-app                                              │
│  spec:                                                        │
│    # Base component parameters                               │
│    maxReplicas: 3                                            │
│    rollingUpdate:                                            │
│      maxSurge: 2                                             │
│    scaleToZero:                                              │
│      pendingRequests: 50                                     │
│                                                               │
│    # Persistent Volume addon parameters                      │
│    persistentVolume:                                         │
│      volumeName: data                                        │
│      size: 50Gi                                              │
│      storageClass: fast                                      │
│      mountPath: /app/data                                    │
│      containerName: app                                      │
│                                                               │
│    # TLS Certificate addon parameters                        │
│    tlsCertificate:                                           │
│      issuer: letsencrypt-prod                                │
│      domains:                                                │
│        - app.example.com                                     │
│      ingressName: public-ingress                             │
│  ```                                                          │
│                                                               │
│  [View Full JSON Schema]  [Download OpenAPI Spec]           │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## UI Components Library

### Reusable Components

#### 1. SchemaFormRenderer
Renders any JSON Schema as a form.

```typescript
interface SchemaFormRendererProps {
  schema: JSONSchema;
  layout?: UILayout;
  value: any;
  onChange: (value: any) => void;
  context?: {
    componentDefinition?: ComponentDefinition;
    containers?: string[];
    resources?: Resource[];
  };
}

function SchemaFormRenderer({
  schema,
  layout,
  value,
  onChange,
  context
}: SchemaFormRendererProps) {
  // Generic form rendering logic
}
```

#### 2. AddonCard
Displays addon in selection screen.

```typescript
interface AddonCardProps {
  addon: Addon;
  selected: boolean;
  onToggle: () => void;
  onConfigure: () => void;
}
```

#### 3. ResourceDiffViewer
Shows before/after comparison with highlighted changes.

```typescript
interface ResourceDiffViewerProps {
  original: KubernetesResource;
  modified: KubernetesResource;
  modifications: Modification[];
}
```

#### 4. ImpactPreview
Shows summary of addon impact.

```typescript
interface ImpactPreviewProps {
  addon: Addon;
  config: any;
  componentDefinition: ComponentDefinition;
}
```

---

## API Contract

### Backend API Endpoints

#### GET /api/v1/component-definitions
List all available ComponentDefinitions.

**Response:**
```json
[
  {
    "name": "web-app",
    "displayName": "Web Application",
    "description": "HTTP service with autoscaling",
    "version": "1.0"
  }
]
```

#### GET /api/v1/addons
List all available addons with metadata.

**Response:**
```json
[
  {
    "name": "persistent-volume",
    "displayName": "Persistent Volume",
    "description": "Add persistent storage",
    "icon": "storage",
    "category": "storage",
    "version": "1.0",
    "schema": { /* JSON Schema */ },
    "ui": { /* UI hints */ }
  }
]
```

#### GET /api/v1/component-definitions/:name
Get full ComponentDefinition with resources.

#### GET /api/v1/addons/:name
Get full Addon spec.

#### POST /api/v1/component-types/compose
Compose ComponentDefinition + Addons and return preview.

**Request:**
```json
{
  "componentDefinition": "web-app",
  "addons": [
    {
      "name": "persistent-volume",
      "config": {
        "volumeName": "data",
        "size": "50Gi",
        "mountPath": "/app/data",
        "containerName": "app"
      }
    }
  ]
}
```

**Response:**
```json
{
  "crdSchema": { /* Generated JSON Schema */ },
  "resources": [ /* Final K8s resources */ ],
  "modifications": [ /* List of changes */ ]
}
```

#### POST /api/v1/component-types
Register composed component type.

**Request:**
```json
{
  "name": "web-app-with-storage",
  "displayName": "Web App with Storage",
  "componentDefinition": "web-app",
  "addons": [ /* addon configs */ ]
}
```

---

## Validation & Error Handling

### Real-time Validation

As user fills addon configuration form:

```javascript
function validateAddonConfig(addon, config, componentDefinition) {
  const errors = [];

  // 1. JSON Schema validation
  const schemaErrors = validateJSONSchema(addon.spec.schema, config);
  errors.push(...schemaErrors);

  // 2. Custom CEL validation rules
  addon.spec.validation?.rules.forEach(rule => {
    const result = evaluateCEL(rule.expression, {
      spec: config,
      resources: componentDefinition.spec.resources
    });

    if (!result) {
      errors.push({
        field: rule.name,
        message: renderTemplate(rule.message, config)
      });
    }
  });

  // 3. Target validation (e.g., container exists)
  addon.spec.targets?.forEach(target => {
    if (target.containerName) {
      const containers = extractContainers(componentDefinition);
      const containerName = renderTemplate(target.containerName, config);

      if (!containers.includes(containerName)) {
        errors.push({
          message: `Container '${containerName}' not found`
        });
      }
    }
  });

  return errors;
}
```

### Dependency/Conflict Warnings

```javascript
function validateAddonComposition(selectedAddons) {
  const errors = [];

  selectedAddons.forEach(addon => {
    // Check dependencies
    addon.spec.dependencies?.requires?.forEach(dep => {
      const hasRequired = selectedAddons.some(a => a.metadata.name === dep.addon);
      if (!hasRequired) {
        errors.push({
          addon: addon.metadata.name,
          type: 'missing-dependency',
          message: `Requires addon: ${dep.addon}. ${dep.reason}`
        });
      }
    });

    // Check conflicts
    addon.spec.dependencies?.conflictsWith?.forEach(conflict => {
      const hasConflict = selectedAddons.some(a => a.metadata.name === conflict.addon);
      if (hasConflict) {
        errors.push({
          addon: addon.metadata.name,
          type: 'conflict',
          message: `Conflicts with: ${conflict.addon}. ${conflict.reason}`
        });
      }
    });
  });

  return errors;
}
```

---

## Summary

The UI integration is completely **generic** and **metadata-driven**:

1. **No special-casing**: All addons rendered using same logic
2. **Schema-driven forms**: JSON Schema → automatic form generation
3. **Smart queries**: `queryContainers` and `queryResources` hints enable dynamic dropdowns
4. **Impact preview**: Show changes before applying
5. **Validation**: Real-time feedback using JSON Schema + CEL rules
6. **Composability**: Manage multiple addons with dependency validation

This approach allows Platform Engineers to define custom addons and have them automatically appear in the UI with proper form rendering, validation, and preview—no frontend changes required.
