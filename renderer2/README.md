# renderer2

This module refactors the original experimental renderer into a reusable package-oriented layout.  
It separates generic rendering utilities (CEL template evaluation, context assembly, patch application) from Component-specific orchestration so controllers and CLIs can mix and match the building blocks they need.

## Project structure

```
renderer2/
├── main.go                       # CLI harness using the reusable packages
├── README.md
├── go.mod / go.sum
└── pkg/
    ├── component/                # Component-aware orchestration (staging, addon ordering)
    ├── context/                  # Builders that assemble CEL input contexts from Component, EnvSettings, etc.
    ├── parser/                   # YAML/JSON loader helpers + schema validation
    ├── patch/                    # Path traversal and patch operations
    ├── pipeline/                 # Generic rendering flow (render base ↔ apply addon)
    ├── schema/                   # simpleschema/OpenAPI helpers and default extraction
    ├── template/                 # CEL engine with omit/merge helpers
    └── types/                    # Shared type definitions
```

Key improvements over the first renderer:

- **Reusable layers** – templating and patching packages accept plain `map[string]interface{}` so they can back future controllers.
- **Schema-backed defaults** – `pkg/schema` converts the ComponentTypeDefinition and Addon schemas into OpenAPI, extracts defaults, and feeds them into the rendering inputs.
- **Schema validation CLI** – `main.go` regenerates JSON schemas before rendering to catch malformed templates early.

Running the demo CLI:

```bash
cd renderer2
go run main.go
```

The command re-generates JSON schemas under `renderer/examples/schemas/` and writes rendered manifests to `renderer/examples/expected-output/<env>/`.

## Patch operations

Addons patch already-rendered resources using JSON pointer–like paths with a few extensions (array filters, deep merge). Under the hood, renderer2 delegates the standard JSON Patch verbs—`add`, `replace`, `remove`, `test`, `copy`, and `move`—to the battle-tested [`github.com/evanphx/json-patch`](https://github.com/evanphx/json-patch) implementation; array filters are resolved into concrete JSON Pointer paths before we invoke the library. Merge-style behaviour (`merge` for deep merge, `mergeShallow` for single-level overlays) remains a custom extension implemented inside renderer2. The engine therefore supports the following operations: `add`, `replace`, `remove`, `merge`, `mergeShallow`, `test`, `copy`, and `move`.

### `add`

Delegated to the JSON Patch engine; renderer2 resolves filters and parents, then hands the operation to `github.com/evanphx/json-patch`. Sets or appends a value. If the final path segment is:

- a plain key (`/spec/template/spec/containers/0/image`) – the value is assigned.
- `-` after an array (`/spec/template/spec/containers/-`) – the value is appended.
- an array filter (`/spec/template/spec/containers/[?(@.name=='app')]/env/-`) – the filter selects items before the value is appended.

**Example**: add a new volume mount to the app container.

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
      where: ${resource.metadata.name == metadata.name}
    operations:
      - op: add
        path: /spec/template/spec/containers/[?(@.name=='app')]/volumeMounts/-
        value:
          name: logs
          mountPath: /var/log/app
```

### `replace`

Also delegated to the JSON Patch engine. Same path semantics as `add`, but the target must already exist (otherwise the patch errors). Useful when you know the key is present and want to change just its value.

**Example**: force the first container image tag.

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
      name: ${metadata.name}
    operations:
      - op: replace
        path: /spec/template/spec/containers/0/image
        value: ${spec.forcedImage}
```

### `remove`

Deletes the field or array element at the path. Array filters can be used to drop matching items.

**Example**: remove the pod anti-affinity from a Deployment when a feature flag is disabled.

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
    operations:
      - op: remove
        path: /spec/template/spec/affinity/podAntiAffinity
```

### `merge`

Performs a deep merge of maps. This is a renderer2-specific extension (not part of JSON Patch) that is handy when you need to add or override a handful of keys without rebuilding the entire object—especially for metadata or resource requirements. Paths can include array filters; when they resolve to an object, the merge happens inside that object.

**Example**: standardize annotations without wiping out user-defined ones.

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
      where: ${has(resource.metadata.annotations) && resource.metadata.annotations.exists(k, k.key == "team")}
    operations:
      - op: merge
        path: /spec/template/metadata/annotations
        value:
          security.openchoreo.dev/enforce: "true"
          sidecar.opa.policy: sandbox
```

Another example, merging CPU/memory requests into the default container:

```yaml
patches:
  - target:
      group: apps
      version: v1
      kind: Deployment
    operations:
      - op: merge
        path: /spec/template/spec/containers/[?(@.name=='app')]/resources/requests
        value:
          cpu: 500m
          memory: 512Mi
```

### `mergeShallow`

Overlays keys one level deep without recursing into nested maps. Like `merge`, this is a renderer2-only extension. Values provided in the patch replace the existing value for the same key but leave sibling keys untouched. This is useful for metadata maps (such as annotations) when you want to enforce or override known keys without performing a deep merge.

**Example**: augment pod template annotations while preserving any existing entries.

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

### `test`, `copy`, `move`

Because renderer2 delegates to the standard JSON Patch engine, addons can also use `test`, `copy`, and `move`. A failing `test` aborts the addon with a clear error.

## Iterating with `forEach`

When an addon needs to emit similar operations for every item in a list, `forEach` can bind the current item to `${item}` and repeat the enclosed operations. This keeps CEL logic minimal by letting the patch runner drive iteration.

**Example**: mount each entry from `spec.mounts` into the rendered container.

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

## Array filters

Paths can filter arrays using the syntax `[?(@.field=='value')]`. The filter selects matching objects before the operation applies. For example, `/spec/template/spec/containers/[?(@.name=='app')]/env/-` means “find the container whose `name` equals `app`, then append to its `env` array.”

## Working with defaults

Default values defined in the ComponentTypeDefinition or Addon schema are resolved automatically (via simpleschema ➜ OpenAPI). This guarantees features such as `includeWhen: ${spec.pdbEnabled}` work even when the component doesn’t set `pdbEnabled` explicitly—the default flows into the rendering context.

## Future work

- Additional patch selector syntaxes (e.g., `@.metadata.labels['app']`).
- Pluggable expression matchers for `target.selector`.
- Controller-specific orchestration packages for other OpenChoreo resources (snapshots, releases, etc.).

Contributions and feedback welcome!
