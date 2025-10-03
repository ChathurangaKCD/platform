# ComponentType Specification

## Overview

A **ComponentType** is the intermediate resource that Platform Engineers create by composing a ComponentDefinition with Addons. It generates a CRD that developers use to create component instances.

## Resource Hierarchy

```
ComponentDefinition (PE-authored base template)
        +
    Addons (PE-selected and configured)
        ↓
ComponentType (PE-composed, generates CRD)
        ↓
Component Instance (Developer-created)
        +
   EnvBinding (Per-environment overrides)
        ↓
Final K8s Resources (Deployed to cluster)
```

## ComponentType CRD Structure

```yaml
apiVersion: platform/v1alpha1
kind: ComponentType
metadata:
  name: production-web-app
  labels:
    composedFrom: web-app  # Source ComponentDefinition
spec:
  # Reference to base ComponentDefinition
  componentDefinitionRef:
    name: web-app
    version: "1.0"

  # PE-configured addons (baked into resources, hidden from devs)
  platformAddons:
    - name: persistent-volume
      instanceId: app-data        # Required when using same addon multiple times
      config:
        # parameters (static)
        volumeName: app-data
        mountPath: /app/data
        containerName: app

        # envOverrides (can be overridden per env)
        size: 50Gi
        storageClass: fast

    - name: persistent-volume
      instanceId: cache-data      # Different instance of same addon
      config:
        volumeName: cache-data
        mountPath: /app/cache
        size: 20Gi
        storageClass: standard

    - name: network-policy
      instanceId: default         # Always required, even for single instance
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress"
            ports: [8080]

  # Addons available for developers to opt-into
  developerAddons:
    allowed:
      - name: config-files
        description: "Mount ConfigMaps/Secrets"

      - name: logging-sidecar
        description: "Add logging sidecar"
        # Optional: PE-provided defaults
        defaults:
          enabled: true
          logLevel: info

      - name: init-container
        description: "Add initialization container"

  # Validation rules
  validation:
    rules:
      - name: replica-limit
        expression: "spec.maxReplicas <= 20"
        message: "maxReplicas cannot exceed 20 for this component type"

status:
  # Generated CRD details
  generatedCRD:
    apiVersion: platform/v1alpha1
    kind: ProductionWebApp

  # Applied platform addons
  appliedPlatformAddons:
    - name: persistent-volume
      version: "1.0"
      appliedAt: "2025-01-15T10:30:00Z"

    - name: network-policy
      version: "1.0"
      appliedAt: "2025-01-15T10:30:00Z"

  conditions:
    - type: Ready
      status: "True"
      reason: "CRDGenerated"
      message: "ComponentType is ready for use"
```

## Composition Process

### 1. PE Creates ComponentType

**Via UI:**
1. Select base ComponentDefinition: `web-app`
2. Add platform addons:
   - `persistent-volume` (configure: size, mount path, etc.)
   - `network-policy` (configure: ingress rules)
3. Select developer-allowed addons:
   - `config-files` (allow devs to add ConfigMaps)
   - `logging-sidecar` (allow devs to configure logging)
4. Preview generated CRD and resources
5. Save as `production-web-app` ComponentType

**Via YAML:**
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
        defaults:
          enabled: true
```

### 2. Controller Generates CRD

The ComponentType controller:

1. Loads `web-app` ComponentDefinition
2. Applies `platformAddons` patches to resources
3. Merges `developerAddons` schemas into CRD schema
4. Generates CRD: `ProductionWebApp`
5. Installs CRD in cluster

**Generated CRD Schema:**
```json
{
  "apiVersion": "platform/v1alpha1",
  "kind": "ProductionWebApp",
  "spec": {
    "type": "object",
    "properties": {
      "buildRef": {
        "type": "object",
        "description": "Reference to build"
      },
      "endpoints": {
        "type": "array",
        "description": "Endpoint configurations"
      },

      // From ComponentDefinition
      "maxReplicas": {
        "type": "integer",
        "default": 1
      },
      "rollingUpdate": {
        "type": "object",
        "properties": {
          "maxSurge": {"type": "integer", "default": 1}
        }
      },

      // Developer-allowed addon: config-files
      "configFiles": {
        "type": "object",
        "properties": {
          "configs": {
            "type": "array",
            "items": { /* ... */ }
          }
        }
      },

      // Developer-allowed addon: logging-sidecar
      "loggingSidecar": {
        "type": "object",
        "properties": {
          "enabled": {"type": "boolean", "default": true},
          "logLevel": {"type": "string", "default": "info"}
        }
      }
    }
  }
}
```

**Note:** Platform addon parameters (persistent-volume, network-policy) are NOT in the schema—they're already applied to resources.

### 3. Developer Uses Generated CRD

```yaml
apiVersion: platform/v1alpha1
kind: ProductionWebApp  # ← Generated from ComponentType
metadata:
  name: customer-portal
  namespace: my-project
spec:
  # Component parameters
  buildRef:
    name: customer-portal-build

  endpoints:
    - name: api
      port: 8080
      visibility: public

  maxReplicas: 5

  # Developer-allowed addon: config-files
  configFiles:
    configs:
      - name: app-config
        type: configmap
        mountPath: /etc/config
        files:
          - fileName: config.yaml
            content: |
              database: postgres
              cache: redis

  # Developer-allowed addon: logging-sidecar
  loggingSidecar:
    enabled: true
    logLevel: debug  # Override default "info"
```

### 4. Developer Creates EnvBinding

```yaml
apiVersion: platform/v1alpha1
kind: EnvBinding
metadata:
  name: customer-portal-prod
spec:
  owner:
    componentName: customer-portal
  environment: production

  # Override ComponentDefinition envOverrides
  overrides:
    maxReplicas: 20

  # Override platform addon envOverrides
  platformAddonOverrides:
    persistent-volume:     # Addon name
      app-data:            # instanceId (multiple instances)
        size: 200Gi        # ✓ Allowed (in envOverrides)
        storageClass: premium
      cache-data:          # Different instance
        size: 100Gi
        storageClass: fast
        # mountPath: /foo  # ✗ Not allowed (in parameters)

    network-policy:        # Single instance - still uses instanceId
      default:             # instanceId
        allowIngress:
          - from: "namespace:production-gateway"

  # Override developer addon envOverrides
  addonOverrides:
    loggingSidecar:        # Single instance
      logLevel: warn       # Less verbose in prod
      outputDestination: elasticsearch.prod.svc:9200
```

## Benefits of ComponentType

### Separation of Concerns
- **PEs control infrastructure**: Storage, networking, security policies
- **Developers control application**: Config, logging, app-specific settings
- Clear boundary between platform and application

### Reusability
- One ComponentDefinition → many ComponentTypes
- E.g., `web-app` → `dev-web-app`, `production-web-app`, `high-security-web-app`
- Each with different platform addons

### Governance
- PEs enforce policies via platform addons
- Developers can't bypass security/networking rules
- PEs control which addons developers can use

### Developer Experience
- Developers see simple, focused CRD
- No clutter from infrastructure concerns
- Type-safe, validated configuration

### Environment Progression
- `envOverrides` allow environment-specific tuning
- Dev: small volumes, debug logging
- Prod: large volumes, warn logging, premium storage
- Controlled via EnvBinding

## Examples

### Example 1: High-Security Web App

```yaml
apiVersion: platform/v1alpha1
kind: ComponentType
metadata:
  name: high-security-web-app
spec:
  componentDefinitionRef:
    name: web-app

  platformAddons:
    - name: network-policy
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress-gateway"
            ports: [8080]
        allowEgress:
          - to: "namespace:databases"
            ports: [5432]

    - name: pod-security-policy
      config:
        runAsNonRoot: true
        readOnlyRootFilesystem: true
        allowPrivilegeEscalation: false

    - name: resource-limits
      config:
        containers:
          - name: app
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 1000m
              memory: 1Gi

  developerAddons:
    allowed:
      - name: config-files  # Devs can add ConfigMaps only
```

### Example 2: Development Web App

```yaml
apiVersion: platform/v1alpha1
kind: ComponentType
metadata:
  name: dev-web-app
spec:
  componentDefinitionRef:
    name: web-app

  platformAddons:
    - name: persistent-volume
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 10Gi  # Small for dev
        storageClass: standard

  developerAddons:
    allowed:
      - name: config-files
      - name: logging-sidecar
        defaults:
          logLevel: debug  # Verbose in dev
      - name: init-container  # Allow custom init containers in dev
```

### Example 3: Stateless API Service

```yaml
apiVersion: platform/v1alpha1
kind: ComponentType
metadata:
  name: stateless-api
spec:
  componentDefinitionRef:
    name: web-app

  platformAddons:
    - name: horizontal-autoscaler
      config:
        minReplicas: 2
        maxReplicas: 50
        targetCPU: 70

    - name: service-mesh
      config:
        enabled: true
        mtls: true

  developerAddons:
    allowed:
      - name: config-files
```

## ComponentType Lifecycle

### Creation
1. PE defines ComponentType YAML
2. Controller validates:
   - ComponentDefinition exists
   - Addons exist and are compatible
   - No conflicts between addons
3. Controller applies platform addons
4. Controller generates CRD schema
5. Controller installs CRD

### Updates
1. PE modifies ComponentType
2. Controller regenerates CRD
3. Existing Component instances validated against new schema
4. Breaking changes require migration strategy

### Versioning
ComponentTypes should be versioned:
```yaml
metadata:
  name: production-web-app-v2
spec:
  componentDefinitionRef:
    name: web-app
    version: "2.0"
```

Allows gradual migration from v1 → v2.

### Deletion
1. PE deletes ComponentType
2. Controller marks CRD for deletion
3. Existing Component instances must be migrated first
4. CRD deleted after all instances removed

## Advanced Features

### Conditional Addons

Apply addons based on conditions:

```yaml
platformAddons:
  - name: persistent-volume
    condition: "${metadata.labels.stateful == 'true'}"
    config:
      volumeName: data
      mountPath: /data
```

### Addon Presets

Provide multiple preset configurations:

```yaml
developerAddons:
  allowed:
    - name: logging-sidecar
      presets:
        - name: basic
          config:
            logLevel: info
            resources:
              memory: 128Mi

        - name: advanced
          config:
            logLevel: debug
            resources:
              memory: 512Mi
            plugins: [elasticsearch, kafka]
```

Developers select preset:
```yaml
spec:
  loggingSidecar:
    preset: advanced
```

### Cross-Addon Configuration

Addons can reference each other:

```yaml
platformAddons:
  - name: persistent-volume
    config:
      volumeName: database-data
      mountPath: /var/lib/postgresql

  - name: database-init
    config:
      volumeRef: database-data  # References volume addon
      initScript: |
        CREATE DATABASE myapp;
```

## Summary

ComponentType provides:
1. **Composition**: Combine ComponentDefinition + Addons
2. **Separation**: PE infrastructure addons vs developer app addons
3. **Code Generation**: Auto-generate CRD for developers
4. **Governance**: Enforce policies via platform addons
5. **Flexibility**: Same base, multiple types for different needs
6. **Environment Awareness**: EnvBinding overrides for `envOverrides`
