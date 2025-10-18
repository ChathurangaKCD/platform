# Adapter plan: renderer2 → kyaml JSON Patch

Goal (implemented in `pkg/patch`/`pkg/pipeline`): keep renderer2’s expressive path syntax (CEL-evaluated strings, array filters, merge option, CEL-based targeting) while delegating the actual RFC 6902 work (“add”, “replace”, “remove”, “test”, “copy”, “move”) to a battle-tested library such as `github.com/evanphx/json-patch/v5`.

## Current behaviour to preserve

1. **Array filters** – paths may include `[?(@.field=='value')]`, optionally nested.
2. **Automatic creation of parent objects** when adding into missing maps/arrays (e.g., auto-create `spec.template.spec.volumes` when appending the first volume).
3. **Merge op** – deep merge of maps to avoid clobbering sibling keys.
4. **Multiple matches** – a single path expression may target several items.
5. **CEL-driven values & paths** – path strings have already been evaluated before `ApplyPatch`.

## Proposed flow

```
raw path "/spec/template/spec/containers/[?(@.name=='app')]/env/-"
           │
           ├──> resolve contexts (array filters)
           │       -> "/spec/template/spec/containers/0/env/-"
           │       -> "/spec/template/spec/containers/1/env/-"
           │
           ├──> ensure parent objects/arrays exist (for add)
           │
           └──> build RFC6902 operations, delegate to jsonpatch/kyaml
```

### 1. Path resolver

- Split path on `/`, tracking empty head.
- For each segment:
  - If it contains a filter `[?(@.field=='value')]`, evaluate the filter against the current list value and emit all `(index, value)` pairs that match.
  - If the segment contains an explicit index (`foo[0]`), expand to `foo` + index.
  - Escape keys for JSON Pointer (`/` → `~1`, `~` → `~0`).
- Maintain a slice of “contexts” `(pointer string, current value, parent)`; expand the slice when filters or array indices are encountered.
- Return the list of concrete JSON pointers (and optionally the parent nodes for pre-creation).

### 2. Parent creation (for add)

RFC 6902 requires the parent of the target pointer to exist. To keep previous behaviour:

- For each pointer returned by the resolver:
  - Inspect all but the last segment; create intermediate maps in the resource if missing.
  - For array parents:
    - If the array is missing and the final op is an append (`/-`), create an empty slice.
    - **No implicit extension for numeric indices.** If a pointer uses an explicit index (`/containers/2/...`) and that index does not exist, let the RFC 6902 engine return an error. Authors should add the entire array element explicitly in that case.

This logic can reuse portions of the current `setValue` helper; the key difference is that we prepare the structure *before* invoking jsonpatch.

### 3. RFC 6902 execution

For each concrete pointer:

- Build a tiny JSON patch document (single operation).
  - Example: `{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{...}}`
- Marshal the current resource map to JSON (`json.Marshal`).
- Apply the patch via `jsonpatch.DecodePatch`.
- Unmarshal the result back into `map[string]interface{}` (replace the original map contents).

Applying operations sequentially keeps the resource up to date for the next pointer.

### 4. Merge op

`merge` stays custom:

- Use the path resolver to locate map candidates.
- For each pointer, navigate to the map in the resource and perform a deep merge (`DeepMerge(existing, value)`).
- No RFC 6902 patch is needed.

## Edge cases

- **Multiple removals in the same array** – apply in descending index order to avoid shifting elements mid-loop.
- **Unescaped keys** – ensure pointer segments are JSON Pointer escaped before submitting to jsonpatch.
- **Non-map at expected location** – follow current behaviour (error out or overwrite based on op).
- **Missing filter matches** – simply skip (current behaviour).
- **Unsupported filter syntax** – return an error (maintain parity with current limitations).

## Dependencies

- Add `github.com/evanphx/json-patch` (or use kyaml’s wrapper – internally it wraps the same lib).
- Possibly move resolver helpers into a new file (`resolver.go`) to keep the main patch file tidy.

## Migration steps

1. Introduce the resolver (`expandPointers`) and parent-preparation functions while keeping existing patch execution.
2. Once resolver is stable, swap out the in-house add/replace/remove implementations with the jsonpatch delegate.
3. Keep `merge`, `parsePath`, and deep merge logic for map operations.
4. Add tests covering:
   - Append to missing array (parent creation).
   - Filtered paths hitting multiple matches.
   - Removal of filtered entries.
   - Merge vs add interplay.

This design now powers `pkg/patch`: array-filtered paths are resolved into concrete JSON Pointers, parents are created for `add`, and each operation is executed through the json-patch engine. The custom deep-merge handler remains for `merge`.
