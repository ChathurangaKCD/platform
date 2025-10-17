package schema

import (
	"encoding/json"
	"fmt"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ComponentDefaults computes default values defined in a ComponentTypeDefinition schema.
func ComponentDefaults(schema types.Schema) (map[string]interface{}, error) {
	return computeDefaults(schema.Parameters, schema.EnvOverrides, schema.Types)
}

// AddonDefaults computes default values defined in an Addon schema.
func AddonDefaults(schema types.Schema) (map[string]interface{}, error) {
	return computeDefaults(schema.Parameters, schema.EnvOverrides, schema.Types)
}

func computeDefaults(parameters map[string]interface{}, envOverrides map[string]interface{}, customTypes map[string]interface{}) (map[string]interface{}, error) {
	merged := make(map[string]interface{})
	for key, value := range parameters {
		merged[key] = value
	}
	for key, value := range envOverrides {
		merged[key] = value
	}

	if len(merged) == 0 {
		return map[string]interface{}{}, nil
	}

	jsonSchema, err := simpleschema.ToOpenAPISpec(merged, customTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to OpenAPI: %w", err)
	}

	defaults := extractObjectDefaults(jsonSchema)
	if defaults == nil {
		return map[string]interface{}{}, nil
	}

	result, ok := defaults.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, nil
	}
	return result, nil
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
