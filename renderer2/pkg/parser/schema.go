package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// GenerateJSONSchema converts a ComponentTypeDefinition schema into OpenAPI v3 JSONSchema.
func GenerateJSONSchema(ctd *types.ComponentTypeDefinition) (*extv1.JSONSchemaProps, error) {
	merged := make(map[string]interface{})
	for key, value := range ctd.Spec.Schema.Parameters {
		merged[key] = value
	}
	for key, value := range ctd.Spec.Schema.EnvOverrides {
		merged[key] = value
	}

	jsonSchema, err := simpleschema.ToOpenAPISpec(merged, ctd.Spec.Schema.Types)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ComponentTypeDefinition schema: %w", err)
	}

	sortRequiredFields(jsonSchema)
	return jsonSchema, nil
}

// GenerateAddonJSONSchema converts an Addon schema into OpenAPI v3 JSONSchema.
func GenerateAddonJSONSchema(addon *types.Addon) (*extv1.JSONSchemaProps, error) {
	merged := make(map[string]interface{})
	for key, value := range addon.Spec.Schema.Parameters {
		merged[key] = value
	}
	for key, value := range addon.Spec.Schema.EnvOverrides {
		merged[key] = value
	}

	jsonSchema, err := simpleschema.ToOpenAPISpec(merged, addon.Spec.Schema.Types)
	if err != nil {
		return nil, fmt.Errorf("failed to generate addon schema: %w", err)
	}

	sortRequiredFields(jsonSchema)
	return jsonSchema, nil
}

// ValidateSchemas generates JSON Schemas for the component definition and addons and writes them to disk.
func ValidateSchemas(ctd *types.ComponentTypeDefinition, addons map[string]*types.Addon, outputDir string) error {
	fmt.Println("\n=== Generating JSON Schemas ===")

	ctdSchema, err := GenerateJSONSchema(ctd)
	if err != nil {
		return err
	}

	if err := printSchema(ctd.Metadata.Name, ctdSchema); err != nil {
		return err
	}
	if err := WriteSchemaToFile(ctdSchema, outputDir, ctd.Metadata.Name+"-schema.json"); err != nil {
		return err
	}

	for name, addon := range addons {
		addonSchema, err := GenerateAddonJSONSchema(addon)
		if err != nil {
			return err
		}
		if err := printSchema(name, addonSchema); err != nil {
			return err
		}
		if err := WriteSchemaToFile(addonSchema, outputDir, name+"-schema.json"); err != nil {
			return err
		}
	}

	return nil
}

// WriteSchemaToFile saves the given schema to the provided directory.
func WriteSchemaToFile(schema *extv1.JSONSchemaProps, outputDir, filename string) error {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create schema directory: %w", err)
	}

	path := filepath.Join(outputDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}
	fmt.Printf("  → Written to %s\n", path)
	return nil
}

func printSchema(name string, schema *extv1.JSONSchemaProps) error {
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	fmt.Printf("%s JSON Schema:\n%s\n\n", name, string(data))
	return nil
}

func sortRequiredFields(schema *extv1.JSONSchemaProps) {
	if schema == nil {
		return
	}

	if len(schema.Required) > 0 {
		sort.Strings(schema.Required)
	}

	if schema.Properties != nil {
		for _, prop := range schema.Properties {
			sortRequiredFields(&prop)
		}
	}

	if schema.Items != nil && schema.Items.Schema != nil {
		sortRequiredFields(schema.Items.Schema)
	}

	if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
		sortRequiredFields(schema.AdditionalProperties.Schema)
	}
}
