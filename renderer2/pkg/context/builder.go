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
) map[string]interface{} {
	spec := cloneMap(component.Spec.Parameters)

	if envSettings != nil {
		for key, value := range envSettings.Spec.Overrides {
			spec[key] = value
		}
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
) map[string]interface{} {
	config := cloneMap(addonInstance.Config)

	if envSettings != nil && envSettings.Spec.AddonOverrides != nil {
		if overrides, ok := envSettings.Spec.AddonOverrides[addonInstance.InstanceID]; ok {
			for key, value := range overrides {
				config[key] = value
			}
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
	result := make(map[string]interface{}, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}

func cloneStringMap(src map[string]string) map[string]string {
	result := make(map[string]string, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}
