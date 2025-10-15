# Rethinking Component Definitions

**Authors**:
@ChathurangaKCD

**Reviewers**:

**Created Date**:
2025-01-15

**Status**:
Submitted

**Related Issues/PRs**:
_TBD_

---

## Summary

This proposal introduces **ComponentTypeDefinitions with Addons** to replace OpenChoreo's current rigid component types (Services, WebApps, ScheduledTasks). The new approach enables platform engineers to define atomic, composable component types using CEL-based templates that stay close to Kubernetes primitives, while addons provide reusable cross-cutting capabilities. This design maintains a simplified developer experience through a single Component CRD while giving platform engineers full control and extensibility.

---

## Motivation

OpenChoreo's current component types (**Services**, **WebApps**, and **ScheduledTasks**) provide developer-centric abstractions where developers have direct control over the entire Deployment specification. While this gives developers flexibility, it creates several challenges:

- **Over-abstraction from Kubernetes APIs**: Current component types mask native Kubernetes resources, making the mental model harder for Kubernetes-savvy users to understand OpenChoreo's Components.

- **Limited platform engineer control**: ComponentClasses can only override portions of developer-defined Deployment specs. Platform engineers cannot enforce organizational policies, resource limits, or deployment configurations as templates that developers instantiate.

- **No composition model**: Platform engineers cannot define their own component types by combining Kubernetes primitives, OpenChoreo concepts, and external CRDs of other tools they may be using.

- **No environment-specific overrides**: There is no way to override or patch environment-specific parameters (like resource limits or replica counts) in a structured, validated manner.

As a result, platform engineers lack the control needed to enforce organizational standards, and they may be forced to bypass OpenChoreo entirely if their requirements do not fit into one of OpenChoreo's opinionated component types.

---

## Goals

- **Extensibility**: Platform engineers can model organizational needs without forking or bypassing OpenChoreo. Addons enable cross-cutting capabilities (storage, networking, security) to be reused across component types.

- **Alignment with Kubernetes**: Reduces friction by staying consistent with native APIs, making it easier for Kubernetes-savvy teams to understand and adopt OpenChoreo.

- **Developer Experience**: Developers get a consistent, simplified interface (single Component CRD) with guardrails, while platform engineers retain full control. OpenChoreo's developer abstractions like endpoints, connections and config schemas are retained without restricting PE control.

- **Reusability**: Define ComponentTypeDefinitions and Addons once, compose them in multiple ways across different components.

---

## Non-Goals

- **Replace Kubernetes primitives**: This proposal does not aim to abstract away or replace Kubernetes resources. Instead, it works with them to provide a higher-level interface while maintaining compatibility.

- **Support multiple primary workload resources**: Each ComponentTypeDefinition is limited to a single primary workload resource (Deployment, StatefulSet, CronJob, or Job). The `id` of this resource must match the workload kind in lowercase. Additional resources (Services, HPAs, PDBs, etc.) and custom resources can be defined in the resources section and via addons.

- **Backwards compatibility with existing component types**: Migrating existing Services, WebApps, and ScheduledTasks to the new model is not covered in this proposal and will be addressed separately.

---

## Impact

This proposal affects multiple areas of the OpenChoreo system:

**API Server & CRDs:**

- Introduction of new CRDs: `ComponentTypeDefinition`, `Addon`, and `EnvSettings`
- New `v1alpha2` version of the `Component` CRD with `componentType` and `addons[]` fields
- New validation webhooks for CEL template validation and schema enforcement

**Controllers:**

- New controller for rendering ComponentTypeDefinitions with CEL templating
- Enhanced component controller to handle addon composition and patching
- Environment-specific override reconciliation in deployment controllers

**CLI & Developer Experience:**

- CLI commands to create and manage ComponentTypeDefinitions and Addons
- Support for validating and previewing rendered resources before deployment
- Auto-generation of forms/prompts based on component type schemas

**Portal/UI:**

- Dynamic form generation from ComponentTypeDefinition schemas
- Addon marketplace/catalog for discovering available addons
- Visual editor for composing components with addons

**Backward Compatibility:**

- Existing component types (Service, WebApp, ScheduledTask) will continue to work
- Migration path will be provided separately to convert existing components to the new model

---

## Design

### Overview

![component_types](https://github.com/user-attachments/assets/2c9968ce-34b2-42c2-afbe-19fd62010104)

This design introduces three core mechanisms explained below with concrete examples:

### 1. Template-Based ComponentTypeDefinitions

ComponentTypeDefinitions use templates to generate Kubernetes resources dynamically, staying close to native K8s APIs while adding parameterization.

- Instead of static YAML, use **CEL (Common Expression Language)** templates that pull data from multiple sources, including developer inputs, the workload spec, and references to all resources defined within the template.

- Types, validations and expressions from CEL allow UIs to be autogenerated from the template, enabling dynamic forms for these resources in the OpenChoreo portal (and in the CLI).

- **Workload Type Constraint**: Each ComponentTypeDefinition must define exactly one primary workload resource (Deployment, StatefulSet, CronJob, or Job). The `id` of this resource must match the workload kind in lowercase (e.g., `id: deployment` for kind `Deployment`). Additional supporting resources (Service, HPA, PDB, etc.) can be included in the resources section.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentTypeDefinition
metadata:
  name: web-app
spec:
  # Schema defines what developers can configure
  schema:
    parameters:
      # Static across environments and exposed as inputs to the developer when creating a Component of this type
      # Examples provided after Component definition section

    envOverrides:
      # Can be overriden per environment via EnvironmentSettings by the PE
      # Examples provided after Component definition section

  # Templates generate K8s resources dynamically
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${component.metadata.name}
        spec:
          selector:
            matchLabels: |
              ${component.podSelectors}
          template:
            metadata:
              labels: |
                ${merge({"app": component.metadata.name}, component.podSelectors)}
            spec:
              terminationGracePeriodSeconds: ${spec.lifecycle.terminationGracePeriodSeconds}
              containers:
                - name: app
                  image: ${build.image}
                  imagePullPolicy: ${spec.lifecycle.imagePullPolicy}
                  command: ${spec.runtime.command}
                  args: ${spec.runtime.args}
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
          name: ${component.metadata.name}
        spec:
          scaleTargetRef:
            apiVersion: apps/v1
            kind: Deployment
            name: ${component.metadata.name}
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
          name: ${component.metadata.name}
        spec:
          selector:
            matchLabels:
              app: ${component.metadata.name}
          minAvailable: 1
```

**Key insight:** Templates access data from different sources at different times:

- `${component.metadata.*}` - Component instance metadata (name, namespace, labels, annotations)
- `${component.podSelectors}` - Platform-injected pod selectors (e.g., `openchoreo.io/component-id`, `openchoreo.io/environment`, `openchoreo.io/project-id`) for component identity and environment tracking
- `${spec.*}` - Developer configuration from Component (merged parameters + envOverrides)
- `${build.*}` - Build context from Component's build field
- `${workload.*}` - Application metadata extracted from source repo at build time

### 2. Addons for Composability

- Addons are atomic, reusable units that modify or augment the underlying Kubernetes resources without requiring separate component type definitions for every variation.
- This avoids combinatorial explosion of rigid component types (no need for "web-app-with-pvc" vs "web-app-with-X", instead PEs can define their own addons that developers can compose into their Components for cross-cutting capabilities).
- OpenChoreo can provide a set of pre-built Addons for common use cases, and PEs can write their own as well. Developers should be able to discover these via the OpenChoreo console/CLI.

Addons can:

- **Create** new resources (PVCs & Volumes, PDBs, ResourceQuotas, Certificates, etc.)
- **Modify (patch)** existing resources (add volumes, sidecars, init containers, topology spread constraints, pod security policies, etc)

### 3. Component CRD - Single Unified Resource

Instead of generating multiple CRDs, developers use a single **Component** CRD with a `componentType` field and `addons[]` array:

```yaml
apiVersion: openchoreo.dev/v1alpha2
kind: Component
metadata:
  name: checkout-service
spec:
  # Select which ComponentTypeDefinition to use
  componentType: web-app

  # Parameters from ComponentTypeDefinition (oneOf schema based on componentType).
  # Like all other parameters, these are typed, validated and can have default values (so can be optional when necessary).
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

  # Build field (added to CRD schema by OpenChoreo, populated by developer)
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

**Component CRD Schema:**

The Component CRD uses a **oneOf schema** for the `parameters` field based on the `componentType`:

- When `componentType: web-app`, the `parameters` field schema is the **merged schema** of the ComponentTypeDefinition's `parameters` and `envOverrides`
- This allows developers to configure both static parameters and environment-overridable settings in one place
- At runtime, these are split: `parameters` remain static, `envOverrides` can be overridden in EnvSettings
- Templates access merged values via `${spec.*}` (e.g., `${spec.lifecycle.terminationGracePeriodSeconds}`, `${spec.resources.requests.cpu}`)

**Workload Spec (extracted from source repo at build time):**

- The workload spec will be a developer-facing resource that's committed to the application's source repository (as the "workload.yaml").
- This will contain OpenChoreo developer concepts like `endpoints`, `connections`, and `config schema(?)` and will be proceed by the system and uses it as (automatic) inputs to the ComponentTypeDefinitions when composing the final Kubernetes resources.

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

This workload spec is available as `${workload.*}` in ComponentTypeDefinition templates, so that PEs can also build additional capabilities based on the values provided here by the developers.

**Example ComponentTypeDefinition schema with parameters and envOverrides:**

```yaml
schema:
  parameters:
    # Component-level parameters - static across environments
    runtime:
      command: array<string> | default=[]
      args: array<string> | default=[]
    lifecycle:
      terminationGracePeriodSeconds: integer | default=30
      imagePullPolicy: string | default=IfNotPresent enum="Always,IfNotPresent,Never"

  envOverrides:
    # Environment-overridable parameters
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
```

**Example Addon 1: Persistent Volume Claim with Mount** - A PE defined Addon that allows developers to add a persistent volume mount to a Component.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Addon
metadata:
  name: add-volume-mount
spec:
  displayName: "Persistent Volume Claim with Mount"
  description: "Provides a persistent volume and mounts it to the component container"

  schema:
    parameters: # Developer-facing parameters
      volumeName: string | required=true
      mountPath: string | required=true # Developer decides where in the container to mount the volume
      containerName: string | default="main" # Which container to mount to (defaults to app)

    envOverrides: # Platform engineers can override per environment
      size: string | default=10Gi
      storageClass: string | default=standard

  creates:
    - apiVersion: v1
      kind: PersistentVolumeClaim
      metadata:
        name: ${component.metadata.name}-${instanceId}
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: ${spec.size}
        storageClassName: ${spec.storageClass}

  patches:
    # Attach PVC as a volume in the pod spec
    - target:
        resourceType: Deployment
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: ${spec.volumeName}
          persistentVolumeClaim:
            claimName: ${component.metadata.name}-${instanceId}

    # Mount the PVC into the developer-specified container at the mountPath
    - target:
        resourceType: Deployment
        containerName: ${spec.containerName}
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name=='${spec.containerName}')]/volumeMounts/-
        value:
          name: ${spec.volumeName}
          mountPath: ${spec.mountPath}
```

**Example Addon 2: EmptyDir Volume with Sidecar** - Allows legacy applications to stream their file based logs to stdout for log collection.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Addon
metadata:
  name: add-file-logging-sidecar
spec:
  displayName: "Stream File Logs to Stdout"
  description: "Pushes logs from a log file to stdout so that logs will be collected by the system"

  schema:
    parameters:
      logFilePath: string | required=true
      containerName: string | default="main" # Which container to mount to (defaults to app)

  patches:
    # Inject sidecar container
    - target:
        resourceType: Deployment
      patch:
        op: add
        path: /spec/template/spec/containers/-
        value:
          name: log-sidecar
          image: busybox
          command: ["/bin/sh", "-c"]
          args:
            - |
              echo "Starting log tail for ${spec.logFilePath}"
              tail -n+1 -F ${spec.logFilePath}
          volumeMounts:
            - name: app-logs
              mountPath: /logs

    # Ensure main container has log volume
    - target:
        resourceType: Deployment
        containerName: app
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name==${spec.containerName}')]/volumeMounts/-
        value:
          name: app-logs
          mountPath: /logs

    # Add volume for log directory (shared between app + sidecar)
    - target:
        resourceType: Deployment
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: app-logs
          emptyDir: {}
```

### Step 4: Environment-Specific Overrides

- As both `ComponentTypeDefinitions` and `Addons` define `envOverrides` fields, this `EnvSettings` resource targeting a particular `Environment` can override the default values that need to be adjusted for a given environment.
- This is a more structured, strongly-typed and validate-able UX for the PE as opposed to using something like Kustomize patches.

**Example: EnvSettings for the `production` environment for a component named "checkout-service"**

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: EnvSettings
metadata:
  name: checkout-service-prod
spec:
  owner:
    componentName: checkout-service
  environment: production

  # Override values in the Component for this environment.
  # These fields are defined in the ComponentTypeDefinition's envOverrides section that instantiated this component.
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

  # Override addon values for the environment (keyed by addon name, then instanceId).
  # These fields are defined in the addon's envOverrides section.
  addonOverrides:
    persistent-volume-claim: # Addon name
      app-data: # instanceId
        size: 200Gi # Much larger in prod
        storageClass: premium
```
