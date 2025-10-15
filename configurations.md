# Seperating config schema & the mapping from configGroups to configs schema.

- source repo only contains the config schema defining what configs are required & how it should be mounted ( env/mount path)
- how the config group fields are mapped to a particular component is defined at the component level.
  Bit verbose, but this has the following advantages.
- In a gitops scenario, configGroup/secretRef references will be availbale in a single repo/branch (subject to PE practices) instead of scattering configGroupKeyRefs across many source repos.
- PE will use thier own naming conventions for parameters & config groups names. Having those in a single repo means, can be easily renamed/modifed instead of having to send PRs across multiple source repos.

## workload yaml from source repo

```yaml
configurations:
  env: # for validation + defaults
    - name: DEBUG_LEVEL
      value: info # default value provided

    - name: DB_URL
      # no default value
      # not a secret

    - name: DB_PASSWORD
      isSecret: true # mapped to a secret ref
      # no default value

  file: # required to specifiy mount paths
    - name: config.toml
      mountPath: /app/config.toml
      # value provided at runtime

    - name: templates.yaml
      mountPath: /app/templates
      sourcePath: /templates/template.yaml # default from source repo
```

## component resource, commited to gitops repo.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: Component
configurationMappings:
  env:
    # Direct key reference
    - name: DB_PASSWORD
      configGroupKeyRef:
        name: mysql-db-config
        key: password
    # Import entire config group as env vars
    - name: dbConfigs
      valueFrom:
        configGroupRef:
          name: mysql-db-config
  file:
    # Mount specific file from config group
    - name: config.toml
      configGroupKeyRef:
        name: config-file
        key: config.toml
    # Mount entire config group as files
    - name: file-configs
      valueFrom:
        configGroupRef:
          name: file-secret
```

# controller can resolve configGroups values/secretRefs for the relavent environment & make it available in the context under `${configurations}`.

e.g:

```
  configurations: {
    envs: [
      {name, value}
    ],
    files: [
      {mountPath, content}
    ]
  },
  secrets: {
    envs: [
      {name, valueRef}
    ],
    files: [
      {mountPath, valueRef}
    ]
  },
```

Can be included in the final k8s manifest by making it part of the componentTypeDefinition or a addon.

```yaml
apiVersion: openchoreo.dev/v1alpha1
kind: ComponentTypeDefinition
resources:
  - id: deployment
    template:
      # deployment template with envFrom/volMounts etc for below resources
  - id: env-configs
    template:
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: env-app-configs
        namespace: default
      data: ${configurations.envs}
  - id: file-configs
    forEach: ${configurations.files}
    template:
      apiVersion: external-secrets.io/v1
      kind: SecretStore
      metadata:
        name: app-config
        namespace: default
      data: ${item.value}
```
