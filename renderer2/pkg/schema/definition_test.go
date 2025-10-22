package schema

import "testing"

func TestExtractDefaults_ArrayFieldBehaviour(t *testing.T) {
	def := Definition{
		Types: map[string]any{
			"Item": map[string]any{
				"name": "string | default=default-name",
			},
		},
		Schemas: []map[string]any{
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
		Types: map[string]any{
			"Item": map[string]any{
				"name": "string | default=default-name",
			},
		},
		Schemas: []map[string]any{
			{
				"list": "[]Item | default=[{\"name\":\"custom\"}]",
			},
		},
	}

	defaults, err = ExtractDefaults(defWithArrayDefault)
	if err != nil {
		t.Fatalf("ExtractDefaults returned error: %v", err)
	}
	got, ok := defaults["list"].([]any)
	if !ok {
		t.Fatalf("expected slice default, got %T (%v)", defaults["list"], defaults["list"])
	}
	if len(got) != 1 || got[0].(map[string]any)["name"] != "custom" {
		t.Fatalf("unexpected array default: %v", got)
	}
}
