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
      lifecycle:
        terminationGracePeriodSeconds: integer | default=30
        imagePullPolicy: string | default=IfNotPresent enum="Always,IfNotPresent,Never"

    envOverrides:
      # Can vary per environment
      resources:
        requests:
          cpu: string | default=100m
          memory: string | default=256Mi
        limits:
          cpu: string | default=500m
          memory: string | default=512Mi
      autoscaling:
        enabled: boolean | default=false
        minReplicas: integer | default=1
        maxReplicas: integer | default=10
        targetCPUUtilization: integer | default=80

  # Templates generate K8s resources dynamically
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${metadata.name}
        spec:
          selector:
            matchLabels:
              app: ${metadata.name}
          template:
            metadata:
              labels:
                app: ${metadata.name}
            spec:
              terminationGracePeriodSeconds: ${spec.lifecycle.terminationGracePeriodSeconds}
              containers:
                - name: app
                  image: ${build.image}
                  imagePullPolicy: ${spec.lifecycle.imagePullPolicy}
                  ports: |
                    ${workload.endpoints.map(e, {"containerPort": e.port})}
                  resources:
                    requests:
                      cpu: ${spec.resources.requests.cpu}
                      memory: ${spec.resources.requests.memory}
                    limits:
                      cpu: ${spec.resources.limits.cpu}
                      memory: ${spec.resources.limits.memory}

    - id: hpa
      condition: ${spec.autoscaling.enabled}
      template:
        apiVersion: autoscaling/v2
        kind: HorizontalPodAutoscaler
        metadata:
          name: ${metadata.name}
        spec:
          scaleTargetRef:
            apiVersion: apps/v1
            kind: Deployment
            name: ${metadata.name}
          minReplicas: ${spec.autoscaling.minReplicas}
          maxReplicas: ${spec.autoscaling.maxReplicas}
          metrics:
            - type: Resource
              resource:
                name: cpu
                target:
                  type: Utilization
                  averageUtilization: ${spec.autoscaling.targetCPUUtilization}

    - id: pdb
      condition: ${spec.autoscaling.enabled}
      template:
        apiVersion: policy/v1
        kind: PodDisruptionBudget
        metadata:
          name: ${metadata.name}
        spec:
          selector:
            matchLabels:
              app: ${metadata.name}
          minAvailable: 1
```

**Key insight:** Templates access data from different sources at different times:

- `${metadata.*}` - Component instance metadata
- `${spec.*}` - Developer configuration from Component (merged parameters + envOverrides)
- `${build.*}` - Build context from Component's build field
- `${workload.*}` - Application metadata extracted from source repo at build time

### 2. Addons for Composability

Addons are atomic, reusable units that modify or augment ComponentTypeDefinitions without requiring separate component type definitions for every variation.

Addons can:

- **Create** new resources (PVCs, NetworkPolicies, Certificates)
- **Modify** existing resources (add volumes, sidecars, environment variables)

**Addon 1: PVC Addon** - Creates PVC and adds volume to pod

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume-claim
spec:
  displayName: "Persistent Volume Claim"

  schema:
    parameters:
      volumeName: string | required=true
    envOverrides:
      size: string | default=10Gi
      storageClass: string | default=standard

  creates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: ${metadata.name}-${instanceId}
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: ${spec.size}
        storageClassName: ${spec.storageClass}

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

**Addon 2: Volume Mount Addon** - Mounts a volume to a specific container

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: volume-mount
spec:
  displayName: "Volume Mount"

  schema:
    parameters:
      volumeName: string | required=true
      mountPath: string | required=true
      containerName: string | required=true
      subPath: string | default=""
      readOnly: boolean | default=false

  patches:
    - target:
        resourceType: Deployment
        containerName: ${spec.containerName}
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name=='${spec.containerName}')]/volumeMounts/-
        value:
          name: ${spec.volumeName}
          mountPath: ${spec.mountPath}
          subPath: ${spec.subPath}
          readOnly: ${spec.readOnly}
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
    lifecycle:
      terminationGracePeriodSeconds: 60
      imagePullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 200m
        memory: 512Mi
      limits:
        cpu: 1000m
        memory: 1Gi
    autoscaling:
      enabled: false
      minReplicas: 2
      maxReplicas: 10
      targetCPUUtilization: 80

  # Addon instances (developer chooses which addons to use)
  addons:
    # Create PVC and add volume to pod
    - name: persistent-volume-claim
      instanceId: app-data
      config:
        volumeName: app-data-vol
        size: 50Gi
        storageClass: fast

    # Mount the volume to the app container
    - name: volume-mount
      instanceId: app-data-mount
      config:
        volumeName: app-data-vol
        mountPath: /app/data
        containerName: app
        readOnly: false

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
configSchema: ... # Design TBD
```

This workload spec is available as `${workload.*}` in ComponentTypeDefinition templates.

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
    resources:
      requests:
        cpu: 500m
        memory: 1Gi
      limits:
        cpu: 2000m
        memory: 2Gi
    autoscaling:
      enabled: true
      minReplicas: 5
      maxReplicas: 50
      targetCPUUtilization: 70

  # Override addon envOverrides (keyed by addon name, then instanceId)
  addonOverrides:
    persistent-volume-claim: # Addon name
      app-data: # instanceId
        size: 200Gi # Much larger in prod
        storageClass: premium
```
