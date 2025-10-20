package schema

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kubernetes-sigs/kro/pkg/simpleschema"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Definition represents a schematized object assembled from one or more field maps.
type Definition struct {
	Types   map[string]interface{}
	Schemas []map[string]interface{}
}

// ToOpenAPISchema converts the definition into an OpenAPI schema.
func ToOpenAPISchema(def Definition) (*extv1.JSONSchemaProps, error) {
	merged := mergeFieldMaps(def.Schemas)
	if len(merged) == 0 {
		return &extv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]extv1.JSONSchemaProps{},
		}, nil
	}

	jsonSchema, err := simpleschema.ToOpenAPISpec(merged, def.Types)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to OpenAPI: %w", err)
	}

	sortRequiredFields(jsonSchema)
	return jsonSchema, nil
}

// ExtractDefaults traverses the definition and returns its default values as a map.
func ExtractDefaults(def Definition) (map[string]interface{}, error) {
	schema, err := ToOpenAPISchema(def)
	if err != nil {
		return nil, err
	}

	defaults := extractObjectDefaults(schema)
	if defaults == nil {
		return map[string]interface{}{}, nil
	}

	result, ok := defaults.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}, nil
	}
	return result, nil
}

func mergeFieldMaps(maps []map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for _, fields := range maps {
		mergeInto(result, fields)
	}
	return result
}

func mergeInto(dst map[string]interface{}, src map[string]interface{}) {
	if src == nil {
		return
	}
	if dst == nil {
		// should not happen, but guard anyway
		return
	}
	for k, v := range src {
		if vMap, ok := v.(map[string]interface{}); ok {
			existing, ok := dst[k].(map[string]interface{})
			if !ok {
				dst[k] = deepCopyMap(vMap)
				continue
			}
			mergeInto(existing, vMap)
			continue
		}
		dst[k] = deepCopyValue(v)
	}
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(src))
	for k, v := range src {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v interface{}) interface{} {
	switch typed := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(typed)
	case []interface{}:
		result := make([]interface{}, len(typed))
		for i, item := range typed {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		return typed
	}
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
