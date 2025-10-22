package component

import (
	"fmt"

	"github.com/chathurangada/cel_playground/renderer2/pkg/patch"
	"github.com/chathurangada/cel_playground/renderer2/pkg/pipeline"
	"github.com/chathurangada/cel_playground/renderer2/pkg/template"
	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

// Renderer exposes high-level rendering for ComponentTypeDefinitions plus addons.
type Renderer struct {
	base    *pipeline.RendererCoordinates
	matcher patch.Matcher
}

// NewRenderer builds a component-aware renderer from the shared template engine.
func NewRenderer(engine *template.Engine, matcher patch.Matcher) *Renderer {
	return &Renderer{
		base:    pipeline.NewRenderer(engine),
		matcher: matcher,
	}
}

// RenderAll renders base resources and sequentially applies addon instances.
func (r *Renderer) RenderAll(
	definition *types.ComponentTypeDefinition,
	component *types.Component,
	envSettings *types.EnvSettings,
	addonMap map[string]*types.Addon,
	additionalCtx *types.AdditionalContext,
	workload map[string]any,
) ([]map[string]any, error) {
	return r.RenderWithAddonLimit(definition, component, envSettings, addonMap, additionalCtx, workload, len(component.Spec.Addons))
}

// RenderWithAddonLimit renders base resources and applies addons up to addonLimit (count from component.Spec.Addons).
func (r *Renderer) RenderWithAddonLimit(
	definition *types.ComponentTypeDefinition,
	component *types.Component,
	envSettings *types.EnvSettings,
	addonMap map[string]*types.Addon,
	additionalCtx *types.AdditionalContext,
	workload map[string]any,
	addonLimit int,
) ([]map[string]any, error) {
	resources, err := r.base.RenderComponentResources(definition, component, envSettings, additionalCtx, workload)
	if err != nil {
		return nil, err
	}

	if addonLimit < 0 || addonLimit > len(component.Spec.Addons) {
		addonLimit = len(component.Spec.Addons)
	}

	for i := 0; i < addonLimit; i++ {
		instance := component.Spec.Addons[i]
		addon, ok := addonMap[instance.Name]
		if !ok {
			return nil, fmt.Errorf("addon %s not found", instance.Name)
		}

		resources, err = r.base.ApplyAddon(resources, addon, instance, component, envSettings, additionalCtx, r.matcher)
		if err != nil {
			return nil, err
		}
	}

	return resources, nil
}
