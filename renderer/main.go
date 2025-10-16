package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/chathurangada/cel_playground/renderer/pkg/parser"
	"github.com/chathurangada/cel_playground/renderer/pkg/renderer"
	"github.com/chathurangada/cel_playground/renderer/pkg/types"
	"gopkg.in/yaml.v3"
)

func main() {
	examplesDir := "examples"
	outputDir := filepath.Join(examplesDir, "expected-output")

	// 1. Load Component Type Definition
	ctdPath := filepath.Join(examplesDir, "component-type-definitions", "deployment-component.yaml")
	ctd, err := parser.LoadComponentTypeDefinition(ctdPath)
	if err != nil {
		log.Fatalf("Failed to load component type definition: %v", err)
	}
	fmt.Printf("Loaded ComponentTypeDefinition: %s\n", ctd.Metadata.Name)

	// 2. Load Component
	componentPath := filepath.Join(examplesDir, "components", "example-component.yaml")
	component, err := parser.LoadComponent(componentPath)
	if err != nil {
		log.Fatalf("Failed to load component: %v", err)
	}
	fmt.Printf("Loaded Component: %s\n", component.Metadata.Name)

	// 3. Load Addons
	addonDir := filepath.Join(examplesDir, "addons")
	addonNames := make([]string, 0, len(component.Spec.Addons))
	for _, addonInstance := range component.Spec.Addons {
		addonNames = append(addonNames, addonInstance.Name)
	}
	addons, err := parser.LoadAddons(addonDir, addonNames)
	if err != nil {
		log.Fatalf("Failed to load addons: %v", err)
	}
	fmt.Printf("Loaded %d addons\n", len(addons))

	// 4. Load additional context (pod selectors, configurations, secrets)
	additionalCtxPath := filepath.Join(examplesDir, "additional_context.json")
	additionalCtx, err := parser.LoadAdditionalContext(additionalCtxPath)
	if err != nil {
		log.Printf("Warning: Could not load additional context: %v", err)
		additionalCtx = nil // Continue without additional context
	} else {
		fmt.Printf("Loaded additional context (pod selectors, configurations, secrets)\n")
	}

	// 5. Clean and recreate schema output directory
	schemaOutputDir := filepath.Join(examplesDir, "schemas")
	if err := os.RemoveAll(schemaOutputDir); err != nil {
		log.Fatalf("Failed to clean schemas directory: %v", err)
	}

	// Validate schemas and generate JSON schemas
	if err := parser.ValidateSchemas(ctd, addons, schemaOutputDir); err != nil {
		log.Fatalf("Failed to validate schemas: %v", err)
	}

	// 5. Generate stages dynamically from Component's addon list
	stages := generateStages(component)
	fmt.Printf("Generated %d stages\n", len(stages))

	// 5. Load environment settings
	envConfigs := map[string]*types.EnvSettings{
		"no-env": nil, // No environment settings
	}

	devEnvPath := filepath.Join(examplesDir, "env-settings", "dev-env.yaml")
	devEnv, err := parser.LoadEnvSettings(devEnvPath)
	if err != nil {
		log.Printf("Warning: Could not load dev environment settings: %v", err)
	} else {
		envConfigs["dev"] = devEnv
	}

	prodEnvPath := filepath.Join(examplesDir, "env-settings", "prod-env.yaml")
	prodEnv, err := parser.LoadEnvSettings(prodEnvPath)
	if err != nil {
		log.Printf("Warning: Could not load prod environment settings: %v", err)
	} else {
		envConfigs["prod"] = prodEnv
	}

	// 6. Clean and recreate expected-output directory
	if err := os.RemoveAll(outputDir); err != nil {
		log.Fatalf("Failed to clean output directory: %v", err)
	}

	// 7. Render for each environment and stage
	for envName, envSettings := range envConfigs {
		envOutputDir := filepath.Join(outputDir, envName)
		if err := os.MkdirAll(envOutputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory %s: %v", envOutputDir, err)
		}

		fmt.Printf("\nRendering for environment: %s\n", envName)

		for _, stage := range stages {
			fmt.Printf("  Rendering %s...\n", stage.Name)

			resources, err := renderStage(ctd, component, addons, stage.AddonCount, envSettings, additionalCtx)
			if err != nil {
				log.Fatalf("Failed to render stage %s: %v", stage.Name, err)
			}

			// Write output
			outputPath := filepath.Join(envOutputDir, stage.Name+".yaml")
			if err := writeOutput(resources, outputPath); err != nil {
				log.Fatalf("Failed to write output to %s: %v", outputPath, err)
			}

			fmt.Printf("    Written to %s (%d resources)\n", outputPath, len(resources))
		}
	}

	fmt.Println("\n✅ Rendering complete!")
}

// generateStages dynamically generates stages from Component's addon list
func generateStages(component *types.Component) []types.Stage {
	stages := []types.Stage{
		{Name: "stage-1-base", AddonCount: 0},
	}

	// Map addon names to short names for cleaner output files
	shortNames := map[string]string{
		"persistent-volume-claim": "pvc",
		"sidecar-container":       "sidecar",
		"emptydir-volume":         "emptydir",
	}

	// Generate one stage per addon
	for i, addonInstance := range component.Spec.Addons {
		shortName := shortNames[addonInstance.Name]
		if shortName == "" {
			shortName = addonInstance.Name
		}

		stageName := fmt.Sprintf("stage-%d-with-%s", i+2, shortName)
		stages = append(stages, types.Stage{
			Name:       stageName,
			AddonCount: i + 1,
		})
	}

	return stages
}

// renderStage renders resources for a specific stage
func renderStage(
	ctd *types.ComponentTypeDefinition,
	component *types.Component,
	addons map[string]*types.Addon,
	addonCount int,
	envSettings *types.EnvSettings,
	additionalCtx *parser.AdditionalContext,
) ([]map[string]interface{}, error) {
	// 1. Build inputs by merging component and environment settings
	inputs := renderer.BuildInputs(component, envSettings, additionalCtx)

	// 2. Render base resources from ComponentTypeDefinition
	resources, err := renderer.RenderBaseResources(ctd, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to render base resources: %w", err)
	}

	// 3. Apply addons incrementally (0 to addonCount-1)
	for i := 0; i < addonCount; i++ {
		addonInstance := component.Spec.Addons[i]
		addon, ok := addons[addonInstance.Name]
		if !ok {
			return nil, fmt.Errorf("addon %s not found", addonInstance.Name)
		}

		// Build addon-specific inputs
		addonInputs := renderer.BuildAddonInputs(component, addonInstance, envSettings, additionalCtx)

		// Apply addon
		resources, err = renderer.ApplyAddon(resources, addon, addonInstance, addonInputs)
		if err != nil {
			return nil, fmt.Errorf("failed to apply addon %s: %w", addonInstance.Name, err)
		}
	}

	return resources, nil
}

// writeOutput writes resources to a YAML file
func writeOutput(resources []map[string]interface{}, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	// Write each resource as a separate YAML document
	for i, resource := range resources {
		// Debug: check for unexpected types in resource
		if err := checkForUnexpectedTypes(resource, ""); err != nil {
			return fmt.Errorf("unexpected type in resource %d: %w", i, err)
		}
		if err := encoder.Encode(resource); err != nil {
			return fmt.Errorf("failed to encode resource: %w", err)
		}
	}

	return nil
}

func checkForUnexpectedTypes(data interface{}, path string) error {
	switch v := data.(type) {
	case map[string]interface{}:
		for k, val := range v {
			newPath := path + "." + k
			if err := checkForUnexpectedTypes(val, newPath); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, val := range v {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if err := checkForUnexpectedTypes(val, newPath); err != nil {
				return err
			}
		}
	case string, int, int64, float64, bool, nil:
		// These are fine
	default:
		return fmt.Errorf("unexpected type %T at path %s: %+v", v, path, v)
	}
	return nil
}
