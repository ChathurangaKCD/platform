package pipeline

import (
	"fmt"

	"github.com/chathurangada/cel_playground/renderer2/pkg/context"
	"github.com/chathurangada/cel_playground/renderer2/pkg/patch"
	"github.com/chathurangada/cel_playground/renderer2/pkg/template"
	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

// RendererCoordinates orchestrates generic rendering workflows that other controllers can consume.
type RendererCoordinates struct {
	TemplateEngine *template.Engine
}

// NewRenderer constructs a renderer using the provided CEL engine.
func NewRenderer(engine *template.Engine) *RendererCoordinates {
	return &RendererCoordinates{TemplateEngine: engine}
}

// RenderComponentResources renders base resources for a ComponentTypeDefinition.
func (r *RendererCoordinates) RenderComponentResources(
	definition *types.ComponentTypeDefinition,
	component *types.Component,
	envSettings *types.EnvSettings,
	additionalCtx *types.AdditionalContext,
	workload map[string]interface{},
) ([]map[string]interface{}, error) {
	inputs := context.BuildComponentContext(component, envSettings, additionalCtx, workload)
	return r.renderResourceTemplates(definition.Spec.Resources, inputs)
}

// ApplyAddon composes addon creates and patches against already rendered resources.
func (r *RendererCoordinates) ApplyAddon(
	baseResources []map[string]interface{},
	addon *types.Addon,
	addonInstance types.AddonInstance,
	component *types.Component,
	envSettings *types.EnvSettings,
	additionalCtx *types.AdditionalContext,
	matcher patch.Matcher,
) ([]map[string]interface{}, error) {
	inputs := context.BuildAddonContext(component, addonInstance, envSettings, additionalCtx)

	// Render creates
	for _, createTemplate := range addon.Spec.Creates {
		rendered, err := r.TemplateEngine.Render(createTemplate, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to render addon create template %s/%s: %w", addon.Metadata.Name, addonInstance.InstanceID, err)
		}

		renderedMap, ok := rendered.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("addon create template must render to an object (addon %s)", addon.Metadata.Name)
		}

		cleaned := template.RemoveOmittedFields(renderedMap).(map[string]interface{})
		baseResources = append(baseResources, cleaned)
	}

	// Apply patches
	for _, patchSpec := range addon.Spec.Patches {
		if err := r.applyPatchSpec(baseResources, patchSpec, inputs, matcher); err != nil {
			return nil, fmt.Errorf("failed to apply addon patch: %w", err)
		}
	}

	return baseResources, nil
}

func (r *RendererCoordinates) applyPatchSpec(resources []map[string]interface{}, spec types.PatchSpec, inputs map[string]interface{}, matcher patch.Matcher) error {
	targets := patch.FindTargetResources(resources, spec.Target, matcher)

	if spec.ForEach != "" {
		// Evaluate iteration list
		itemsRaw, err := r.TemplateEngine.Render(spec.ForEach, inputs)
		if err != nil {
			return fmt.Errorf("failed to evaluate patch forEach expression: %w", err)
		}

		items, ok := itemsRaw.([]interface{})
		if !ok {
			return fmt.Errorf("forEach expression must evaluate to an array, got %T", itemsRaw)
		}

		varName := spec.Var
		if varName == "" {
			varName = "item"
		}

		for _, item := range items {
			itemInputs := cloneMap(inputs)
			itemInputs[varName] = item

			for _, target := range targets {
				if err := patch.ApplyPatch(target, spec.Patch, itemInputs, r.TemplateEngine.Render); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, target := range targets {
		if err := patch.ApplyPatch(target, spec.Patch, inputs, r.TemplateEngine.Render); err != nil {
			return err
		}
	}

	return nil
}

func (r *RendererCoordinates) renderResourceTemplates(templates []types.ResourceTemplate, inputs map[string]interface{}) ([]map[string]interface{}, error) {
	var resources []map[string]interface{}

	for _, tmpl := range templates {
		include, err := r.shouldInclude(tmpl, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate includeWhen for resource %s: %w", tmpl.ID, err)
		}
		if !include {
			continue
		}

		if tmpl.ForEach != "" {
			rendered, err := r.TemplateEngine.Render(tmpl.ForEach, inputs)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate forEach for resource %s: %w", tmpl.ID, err)
			}

			items, ok := rendered.([]interface{})
			if !ok {
				return nil, fmt.Errorf("forEach expression for resource %s must return an array, got %T", tmpl.ID, rendered)
			}

			varName := tmpl.Var
			if varName == "" {
				varName = "item"
			}

			for _, item := range items {
				itemInputs := cloneMap(inputs)
				itemInputs[varName] = item

				resource, err := r.TemplateEngine.Render(tmpl.Template, itemInputs)
				if err != nil {
					return nil, fmt.Errorf("failed to render resource %s: %w", tmpl.ID, err)
				}

				resourceMap, ok := resource.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("resource template must render to an object: %s", tmpl.ID)
				}

				cleaned := template.RemoveOmittedFields(resourceMap).(map[string]interface{})
				resources = append(resources, cleaned)
			}
			continue
		}

		resource, err := r.TemplateEngine.Render(tmpl.Template, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to render resource %s: %w", tmpl.ID, err)
		}

		resourceMap, ok := resource.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("resource template must render to an object: %s", tmpl.ID)
		}

		cleaned := template.RemoveOmittedFields(resourceMap).(map[string]interface{})
		resources = append(resources, cleaned)
	}

	return resources, nil
}

func (r *RendererCoordinates) shouldInclude(tmpl types.ResourceTemplate, inputs map[string]interface{}) (bool, error) {
	if tmpl.IncludeWhen == "" {
		return true, nil
	}

	result, err := r.TemplateEngine.Render(tmpl.IncludeWhen, inputs)
	if err != nil {
		return false, err
	}

	include, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("includeWhen must evaluate to bool, got %T", result)
	}

	return include, nil
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}
