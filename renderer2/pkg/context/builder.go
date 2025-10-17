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
	workload map[string]interface{},
	defaults map[string]interface{},
) map[string]interface{} {
	spec := deepCopyMap(defaults)

	if component.Spec.Parameters != nil {
		mergeInto(spec, component.Spec.Parameters)
	}

	if envSettings != nil {
		mergeInto(spec, envSettings.Spec.Overrides)
	}

	ctx := map[string]interface{}{
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
	defaults map[string]interface{},
) map[string]interface{} {
	config := deepCopyMap(defaults)

	if addonInstance.Config != nil {
		mergeInto(config, addonInstance.Config)
	}

	if envSettings != nil && envSettings.Spec.AddonOverrides != nil {
		if overrides, ok := envSettings.Spec.AddonOverrides[addonInstance.InstanceID]; ok {
			mergeInto(config, overrides)
		}
	}

	ctx := map[string]interface{}{
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

func buildMetadata(md types.Metadata) map[string]interface{} {
	return map[string]interface{}{
		"name":        md.Name,
		"namespace":   md.Namespace,
		"labels":      cloneStringMap(md.Labels),
		"annotations": cloneStringMap(md.Annotations),
	}
}

func buildFromComponent(build types.BuildSpec, additionalCtx *types.AdditionalContext) map[string]interface{} {
	if additionalCtx != nil && additionalCtx.Build.Image != "" {
		return map[string]interface{}{
			"image": additionalCtx.Build.Image,
		}
	}

	if build.Image != "" {
		return map[string]interface{}{
			"image": build.Image,
		}
	}

	return map[string]interface{}{}
}

func toInterfaceMap(input map[string]string) map[string]interface{} {
	result := make(map[string]interface{}, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func convertConfiguration(config types.ConfigurationData) map[string]interface{} {
	envs := make([]interface{}, len(config.Envs))
	for i, env := range config.Envs {
		envs[i] = map[string]interface{}{
			"name":  env.Name,
			"value": env.Value,
		}
	}

	files := make([]interface{}, len(config.Files))
	for i, file := range config.Files {
		files[i] = map[string]interface{}{
			"name":      file.Name,
			"mountPath": file.MountPath,
			"content":   file.Content,
		}
	}

	return map[string]interface{}{
		"envs":  envs,
		"files": files,
	}
}

func convertSecrets(secrets types.SecretData) map[string]interface{} {
	envs := make([]interface{}, len(secrets.Envs))
	for i, env := range secrets.Envs {
		envs[i] = map[string]interface{}{
			"name":     env.Name,
			"valueRef": env.ValueRef,
		}
	}

	files := make([]interface{}, len(secrets.Files))
	for i, file := range secrets.Files {
		files[i] = map[string]interface{}{
			"name":      file.Name,
			"mountPath": file.MountPath,
			"valueRef":  file.ValueRef,
		}
	}

	return map[string]interface{}{
		"envs":  envs,
		"files": files,
	}
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	return deepCopyMap(src)
}

func cloneStringMap(src map[string]string) map[string]string {
	result := make(map[string]string, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(src))
	for key, value := range src {
		result[key] = deepCopyValue(value)
	}
	return result
}

func deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		return deepCopyMap(v)
	case []interface{}:
		copied := make([]interface{}, len(v))
		for i, item := range v {
			copied[i] = deepCopyValue(item)
		}
		return copied
	default:
		return v
	}
}

func mergeInto(dst map[string]interface{}, src map[string]interface{}) {
	if dst == nil || src == nil {
		return
	}

	for key, value := range src {
		if valueMap, ok := value.(map[string]interface{}); ok {
			existing, exists := dst[key]
			if !exists {
				dst[key] = deepCopyMap(valueMap)
				continue
			}

			existingMap, ok := existing.(map[string]interface{})
			if !ok {
				dst[key] = deepCopyMap(valueMap)
				continue
			}

			mergeInto(existingMap, valueMap)
			continue
		}

		if valueSlice, ok := value.([]interface{}); ok {
			dst[key] = deepCopyValue(valueSlice)
			continue
		}

		dst[key] = value
	}
}
