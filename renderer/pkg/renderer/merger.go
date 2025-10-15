package renderer

import (
	"github.com/chathurangada/cel_playground/renderer/pkg/parser"
	"github.com/chathurangada/cel_playground/renderer/pkg/types"
)

// BuildInputs creates the input context for CEL evaluation by merging Component and EnvSettings
func BuildInputs(
	component *types.Component,
	envSettings *types.EnvSettings,
	additionalCtx *parser.AdditionalContext,
) map[string]interface{} {
	// Start with component parameters
	spec := make(map[string]interface{})
	for k, v := range component.Spec.Parameters {
		spec[k] = v
	}

	// Merge envSettings overrides if provided
	if envSettings != nil {
		for k, v := range envSettings.Spec.Overrides {
			spec[k] = v
		}
	}

	inputs := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      component.Metadata.Name,
			"namespace": component.Metadata.Namespace,
			"labels":    component.Metadata.Labels,
		},
		"spec": spec,
	}

	// Add additional context if provided
	if additionalCtx != nil {
		inputs["podSelectors"] = convertToInterfaceMap(additionalCtx.PodSelectors)
		inputs["build"] = buildContextFromAdditionalContext(additionalCtx.Build)
		inputs["configurations"] = convertConfigurationData(additionalCtx.Configurations)
		inputs["secrets"] = convertSecretData(additionalCtx.Secrets)
	} else {
		// Fallback to component build spec if no additional context
		inputs["build"] = buildContextFromBuildSpec(component.Spec.Build)
	}

	return inputs
}

// BuildAddonInputs creates the input context for addon CEL evaluation
func BuildAddonInputs(
	component *types.Component,
	addonInstance types.AddonInstance,
	envSettings *types.EnvSettings,
	additionalCtx *parser.AdditionalContext,
) map[string]interface{} {
	// Start with addon config
	config := make(map[string]interface{})
	for k, v := range addonInstance.Config {
		config[k] = v
	}

	// Merge envSettings addon overrides if provided
	if envSettings != nil && envSettings.Spec.AddonOverrides != nil {
		if overrides, ok := envSettings.Spec.AddonOverrides[addonInstance.InstanceID]; ok {
			for k, v := range overrides {
				config[k] = v
			}
		}
	}

	inputs := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      component.Metadata.Name,
			"namespace": component.Metadata.Namespace,
			"labels":    component.Metadata.Labels,
		},
		"spec":       config,
		"instanceId": addonInstance.InstanceID,
	}

	// Add additional context if provided
	if additionalCtx != nil {
		inputs["podSelectors"] = convertToInterfaceMap(additionalCtx.PodSelectors)
		inputs["build"] = buildContextFromAdditionalContext(additionalCtx.Build)
		inputs["configurations"] = convertConfigurationData(additionalCtx.Configurations)
		inputs["secrets"] = convertSecretData(additionalCtx.Secrets)
	} else {
		// Fallback to component build spec if no additional context
		inputs["build"] = buildContextFromBuildSpec(component.Spec.Build)
	}

	return inputs
}

func buildContextFromBuildSpec(build types.BuildSpec) map[string]interface{} {
	return map[string]interface{}{
		"image": build.Image,
	}
}

func buildContextFromAdditionalContext(build parser.BuildData) map[string]interface{} {
	return map[string]interface{}{
		"image": build.Image,
	}
}

// convertToInterfaceMap converts map[string]string to map[string]interface{}
func convertToInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}

// convertConfigurationData converts ConfigurationData to map for CEL
func convertConfigurationData(config parser.ConfigurationData) map[string]interface{} {
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
			"mountPath": file.MountPath,
			"content":   file.Content,
		}
	}

	return map[string]interface{}{
		"envs":  envs,
		"files": files,
	}
}

// convertSecretData converts SecretData to map for CEL
func convertSecretData(secrets parser.SecretData) map[string]interface{} {
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
			"mountPath": file.MountPath,
			"valueRef":  file.ValueRef,
		}
	}

	return map[string]interface{}{
		"envs":  envs,
		"files": files,
	}
}

// DeepMerge deeply merges two maps, with values from 'override' taking precedence
func DeepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base
	for k, v := range base {
		result[k] = v
	}

	// Merge override
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			// If both are maps, recursively merge
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overrideMap, ok := v.(map[string]interface{}); ok {
					result[k] = DeepMerge(baseMap, overrideMap)
					continue
				}
			}
		}
		// Otherwise, override wins
		result[k] = v
	}

	return result
}
