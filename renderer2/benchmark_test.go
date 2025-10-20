package main

import (
	"path/filepath"
	"testing"

	"github.com/chathurangada/cel_playground/renderer2/pkg/component"
	"github.com/chathurangada/cel_playground/renderer2/pkg/parser"
	"github.com/chathurangada/cel_playground/renderer2/pkg/template"
	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

func BenchmarkRenderExamples(b *testing.B) {
	examplesDir := "examples"

	engine := template.NewEngine()
	renderer := component.NewRenderer(engine, nil)

	ctdPath := filepath.Join(examplesDir, "component-type-definitions", "deployment-component.yaml")
	ctd, err := parser.LoadComponentTypeDefinition(ctdPath)
	if err != nil {
		b.Fatalf("failed to load component type definition: %v", err)
	}

	componentPath := filepath.Join(examplesDir, "components", "example-component.yaml")
	componentDef, err := parser.LoadComponent(componentPath)
	if err != nil {
		b.Fatalf("failed to load component: %v", err)
	}

	addonDir := filepath.Join(examplesDir, "addons")
	addons, err := parser.LoadAddons(addonDir, nil)
	if err != nil {
		b.Fatalf("failed to load addons: %v", err)
	}

	additionalCtxPath := filepath.Join(examplesDir, "additional_context.json")
	additionalCtx, err := parser.LoadAdditionalContext(additionalCtxPath)
	if err != nil {
		b.Fatalf("failed to load additional context: %v", err)
	}

	prodEnvPath := filepath.Join(examplesDir, "env-settings", "prod-env.yaml")
	prodSettings, err := parser.LoadEnvSettings(prodEnvPath)
	if err != nil {
		b.Fatalf("failed to load prod env settings: %v", err)
	}

	envSettings := map[string]*types.EnvSettings{
		"prod": prodSettings,
	}

	stages := generateStages(componentDef)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for envName, envSetting := range envSettings {
			for _, stage := range stages {
				_, err := renderer.RenderWithAddonLimit(ctd, componentDef, envSetting, addons, additionalCtx, nil, stage.AddonCount)
				if err != nil {
					b.Fatalf("render failed for env %s stage %s: %v", envName, stage.Name, err)
				}
			}
		}
	}
}
