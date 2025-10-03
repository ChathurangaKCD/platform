# ComponentTypeDefinitions with Addons

This document explains how ComponentTypeDefinitions with Addons achieve the goals outlined in the main proposal: making component definitions **atomic, composable, and close to Kubernetes primitives**.

> **Full technical details**: See [detailed documentation](https://github.com/openchoreo/openchoreo/tree/main/docs/component-definitions) in the repository.

---

## Overview

To enable extensible composition while staying close to Kubernetes APIs, we introduce two core mechanisms:

1. **Template-based ComponentTypeDefinitions** - Generate K8s resources dynamically using CEL expressions
2. **Addons** - Reusable, composable units that augment ComponentTypeDefinitions

This approach allows Platform Engineers to:

- Define base component templates once (ComponentTypeDefinitions)
- Create reusable infrastructure addons (storage, networking, security)
- Allow developers to compose ComponentTypeDefinitions with Addons
- Avoid creating separate definitions for every variation

Developers then:

- Create Component resources (single CRD: `kind: Component`)
- Select which ComponentTypeDefinition to use
- Add addon instances to customize behavior
- Get a unified, simple interface

---

## Core Mechanisms

### 1. Template-Based ComponentTypeDefinitions

ComponentTypeDefinitions use templates to generate Kubernetes resources dynamically, staying close to native K8s APIs while adding parameterization.

Instead of static YAML, use **CEL (Common Expression Language)** templates that pull data from multiple sources:

```yaml
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: web-app
spec:
  # Schema defines what developers can configure
  schema:
    parameters:
      # Static across environments
      appType: string | default=stateless

    envOverrides:
      # Can vary per environment
      maxReplicas: integer | default=3

  # Templates generate K8s resources dynamically
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${metadata.name}  # From Component instance
        spec:
          replicas: ${spec.parameters.maxReplicas}  # From developer's Component
          template:
            spec:
              containers:
                - name: app
                  image: ${spec.build.image}  # Platform-injected at runtime
                  ports: ${workload.endpoints.map(e, {"containerPort": e.port})}  # From workload spec
```

**Key insight:** Templates access data from different sources at different times:

- `${metadata.*}` - Component instance metadata
- `${spec.parameters.*}` - Developer configuration from Component
- `${spec.build.*}` - Platform runtime context (injected by platform)
- `${workload.*}` - Application metadata extracted from source repo at build time

### 2. Addons for Composability

Addons are atomic, reusable units that modify or augment ComponentTypeDefinitions without requiring separate component type definitions for every variation.

Addons can:

- **Create** new resources (PVCs, NetworkPolicies, Certificates)
- **Modify** existing resources (add volumes, sidecars, environment variables)

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume
spec:
  displayName: "Persistent Volume"

  schema:
    parameters:
      volumeName: string | required=true
      mountPath: string | required=true
    envOverrides:
      size: string | default=10Gi # Can differ per environment
      storageClass: string | default=standard

  # What this addon creates
  creates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: ${metadata.name}-${instanceId}
      spec:
        resources:
          requests:
            storage: ${spec.size}
        storageClassName: ${spec.storageClass}

  # How this addon modifies existing resources
  patches:
    - target:
        resourceType: Deployment
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: ${spec.volumeName}
          persistentVolumeClaim:
            claimName: ${metadata.name}-${instanceId}
```

### 3. Component CRD - Single Unified Resource

Instead of generating multiple CRDs, developers use a single **Component** CRD with a `componentType` field and `addons[]` array:

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: checkout-service
spec:
  # Select which ComponentTypeDefinition to use
  componentType: web-app

  # Parameters from ComponentTypeDefinition (oneOf schema based on componentType)
  parameters:
    maxReplicas: 5

  # Addon instances (developer chooses which addons to use)
  addons:
    - name: persistent-volume
      instanceId: app-data # Always required
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 50Gi
        storageClass: fast

    - name: network-policy
      instanceId: default
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress"
            ports: [8080]

  # Build field (added to CRD schema by platform, populated by developer)
  build:
    repository:
      url: https://github.com/myorg/checkout-service
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
```

**Workload Spec (extracted from source repo at build time):**

The platform extracts workload metadata from the source repository (e.g., `workload.yaml`) and uses it as input to ComponentTypeDefinition templates:

```yaml
# workload.yaml in source repo
configSchemaPath: ./schemas/config.json
endpoints:
  - name: api
    type: http
    port: 8080
    schemaPath: ./openapi/api.yaml

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

This workload spec is available as `${workload.*}` in ComponentTypeDefinition templates.

---

## The Workflow: End-to-End

### Step 1: PE Creates Base ComponentTypeDefinition

Platform Engineer defines a reusable template:

```yaml
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: web-app
spec:
  schema:
    envOverrides:
      maxReplicas: integer | default=3
      resources:
        requests:
          cpu: string | default=100m
          memory: string | default=256Mi

  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        spec:
          replicas: ${spec.parameters.maxReplicas}
          template:
            spec:
              containers:
                - name: app
                  image: ${spec.build.image}
                  resources:
                    requests:
                      cpu: ${spec.parameters.resources.requests.cpu}
                      memory: ${spec.parameters.resources.requests.memory}
                  ports: ${workload.endpoints.map(e, {"containerPort": e.port})}

    - id: service
      template:
        apiVersion: v1
        kind: Service
        spec:
          ports: ${workload.endpoints.map(e, {"name": e.name, "port": e.port})}

    - id: ingress
      forEach: ${workload.endpoints.filter(e, e.visibility == "public")}
      template:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          name: ${metadata.name}-${item.name}
        spec:
          rules:
            - host: ${item.host}
              http:
                paths:
                  - path: ${item.path}
                    backend:
                      service:
                        name: ${metadata.name}-svc
```

**Key features:**

- Uses `forEach` to generate multiple Ingresses based on workload endpoints
- Pulls endpoints from workload spec in Component
- Separates environment-specific config (`envOverrides`) from static config (`parameters`)

### Step 2: PE Creates Addons

Platform Engineer creates reusable addons for common infrastructure needs:

**Addon 1: Persistent Storage**

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
    envOverrides:
      size: string | default=10Gi
      storageClass: string | default=standard

  creates:
    - kind: PersistentVolumeClaim
      # ... creates PVC

  patches:
    - target: { resourceType: Deployment }
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        # ... adds volume mount
```

**Addon 2: Network Isolation**

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: network-policy
spec:
  schema:
    parameters:
      denyAll: boolean | default=true
      allowIngress: "[]object"
      allowEgress: "[]object"

  creates:
    - kind: NetworkPolicy
      # ... creates strict network policy
```

**Addon 3: Application Config**

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: config-files
spec:
  schema:
    parameters:
      configs: "[]object"
        name: string
        type: string | enum="configmap,secret"
        mountPath: string
        files: "[]object"

  creates:
    - forEach: ${spec.configs}
      resource:
        kind: ConfigMap  # or Secret
        # ... creates config/secret per item
```

### Step 3: Developer Creates Component

Developer uses the Component CRD to compose ComponentTypeDefinition with addons:

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: checkout-service
  namespace: my-project
spec:
  # Select ComponentTypeDefinition
  componentType: web-app

  # Component parameters
  parameters:
    maxReplicas: 5
    resources:
      requests:
        cpu: 200m
        memory: 512Mi

  # Addon instances
  addons:
    - name: persistent-volume
      instanceId: app-data
      config:
        volumeName: app-data
        mountPath: /app/data
        size: 50Gi # envOverride - can vary per environment
        storageClass: fast

    - name: network-policy
      instanceId: default
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress-gateway"
            ports: [8080]
        allowEgress:
          - to: "namespace:backend-services"
            ports: [8080]

    - name: config-files
      instanceId: app-config
      config:
        configs:
          - name: payment-config
            type: secret
            mountPath: /etc/payment
            files:
              - fileName: credentials.json
                content: |
                  {"api_key": "..."}

  # Build field (added to CRD schema by platform, populated by developer)
  build:
    repository:
      url: https://github.com/myorg/checkout-service
      revision:
        branch: main
    templateRef:
      name: docker
```

**Developer also defines workload metadata in source repo** (`workload.yaml`):

```yaml
# checkout-service/workload.yaml
configSchemaPath: ./schemas/config.json
endpoints:
  - name: api
    type: http
    port: 8080
    path: /
    visibility: public

connections:
  - name: payment-service
    type: api
    params:
      componentName: payment
      endpoint: api
    inject:
      env:
        - name: PAYMENT_SERVICE_URL
          value: "{{ .host }}:{{ .port }}"
```

**Developer experience:**

- Simple Component CRD
- Choose ComponentTypeDefinition via `componentType` field
- Configure component parameters
- Add addons via `addons[]` array with `instanceId`
- Provide `build` field with repository and template info
- Define workload metadata (endpoints, connections) in source repo (`workload.yaml`)

### Step 4: Environment-Specific Overrides

EnvSettings for production environment:

```yaml
apiVersion: platform/v1alpha1
kind: EnvSettings
metadata:
  name: checkout-service-prod
spec:
  owner:
    componentName: checkout-service
  environment: production

  # Override component envOverrides
  overrides:
    maxReplicas: 20
    resources:
      requests:
        cpu: 500m
        memory: 1Gi

  # Override addon envOverrides (keyed by instanceId)
  addonOverrides:
    app-data: # instanceId of persistent-volume addon
      size: 200Gi # Much larger in prod
      storageClass: premium
```

**Key features:**

- Same component, different configuration per environment
- Can override `envOverrides` from component and addons
- Cannot override `parameters` (those are static)
- `addonOverrides` keyed by `instanceId`

### Step 5: Platform Renders Final Resources

When the controller reconciles, it:

1. Loads ComponentTypeDefinition (`web-app`)
2. Loads Component instance
3. Extracts workload metadata from source repo (at build time)
4. Applies addon instances from Component's `addons[]` array
5. Loads EnvSettings
6. Renders final Kubernetes resources

**Example output** (simplified):

```yaml
# Deployment (from ComponentTypeDefinition + addons)
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 20 # From EnvSettings override
  template:
    spec:
      containers:
        - name: app
          image: gcr.io/company/checkout:v2.3.1 # From build
          ports:
            - containerPort: 8080 # From workload endpoints
          resources:
            requests:
              cpu: 500m # From EnvSettings override
              memory: 1Gi
          volumeMounts:
            - name: app-data # From persistent-volume addon
              mountPath: /app/data
            - name: payment-config # From config-files addon
              mountPath: /etc/payment

      volumes:
        - name: app-data # From persistent-volume addon
          persistentVolumeClaim:
            claimName: checkout-service-app-data
        - name: payment-config # From config-files addon
          secret:
            secretName: checkout-service-payment-config

---
# PVC (from persistent-volume addon)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: checkout-service-app-data
spec:
  resources:
    requests:
      storage: 200Gi # From EnvSettings override
  storageClassName: premium # From EnvSettings override

---
# NetworkPolicy (from network-policy addon)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
spec:
  podSelector:
    matchLabels:
      app: checkout-service
  policyTypes: [Ingress, Egress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress-gateway
      ports: [8080]

---
# Service (from ComponentTypeDefinition)
apiVersion: v1
kind: Service
spec:
  ports:
    - name: api
      port: 8080 # From workload endpoints

---
# Ingress (from ComponentTypeDefinition forEach)
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: checkout-service-api
spec:
  rules:
    - host: checkout.example.com # From EnvSettings
      http:
        paths:
          - path: / # From workload endpoints
            backend:
              service:
                name: checkout-service-svc
```

---

## Key Benefits Illustrated

### 1. Simplified Architecture

**Single Component CRD for all component types:**

```
web-app ComponentTypeDefinition
    ↓
Component (kind: Component, componentType: web-app)
    +
Addons (in Component.spec.addons[])
    ↓
Final K8s Resources
```

No intermediate resources, no generated CRDs per type.

### 2. Developer Control

Developers explicitly choose addons:

```yaml
spec:
  addons:
    - name: persistent-volume
      instanceId: data
      config: { ... }
    - name: network-policy
      instanceId: default
      config: { ... }
```

Full control over which addons to use and how to configure them.

### 3. Environment Progression

Same component definition, different configurations:

```
Development:
  - maxReplicas: 2
  - storage: 10Gi (standard)

Staging:
  - maxReplicas: 5
  - storage: 50Gi (fast)

Production:
  - maxReplicas: 20
  - storage: 200Gi (premium)
```

### 4. Multiple Addon Instances

Use the same addon multiple times:

```yaml
addons:
  - name: persistent-volume
    instanceId: app-data
    config:
      volumeName: app-data
      mountPath: /app/data
      size: 100Gi

  - name: persistent-volume
    instanceId: cache-data
    config:
      volumeName: cache-data
      mountPath: /app/cache
      size: 20Gi
```

`instanceId` required for all addons ensures consistent override structure.

---

## Simplified Composition Model

**No two-stage composition**, just runtime composition:

```
ComponentTypeDefinition (base template)
        +
Component (with addon instances)
        +
Build Context (platform-injected)
        ↓
[Runtime Composition Engine]
        ↓
Final Kubernetes Resources
```

**What happens:**

1. Developer creates Component (with componentType, parameters, addons, build)
2. Developer defines workload metadata in source repo (`workload.yaml`)
3. Controller loads ComponentTypeDefinition
4. Controller extracts workload metadata from source repo (at build time)
5. Controller applies addon instances from Component's `addons[]` array
6. Controller loads EnvSettings for environment
7. Controller applies environment overrides
8. Controller renders final K8s resources using Component spec, build info, and workload metadata
9. Resources applied to cluster

**Result:** Simple, unified model with runtime composition and build-time workload extraction.

---

## Real-World Example: E-Commerce Platform

**Scenario:** Company needs to deploy multiple microservices with different requirements.

### PE Creates ComponentTypeDefinitions

```yaml
# For web services
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: web-app
spec:
  schema:
    envOverrides:
      maxReplicas: integer | default=3
  resources:
    - id: deployment
      template:
        kind: Deployment
        # ... deployment spec

---
# For workers
apiVersion: platform/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: worker
spec:
  schema:
    parameters:
      queueName: string | required=true
      concurrency: integer | default=1
  resources:
    - id: deployment
      template:
        kind: Deployment
        # ... worker deployment spec
```

### Developers Use Appropriate Type

**Product catalog service** (web):

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: product-catalog
spec:
  componentType: web-app
  parameters:
    maxReplicas: 10
  # Workload metadata in source repo:
  # - endpoints (grpc on port 5050)
```

**Order processor** (worker):

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: order-processor
spec:
  componentType: worker
  parameters:
    queueName: orders
    concurrency: 5
  addons:
    - name: config-files
      instanceId: processor-config
      config:
        configs:
          - name: processor-config
            type: configmap
            mountPath: /etc/processor
  # Workload metadata in source repo:
  # - queue configuration
```

**Result:**

- Product team gets web service with auto-scaling
- Order processing team gets worker with queue integration
- Both use the same base system (Component CRD)
- Different ComponentTypeDefinitions for different needs

---

## How This Achieves the Proposal Goals

### 1. Atomic, Composable CRDs

**Addons as atomic units:**

- `persistent-volume` addon encapsulates storage provisioning
- `network-policy` addon encapsulates network isolation
- `config-files` addon encapsulates configuration management

**Composition in action:**

```
web-app (ComponentTypeDefinition)
  + persistent-volume (addon)
  + network-policy (addon)
  = Component (with both addons)
```

Developers compose atomic pieces rather than maintaining monolithic definitions.

### 2. Close to Kubernetes APIs

**Native K8s resources in templates:**

```yaml
resources:
  - id: deployment
    template:
      apiVersion: apps/v1
      kind: Deployment # Native K8s Deployment
      spec:
        replicas: ${spec.parameters.maxReplicas}
```

**Addons create/modify native resources:**

- Addons work with Deployment, Service, Ingress, PVC directly
- No abstraction layer hiding Kubernetes concepts
- Platform Engineers familiar with K8s can immediately understand the system

### 3. Extensible Composition

**Multiple composition points:**

**Base ComponentTypeDefinition:**

```yaml
resources:
  - kind: Deployment
  - kind: Service
  - kind: Ingress
```

**Addons (developer-selected):**

```yaml
addons:
  - persistent-volume # Creates PVC, patches Deployment
  - network-policy # Creates NetworkPolicy
  - config-files # Creates ConfigMap/Secret
```

**External CRDs:**
Addons can create resources from external CRDs (Crossplane, cloud provider operators, etc.):

```yaml
creates:
  - apiVersion: database.example.com/v1
    kind: PostgresInstance # External CRD
```

### 4. Parameterization & Environment Awareness

**Two-level schema:**

`parameters` - Static across environments:

```yaml
schema:
  parameters:
    volumeName: string
    mountPath: string
```

`envOverrides` - Vary per environment:

```yaml
schema:
  envOverrides:
    size: string | default=10Gi
    storageClass: string | default=standard
```

**Environment progression:**

```yaml
# Development EnvSettings
addonOverrides:
  app-data:                 # instanceId
    size: 10Gi
    storageClass: standard

# Production EnvSettings
addonOverrides:
  app-data:                 # instanceId
    size: 200Gi
    storageClass: premium
```

**Note:** EnvSettings uses `addonOverrides` keyed by `instanceId`, ensuring consistent structure for all addons.

Same component definition, different configurations per environment.

---

## Summary

This proposal introduces **ComponentTypeDefinitions with Addons** to achieve the goals outlined in the main proposal:

**Simplified Architecture:**

- Single Component CRD for all component types
- No intermediate resources or generated CRDs
- ComponentTypeDefinition as base template
- Addons as composable units
- Runtime composition when Component is created

**Atomic & Composable:**

- Addons are self-contained units encapsulating specific concerns (storage, networking, security)
- Developers compose ComponentTypeDefinition + Addons in Component resource
- Simple, reusable pieces

**Close to Kubernetes APIs:**

- Templates generate native K8s resources (Deployment, Service, Ingress, etc.)
- Addons create and modify standard K8s resources directly
- No abstraction layer hiding Kubernetes concepts

**Extensible Composition:**

- Developers choose which addons to use
- Multiple instances of same addon supported via `instanceId`
- Support for external CRDs (Crossplane, cloud providers, custom operators)
- Runtime composition model

**Parameterization & Environment Awareness:**

- Schema separates static config (`parameters`) from environment-specific config (`envOverrides`)
- EnvSettings provides environment-specific overrides for components and addons
- Developers get unified Component CRD with clear override semantics

**Result:** Developers can compose ComponentTypeDefinitions with Addons using a simple, unified Component CRD, while the platform handles all composition at runtime.
