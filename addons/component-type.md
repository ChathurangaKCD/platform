# Component CRD Specification

## Overview

A **Component** is the single CRD that developers use to define their application components. It references a ComponentTypeDefinition and can include addon instances.

## Resource Hierarchy

```
ComponentTypeDefinition (PE-authored base template)
        +
    Addons (PE-authored, developer-selected)
        ↓
Component (Developer-created, kind: Component)
        +
   EnvSettings (Per-environment overrides)
        ↓
Final K8s Resources (Deployed to cluster)
```

## Component CRD Structure

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
  namespace: my-project
spec:
  # Which ComponentTypeDefinition to use
  componentType: web-app

  # Parameters from ComponentTypeDefinition
  # (merged parameters + envOverrides, oneOf schema based on componentType)
  parameters:
    maxReplicas: 5
    resources:
      requests:
        cpu: 200m
        memory: 512Mi

  # Addon instances
  addons:
    - name: persistent-volume
      instanceId: app-data        # Always required
      config:
        # parameters (static)
        volumeName: app-data
        mountPath: /app/data
        containerName: app

        # envOverrides (overridable per environment)
        size: 50Gi
        storageClass: fast

    - name: persistent-volume
      instanceId: cache-data      # Different instance
      config:
        volumeName: cache-data
        mountPath: /app/cache
        size: 20Gi
        storageClass: standard

    - name: network-policy
      instanceId: default         # Always required
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress"
            ports: [8080]

  # Build field (added to CRD schema by platform, populated by developer)
  build:
    repository:
      url: https://github.com/myorg/customer-service
      revision:
        branch: main
      appPath: .
    templateRef:
      name: docker
      parameters:
        - name: docker-context
          value: .
        - name: dockerfile-path
          value: ./Dockerfile

status:
  # Reconciliation status
  conditions:
    - type: Ready
      status: "True"
      reason: "ResourcesApplied"
      message: "All resources successfully applied"

  observedGeneration: 5
  lastReconcileTime: "2025-01-15T10:30:00Z"
```

**Workload Metadata:**

Developer defines workload metadata (endpoints, connections, etc.) in the source repository (e.g., `workload.yaml`). The platform extracts this at build time and makes it available to ComponentTypeDefinition templates via `${workload.*}`.

```yaml
# customer-portal/workload.yaml (in source repo)
configSchemaPath: ./schemas/config.json
endpoints:
  - name: api
    type: http
    port: 8080
    schemaPath: ./openapi/api.yaml
  - name: grpc-endpoint
    type: gRPC
    port: 5050
    schemaPath: ./proto/service.proto

connections:
  - name: productcatalog
    type: api
    params:
      projectName: gcp-microservice-demo
      componentName: productcatalog
      endpoint: grpc-endpoint
    inject:
      env:
        - name: PRODUCT_CATALOG_SERVICE_ADDR
          value: "{{ .host }}:{{ .port }}"
```

## How It Works

### 1. Developer Creates Component

Developer creates a Component resource specifying:

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: my-app
spec:
  # Select ComponentTypeDefinition
  componentType: web-app

  # Configure parameters from ComponentTypeDefinition
  parameters:
    maxReplicas: 5

  # Add addon instances
  addons:
    - name: persistent-volume
      instanceId: app-data
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 50Gi
        storageClass: fast

  build:
    repository:
      url: https://github.com/myorg/my-app
      revision:
        branch: main
```

### 2. Platform Reconciles Component

The controller:

1. Loads ComponentTypeDefinition (`web-app`)
2. Loads addons specified in `spec.addons[]`
3. Validates addon compatibility
4. Extracts workload metadata from source repo at build time (`workload.yaml`)
5. Applies addons to ComponentTypeDefinition resources
6. Renders final K8s resources using Component parameters, build context, and workload metadata
7. Applies to cluster

### 3. Developer Creates EnvSettings

For environment-specific overrides:

```yaml
apiVersion: platform/v1alpha1
kind: EnvSettings
metadata:
  name: my-app-prod
spec:
  owner:
    componentName: my-app
  environment: production

  # Override component envOverrides
  overrides:
    maxReplicas: 20
    resources:
      requests:
        cpu: 500m
        memory: 1Gi

  # Override addon envOverrides
  addonOverrides:
    persistent-volume:       # Addon name
      app-data:              # instanceId
        size: 200Gi          # Much larger in prod
        storageClass: premium
```

## Component Schema Structure

The Component CRD has a **oneOf schema** for parameters based on `componentType`:

```yaml
# Component CRD schema
spec:
  type: object
  properties:
    componentType:
      type: string
      enum: [web-app, worker, database, ...]  # All available ComponentTypeDefinitions

    parameters:
      # oneOf based on componentType
      oneOf:
        - # If componentType == "web-app"
          properties:
            maxReplicas:
              type: integer
              default: 3
            resources:
              type: object
              properties:
                requests:
                  type: object
                  properties:
                    cpu:
                      type: string
                      default: 100m
                    memory:
                      type: string
                      default: 256Mi

        - # If componentType == "worker"
          properties:
            concurrency:
              type: integer
              default: 1
            queueName:
              type: string

    addons:
      type: array
      items:
        type: object
        required: [name, instanceId, config]
        properties:
          name:
            type: string
          instanceId:
            type: string
          config:
            type: object  # Schema varies by addon

    build:
      type: object
      # Platform-injected, read-only for developers
```

## Developer Workload Spec

Developers define application-specific metadata in the source repository (`workload.yaml`):

```yaml
# workload.yaml (in source repo)

# Schema for application configuration
configSchemaPath: ./schemas/config.json

# Endpoints exposed by the application
endpoints:
  grpc-endpoint:
    type: gRPC
    port: 5050
    schemaPath: ./proto/productcatalog_service.proto

# Connections to other components
connections:
  productcatalog:
    type: api
    params:
      projectName: gcp-microservice-demo
      componentName: productcatalog
      endpoint: grpc-endpoint
    inject:
      env:
        - name: PRODUCT_CATALOG_SERVICE_ADDR
          value: "{{ .host }}:{{ .port }}"
```

This metadata is extracted at build time and available to ComponentTypeDefinition templates via `${workload.*}`.

## Examples

### Example 1: Simple Web App

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: frontend
spec:
  componentType: web-app

  parameters:
    maxReplicas: 3

  addons:
    - name: network-policy
      instanceId: default
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress-gateway"

# Workload metadata in source repo (workload.yaml):
# endpoints:
#   - name: http
#     port: 8080
```

### Example 2: Stateful Service with Multiple Volumes

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: database-proxy
spec:
  componentType: web-app

  parameters:
    maxReplicas: 3
    resources:
      requests:
        cpu: 500m
        memory: 1Gi

  addons:
    - name: persistent-volume
      instanceId: data
      config:
        volumeName: data
        mountPath: /var/lib/data
        size: 100Gi
        storageClass: premium

    - name: persistent-volume
      instanceId: logs
      config:
        volumeName: logs
        mountPath: /var/log
        size: 20Gi
        storageClass: standard

    - name: network-policy
      instanceId: default
      config:
        allowIngress:
          - from: "namespace:application"
            ports: [5432]
        allowEgress:
          - to: "namespace:database-cluster"
            ports: [5432]
```

### Example 3: Worker with Config Files

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: order-processor
spec:
  componentType: worker

  parameters:
    concurrency: 5
    queueName: orders

  addons:
    - name: config-files
      instanceId: default
      config:
        configs:
          - name: processor-config
            type: configmap
            mountPath: /etc/processor
            files:
              - fileName: config.yaml
                content: |
                  processing:
                    batchSize: 100
                    timeout: 30s

    - name: logging-sidecar
      instanceId: default
      config:
        enabled: true
        logLevel: info
```

## Component Lifecycle

### Creation

1. Developer defines Component YAML (including build field)
2. Platform validates:
   - ComponentTypeDefinition exists
   - Addons exist and are compatible
   - Parameters match ComponentTypeDefinition schema
3. Controller creates Component resource

### Reconciliation

1. Controller loads ComponentTypeDefinition
2. Controller loads addons
3. Controller loads workload metadata from source repo
4. Controller applies addons to base resources
5. Controller loads EnvSettings for environment
6. Controller applies environment overrides
7. Controller renders final K8s resources
8. Controller applies resources to cluster

### Updates

1. Developer modifies Component
2. Controller detects change
3. Controller re-reconciles with new spec
4. Resources updated in cluster

### Deletion

1. Developer deletes Component
2. Controller deletes all generated K8s resources
3. Component removed

## Environment Progression

Same component, different configurations per environment:

**Development:**
```yaml
# Component
spec:
  addons:
    - name: persistent-volume
      instanceId: data
      config:
        size: 10Gi
        storageClass: standard

# No EnvSettings needed
```

**Production:**
```yaml
# Component (same as dev)
spec:
  addons:
    - name: persistent-volume
      instanceId: data
      config:
        size: 10Gi
        storageClass: standard

# EnvSettings for production
---
apiVersion: platform/v1alpha1
kind: EnvSettings
metadata:
  name: my-app-prod
spec:
  environment: production
  addonOverrides:
    persistent-volume:
      data:
        size: 200Gi
        storageClass: premium
```

## Benefits of This Design

### 1. Simplification
- No intermediate ComponentType resource
- Single Component CRD for all component types
- Developers directly express their intent

### 2. Flexibility
- Developers choose which addons to use
- Developers configure addon parameters
- Multiple instances of same addon supported

### 3. Type Safety
- oneOf schema ensures parameters match componentType
- Addon configs validated against addon schemas
- EnvSettings validated against envOverrides

### 4. Environment Awareness
- EnvSettings override only envOverrides
- Parameters stay consistent across environments
- Clear separation of static vs environment-specific config

### 5. Platform Control
- Platform defines Component CRD schema (including build field)
- Platform validates addon compatibility
- Platform enforces ComponentTypeDefinition constraints

## Summary

The Component CRD provides:
1. **Single resource**: Developers create Component, not type-specific CRDs
2. **Explicit composition**: Developers specify componentType and addons
3. **Type-safe parameters**: oneOf schema based on componentType
4. **Addon instances**: Array of addons with instanceId and config
5. **Build integration**: build field in schema for repository and template info
6. **Environment awareness**: EnvSettings for environment-specific overrides
