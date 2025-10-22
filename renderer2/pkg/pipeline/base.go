package pipeline

import (
	"fmt"
	"strings"

	"github.com/chathurangada/cel_playground/renderer2/pkg/context"
	"github.com/chathurangada/cel_playground/renderer2/pkg/patch"
	"github.com/chathurangada/cel_playground/renderer2/pkg/schema"
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
	workload map[string]any,
) ([]map[string]any, error) {
	definitionSchema := schema.Definition{
		Types: definition.Spec.Schema.Types,
		Schemas: []map[string]any{
			definition.Spec.Schema.Parameters,
			definition.Spec.Schema.EnvOverrides,
		},
	}

	componentDefaults, err := schema.ExtractDefaults(definitionSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate component defaults: %w", err)
	}

	inputs := context.BuildComponentContext(component, envSettings, additionalCtx, workload, componentDefaults)
	return r.renderResourceTemplates(definition.Spec.Resources, inputs)
}

// ApplyAddon composes addon creates and patches against already rendered resources.
func (r *RendererCoordinates) ApplyAddon(
	baseResources []map[string]any,
	addon *types.Addon,
	addonInstance types.AddonInstance,
	component *types.Component,
	envSettings *types.EnvSettings,
	additionalCtx *types.AdditionalContext,
	matcher patch.Matcher,
) ([]map[string]any, error) {
	addonSchema := schema.Definition{
		Types: addon.Spec.Schema.Types,
		Schemas: []map[string]any{
			addon.Spec.Schema.Parameters,
			addon.Spec.Schema.EnvOverrides,
		},
	}
	addonDefaults, err := schema.ExtractDefaults(addonSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate defaults for addon %s: %w", addon.Metadata.Name, err)
	}

	inputs := context.BuildAddonContext(component, addonInstance, envSettings, additionalCtx, addonDefaults)

	// Render creates
	for _, createTemplate := range addon.Spec.Creates {
		rendered, err := r.TemplateEngine.Render(createTemplate, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to render addon create template %s/%s: %w", addon.Metadata.Name, addonInstance.InstanceID, err)
		}

		renderedMap, ok := rendered.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("addon create template must render to an object (addon %s)", addon.Metadata.Name)
		}

		cleaned := template.RemoveOmittedFields(renderedMap).(map[string]any)
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
func (r *RendererCoordinates) applyPatchSpec(resources []map[string]any, spec types.PatchSpec, inputs map[string]any, matcher patch.Matcher) error {
	targets := patch.FindTargetResources(resources, spec.Target, matcher)

	if len(spec.Operations) == 0 {
		return nil
	}

	// Helper to evaluate the where clause for a given target with provided inputs.
	matchTarget := func(where string, target map[string]any, baseInputs map[string]any) (bool, error) {
		if where == "" {
			return true, nil
		}

		previous, had := baseInputs["resource"]
		baseInputs["resource"] = target

		result, err := r.TemplateEngine.Render(where, baseInputs)
		if had {
			baseInputs["resource"] = previous
		} else {
			delete(baseInputs, "resource")
		}

		if err != nil {
			if isMissingDataError(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to evaluate target.where: %w", err)
		}
		boolResult, ok := result.(bool)
		if !ok {
			return false, fmt.Errorf("target.where must evaluate to a boolean, got %T", result)
		}
		return boolResult, nil
	}

	executeOperations := func(target map[string]any, baseInputs map[string]any) error {
		previous, had := baseInputs["resource"]
		baseInputs["resource"] = target
		for _, op := range spec.Operations {
			if err := patch.ApplyOperation(target, op, baseInputs, r.TemplateEngine.Render); err != nil {
				if had {
					baseInputs["resource"] = previous
				} else {
					delete(baseInputs, "resource")
				}
				return err
			}
		}
		if had {
			baseInputs["resource"] = previous
		} else {
			delete(baseInputs, "resource")
		}
		return nil
	}

	if spec.ForEach != "" {
		// Evaluate iteration list
		itemsRaw, err := r.TemplateEngine.Render(spec.ForEach, inputs)
		if err != nil {
			return fmt.Errorf("failed to evaluate patch forEach expression: %w", err)
		}

		items, ok := itemsRaw.([]any)
		if !ok {
			return fmt.Errorf("forEach expression must evaluate to an array, got %T", itemsRaw)
		}

		varName := spec.Var
		if varName == "" {
			varName = "item"
		}

		previous, hadVar := inputs[varName]
		for _, item := range items {
			inputs[varName] = item

			for _, target := range targets {
				match, err := matchTarget(spec.Target.Where, target, inputs)
				if err != nil {
					if hadVar {
						inputs[varName] = previous
					} else {
						delete(inputs, varName)
					}
					return err
				}
				if !match {
					continue
				}
				if err := executeOperations(target, inputs); err != nil {
					if hadVar {
						inputs[varName] = previous
					} else {
						delete(inputs, varName)
					}
					return err
				}
			}
		}
		if hadVar {
			inputs[varName] = previous
		} else {
			delete(inputs, varName)
		}
		return nil
	}

	for _, target := range targets {
		match, err := matchTarget(spec.Target.Where, target, inputs)
		if err != nil {
			return err
		}
		if !match {
			continue
		}
		if err := executeOperations(target, inputs); err != nil {
			return err
		}
	}

	return nil
}

func (r *RendererCoordinates) renderResourceTemplates(templates []types.ResourceTemplate, inputs map[string]any) ([]map[string]any, error) {
	var resources []map[string]any

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

			items, ok := rendered.([]any)
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

				resourceMap, ok := resource.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("resource template must render to an object: %s", tmpl.ID)
				}

				cleaned := template.RemoveOmittedFields(resourceMap).(map[string]any)
				resources = append(resources, cleaned)
			}
			continue
		}

		resource, err := r.TemplateEngine.Render(tmpl.Template, inputs)
		if err != nil {
			return nil, fmt.Errorf("failed to render resource %s: %w", tmpl.ID, err)
		}

		resourceMap, ok := resource.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("resource template must render to an object: %s", tmpl.ID)
		}

		cleaned := template.RemoveOmittedFields(resourceMap).(map[string]any)
		resources = append(resources, cleaned)
	}

	return resources, nil
}

func (r *RendererCoordinates) shouldInclude(tmpl types.ResourceTemplate, inputs map[string]any) (bool, error) {
	if tmpl.IncludeWhen == "" {
		return true, nil
	}

	result, err := r.TemplateEngine.Render(tmpl.IncludeWhen, inputs)
	if err != nil {
		if isMissingDataError(err) {
			return false, nil
		}
		return false, err
	}

	include, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("includeWhen must evaluate to bool, got %T", result)
	}

	return include, nil
}

func cloneMap(src map[string]any) map[string]any {
	result := make(map[string]any, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}

func isMissingDataError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no such key") ||
		strings.Contains(msg, "no such field") ||
		strings.Contains(msg, "undefined variable")
}
