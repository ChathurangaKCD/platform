package renderer

import (
	"github.com/chathurangada/cel_playground/renderer/pkg/types"
)

// BuildInputs creates the input context for CEL evaluation by merging Component and EnvSettings
func BuildInputs(
	component *types.Component,
	envSettings *types.EnvSettings,
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
		"spec":         spec,
		"build":        buildContextFromBuildSpec(component.Spec.Build),
		"podSelectors": convertToInterfaceMap(component.Spec.PodSelectors),
	}

	return inputs
}

// BuildAddonInputs creates the input context for addon CEL evaluation
func BuildAddonInputs(
	component *types.Component,
	addonInstance types.AddonInstance,
	envSettings *types.EnvSettings,
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
		"spec":         config,
		"instanceId":   addonInstance.InstanceID,
		"build":        buildContextFromBuildSpec(component.Spec.Build),
		"podSelectors": convertToInterfaceMap(component.Spec.PodSelectors),
	}

	return inputs
}

func buildContextFromBuildSpec(build types.BuildSpec) map[string]interface{} {
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
