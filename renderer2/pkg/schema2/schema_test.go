package schema2

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConverter_JSONMatchesExpected(t *testing.T) {
	const typesYAML = ``
	const schemaYAML = `
name: string
replicas: 'integer | default=1'
`
	const expected = `{
  "type": "object",
  "required": [
    "name"
  ],
  "properties": {
    "name": {
      "type": "string"
    },
    "replicas": {
      "type": "integer",
      "default": 1
    }
  }
}`

	assertConvertedSchema(t, typesYAML, schemaYAML, expected)
}

func TestConverter_ArrayDefaultParsing(t *testing.T) {
	const typesYAML = `
Item:
  name: 'string | default=default-name'
`
	const schemaYAML = `
items: '[]Item | default=[{"name":"custom"}]'
`
	const expected = `{
  "type": "object",
  "properties": {
    "items": {
      "type": "array",
      "default": [
        {
          "name": "custom"
        }
      ],
      "items": {
        "type": "object",
        "properties": {
          "name": {
            "type": "string",
            "default": "default-name"
          }
        }
      }
    }
  }
}`

	assertConvertedSchema(t, typesYAML, schemaYAML, expected)
}

func TestConverter_DefaultRequiredBehaviour(t *testing.T) {
	const typesYAML = ``
	const schemaYAML = `
mustProvide: string
hasDefault: 'integer | default=5'
explicitOpt: 'boolean | required=false'
`
	const expected = `{
  "type": "object",
  "required": [
    "mustProvide"
  ],
  "properties": {
    "explicitOpt": {
      "type": "boolean"
    },
    "hasDefault": {
      "type": "integer",
      "default": 5
    },
    "mustProvide": {
      "type": "string"
    }
  }
}`

	assertConvertedSchema(t, typesYAML, schemaYAML, expected)
}

func TestConverter_CustomTypeJSONMatchesExpected(t *testing.T) {
	const typesYAML = `
Resources:
  cpu: 'string | default=100m'
  memory: string
`
	const schemaYAML = `
resources: Resources
`
	const expected = `{
  "type": "object",
  "required": [
    "resources"
  ],
  "properties": {
    "resources": {
      "type": "object",
      "required": [
        "memory"
      ],
      "properties": {
        "cpu": {
          "type": "string",
          "default": "100m"
        },
        "memory": {
          "type": "string"
        }
      }
    }
  }
}`

	assertConvertedSchema(t, typesYAML, schemaYAML, expected)
}

func assertSchemaJSON(t *testing.T, schema interface{}, expected string) {
	t.Helper()

	actualBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	if string(actualBytes) != expected {
		t.Fatalf("schema JSON mismatch\nexpected:\n%s\nactual:\n%s", expected, string(actualBytes))
	}
}

func assertConvertedSchema(t *testing.T, typesYAML, schemaYAML, expected string) {
	t.Helper()

	var types map[string]interface{}
	if strings.TrimSpace(typesYAML) != "" {
		types = parseYAMLMap(t, typesYAML)
	}
	root := parseYAMLMap(t, schemaYAML)

	converter := NewConverter(types)
	schema, err := converter.Convert(root)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	assertSchemaJSON(t, schema, expected)
}

func parseYAMLMap(t *testing.T, doc string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := yaml.Unmarshal([]byte(doc), &out); err != nil {
		t.Fatalf("failed to parse yaml: %v", err)
	}
	return out
}
