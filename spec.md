## Overview

The system uses CEL (Common Expression Language) templates in ComponentDefinitions to dynamically generate Kubernetes resources. Template variables come from multiple sources:

### CEL Template Context

Templates have access to:

1. **`${metadata.*}`** - Component instance metadata (name, namespace, labels)

   - Source: Component CR metadata

2. **`${spec.*}`** - Component spec parameters

   - Source: Component CR spec (from generated CRD schema)
   - Includes: parameters, envOverrides, and platform-injected fields

3. **`${build.*}`** - Build context (image, tag, etc.)

   - Source: Platform-injected global variable
   - Resolved from `buildRef` at runtime

4. **`${workload.*}`** - Application workload metadata

   - Source: `workload.yaml` from developer's source repository
   - Contains: endpoints, resource requirements, health checks, etc.
   - `${workload.endpoints}` is the primary way to access endpoint configurations in templates

### Developer's Source Repository

Developers include a `workload.yaml` file in their source repository that defines application metadata (endpoints, resource requirements, health checks, etc.). See the Developer Workflow section below for a complete example.

---

## Platform Engineer Defines ComponentDefinition

Platform Engineer creates a ComponentDefinition with CEL templates that reference the context variables:

```yaml
apiVersion: platform/v1alpha1
kind: ComponentDefinition
metadata:
  name: web-app
schema:
  classVersion: "1" # a simple versioning system for PEs
  envOverrides:
    # both schemas merged for the CRD schema generation
    maxReplicas: number | default=1
    rollingUpdate:
      maxSurge: number | default=1 maximum=5
  parameters:
    scaleToZero:
      pendingRequests: number | maximum=100
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${metadata.name}
        spec:
          replicas: ${spec.maxReplicas}
          strategy:
            type: RollingUpdate
            rollingUpdate:
              maxSurge: ${spec.rollingUpdate.maxSurge}
              maxUnavailable: 1
          selector:
            matchLabels:
              app: ${metadata.name}
          template:
            metadata:
              labels:
                app: ${metadata.name}
            spec:
              containers:
                - name: app
                  image: ${build.image}  # Platform-injected from build context
                  ports: ${workload.endpoints.map(e, {"containerPort": e.port})}  # From workload.yaml

                - name: cloud-sql-proxy
                  image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.8.0
                  args:
                    - "--structured-logs"
                    - "--port=5432"
                  securityContext:
                    runAsNonRoot: true
                  resources:
                    requests:
                      memory: "256Mi"
                      cpu: "100m"

    - id: service
      template:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${metadata.name}-svc
        spec:
          selector:
            app: ${metadata.name}
          ports: ${workload.endpoints.map(e, {"name": e.name, "port": e.port, "targetPort": e.port, "protocol": e.protocol == "udp" ? "UDP" : "TCP"})}  # From workload.yaml

    - id: public-ingress
      forEach: ${workload.endpoints.filter(e, e.visibility == "public")}  # From workload.yaml
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
                    pathType: Prefix
                    backend:
                      service:
                        name: ${metadata.name}-svc
                        port:
                          number: ${item.port}

    - id: http-scaler
      template:
        apiVersion: http.keda.sh/v1alpha1
        kind: HTTPScaledObject
        metadata:
          name: ${metadata.name}
        spec:
          hosts:
            - ${workload.endpoints.filter(e, e.visibility == "public").map(e, e.host)}
          pathPrefixes:
            - ${workload.endpoints.filter(e, e.visibility == "public").map(e, e.path)}
          scaleTargetRef:
            name: ${metadata.name}
            kind: Deployment
            apiVersion: apps/v1
          replicas:
            min: ${spec.scaleToZero.pendingRequests > 0 ? 0 : 1}  # From Component spec
            max: ${spec.maxReplicas}  # From Component spec
          scalingMetric:
            requestRate:
              granularity: 1s
              targetValue: ${spec.scaleToZero.pendingRequests}
              window: 1m
```

---

## Generated CRD Schema

The platform generates a CRD for developers based on the ComponentDefinition schema. The generated CRD:

- Contains both `parameters` and `envOverrides` from ComponentDefinition
- Includes platform-injected field: `buildRef`
- Developers interact with this CRD, not the raw ComponentDefinition
- Endpoints come from `workload.yaml` and are NOT part of the Component CR spec

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "apiVersion": {
      "type": "string",
      "const": "platform/v1alpha1"
    },
    "kind": {
      "type": "string",
      "const": "WebApp"
    },
    "metadata": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string"
        },
        "namespace": {
          "type": "string"
        }
      },
      "required": ["name"]
    },
    "spec": {
      "type": "object",
      "properties": {
        "buildRef": {
          "type": "object",
          "properties": {
            "name": {
              "type": "string"
            }
          },
          "required": ["name"],
          "description": "Reference to RepositoryBuild (auto-injected by platform)"
        },
        "maxReplicas": {
          "type": "integer",
          "default": 1,
          "minimum": 0,
          "description": "Maximum number of replicas for the web service"
        },
        "rollingUpdate": {
          "type": "object",
          "properties": {
            "maxSurge": {
              "type": "integer",
              "default": 1,
              "minimum": 0,
              "maximum": 5,
              "description": "Maximum number of pods that can be created over the desired replica count during an update"
            }
          }
        },
        "scaleToZero": {
          "type": "object",
          "properties": {
            "pendingRequests": {
              "type": "integer",
              "maximum": 100,
              "minimum": 0,
              "description": "Maximum number of pending requests before scaling up from zero"
            }
          }
        }
      },
      "required": ["buildRef"]
    }
  },
  "required": ["apiVersion", "kind", "metadata", "spec"]
}
```

---

## Developer Workflow

### Step 1: Developer's Source Repository

Developer creates `workload.yaml` in their source repo:

```yaml
# myapp/workload.yaml
endpoints:
  - name: api
    port: 8080
    path: /api
    visibility: public
    protocol: http

resources:
  requests:
    cpu: 100m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi

healthCheck:
  liveness:
    path: /health
    port: 8080
  readiness:
    path: /ready
    port: 8080
```

### Step 2: Developer Defines Repository Build

```yaml
apiVersion: platform/v1alpha1
kind: RepositoryBuildConfiguration
metadata:
  name: customer-portal-build
  namespace: my-project
spec:
  repository:
    url: https://github.com/myorg/customer-portal
    revision:
      branch: main
      tag: ""
      commit: ""
    appPath: .
    credentials: github-secret

  templateRef:
    name: docker
    parameters:
      - name: docker-context
        value: .
      - name: dockerfile-path
        value: ./Dockerfile
```

### Step 3: Developer Instantiates Component

The platform automatically extracts `endpoints` from `workload.yaml` and injects into the Component spec:

```yaml
apiVersion: platform/v1alpha1
kind: WebAppComponent
metadata:
  name: customer-portal
  namespace: my-project
spec:
  # Platform auto-injected fields
  buildRef:
    name: customer-portal-build

  # Fields from envOverrides (can be customized)
  maxReplicas: 3
  rollingUpdate:
    maxSurge: 2

  # Fields from parameters
  scaleToZero:
    pendingRequests: 50
```

**Note:** Endpoints come from `workload.yaml` and are NOT part of the Component CR spec. They are available to templates via `${workload.endpoints}`.

### Step 4: Environment-Specific Overrides

Developer/PE creates EnvBinding for production environment:

```yaml
apiVersion: platform/v1alpha1
kind: EnvBinding
metadata:
  name: customer-portal-prod-binding
  namespace: my-project
spec:
  owner:
    componentName: customer-portal
    namespace: my-project
  environment: production

  # Override envOverrides fields (maxReplicas, rollingUpdate)
  overrides:
    maxReplicas: 10
    rollingUpdate:
      maxSurge: 5

  # Environment-specific endpoint hosts (only name + host)
  endpoints:
    - name: api
      host: api.production.example.com

    - name: admin  # Additional endpoint for prod only
      host: admin.production.example.com
```

**Note:** EnvBinding can override:

- `envOverrides` fields (like `maxReplicas`)
- `endpoints` - only endpoint name and host mapping (port, path, visibility, protocol come from workload.yaml)
- Cannot override `parameters` (those are static across all environments)

---

## Environment Promotion Workflow

1. **Automatic Application**: Changes to Component or ComponentDefinition automatically apply to the first environment (e.g., development)

2. **Controlled Promotion**: For subsequent environments, changes require explicit promotion:

   **Via CLI/API:**
   ```bash
   oc promote component customer-portal --from development --to staging
   ```
   - Creates ComponentEnvSnapshot capturing current state
   - Updates EnvBinding with snapshot reference

   **Via GitOps:**
   ```bash
   # Step 1: Manually create snapshot
   oc snapshot create customer-portal --environment staging
   # Returns: customer-portal-staging-20250102-001

   # Step 2: Update EnvBinding in Git with snapshot reference
   ```
   ```yaml
   apiVersion: platform/v1alpha1
   kind: EnvBinding
   metadata:
     name: customer-portal-staging
   spec:
     owner:
       componentName: customer-portal
     environment: staging

     componentEnvSnapshotRef:
       name: customer-portal-staging-20250102-001  # ← Must reference snapshot

     overrides:
       maxReplicas: 5
   ```

3. **Snapshot Isolation**: Each environment runs with its captured snapshot, preventing unexpected propagation of changes

---

## EnvBinding CRD Schema

```json
{
  "type": "object",
  "properties": {
    "owner": {
      "type": "object",
      "properties": {
        "componentName": {
          "type": "string",
          "description": "Name of the component to bind to environment"
        },
        "namespace": {
          "type": "string",
          "description": "Namespace of the component"
        }
      },
      "required": ["componentName", "namespace"]
    },
    "environment": {
      "type": "string",
      "enum": ["development", "staging", "production"],
      "description": "Target environment for this binding"
    },
    "overrides": {
      "type": "object",
      "additionalProperties": true,
      "description": "Environment-specific overrides for component fields. Structure depends on the ComponentDefinition schema"
    },
    "endpoints": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "description": "Endpoint name (must match an endpoint from workload.yaml)"
          },
          "host": {
            "type": "string",
            "description": "Environment-specific host for this endpoint"
          }
        },
        "required": ["name", "host"]
      },
      "description": "Environment-specific host mappings for endpoints (port, path, visibility, protocol come from workload.yaml)"
    },
    "componentEnvSnapshotRef": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string",
          "description": "Name of the ComponentEnvSnapshot resource"
        }
      },
      "required": ["name"],
      "description": "Reference to the immutable component environment snapshot"
    }
  },
  "required": ["owner", "environment"],
  "additionalProperties": false
}
```

---

## ComponentEnvSnapshot Resource

Immutable resource that captures component configuration state for an environment:

```yaml
apiVersion: platform/v1alpha1
kind: ComponentEnvSnapshot
metadata:
  name: customer-portal-prod-20250101-001
  namespace: my-project
  labels:
    component: customer-portal
    environment: production
spec:
  component:
    name: customer-portal
    namespace: my-project

  environment: production

  # Complete snapshot of ComponentDefinition
  componentDefinitionSnapshot:
    apiVersion: platform/v1alpha1
    kind: ComponentDefinition
    # ... complete ComponentDefinition spec

  # Complete snapshot of Component
  componentSnapshot:
    apiVersion: platform/v1alpha1
    kind: WebAppComponent
    # ... complete Component spec
```

### ComponentEnvSnapshot JSON Schema

```json
{
  "type": "object",
  "properties": {
    "component": {
      "type": "object",
      "properties": {
        "name": {
          "type": "string"
        },
        "namespace": {
          "type": "string"
        }
      },
      "required": ["name", "namespace"]
    },
    "environment": {
      "type": "string",
      "enum": ["development", "staging", "production"],
      "description": "Environment this snapshot applies to"
    },
    "componentDefinitionSnapshot": {
      "type": "object",
      "description": "Complete snapshot of ComponentDefinition"
    },
    "componentSnapshot": {
      "type": "object",
      "description": "Complete snapshot of Component spec"
    }
  },
  "required": [
    "component",
    "environment",
    "componentDefinitionSnapshot",
    "componentSnapshot"
  ]
}
```

---

## Summary: Data Flow

### 1. At Component Creation Time

```
Developer's Source Repo (workload.yaml)
        ↓
   [Platform Extracts]
        ↓
Component CR (spec.endpoints auto-populated)
        +
Component CR (spec.parameters, spec.envOverrides)
        +
Platform Globals (build.*, env.*)
        ↓
   [CEL Template Evaluation]
        ↓
Generated K8s Resources
```

### 2. CEL Template Context Sources

| Variable                              | Source                                | Example                            |
| ------------------------------------- | ------------------------------------- | ---------------------------------- |
| `${metadata.name}`                    | Component CR metadata                 | `customer-portal`                  |
| `${spec.maxReplicas}`                 | Component CR spec (envOverride)       | `3`                                |
| `${spec.scaleToZero.pendingRequests}` | Component CR spec (parameter)         | `50`                               |
| `${workload.endpoints}`               | Developer's workload.yaml             | `[{name: "api", port: 8080, ...}]` |
| `${build.image}`                      | Platform-injected, resolved from buildRef | `gcr.io/project/app:v1.2.3`        |

### 3. Key Points

1. **workload.yaml is the source of truth** for application metadata (endpoints, resources, health checks)
2. **Endpoints are NOT in Component CR** - they come exclusively from workload.yaml
3. **EnvBinding.endpoints** - only endpoint name + host mapping per environment
4. **Port, path, visibility, protocol** come from workload.yaml and cannot be overridden
5. **CEL templates access** endpoints via `${workload.endpoints}` merged with `${env.endpoints}`
6. **Platform globals** (`build.*`) are injected at runtime by the controller

### 4. Benefits

- **Single source of truth**: workload.yaml travels with the code
- **Convention over configuration**: Endpoints defined once in workload.yaml
- **Minimal environment config**: Only specify what varies (hosts) via EnvBinding
- **Environment-aware**: Different hosts per environment, other endpoint attributes stay consistent
- **Type-safe**: CEL expressions validated against schema
