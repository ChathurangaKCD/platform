```yaml
patches:
  # Fast refresh for env secret stores
  - target:
      group: external-secrets.io
      version: v1beta1
      kind: ExternalSecretStore
      where: ${resource.metadata.name.endsWith("-secret-envs")}
      name: env-configs
    operations:
      - op: add
        path: /spec/refreshInterval
        value: "5m"

  # Default refresh interval for other secret stores
  - target:
      group: external-secrets.io
      version: v1beta1
      kind: ExternalSecretStore
      where: ${!resource.metadata.name.endsWith("-secret-envs")}
    operations:
      - op: add
        path: /spec/refreshInterval
        value: "1h"
```

```yaml
patches:
  - forEach: ${spec.mounts}
    target:
      group: apps
      version: v1
      kind: Deployment
    operations:
      - op: add
        path: /spec/template/spec/containers/[?(@.name=='${item.containerName}')]/volumeMounts/-
        value:
          name: ${spec.volumeName}
          mountPath: ${item.mountPath}
          readOnly: ${has(item.readOnly) ? item.readOnly : false}
          subPath: ${has(item.subPath) ? item.subPath : ""}
```

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
    operations:
      - op: add
        path: /spec/template/spec/containers/[?(@.name=='${spec.containerName}')]/volumeMounts/-
        value:
          name: ${spec.volumeName}
          mountPath: ${spec.mountPath}
          readOnly: ${spec.readOnly}
          subPath: ${spec.subPath}
```

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
    operations:
      - op: add
        path: /spec/template/spec/containers/-
        value:
          name: ${spec.name}
          image: ${spec.image}
```

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
    operations:
      - op: mergeShallow
        path: /spec/template/metadata/annotations
        value:
          custom.annotation/foo: foo
          custom.annotation/bar: bar
```
