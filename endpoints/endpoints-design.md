# Endpoints Design: Two Approaches

## Context

Endpoints expose component workloads externally. They have multiple concerns:
- **Code-bound**: What ports the application listens on, API schemas (OpenAPI, gRPC proto)
- **Infrastructure**: How to expose (Ingress, HTTPRoute), routing rules (path, host)
- **Environment-specific**: Different hosts/visibility per environment

We need to decide where developers define endpoints and how they flow through the system.

---

## EnvSettings Resource

Both approaches use **EnvSettings** as the single resource per component+environment that stores:

1. **Promoted state** (updated during promotion):
   - `image`: Container image from previous environment
   - `endpoints`: Endpoint configuration (from workload.yaml or Component spec)
   - `configSchema`: Application configuration schema
   - Other promotable configurations

2. **Environment-specific overrides**:
   - `overrides`: Component parameter overrides (resources, scaling, etc.)
   - `endpointOverrides`: Endpoint-specific overrides (host, visibility)
   - `addonOverrides`: Addon configuration overrides

**Key design decisions:**
- **No versioned snapshots**: GitOps users have git history; non-GitOps users promote in-place
- **Single resource per component+environment**: Simple, predictable structure
- **Promotion updates EnvSettings**: Copies promoted state from previous env's EnvSettings

---

## Approach 1: Endpoints in Source Repo (workload.yaml)

### Model

**Developer writes workload.yaml in source repo:**
```yaml
# myapp/workload.yaml
endpoints:
  - name: web
    port: 8080
    path: "/"
    type: webapp
    schemaPath: ./openapi/api.yaml

  - name: api
    port: 8080
    path: "/api"
    type: managed-api
    visibility: public
    schemaPath: ./openapi/backend.yaml
```

**Component spec (no endpoints field):**
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Component
metadata:
  name: checkout-service
spec:
  componentType: web-app

  parameters:
    lifecycle:
      terminationGracePeriodSeconds: 60

  build:
    repository:
      url: https://github.com/myorg/checkout-service
      revision:
        branch: main
```

**ComponentTypeDefinition template:**
```yaml
spec:
  resources:
    - id: service
      template:
        apiVersion: v1
        kind: Service
        spec:
          ports: |
            ${workload.endpoints.map(e, {"name": e.name, "port": e.port})}
```

**EnvSettings (stores promoted state + environment-specific overrides):**
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: EnvSettings
metadata:
  name: checkout-service-prod
spec:
  componentRef: checkout-service
  environment: production

  # Promoted state (copied from staging during promotion)
  image: gcr.io/project/checkout:abc123
  endpoints:
    - name: web
      port: 8080
      path: "/"
      type: webapp
      schemaPath: ./openapi/api.yaml
    - name: api
      port: 8080
      path: "/api"
      type: managed-api
      visibility: public
      schemaPath: ./openapi/backend.yaml
  configSchema: {...}

  # Environment-specific overrides
  endpointOverrides:
    web:
      host: checkout.example.com  # PE-assigned readable URL
    api:
      host: api.example.com
      visibility: internal  # Lock down in prod
```

### Flow

**Deploy to Dev:**
1. Developer commits code + workload.yaml to branch
2. Triggers build in OpenChoreo
3. Build system:
   - Builds container image
   - Extracts workload.yaml from git commit
   - Deploys to Dev environment
4. ComponentTypeDefinition templates render using `${workload.endpoints.*}` and `${build.image}`
5. K8s resources created with auto-generated hosts (e.g., `checkout-service-web-dev.example.com`)

**Promotion to Production:**
1. Promote from Dev → Staging → Production
2. For each promotion, OpenChoreo copies from previous environment's EnvSettings to target environment's EnvSettings:
   - Container image
   - Endpoints (from workload.yaml extracted at build time)
   - configSchema
   - Promotable configurations
3. Target environment's EnvSettings `endpointOverrides` section overrides environment-specific fields (custom hosts, visibility changes)
4. ComponentTypeDefinition templates render using promoted endpoints + overrides
5. K8s resources deployed to target environment

### Pros

✅ **Version control alignment**: Endpoints live with code, same git history

✅ **Schema coupling**: Schema file paths (OpenAPI, proto) naturally reference files in same repo

✅ **Branch-based development**: Different branches can have different endpoint configs for experimentation

✅ **No manual sync**: Developer changes endpoint once (workload.yaml), no need to update Component

✅ **Code commit binding**: Endpoint config tied to specific commit, just like the container image

✅ **Promotion simplicity**: Promoting a Component automatically picks up the correct workload.yaml for that git revision

### Cons

❌ **"Magic" injection**: Endpoints aren't visible in Component spec, need to check source code

❌ **Reduced visibility**: PE must look at source repo to understand full component config

❌ **Split configuration**: Component config split across Component YAML + source repo

❌ **Tighter coupling**: Can't promote Component config independently of code changes

❌ **Build-time dependency**: Must access git repo during deployment (can't deploy from just Component YAML)

---

## Approach 2: Endpoints in Component Spec

### Model

**Component spec (with endpoints field):**
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Component
metadata:
  name: checkout-service
spec:
  componentType: web-app

  parameters:
    lifecycle:
      terminationGracePeriodSeconds: 60

  endpoints:
    - name: web
      port: 8080
      path: "/"
      type: webapp
      schemaPath: ./openapi/api.yaml

    - name: api
      port: 8080
      path: "/api"
      type: managed-api
      visibility: public
      schemaPath: ./openapi/backend.yaml

  build:
    repository:
      url: https://github.com/myorg/checkout-service
      revision:
        branch: main
```

**ComponentTypeDefinition template:**
```yaml
spec:
  resources:
    - id: service
      template:
        apiVersion: v1
        kind: Service
        spec:
          ports: |
            ${spec.endpoints.map(e, {"name": e.name, "port": e.port})}
```

**EnvSettings (stores promoted state + environment-specific overrides):**
```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: EnvSettings
metadata:
  name: checkout-service-prod
spec:
  componentRef: checkout-service
  environment: production

  # Promoted state (copied from staging during promotion)
  image: gcr.io/project/checkout:abc123
  endpoints:
    - name: web
      port: 8080
      path: "/"
      type: webapp
      schemaPath: ./openapi/api.yaml
    - name: api
      port: 8080
      path: "/api"
      type: managed-api
      visibility: public
      schemaPath: ./openapi/backend.yaml
  configSchema: {...}

  # Environment-specific overrides
  endpointOverrides:
    web:
      host: checkout.example.com
    api:
      host: api.example.com
      visibility: internal
```

### Flow

**Deploy to Dev:**
1. Developer writes Component YAML with endpoints array
2. Triggers build in OpenChoreo
3. Build system:
   - Builds container image
   - Deploys to Dev environment with Component's endpoint config
4. ComponentTypeDefinition templates render using `${spec.endpoints.*}` and `${build.image}`
5. K8s resources created with auto-generated hosts

**Promotion to Production:**
1. Promote from Dev → Staging → Production
2. For each promotion, OpenChoreo copies from previous environment's EnvSettings to target environment's EnvSettings:
   - Container image
   - Endpoints (from Component spec)
   - configSchema
   - Promotable configurations
3. Target environment's EnvSettings `endpointOverrides` section overrides environment-specific fields (custom hosts, visibility changes)
4. ComponentTypeDefinition templates render using promoted endpoints + overrides
5. K8s resources deployed to target environment

### Pros

✅ **Explicit declaration**: All component config visible in one place

✅ **PE visibility**: Platform engineers can see full config without checking source code

✅ **No magic**: Clear what gets deployed from reading Component YAML

✅ **Independent promotion**: Can promote Component config changes without code changes

✅ **Simpler deployment**: No need to extract from git repo during deployment

✅ **Validation**: Can validate endpoint config at Component creation time

### Cons

❌ **Manual synchronization**: Developer must keep Component endpoints in sync with application code

❌ **Out-of-sync risk**: Easy for Component to say port 8080 while app listens on 9090

❌ **Duplication**: Port/schema info duplicated between code and Component

❌ **Schema path coupling**: schemaPath still references source repo, but endpoint config separated

❌ **Change overhead**: Endpoint changes require updating Component YAML + code

❌ **Git history split**: Endpoint config changes tracked separately from code changes

---

## Comparison Matrix

| Aspect | Approach 1: workload.yaml | Approach 2: Component spec |
|--------|---------------------------|----------------------------|
| **Visibility** | Need to check source repo | Single Component YAML |
| **Sync effort** | Automatic | Manual (developer must sync) |
| **Version control** | Coupled with code | Separate from code |
| **Schema file paths** | Natural (same repo) | References external repo |
| **Promotion** | Promotes with code | Independent promotion |
| **Branch workflows** | Each branch can differ | Shared across branches |
| **Out-of-sync risk** | Low | High |
| **PE experience** | Must check source | Full visibility |
| **Developer experience** | Single source of truth | Duplicate config |
| **Deploy complexity** | Git extraction required | Direct from Component |

---

## Decision Factors

### Choose Approach 1 (workload.yaml) if:

- **Schema files are critical**: OpenAPI specs, proto files must be version-controlled with code
- **Branch-based development**: Teams work on multiple branches with different endpoint configs
- **Avoiding duplication**: Don't want developers to maintain endpoints in two places
- **Atomic deployments**: Want endpoint config tied to specific git commits like container images

### Choose Approach 2 (Component spec) if:

- **Configuration visibility**: PEs need to see full config without accessing source repos
- **Independent promotion**: Need to promote config changes without code changes
- **Simpler deployment**: Don't want git repo dependency during deployment
- **GitOps-style**: Want Component resources as single source of truth

---

## Recommendation

**Approach 1 (workload.yaml)** aligns better with the overall design because:

1. **Consistency with build context**: We already inject `${build.image}` from the build process. Injecting `${workload.endpoints}` follows the same pattern.

2. **Schema file binding**: API schemas (OpenAPI, proto) **must** live in source code. Separating endpoint config from schemas breaks this natural coupling.

3. **Single source of truth**: Developer defines endpoint once, no sync burden.

4. **Promotion safety**: Promoting a Component to production automatically uses the correct workload.yaml for that git revision (same as using the correct container image).

However, we should:

- **Make injection explicit**: Document clearly that `${workload.*}` comes from source repo
- **Validate at build time**: Fail fast if workload.yaml is malformed
- **PE tooling**: Provide CLI tools for PEs to inspect workload.yaml from Component references

---

## Hybrid Alternative (Future Consideration)

Split endpoint concerns:

**Code-bound** (workload.yaml):
```yaml
endpoints:
  - name: api
    port: 8080
    protocol: http
    schemaPath: ./openapi/api.yaml
```

**Infrastructure** (Component spec):
```yaml
spec:
  endpointConfig:
    api:
      type: managed-api
      path: "/api"
```

This gives explicit visibility to infrastructure config while keeping code-bound metadata in the source repo. However, this adds complexity and still requires matching endpoint names between the two locations.
