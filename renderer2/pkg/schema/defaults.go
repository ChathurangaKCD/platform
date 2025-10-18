package schema

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ComponentDefaults computes default values defined in a ComponentTypeDefinition schema.
func ComponentDefaults(schema types.Schema) (map[string]interface{}, error) {
	return computeDefaults(schema)
}

// AddonDefaults computes default values defined in an Addon schema.
func AddonDefaults(schema types.Schema) (map[string]interface{}, error) {
	return computeDefaults(schema)
}

func computeDefaults(spec types.Schema) (map[string]interface{}, error) {
	openAPISchema, err := JSONSchemaFromSpec(spec)
	if err != nil {
		return nil, err
	}

	defaults := extractObjectDefaults(openAPISchema)
	if defaults == nil {
		return map[string]interface{}{}, nil
	}

	result, ok := defaults.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, nil
	}
	return result, nil
}

// JSONSchemaFromSpec converts a schema definition into an OpenAPI JSONSchema object.
func JSONSchemaFromSpec(spec types.Schema) (*extv1.JSONSchemaProps, error) {
	merged := mergeSchemaSpec(spec)
	if len(merged) == 0 {
		return &extv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]extv1.JSONSchemaProps{},
		}, nil
	}

	jsonSchema, err := simpleschema.ToOpenAPISpec(merged, spec.Types)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to OpenAPI: %w", err)
	}

	sortRequiredFields(jsonSchema)
	return jsonSchema, nil
}

func extractObjectDefaults(schema *extv1.JSONSchemaProps) interface{} {
	if schema == nil {
		return nil
	}

	if schema.Default != nil {
		if value, err := decodeJSON(schema.Default.Raw); err == nil {
			return value
		}
	}

	if schema.Type == "object" {
		result := map[string]interface{}{}

		for key, prop := range schema.Properties {
			if value := extractObjectDefaults(&prop); value != nil {
				result[key] = value
			}
		}

		if len(result) > 0 {
			return result
		}
		return nil
	}

	if schema.Type == "array" && schema.Items != nil && schema.Items.Schema != nil {
		if value := extractObjectDefaults(schema.Items.Schema); value != nil {
			return []interface{}{value}
		}
	}

	if schema.Default != nil {
		if value, err := decodeJSON(schema.Default.Raw); err == nil {
			return value
		}
	}

	return nil
}

func decodeJSON(raw []byte) (interface{}, error) {
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mergeSchemaSpec(spec types.Schema) map[string]interface{} {
	merged := make(map[string]interface{})
	for key, value := range spec.Parameters {
		merged[key] = value
	}
	for key, value := range spec.EnvOverrides {
		merged[key] = value
	}
	return merged
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
