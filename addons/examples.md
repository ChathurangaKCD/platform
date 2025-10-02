# Addon Examples

This document provides concrete examples of different types of addons and how they work.

## Example 1: Volume Addon (PE-Only)

### Use Case
Platform Engineers want to provision persistent storage for workloads. This is PE-only because storage provisioning is an infrastructure concern controlled by the platform team.

### Addon Definition

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
  description: "Adds a persistent volume to your workload with configurable size, storage class, and mount path"
  icon: "storage"

  schema:
    # Static parameters - same across all environments
    parameters:
      volumeName:
        type: string
        description: "Name of the volume"
        pattern: "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

      mountPath:
        type: string
        description: "Path where volume should be mounted in container"
        pattern: "^/.*"

      containerName:
        type: string
        description: "Name of container to mount volume to"
        default: "app"

      subPath:
        type: string
        description: "SubPath within the volume"
        default: ""

    # Environment-specific overrides
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

  # What this addon targets
  targets:
    - resourceType: Deployment
      containers: ["${spec.containerName}"]
    - resourceType: StatefulSet
      containers: ["${spec.containerName}"]

  # Resources this addon creates
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

  # How this addon modifies existing resources
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
          subPath: "${spec.subPath}"

  # UI hints for rendering
  ui:
    formLayout:
      - field: volumeName
        width: half
      - field: size
        width: half
      - field: storageClass
        width: half
      - field: accessMode
        width: half
      - field: containerName
        width: half
        queryContainers: true  # UI should populate from ComponentDefinition
      - field: mountPath
        width: full
      - field: mountPermissions
        width: half
      - field: subPath
        width: half
```

### Usage in UI

When PE selects this addon:
1. UI renders form based on schema
2. "containerName" field shows dropdown of containers from ComponentDefinition
3. UI shows preview: "Will create 1 PVC, modify 1 Deployment"
4. PE fills: `volumeName: data, size: 50Gi, mountPath: /app/data, containerName: app`
5. UI shows diff of what resources will be modified

### Developer Experience

After composition, developer sees this in their CRD:

```yaml
apiVersion: platform/v1alpha1
kind: WebAppComponent
metadata:
  name: customer-portal
spec:
  # ... other fields ...

  # Volume addon parameters injected into schema
  persistentVolume:
    volumeName: data
    size: 50Gi
    storageClass: fast
    mountPath: /app/data
    containerName: app
```

---

## Example 2: Logging Sidecar Addon (Developer-Allowed)

### Use Case
Allow developers to configure log forwarding for their applications. This is developer-allowed because logging configuration is application-specific.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: logging-sidecar
  labels:
    category: observability
    version: "1.0"
    allowedFor: developer  # Developers can opt-in
spec:
  displayName: "Logging Sidecar"
  description: "Injects a Fluent Bit sidecar for log forwarding"
  icon: "log"

  schema:
    # Static parameters
    parameters:
      enabled:
        type: boolean
        default: true
        description: "Enable logging sidecar"

    # Environment-specific overrides
    envOverrides:
      logLevel:
        type: string
        enum: ["debug", "info", "warn", "error"]
        default: "info"
        description: "Log level (dev might use debug, prod uses warn)"

      outputDestination:
        type: string
        description: "Log destination endpoint"
        default: "elasticsearch.logging.svc:9200"

      resources:
        type: object
        properties:
          memory:
            type: string
            default: "128Mi"
          cpu:
            type: string
            default: "100m"

  targets:
    - resourceType: Deployment
    - resourceType: StatefulSet

  patches:
    - target:
        resourceType: [Deployment, StatefulSet]
      patch:
        op: add
        path: /spec/template/spec/containers/-
        value:
          name: fluent-bit
          image: fluent/fluent-bit:2.1
          env:
            - name: LOG_LEVEL
              value: "${spec.logLevel}"
            - name: OUTPUT_DESTINATION
              value: "${spec.outputDestination}"
          resources:
            requests:
              memory: "${spec.resources.memory}"
              cpu: "${spec.resources.cpu}"
            limits:
              memory: "${spec.resources.memory}"
              cpu: "${spec.resources.cpu}"
          volumeMounts:
            - name: varlog
              mountPath: /var/log
              readOnly: true

    - target:
        resourceType: [Deployment, StatefulSet]
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: varlog
          emptyDir: {}

  ui:
    formLayout:
      - field: enabled
        width: full
        type: toggle
      - field: logLevel
        width: half
      - field: outputDestination
        width: full
      - field: resources.memory
        width: half
      - field: resources.cpu
        width: half
```

---

## Example 3: ConfigMap/Secret Addon

### Use Case
Mount configuration files or secrets into containers.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: config-files
  labels:
    category: configuration
    version: "1.0"
spec:
  displayName: "Configuration Files"
  description: "Mount ConfigMaps or Secrets as files in your containers"
  icon: "file-text"

  schema:
    type: object
    properties:
      configs:
        type: array
        items:
          type: object
          properties:
            name:
              type: string
              description: "Config name"

            type:
              type: string
              enum: ["configmap", "secret"]
              default: "configmap"

            mountPath:
              type: string
              description: "Mount path in container"

            containerName:
              type: string
              default: "app"

            files:
              type: array
              description: "Files to include in ConfigMap/Secret"
              items:
                type: object
                properties:
                  fileName:
                    type: string
                  content:
                    type: string
                    format: textarea
                required: ["fileName", "content"]

          required: ["name", "type", "mountPath", "files"]

  targets:
    - resourceType: Deployment
    - resourceType: StatefulSet

  creates:
    - forEach: "${spec.configs.filter(c, c.type == 'configmap')}"
      resource:
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: "${metadata.name}-${item.name}"
        data: "${item.files.reduce((acc, f) => {acc[f.fileName] = f.content; return acc;}, {})}"

    - forEach: "${spec.configs.filter(c, c.type == 'secret')}"
      resource:
        apiVersion: v1
        kind: Secret
        metadata:
          name: "${metadata.name}-${item.name}"
        type: Opaque
        stringData: "${item.files.reduce((acc, f) => {acc[f.fileName] = f.content; return acc;}, {})}"

  patches:
    - forEach: "${spec.configs}"
      target:
        resourceType: [Deployment, StatefulSet]
      patch:
        op: add
        path: /spec/template/spec/volumes/-
        value:
          name: "${item.name}"
          configMap:
            name: "${metadata.name}-${item.name}"
          # Will use secret instead if type == 'secret'

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
          readOnly: true

  ui:
    formLayout:
      - field: configs
        type: array
        addButtonLabel: "Add Configuration"
        itemLayout:
          - field: name
            width: half
          - field: type
            width: half
          - field: containerName
            width: half
            queryContainers: true
          - field: mountPath
            width: half
          - field: files
            type: array
            addButtonLabel: "Add File"
            itemLayout:
              - field: fileName
                width: full
              - field: content
                width: full
                type: code-editor
                language: text
```

---

## Example 4: Network Policy Addon

### Use Case
Add network policies to restrict ingress/egress traffic.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: network-policy
  labels:
    category: security
    version: "1.0"
spec:
  displayName: "Network Policy"
  description: "Define network isolation and traffic rules for your workload"
  icon: "shield"

  schema:
    type: object
    properties:
      allowIngress:
        type: array
        description: "Allowed ingress sources"
        items:
          type: object
          properties:
            from:
              type: string
              description: "Source (namespace, pod selector, or CIDR)"
            ports:
              type: array
              items:
                type: integer

      allowEgress:
        type: array
        description: "Allowed egress destinations"
        items:
          type: object
          properties:
            to:
              type: string
              description: "Destination (namespace, pod selector, or CIDR)"
            ports:
              type: array
              items:
                type: integer

      denyAll:
        type: boolean
        description: "Deny all traffic except explicitly allowed"
        default: false

  creates:
    - apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      metadata:
        name: "${metadata.name}-netpol"
      spec:
        podSelector:
          matchLabels:
            app: "${metadata.name}"
        policyTypes:
          - Ingress
          - Egress
        ingress: "${spec.allowIngress.map(rule, {from: [parseSelector(rule.from)], ports: rule.ports.map(p, {protocol: 'TCP', port: p})})}"
        egress: "${spec.allowEgress.map(rule, {to: [parseSelector(rule.to)], ports: rule.ports.map(p, {protocol: 'TCP', port: p})})}"

  ui:
    formLayout:
      - field: denyAll
        type: toggle
        width: full
      - field: allowIngress
        type: array
        addButtonLabel: "Add Ingress Rule"
      - field: allowEgress
        type: array
        addButtonLabel: "Add Egress Rule"
```

---

## Example 5: Resource Limits Addon

### Use Case
Enforce resource quotas and limits on containers.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: resource-limits
  labels:
    category: resources
    version: "1.0"
spec:
  displayName: "Resource Limits"
  description: "Set CPU and memory requests/limits for containers"
  icon: "cpu"

  schema:
    type: object
    properties:
      containers:
        type: array
        items:
          type: object
          properties:
            name:
              type: string

            requests:
              type: object
              properties:
                cpu:
                  type: string
                  pattern: "^[0-9]+m?$"
                  default: "100m"
                memory:
                  type: string
                  pattern: "^[0-9]+[EPTGMK]i$"
                  default: "128Mi"

            limits:
              type: object
              properties:
                cpu:
                  type: string
                  pattern: "^[0-9]+m?$"
                  default: "500m"
                memory:
                  type: string
                  pattern: "^[0-9]+[EPTGMK]i$"
                  default: "512Mi"

          required: ["name"]

  targets:
    - resourceType: Deployment
    - resourceType: StatefulSet
    - resourceType: Job
    - resourceType: CronJob

  patches:
    - forEach: "${spec.containers}"
      target:
        resourceType: [Deployment, StatefulSet]
        containerName: "${item.name}"
      patch:
        op: add
        path: /spec/template/spec/containers/[?(@.name=='${item.name}')]/resources
        value:
          requests:
            cpu: "${item.requests.cpu}"
            memory: "${item.requests.memory}"
          limits:
            cpu: "${item.limits.cpu}"
            memory: "${item.limits.memory}"

  ui:
    formLayout:
      - field: containers
        type: array
        addButtonLabel: "Add Container Limits"
        itemLayout:
          - field: name
            width: full
            queryContainers: true
          - field: requests.cpu
            width: half
          - field: requests.memory
            width: half
          - field: limits.cpu
            width: half
          - field: limits.memory
            width: half
```

---

## Example 6: TLS/Certificate Addon

### Use Case
Automatically provision TLS certificates and configure Ingress for HTTPS.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: tls-certificate
  labels:
    category: security
    version: "1.0"
spec:
  displayName: "TLS Certificate"
  description: "Provision TLS certificates using cert-manager and configure HTTPS"
  icon: "lock"

  schema:
    type: object
    properties:
      issuer:
        type: string
        description: "cert-manager issuer name"
        default: "letsencrypt-prod"

      domains:
        type: array
        description: "Domain names for certificate"
        items:
          type: string
          format: hostname

      ingressName:
        type: string
        description: "Name of Ingress resource to update"

  targets:
    - resourceType: Ingress

  creates:
    - apiVersion: cert-manager.io/v1
      kind: Certificate
      metadata:
        name: "${metadata.name}-tls"
      spec:
        secretName: "${metadata.name}-tls-secret"
        issuerRef:
          name: "${spec.issuer}"
          kind: ClusterIssuer
        dnsNames: "${spec.domains}"

  patches:
    - target:
        resourceType: Ingress
        resourceId: "${spec.ingressName}"
      patch:
        op: add
        path: /spec/tls/-
        value:
          hosts: "${spec.domains}"
          secretName: "${metadata.name}-tls-secret"

  ui:
    formLayout:
      - field: issuer
        width: full
      - field: domains
        type: array
        addButtonLabel: "Add Domain"
      - field: ingressName
        width: full
        queryResources:
          type: Ingress
```

---

## Example 7: Init Container Addon

### Use Case
Run initialization tasks before main container starts.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: init-container
  labels:
    category: lifecycle
    version: "1.0"
spec:
  displayName: "Init Container"
  description: "Add initialization containers that run before your main application"
  icon: "play-circle"

  schema:
    type: object
    properties:
      initContainers:
        type: array
        items:
          type: object
          properties:
            name:
              type: string

            image:
              type: string

            command:
              type: array
              items:
                type: string

            args:
              type: array
              items:
                type: string

            volumeMounts:
              type: array
              items:
                type: object
                properties:
                  name:
                    type: string
                  mountPath:
                    type: string

          required: ["name", "image"]

  targets:
    - resourceType: Deployment
    - resourceType: StatefulSet

  patches:
    - forEach: "${spec.initContainers}"
      target:
        resourceType: [Deployment, StatefulSet]
      patch:
        op: add
        path: /spec/template/spec/initContainers/-
        value:
          name: "${item.name}"
          image: "${item.image}"
          command: "${item.command}"
          args: "${item.args}"
          volumeMounts: "${item.volumeMounts}"

  ui:
    formLayout:
      - field: initContainers
        type: array
        addButtonLabel: "Add Init Container"
        itemLayout:
          - field: name
            width: half
          - field: image
            width: half
          - field: command
            type: array
            addButtonLabel: "Add Command"
          - field: args
            type: array
            addButtonLabel: "Add Argument"
```
