# Patch Capabilities vs. Kustomize

This document captures how the renderer2 patch engine differs from the patching options provided by Kustomize (and the underlying kyaml library).

## Summary

| Capability | renderer2 `patch` package | Kustomize `patchesJson6902` (JSON Patch) | Kustomize `patchesStrategicMerge` | Notes |
|------------|--------------------------|------------------------------------------|-----------------------------------|-------|
| Basic add/replace/remove | ✅ | ✅ (RFC 6902) | ✅ (merge entire resource) | Behaviour matches JSON Patch. |
| Append to array with trailing `-` | ✅ | ✅ | ⚠️ (requires strategic-merge map semantics) | Both engines follow RFC 6902. |
| Filter array entries (`[?(@.name=='app')]`) | ✅ | ❌ | ❌ | Kustomize expects plain JSON Pointer segments. |
| Deep merge of map at arbitrary path | ✅ (`op: merge`) | ❌ | ⚠️ (only via whole-resource strategic merge) | Strategic merge rewrites entire resource unless trimmed carefully. |
| Add/merge while preserving developer-supplied keys | ✅ | ❌ (must resend entire map) | ⚠️ (needs full resource context) | Renderer2 specifies just the keys to merge. |
| Custom selectors (`target.ResourceType`, future label selectors) | ✅ | ❌ | ❌ | Kustomize patches apply to a single resource chosen by user-provided metadata or file ordering. |
| CEL-driven path/value evaluation | ✅ | ❌ | ❌ | Renderer2 paths and values can contain `${…}` expressions. |
| Schema-aware defaults injected before patch applies | ✅ | ❌ | ❌ | Kustomize relies on raw YAML values. |

## JSON Patch (`patchesJson6902`)

Kustomize’s JSON patch support is a thin wrapper over RFC 6902: paths are JSON Pointers (`/metadata/name`, `/spec/template/spec/containers/0/image`), operations are limited to `add`, `remove`, `replace`, `move`, `copy`, `test`.

**Limitations vs renderer2:**

- No array filters – you cannot select an element by field value; you must refer to it by index.
- No partial map updates – to add a single annotation you must resend the entire `metadata.annotations` map.
- No `merge` operation – maps are replaced wholesale.
- No expression support – literals only.

**Useful when:** you know exact indices/keys and are happy to replace the full value.

## Strategic Merge Patch (`patchesStrategicMerge`)

Strategic merge patches allow Kustomize to merge a patch document into a live resource using Kubernetes type metadata (e.g., merging Deployment specs). It does support map merging and list patching for certain list types (keyed by `name`, etc.).

**Limitations vs renderer2:**

- Operates on entire resources. There is no notion of “merge only the map at this JSON pointer”.
- Requires knowing the Kubernetes schema so the patch matches the API shape; it is not generic across arbitrary CRDs without additional configuration.
- Array merging is type-specific and cannot use arbitrary filters.
- No direct support for CEL expressions or addon instance data; patches are static YAML.

**Useful when:** you can author overlays that mirror the resource structure and leverage Kubernetes-style merge keys.

## kyaml filters (Go API)

kyaml exposes lower-level filters (`yaml.Lookup`, `yaml.SetField`, JSON6902 patches, etc.) that Kustomize builds on. These follow the same constraints:

- JSON6902 filters accept standard JSON Pointers only.
- Strategic merge relies on Kubernetes resource metadata.
- No built-in helper for “merge this map literal into an arbitrary location” with array filtering.

## Why renderer2 keeps a custom patcher

Renderer2 needs to:

1. Apply patches expressed in templates (paths/values can be CEL expressions).
2. Target nested data using array filters (`[?(@.name=='app')]`) to support multiple addon instances.
3. Merge maps in place so addons can compose without replacing previous keys.
4. Run independently of Kubernetes type metadata (works on arbitrary `map[string]interface{}` generated from templates).

Existing libraries cover pieces of this but not the combination. Until a general-purpose library supports filtered paths + deep merge, the bespoke patcher gives us the fidelity we need.
