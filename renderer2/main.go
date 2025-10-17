package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/chathurangada/cel_playground/renderer2/pkg/component"
	"github.com/chathurangada/cel_playground/renderer2/pkg/parser"
	"github.com/chathurangada/cel_playground/renderer2/pkg/template"
	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"gopkg.in/yaml.v3"
)

func main() {
	examplesDir := filepath.Join("..", "renderer", "examples")
	outputDir := filepath.Join(examplesDir, "expected-output")

	engine := template.NewEngine()
	renderer := component.NewRenderer(engine, nil)

	ctdPath := filepath.Join(examplesDir, "component-type-definitions", "deployment-component.yaml")
	ctd, err := parser.LoadComponentTypeDefinition(ctdPath)
	if err != nil {
		log.Fatalf("failed to load component type definition: %v", err)
	}

	componentPath := filepath.Join(examplesDir, "components", "example-component.yaml")
	componentDef, err := parser.LoadComponent(componentPath)
	if err != nil {
		log.Fatalf("failed to load component: %v", err)
	}

	addonDir := filepath.Join(examplesDir, "addons")
	addonNames := make([]string, 0, len(componentDef.Spec.Addons))
	for _, addon := range componentDef.Spec.Addons {
		addonNames = append(addonNames, addon.Name)
	}
	addons, err := parser.LoadAddons(addonDir, addonNames)
	if err != nil {
		log.Fatalf("failed to load addons: %v", err)
	}

	additionalCtxPath := filepath.Join(examplesDir, "additional_context.json")
	additionalCtx, err := parser.LoadAdditionalContext(additionalCtxPath)
	if err != nil {
		log.Printf("warning: failed to load additional context: %v", err)
	}

	// Validate schemas before rendering
	schemaOutputDir := filepath.Join(examplesDir, "schemas")
	if err := os.RemoveAll(schemaOutputDir); err != nil {
		log.Fatalf("failed to clean schema directory: %v", err)
	}
	if err := parser.ValidateSchemas(ctd, addons, schemaOutputDir); err != nil {
		log.Fatalf("schema validation failed: %v", err)
	}

	envDir := filepath.Join(examplesDir, "env-settings")
	envConfigs := []struct {
		name     string
		settings *types.EnvSettings
	}{
		{name: "no-env", settings: nil},
	}

	if devEnv, err := parser.LoadEnvSettings(filepath.Join(envDir, "dev-env.yaml")); err != nil {
		log.Printf("warning: could not load dev env settings: %v", err)
	} else {
		envConfigs = append(envConfigs, struct {
			name     string
			settings *types.EnvSettings
		}{name: "dev", settings: devEnv})
	}

	if prodEnv, err := parser.LoadEnvSettings(filepath.Join(envDir, "prod-env.yaml")); err != nil {
		log.Printf("warning: could not load prod env settings: %v", err)
	} else {
		envConfigs = append(envConfigs, struct {
			name     string
			settings *types.EnvSettings
		}{name: "prod", settings: prodEnv})
	}

	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("failed to clean output dir: %v", err)
	}

	stages := generateStages(componentDef)

	for _, env := range envConfigs {
		envOutput := filepath.Join(outputDir, env.name)
		if err := os.MkdirAll(envOutput, 0755); err != nil {
			log.Fatalf("failed to create output dir %s: %v", envOutput, err)
		}

		fmt.Printf("\nRendering for environment: %s\n", env.name)
		for _, stage := range stages {
			resources, err := renderer.RenderWithAddonLimit(ctd, componentDef, env.settings, addons, additionalCtx, nil, stage.AddonCount)
			if err != nil {
				log.Fatalf("failed to render stage %s: %v", stage.Name, err)
			}

			outputFile := filepath.Join(envOutput, stage.Name+".yaml")
			if err := writeOutput(resources, outputFile); err != nil {
				log.Fatalf("failed to write output: %v", err)
			}
			fmt.Printf("  wrote %s (%d resources)\n", outputFile, len(resources))
		}
	}

	fmt.Println("\n✅ rendering complete using renderer2")
}

func writeOutput(resources []map[string]interface{}, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	for _, resource := range resources {
		if err := encoder.Encode(resource); err != nil {
			return err
		}
	}
	return nil
}

func generateStages(component *types.Component) []types.Stage {
	stages := []types.Stage{{Name: "stage-1-base", AddonCount: 0}}
	shortNames := map[string]string{
		"persistent-volume-claim": "pvc",
		"sidecar-container":       "sidecar",
		"emptydir-volume":         "emptydir",
	}

	for i := range component.Spec.Addons {
		name := component.Spec.Addons[i].Name
		short := shortNames[name]
		if short == "" {
			short = name
		}
		stages = append(stages, types.Stage{
			Name:       fmt.Sprintf("stage-%d-with-%s", i+2, short),
			AddonCount: i + 1,
		})
	}

	return stages
}
