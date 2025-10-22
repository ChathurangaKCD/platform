package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chathurangada/cel_playground/renderer2/pkg/component"
	"github.com/chathurangada/cel_playground/renderer2/pkg/parser"
	"github.com/chathurangada/cel_playground/renderer2/pkg/template"
	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"gopkg.in/yaml.v3"
)

func main() {
	examplesDir := "examples"
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

	// Extract CEL expressions and write to file
	exprOutput := collectCELExpressions(ctd, addons)
	exprPath := filepath.Join(examplesDir, "cel-expressions.yaml")
	if err := writeYAML(exprPath, exprOutput); err != nil {
		log.Fatalf("failed to write CEL expressions file: %v", err)
	}
	fmt.Printf("\nCollected CEL expressions written to %s\n", exprPath)

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

func writeOutput(resources []map[string]any, path string) error {
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

type celExpressionsOutput struct {
	ComponentTypeDefinition map[string][]string `yaml:"componentTypeDefinition"`
	Addons                  map[string][]string `yaml:"addons"`
}

func collectCELExpressions(ctd *types.ComponentTypeDefinition, addons map[string]*types.Addon) celExpressionsOutput {
	output := celExpressionsOutput{
		ComponentTypeDefinition: make(map[string][]string),
		Addons:                  make(map[string][]string),
	}

	for _, res := range ctd.Spec.Resources {
		key := fmt.Sprintf("resource:%s", res.ID)
		set := make(map[string]struct{})
		addStringExpression(set, res.IncludeWhen)
		addStringExpression(set, res.ForEach)
		collectExpressionsFromValue(res.Template, set)
		if len(set) > 0 {
			output.ComponentTypeDefinition[key] = setToSortedSlice(set)
		}
	}

	for name, addon := range addons {
		set := make(map[string]struct{})
		for _, create := range addon.Spec.Creates {
			collectExpressionsFromValue(create, set)
		}
		for _, patchSpec := range addon.Spec.Patches {
			addStringExpression(set, patchSpec.ForEach)
			addStringExpression(set, patchSpec.Target.Where)
			for _, op := range patchSpec.Operations {
				addStringExpression(set, op.Path)
				collectExpressionsFromValue(op.Value, set)
			}
		}
		if len(set) > 0 {
			output.Addons[name] = setToSortedSlice(set)
		}
	}

	return output
}

func addStringExpression(set map[string]struct{}, value string) {
	if strings.Contains(value, "${") {
		set[value] = struct{}{}
	}
}

func collectExpressionsFromValue(v any, set map[string]struct{}) {
	switch typed := v.(type) {
	case string:
		addStringExpression(set, typed)
	case []any:
		for _, item := range typed {
			collectExpressionsFromValue(item, set)
		}
	case map[string]any:
		for _, item := range typed {
			collectExpressionsFromValue(item, set)
		}
	case map[any]any:
		for _, item := range typed {
			collectExpressionsFromValue(item, set)
		}
	}
}

func setToSortedSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for expr := range set {
		result = append(result, expr)
	}
	sort.Strings(result)
	return result
}

func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
