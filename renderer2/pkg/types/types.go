package types

// Metadata captures Kubernetes object metadata relevant for rendering.
type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// ComponentTypeDefinition declares a reusable workload definition.
type ComponentTypeDefinition struct {
	APIVersion string                      `yaml:"apiVersion"`
	Kind       string                      `yaml:"kind"`
	Metadata   Metadata                    `yaml:"metadata"`
	Spec       ComponentTypeDefinitionSpec `yaml:"spec"`
}

type ComponentTypeDefinitionSpec struct {
	WorkloadType string             `yaml:"workloadType"`
	Schema       Schema             `yaml:"schema"`
	Resources    []ResourceTemplate `yaml:"resources"`
}

type Schema struct {
	Types        map[string]interface{} `yaml:"types,omitempty"`
	Parameters   map[string]interface{} `yaml:"parameters,omitempty"`
	EnvOverrides map[string]interface{} `yaml:"envOverrides,omitempty"`
}

type ResourceTemplate struct {
	ID          string                 `yaml:"id"`
	IncludeWhen string                 `yaml:"includeWhen,omitempty"`
	ForEach     string                 `yaml:"forEach,omitempty"`
	Var         string                 `yaml:"var,omitempty"`
	Template    map[string]interface{} `yaml:"template"`
}

// Addon augments rendered workloads with additional resources or patches.
type Addon struct {
	APIVersion string    `yaml:"apiVersion"`
	Kind       string    `yaml:"kind"`
	Metadata   Metadata  `yaml:"metadata"`
	Spec       AddonSpec `yaml:"spec"`
}

type AddonSpec struct {
	DisplayName   string        `yaml:"displayName,omitempty"`
	Schema        Schema        `yaml:"schema"`
	Creates       []interface{} `yaml:"creates,omitempty"`
	Patches       []PatchSpec   `yaml:"patches,omitempty"`
	Documentation string        `yaml:"documentation,omitempty"`
}

type PatchSpec struct {
	ForEach    string               `yaml:"forEach,omitempty"`
	Var        string               `yaml:"var,omitempty"`
	Target     TargetSpec           `yaml:"target"`
	Operations []JSONPatchOperation `yaml:"operations"`
}

type TargetSpec struct {
	Kind    string `yaml:"kind,omitempty"`
	Group   string `yaml:"group,omitempty"`
	Version string `yaml:"version,omitempty"`
	Name    string `yaml:"name,omitempty"`
	Where   string `yaml:"where,omitempty"`
}

type JSONPatchOperation struct {
	Op    string      `yaml:"op"`
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value,omitempty"`
}

type Component struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   Metadata      `yaml:"metadata"`
	Spec       ComponentSpec `yaml:"spec"`
}

type ComponentSpec struct {
	ComponentType string                 `yaml:"componentType"`
	Parameters    map[string]interface{} `yaml:"parameters,omitempty"`
	Addons        []AddonInstance        `yaml:"addons,omitempty"`
	Build         BuildSpec              `yaml:"build,omitempty"`
}

type AddonInstance struct {
	Name       string                 `yaml:"name"`
	InstanceID string                 `yaml:"instanceId"`
	Config     map[string]interface{} `yaml:"config,omitempty"`
}

type BuildSpec struct {
	Image      string                 `yaml:"image,omitempty"`
	Repository RepositorySpec         `yaml:"repository,omitempty"`
	Template   map[string]interface{} `yaml:"templateRef,omitempty"`
}

type RepositorySpec struct {
	URL      string                 `yaml:"url,omitempty"`
	Revision map[string]interface{} `yaml:"revision,omitempty"`
	AppPath  string                 `yaml:"appPath,omitempty"`
}

type EnvSettings struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   Metadata        `yaml:"metadata"`
	Spec       EnvSettingsSpec `yaml:"spec"`
}

type EnvSettingsSpec struct {
	Environment    string                            `yaml:"environment"`
	Overrides      map[string]interface{}            `yaml:"overrides,omitempty"`
	AddonOverrides map[string]map[string]interface{} `yaml:"addonOverrides,omitempty"`
	Owner          *ComponentRef                     `yaml:"owner,omitempty"`
	ComponentRef   *ComponentRef                     `yaml:"componentRef,omitempty"`
}

type AdditionalContext struct {
	PodSelectors   map[string]string `json:"podSelectors,omitempty"`
	Build          BuildData         `json:"build,omitempty"`
	Configurations ConfigurationData `json:"configurations,omitempty"`
	Secrets        SecretData        `json:"secrets,omitempty"`
}

type BuildData struct {
	Image string `json:"image,omitempty"`
}

type ConfigurationData struct {
	Envs  []NameValuePair     `json:"envs,omitempty"`
	Files []ConfigurationFile `json:"files,omitempty"`
}

type SecretData struct {
	Envs  []SecretEnv  `json:"envs,omitempty"`
	Files []SecretFile `json:"files,omitempty"`
}

type NameValuePair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ConfigurationFile struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	Content   string `json:"content"`
}

type SecretEnv struct {
	Name     string `json:"name"`
	ValueRef string `json:"valueRef"`
}

type SecretFile struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ValueRef  string `json:"valueRef"`
}

type ComponentRef struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type Workload struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   Metadata               `yaml:"metadata"`
	Spec       map[string]interface{} `yaml:"spec"`
}

// Stage defines a rendering stage when progressively applying addons.
type Stage struct {
	Name       string
	AddonCount int
}
