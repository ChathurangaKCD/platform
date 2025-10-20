package schema

import "testing"

func TestExtractDefaults_ArrayFieldBehaviour(t *testing.T) {
	def := Definition{
		Types: map[string]interface{}{
			"Item": map[string]interface{}{
				"name": "string | default=default-name",
			},
		},
		Schemas: []map[string]interface{}{
			{
				"list": "[]Item",
			},
		},
	}

	defaults, err := ExtractDefaults(def)
	if err != nil {
		t.Fatalf("ExtractDefaults returned error: %v", err)
	}
	if _, ok := defaults["list"]; ok {
		t.Fatalf("expected no default array elements when only item defaults are present, got %v", defaults["list"])
	}

	defWithArrayDefault := Definition{
		Types: map[string]interface{}{
			"Item": map[string]interface{}{
				"name": "string | default=default-name",
			},
		},
		Schemas: []map[string]interface{}{
			{
				"list": "[]Item | default=[{\"name\":\"custom\"}]",
			},
		},
	}

	defaults, err = ExtractDefaults(defWithArrayDefault)
	if err != nil {
		t.Fatalf("ExtractDefaults returned error: %v", err)
	}
	got, ok := defaults["list"].([]interface{})
	if !ok {
		t.Fatalf("expected slice default, got %T (%v)", defaults["list"], defaults["list"])
	}
	if len(got) != 1 || got[0].(map[string]interface{})["name"] != "custom" {
		t.Fatalf("unexpected array default: %v", got)
	}
}
