package schema

import (
	"fmt"
	"sort"

	"github.com/chathurangada/cel_playground/renderer2/pkg/schemaextractor"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
)

// Definition represents a schematized object assembled from one or more field maps.
type Definition struct {
	Types   map[string]interface{}
	Schemas []map[string]interface{}
}

// ToJSONSchema converts the definition into an OpenAPI-compatible JSON schema.
func ToJSONSchema(def Definition) (*extv1.JSONSchemaProps, error) {
	merged := mergeFieldMaps(def.Schemas)
	if len(merged) == 0 {
		return &extv1.JSONSchemaProps{
			Type:       "object",
			Properties: map[string]extv1.JSONSchemaProps{},
		}, nil
	}

	converter := schemaextractor.NewConverter(def.Types)
	jsonSchema, err := converter.Convert(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to OpenAPI: %w", err)
	}

	sortRequiredFields(jsonSchema)
	return jsonSchema, nil
}

// ExtractDefaults traverses the definition and returns its default values as a map.
func ExtractDefaults(def Definition) (map[string]interface{}, error) {
	jsonSchemaV1, err := ToJSONSchema(def)
	if err != nil {
		return nil, err
	}

	internal := new(apiext.JSONSchemaProps)
	if err := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(jsonSchemaV1, internal, nil); err != nil {
		return nil, fmt.Errorf("failed to convert schema: %w", err)
	}

	structural, err := apiextschema.NewStructural(internal)
	if err != nil {
		return nil, fmt.Errorf("failed to build structural schema: %w", err)
	}

	result := map[string]interface{}{}
	defaulting.Default(result, structural)
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
