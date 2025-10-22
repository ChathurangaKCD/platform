package context

import (
	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

// BuildComponentContext merges component parameters, env overrides, workload information,
// and additional platform context to feed into template rendering.
func BuildComponentContext(
	component *types.Component,
	envSettings *types.EnvSettings,
	additionalCtx *types.AdditionalContext,
	workload map[string]any,
	defaults map[string]any,
) map[string]any {
	spec := deepCopyMap(defaults)

	if component.Spec.Parameters != nil {
		mergeInto(spec, component.Spec.Parameters)
	}

	if envSettings != nil {
		mergeInto(spec, envSettings.Spec.Overrides)
	}

	ctx := map[string]any{
		"metadata": buildMetadata(component.Metadata),
		"spec":     spec,
		"build":    buildFromComponent(component.Spec.Build, additionalCtx),
	}

	if workload != nil {
		ctx["workload"] = workload
	}

	if additionalCtx != nil {
		ctx["podSelectors"] = toInterfaceMap(additionalCtx.PodSelectors)
		ctx["configurations"] = convertConfiguration(additionalCtx.Configurations)
		ctx["secrets"] = convertSecrets(additionalCtx.Secrets)
	}

	return ctx
}

// BuildAddonContext prepares inputs for addon templates using addon instance configuration,
// environment overrides, and shared metadata.
func BuildAddonContext(
	component *types.Component,
	addonInstance types.AddonInstance,
	envSettings *types.EnvSettings,
	additionalCtx *types.AdditionalContext,
	defaults map[string]any,
) map[string]any {
	config := deepCopyMap(defaults)

	if addonInstance.Config != nil {
		mergeInto(config, addonInstance.Config)
	}

	if envSettings != nil && envSettings.Spec.AddonOverrides != nil {
		if overrides, ok := envSettings.Spec.AddonOverrides[addonInstance.InstanceID]; ok {
			mergeInto(config, overrides)
		}
	}

	ctx := map[string]any{
		"metadata":   buildMetadata(component.Metadata),
		"spec":       config,
		"instanceId": addonInstance.InstanceID,
		"build":      buildFromComponent(component.Spec.Build, additionalCtx),
	}

	if additionalCtx != nil {
		ctx["podSelectors"] = toInterfaceMap(additionalCtx.PodSelectors)
		ctx["configurations"] = convertConfiguration(additionalCtx.Configurations)
		ctx["secrets"] = convertSecrets(additionalCtx.Secrets)
	}

	return ctx
}

func buildMetadata(md types.Metadata) map[string]any {
	return map[string]any{
		"name":        md.Name,
		"namespace":   md.Namespace,
		"labels":      cloneStringMap(md.Labels),
		"annotations": cloneStringMap(md.Annotations),
	}
}

func buildFromComponent(build types.BuildSpec, additionalCtx *types.AdditionalContext) map[string]any {
	if additionalCtx != nil && additionalCtx.Build.Image != "" {
		return map[string]any{
			"image": additionalCtx.Build.Image,
		}
	}

	if build.Image != "" {
		return map[string]any{
			"image": build.Image,
		}
	}

	return map[string]any{}
}

func toInterfaceMap(input map[string]string) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func convertConfiguration(config types.ConfigurationData) map[string]any {
	envs := make([]any, len(config.Envs))
	for i, env := range config.Envs {
		envs[i] = map[string]any{
			"name":  env.Name,
			"value": env.Value,
		}
	}

	files := make([]any, len(config.Files))
	for i, file := range config.Files {
		files[i] = map[string]any{
			"name":      file.Name,
			"mountPath": file.MountPath,
			"content":   file.Content,
		}
	}

	return map[string]any{
		"envs":  envs,
		"files": files,
	}
}

func convertSecrets(secrets types.SecretData) map[string]any {
	envs := make([]any, len(secrets.Envs))
	for i, env := range secrets.Envs {
		envs[i] = map[string]any{
			"name":     env.Name,
			"valueRef": env.ValueRef,
		}
	}

	files := make([]any, len(secrets.Files))
	for i, file := range secrets.Files {
		files[i] = map[string]any{
			"name":      file.Name,
			"mountPath": file.MountPath,
			"valueRef":  file.ValueRef,
		}
	}

	return map[string]any{
		"envs":  envs,
		"files": files,
	}
}

func cloneMap(src map[string]any) map[string]any {
	return deepCopyMap(src)
}

func cloneStringMap(src map[string]string) map[string]string {
	result := make(map[string]string, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}

func deepCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(src))
	for key, value := range src {
		result[key] = deepCopyValue(value)
	}
	return result
}

func deepCopyValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return deepCopyMap(v)
	case []any:
		copied := make([]any, len(v))
		for i, item := range v {
			copied[i] = deepCopyValue(item)
		}
		return copied
	default:
		return v
	}
}

func mergeInto(dst map[string]any, src map[string]any) {
	if dst == nil || src == nil {
		return
	}

	for key, value := range src {
		if valueMap, ok := value.(map[string]any); ok {
			existing, exists := dst[key]
			if !exists {
				dst[key] = deepCopyMap(valueMap)
				continue
			}

			existingMap, ok := existing.(map[string]any)
			if !ok {
				dst[key] = deepCopyMap(valueMap)
				continue
			}

			mergeInto(existingMap, valueMap)
			continue
		}

		if valueSlice, ok := value.([]any); ok {
			dst[key] = deepCopyValue(valueSlice)
			continue
		}

		dst[key] = value
	}
}
