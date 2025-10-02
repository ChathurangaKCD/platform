#### Platform engineer defines component type

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
                  image: ${build.image}  # Resolved from buildRef at runtime
                  ports: ${spec.endpoints.map(e, {"containerPort": e.port})}

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
          ports: ${spec.endpoints.map(e, {"name": e.name, "port": e.port, "targetPort": e.port, "protocol": e.protocol == "udp" ? "UDP" : "TCP"})}

    - id: public-ingress
      forEach: ${spec.endpoints.filter(e, e.visibility == "public")}
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
            - ${spec.endpoints.filter(e, e.visibility == "public").map(e, e.host)}
          pathPrefixes:
            - ${spec.endpoints.filter(e, e.visibility == "public").map(e, e.path)}
          scaleTargetRef:
            name: ${metadata.name}
            kind: Deployment
            apiVersion: apps/v1
          replicas:
            min: ${spec.scaleToZero.pendingRequests > 0 ? 0 : 1}
            max: ${spec.maxReplicas}
          scalingMetric:
            requestRate:
              granularity: 1s
              targetValue: ${spec.scaleToZero.pendingRequests}
              window: 1m
```

#### Generated CRD json schema

- contains both parameters and envOverrides.
- contains platform injected build ref and endpoints fields

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
        "endpoints": {
          "type": "array",
          "items": {
            "$ref": "https://schemas.platform.io/v1alpha1/Endpoint.json"
          },
          "description": "Array of endpoint configurations (auto-injected by platform)"
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
      "required": ["buildRef", "endpointsRef"]
    }
  },
  "required": ["apiVersion", "kind", "metadata", "spec"]
}
```

#### Developer defines repository build

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

#### Developer instantiates component

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
  endpoints:
    - name: api
      port: 8080
      path: /api
      host: api.example.com
      visibility: public
      protocol: http

  # Fields from envOverrides (can be customized)
  maxReplicas: 3
  rollingUpdate:
    maxSurge: 2

  # Fields from parameters
  scaleToZero:
    pendingRequests: 50
```

#### Developer/PE overrides component parameters for individual env

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

  # Component field overrides
  overrides:
    maxReplicas: 10
    rollingUpdate:
      maxSurge: 5

  # Override endpoints for production
  endpoints:
    - name: api
      port: 8080
      path: /api
      host: api.production.example.com
      visibility: public
      protocol: https
    - name: admin
      port: 8080
      path: /admin
      host: admin.production.example.com
      visibility: internal
      protocol: https
```

#### Environment Promotion Workflow

1. **Automatic Application**: Changes to Component or ComponentDefinition automatically apply to the first environment (e.g., development)
2. **Controlled Promotion**: For subsequent environments, changes require explicit promotion via:
   - CLI/API command: `oc promote component customer-portal --from development --to staging`
   - GitOps: Updating EnvBinding in environment-specific Git location
3. **Snapshot Capture**: EnvBinding captures immutable snapshots of both ComponentDefinition and Component at promotion time
4. **Isolation**: Each environment runs with its captured snapshot, preventing unexpected propagation of changes

#### EnvBinding CRD JSON Schema (spec)

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
        "$ref": "https://schemas.platform.io/v1alpha1/Endpoint.json"
      },
      "description": "Environment-specific endpoint configurations"
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

#### ComponentEnvSnapshot Resource

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

#### ComponentEnvSnapshot JSON Schema (spec)

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
