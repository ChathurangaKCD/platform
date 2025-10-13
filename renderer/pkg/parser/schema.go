package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/chathurangada/cel_playground/renderer/pkg/types"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// GenerateJSONSchema generates OpenAPI v3 JSON schemas from ComponentTypeDefinition
func GenerateJSONSchema(ctd *types.ComponentTypeDefinition) (*extv1.JSONSchemaProps, error) {
	// Merge parameters and envOverrides into a single schema object
	mergedSchema := make(map[string]interface{})

	// Add parameters
	for k, v := range ctd.Spec.Schema.Parameters {
		mergedSchema[k] = v
	}

	// Add envOverrides
	for k, v := range ctd.Spec.Schema.EnvOverrides {
		mergedSchema[k] = v
	}

	// Use kro's simpleschema to transform to OpenAPI schema
	jsonSchema, err := simpleschema.ToOpenAPISpec(
		mergedSchema,
		ctd.Spec.Schema.Types,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JSON schema: %w", err)
	}

	// Sort required fields alphabetically
	sortRequiredFields(jsonSchema)

	return jsonSchema, nil
}

// GenerateAddonJSONSchema generates OpenAPI v3 JSON schema for an addon
func GenerateAddonJSONSchema(addon *types.Addon) (*extv1.JSONSchemaProps, error) {
	// Extract the schema
	schemaObj := make(map[string]interface{})

	// Merge parameters and envOverrides
	mergedSchema := make(map[string]interface{})

	// Add parameters
	for k, v := range addon.Spec.Schema.Parameters {
		mergedSchema[k] = v
	}

	// Add envOverrides
	for k, v := range addon.Spec.Schema.EnvOverrides {
		mergedSchema[k] = v
	}

	schemaObj = mergedSchema

	// Use kro's simpleschema to transform to OpenAPI schema
	jsonSchema, err := simpleschema.ToOpenAPISpec(
		schemaObj,
		addon.Spec.Schema.Types,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate addon JSON schema: %w", err)
	}

	// Sort required fields alphabetically
	sortRequiredFields(jsonSchema)

	return jsonSchema, nil
}

// ValidateSchemas validates component and addon schemas and outputs JSON schemas
func ValidateSchemas(ctd *types.ComponentTypeDefinition, addons map[string]*types.Addon, outputDir string) error {
	fmt.Println("\n=== Generating JSON Schemas ===\n")

	// Generate ComponentTypeDefinition schema
	ctdSchema, err := GenerateJSONSchema(ctd)
	if err != nil {
		return fmt.Errorf("failed to generate ComponentTypeDefinition schema: %w", err)
	}

	// Pretty print the schema
	fmt.Printf("ComponentTypeDefinition (%s) JSON Schema:\n", ctd.Metadata.Name)
	schemaJSON, err := json.MarshalIndent(ctdSchema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	fmt.Printf("%s\n\n", string(schemaJSON))

	// Write ComponentTypeDefinition schema to file
	if err := WriteSchemaToFile(ctdSchema, outputDir, ctd.Metadata.Name+"-schema.json"); err != nil {
		return fmt.Errorf("failed to write ComponentTypeDefinition schema: %w", err)
	}

	// Generate addon schemas
	for name, addon := range addons {
		addonSchema, err := GenerateAddonJSONSchema(addon)
		if err != nil {
			return fmt.Errorf("failed to generate schema for addon %s: %w", name, err)
		}

		fmt.Printf("Addon (%s) JSON Schema:\n", name)
		addonSchemaJSON, err := json.MarshalIndent(addonSchema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal addon schema: %w", err)
		}
		fmt.Printf("%s\n\n", string(addonSchemaJSON))

		// Write addon schema to file
		if err := WriteSchemaToFile(addonSchema, outputDir, name+"-schema.json"); err != nil {
			return fmt.Errorf("failed to write addon schema for %s: %w", name, err)
		}
	}

	return nil
}

// WriteSchemaToFile writes a JSON schema to a file
func WriteSchemaToFile(schema *extv1.JSONSchemaProps, outputDir, filename string) error {
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(outputPath, schemaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	fmt.Printf("  → Written to %s\n", outputPath)
	return nil
}

// sortRequiredFields recursively sorts all required fields alphabetically in a schema
func sortRequiredFields(schema *extv1.JSONSchemaProps) {
	if schema == nil {
		return
	}

	// Sort required fields at current level
	if len(schema.Required) > 0 {
		sort.Strings(schema.Required)
	}

	// Recursively sort required fields in nested properties
	if schema.Properties != nil {
		for _, prop := range schema.Properties {
			sortRequiredFields(&prop)
		}
	}

	// Recursively sort required fields in array items
	if schema.Items != nil && schema.Items.Schema != nil {
		sortRequiredFields(schema.Items.Schema)
	}

	// Recursively sort required fields in additionalProperties
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		sortRequiredFields(schema.AdditionalProperties.Schema)
	}
}
