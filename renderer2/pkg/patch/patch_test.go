package patch

import (
	"encoding/json"
	"testing"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/yaml"
)

func TestApplyPatch(t *testing.T) {
	t.Parallel()

	render := func(v interface{}, _ map[string]interface{}) (interface{}, error) {
		return v, nil
	}

	tests := []struct {
		name       string
		initial    string
		operations []types.JSONPatchOperation
		want       string
	}{
		{
			name: "add env entry via array filter",
			initial: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          image: app:v1
          env:
            - name: A
              value: "1"
`,
			operations: []types.JSONPatchOperation{
				{
					Op:   "add",
					Path: "/spec/template/spec/containers/[?(@.name=='app')]/env/-",
					Value: map[string]interface{}{
						"name":  "B",
						"value": "2",
					},
				},
			},
			want: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          image: app:v1
          env:
            - name: A
              value: "1"
            - name: B
              value: "2"
`,
		},
		{
			name: "replace image using index path",
			initial: `
spec:
  template:
    spec:
      containers:
        - name: app
          image: app:v1
`,
			operations: []types.JSONPatchOperation{
				{
					Op:    "replace",
					Path:  "/spec/template/spec/containers/0/image",
					Value: "app:v2",
				},
			},
			want: `
spec:
  template:
    spec:
      containers:
        - name: app
          image: app:v2
`,
		},
		{
			name: "remove first env entry",
			initial: `
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: A
              value: "1"
            - name: B
              value: "2"
`,
			operations: []types.JSONPatchOperation{
				{
					Op:   "remove",
					Path: "/spec/template/spec/containers/[?(@.name=='app')]/env/0",
				},
			},
			want: `
spec:
  template:
    spec:
      containers:
        - name: app
          env:
            - name: B
              value: "2"
`,
		},
		{
			name: "merge annotations without clobbering existing",
			initial: `
spec:
  template:
    metadata:
      annotations:
        existing: "true"
`,
			operations: []types.JSONPatchOperation{
				{
					Op:   "merge",
					Path: "/spec/template/metadata/annotations",
					Value: map[string]interface{}{
						"platform": "enabled",
					},
				},
			},
			want: `
spec:
  template:
    metadata:
      annotations:
        existing: "true"
        platform: enabled
`,
		},
		{
			name: "test operation success",
			initial: `
spec:
  template:
    metadata:
      annotations:
        existing: "true"
`,
			operations: []types.JSONPatchOperation{
				{
					Op:    "test",
					Path:  "/spec/template/metadata/annotations/existing",
					Value: "true",
				},
				{
					Op:    "replace",
					Path:  "/spec/template/metadata/annotations/existing",
					Value: "updated",
				},
			},
			want: `
spec:
  template:
    metadata:
      annotations:
        existing: updated
`,
		},
		{
			name: "add env entry for multiple matches",
			initial: `
spec:
  template:
    spec:
      containers:
        - name: app
          role: worker
          env: []
        - name: logger
          role: worker
          env: []
`,
			operations: []types.JSONPatchOperation{
				{
					Op:   "add",
					Path: "/spec/template/spec/containers/[?(@.role=='worker')]/env/-",
					Value: map[string]interface{}{
						"name":  "SHARED",
						"value": "true",
					},
				},
			},
			want: `
spec:
  template:
    spec:
      containers:
        - name: app
          role: worker
          env:
            - name: SHARED
              value: "true"
        - name: logger
          role: worker
          env:
            - name: SHARED
              value: "true"
`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var resource map[string]interface{}
			if err := yaml.Unmarshal([]byte(tt.initial), &resource); err != nil {
				t.Fatalf("failed to unmarshal initial YAML: %v", err)
			}

			for _, op := range tt.operations {
				if err := ApplyOperation(resource, op, nil, render); err != nil {
					t.Fatalf("ApplyOperation error = %v", err)
				}
			}

			var wantObj map[string]interface{}
			if err := yaml.Unmarshal([]byte(tt.want), &wantObj); err != nil {
				t.Fatalf("failed to unmarshal expected YAML: %v", err)
			}

			if diff := cmpDiff(wantObj, resource); diff != "" {
				t.Fatalf("resource mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestApplyPatchTestOpFailure(t *testing.T) {
	render := func(v interface{}, _ map[string]interface{}) (interface{}, error) {
		return v, nil
	}

	initial := `
spec:
  template:
    metadata:
      annotations:
        existing: "true"
`

	var resource map[string]interface{}
	if err := yaml.Unmarshal([]byte(initial), &resource); err != nil {
		t.Fatalf("failed to unmarshal initial YAML: %v", err)
	}

	op := types.JSONPatchOperation{
		Op:    "test",
		Path:  "/spec/template/metadata/annotations/existing",
		Value: "false",
	}

	if err := ApplyOperation(resource, op, nil, render); err == nil {
		t.Fatalf("expected test operation to fail but succeeded")
	}
}

func cmpDiff(expected, actual map[string]interface{}) string {
	wantJSON, _ := json.Marshal(expected)
	gotJSON, _ := json.Marshal(actual)

	var wantNorm, gotNorm interface{}
	_ = json.Unmarshal(wantJSON, &wantNorm)
	_ = json.Unmarshal(gotJSON, &gotNorm)

	if diff := cmp.Diff(wantNorm, gotNorm); diff != "" {
		return diff
	}
	return ""
}
