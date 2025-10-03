# Addon Examples

This document provides concrete examples of different types of addons and how they work.

## Example 1: Volume Addon

### Use Case
Developers want to provision persistent storage for their workloads.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: persistent-volume
  labels:
    category: storage
    version: "1.0"
spec:
  displayName: "Persistent Volume"
  description: "Adds a persistent volume to your workload with configurable size, storage class, and mount path"
  icon: "storage"

  schema:
    # Static parameters - same across all environments
    parameters:
      volumeName: string | required=true pattern="^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"
      mountPath: string | required=true pattern="^/.*"
      containerName: string | default=app
      subPath: string | default=""

    # Environment-specific overrides
    envOverrides:
      size: string | default=10Gi pattern="^[0-9]+[EPTGMK]i$"
      storageClass: string | default=standard enum="standard,fast,premium"
      accessMode: string | default=ReadWriteOnce enum="ReadWriteOnce,ReadWriteMany,ReadOnlyMany"

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

```

### Usage

When developer adds this addon to a Component:

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  parameters:
    replicas: 3

  addons:
    - name: persistent-volume
      instanceId: app-data  # Required for all addon instances
      config:
        volumeName: data
        mountPath: /app/data
        containerName: app
        size: 50Gi
        storageClass: fast

  build:
    image: gcr.io/project/customer-portal:v1.2.3
```

---

## Example 2: Logging Sidecar Addon

### Use Case
Developers can configure log forwarding for their applications.

### Addon Definition

```yaml
apiVersion: platform/v1alpha1
kind: Addon
metadata:
  name: logging-sidecar
  labels:
    category: observability
    version: "1.0"
spec:
  displayName: "Logging Sidecar"
  description: "Injects a Fluent Bit sidecar for log forwarding"
  icon: "log"

  schema:
    # Static parameters
    parameters:
      enabled: boolean | default=true

    # Environment-specific overrides
    envOverrides:
      logLevel: string | default=info enum="debug,info,warn,error"
      outputDestination: string | default=elasticsearch.logging.svc:9200
      resources:
        memory: string | default=128Mi
        cpu: string | default=100m

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
```

### Usage

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  addons:
    - name: logging-sidecar
      instanceId: logger
      config:
        enabled: true
        logLevel: info
        outputDestination: elasticsearch.logging.svc:9200

  build:
    image: gcr.io/project/customer-portal:v1.2.3
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
    parameters:
      configs: "[]object"
        name: string | required=true
        type: string | default=configmap enum="configmap,secret"
        mountPath: string | required=true
        containerName: string | default=app
        files: "[]object"
          fileName: string | required=true
          content: string | required=true

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
```

### Usage

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  addons:
    - name: config-files
      instanceId: app-config
      config:
        configs:
          - name: app-config
            type: configmap
            mountPath: /etc/config
            containerName: app
            files:
              - fileName: app.yaml
                content: |
                  server:
                    port: 8080

  build:
    image: gcr.io/project/customer-portal:v1.2.3
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
    parameters:
      allowIngress: "[]object"
        from: string | required=true
        ports: "[]integer"
      allowEgress: "[]object"
        to: string | required=true
        ports: "[]integer"
      denyAll: boolean | default=false

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
```

### Usage

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  addons:
    - name: network-policy
      instanceId: netpol
      config:
        denyAll: true
        allowIngress:
          - from: "namespace:ingress"
            ports: [8080]

  build:
    image: gcr.io/project/customer-portal:v1.2.3
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
    envOverrides:
      containers: "[]object"
        name: string | required=true
        requests:
          cpu: string | default=100m pattern="^[0-9]+m?$"
          memory: string | default=128Mi pattern="^[0-9]+[EPTGMK]i$"
        limits:
          cpu: string | default=500m pattern="^[0-9]+m?$"
          memory: string | default=512Mi pattern="^[0-9]+[EPTGMK]i$"

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
```

### Usage

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  addons:
    - name: resource-limits
      instanceId: limits
      config:
        containers:
          - name: app
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 1000m
              memory: 1Gi

  build:
    image: gcr.io/project/customer-portal:v1.2.3
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
    parameters:
      issuer: string | default=letsencrypt-prod
      domains: "[]string" | required=true minItems=1
      ingressName: string | required=true queryResources=Ingress

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
```

### Usage

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  addons:
    - name: tls-certificate
      instanceId: tls
      config:
        issuer: letsencrypt-prod
        domains:
          - customer.example.com
          - www.customer.example.com
        ingressName: main-ingress

  build:
    image: gcr.io/project/customer-portal:v1.2.3
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
    parameters:
      initContainers: "[]object"
        name: string | required=true
        image: string | required=true
        command: "[]string"
        args: "[]string"
        volumeMounts: "[]object"
          name: string | required=true
          mountPath: string | required=true

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
```

### Usage

```yaml
apiVersion: platform/v1alpha1
kind: Component
metadata:
  name: customer-portal
spec:
  componentType: web-app

  addons:
    - name: init-container
      instanceId: db-migration
      config:
        initContainers:
          - name: db-migrate
            image: gcr.io/project/db-migrator:latest
            command: ["/bin/sh"]
            args: ["-c", "migrate up"]
            volumeMounts:
              - name: migrations
                mountPath: /migrations

  build:
    image: gcr.io/project/customer-portal:v1.2.3
```
