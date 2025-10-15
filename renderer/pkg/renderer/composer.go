package renderer

import (
	"fmt"

	"github.com/chathurangada/cel_playground/renderer/pkg/types"
)

// RenderBaseResources renders the base resources from ComponentTypeDefinition
func RenderBaseResources(
	ctd *types.ComponentTypeDefinition,
	inputs map[string]interface{},
) ([]map[string]interface{}, error) {
	var resources []map[string]interface{}

	for _, resourceTemplate := range ctd.Spec.Resources {
		// Check condition if present
		if resourceTemplate.Condition != "" {
			conditionResult, err := EvaluateCELExpressions(resourceTemplate.Condition, inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate condition for resource %s: %w", resourceTemplate.ID, err)
			}
			if boolResult, ok := conditionResult.(bool); ok && !boolResult {
				continue // Skip this resource
			}
		}

		// Handle forEach - render template for each item in array
		if resourceTemplate.ForEach != "" {
			// Evaluate forEach expression to get items
			itemsResult, err := EvaluateCELExpressions(resourceTemplate.ForEach, inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate forEach expression for resource %s: %w", resourceTemplate.ID, err)
			}

			items, ok := itemsResult.([]interface{})
			if !ok {
				return nil, fmt.Errorf("forEach expression for resource %s must return an array, got %T", resourceTemplate.ID, itemsResult)
			}

			// Render template for each item
			for _, item := range items {
				// Create new inputs with current item
				itemInputs := make(map[string]interface{})
				for k, v := range inputs {
					itemInputs[k] = v
				}
				itemInputs["item"] = item

				// Evaluate the template with item context
				rendered, err := EvaluateCELExpressions(resourceTemplate.Template, itemInputs)
				if err != nil {
					return nil, fmt.Errorf("failed to render forEach resource %s: %w", resourceTemplate.ID, err)
				}

				renderedMap, ok := rendered.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("rendered forEach resource %s is not a map", resourceTemplate.ID)
				}

				// Clean up omitted fields
				cleaned := RemoveOmittedFields(renderedMap).(map[string]interface{})
				resources = append(resources, cleaned)
			}
		} else {
			// Single resource (no forEach)
			// Evaluate the template
			rendered, err := EvaluateCELExpressions(resourceTemplate.Template, inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to render resource %s: %w", resourceTemplate.ID, err)
			}

			renderedMap, ok := rendered.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("rendered resource %s is not a map", resourceTemplate.ID)
			}

			// Clean up omitted fields
			cleaned := RemoveOmittedFields(renderedMap).(map[string]interface{})
			resources = append(resources, cleaned)
		}
	}

	return resources, nil
}

// ApplyAddon applies an addon to existing resources
func ApplyAddon(
	resources []map[string]interface{},
	addon *types.Addon,
	addonInstance types.AddonInstance,
	inputs map[string]interface{},
) ([]map[string]interface{}, error) {
	// Create resources defined in addon.Creates
	for _, createTemplate := range addon.Spec.Creates {
		// Evaluate the create template
		rendered, err := EvaluateCELExpressions(createTemplate, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to render created resource: %w", err)
		}

		renderedMap, ok := rendered.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("rendered created resource is not a map")
		}

		cleaned := RemoveOmittedFields(renderedMap).(map[string]interface{})
		resources = append(resources, cleaned)
	}

	// Apply patches defined in addon.Patches
	for _, patchSpec := range addon.Spec.Patches {
		if patchSpec.ForEach != "" {
			// Handle forEach patches
			if err := applyForEachPatch(resources, patchSpec, inputs); err != nil {
				return nil, fmt.Errorf("failed to apply forEach patch: %w", err)
			}
		} else {
			// Handle single patch
			if err := applySinglePatch(resources, patchSpec, inputs); err != nil {
				return nil, fmt.Errorf("failed to apply patch: %w", err)
			}
		}
	}

	return resources, nil
}

func applySinglePatch(
	resources []map[string]interface{},
	patchSpec types.PatchSpec,
	inputs map[string]interface{},
) error {
	// Find target resources
	targets := FindTargetResources(resources, patchSpec.Target)

	for _, target := range targets {
		if err := ApplyPatch(target, patchSpec.Patch, inputs); err != nil {
			return fmt.Errorf("failed to apply patch to target: %w", err)
		}
	}

	return nil
}

func applyForEachPatch(
	resources []map[string]interface{},
	patchSpec types.PatchSpec,
	inputs map[string]interface{},
) error {
	// Evaluate the forEach expression to get items
	itemsResult, err := EvaluateCELExpressions(patchSpec.ForEach, inputs)
	if err != nil {
		return fmt.Errorf("failed to evaluate forEach expression: %w", err)
	}

	items, ok := itemsResult.([]interface{})
	if !ok {
		return fmt.Errorf("forEach expression must return an array, got %T", itemsResult)
	}

	// Apply patch for each item
	for _, item := range items {
		// Create new inputs with current item
		itemInputs := make(map[string]interface{})
		for k, v := range inputs {
			itemInputs[k] = v
		}
		itemInputs["item"] = item

		// Find target resources
		targets := FindTargetResources(resources, patchSpec.Target)

		for _, target := range targets {
			if err := ApplyPatch(target, patchSpec.Patch, itemInputs); err != nil {
				return fmt.Errorf("failed to apply forEach patch to target: %w", err)
			}
		}
	}

	return nil
}
