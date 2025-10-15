package types

// Metadata represents Kubernetes object metadata
type Metadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// ComponentTypeDefinition represents a component type definition
type ComponentTypeDefinition struct {
	APIVersion string                         `yaml:"apiVersion"`
	Kind       string                         `yaml:"kind"`
	Metadata   Metadata                       `yaml:"metadata"`
	Spec       ComponentTypeDefinitionSpec    `yaml:"spec"`
}

// ComponentTypeDefinitionSpec defines the structure of a component type
type ComponentTypeDefinitionSpec struct {
	Schema    Schema             `yaml:"schema"`
	Resources []ResourceTemplate `yaml:"resources"`
}

// Schema defines the schema for component parameters
type Schema struct {
	Types        map[string]interface{} `yaml:"types,omitempty"`
	Parameters   map[string]interface{} `yaml:"parameters,omitempty"`
	EnvOverrides map[string]interface{} `yaml:"envOverrides,omitempty"`
}

// ResourceTemplate represents a template for a Kubernetes resource
type ResourceTemplate struct {
	ID        string                 `yaml:"id"`
	Condition string                 `yaml:"condition,omitempty"`
	ForEach   string                 `yaml:"forEach,omitempty"`
	Template  map[string]interface{} `yaml:"template"`
}

// Addon represents an addon definition
type Addon struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   Metadata   `yaml:"metadata"`
	Spec       AddonSpec  `yaml:"spec"`
}

// AddonSpec defines the structure of an addon
type AddonSpec struct {
	DisplayName string             `yaml:"displayName,omitempty"`
	Schema      Schema             `yaml:"schema"`
	Creates     []interface{}      `yaml:"creates,omitempty"`
	Patches     []PatchSpec        `yaml:"patches,omitempty"`
}

// PatchSpec defines a patch operation
type PatchSpec struct {
	ForEach string                 `yaml:"forEach,omitempty"`
	Target  TargetSpec             `yaml:"target"`
	Patch   Patch                  `yaml:"patch"`
}

// TargetSpec defines the target for a patch
type TargetSpec struct {
	ResourceType string `yaml:"resourceType,omitempty"`
	ResourceID   string `yaml:"resourceId,omitempty"`
	Selector     string `yaml:"selector,omitempty"`
}

// Patch defines the patch operation details
type Patch struct {
	Op    string      `yaml:"op"`
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value,omitempty"`
}

// Component represents a component instance
type Component struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   Metadata      `yaml:"metadata"`
	Spec       ComponentSpec `yaml:"spec"`
}

// ComponentSpec defines the component specification
type ComponentSpec struct {
	ComponentType string                 `yaml:"componentType"`
	Parameters    map[string]interface{} `yaml:"parameters,omitempty"`
	Addons        []AddonInstance        `yaml:"addons,omitempty"`
	Build         BuildSpec              `yaml:"build,omitempty"`
}

// AddonInstance represents an instance of an addon
type AddonInstance struct {
	Name       string                 `yaml:"name"`
	InstanceID string                 `yaml:"instanceId"`
	Config     map[string]interface{} `yaml:"config,omitempty"`
}

// BuildSpec defines the build specification
type BuildSpec struct {
	Image      string                 `yaml:"image,omitempty"`
	Repository RepositorySpec         `yaml:"repository,omitempty"`
	Template   map[string]interface{} `yaml:"templateRef,omitempty"`
}

// RepositorySpec defines repository information
type RepositorySpec struct {
	URL      string                 `yaml:"url,omitempty"`
	Revision map[string]interface{} `yaml:"revision,omitempty"`
	AppPath  string                 `yaml:"appPath,omitempty"`
}

// EnvSettings represents environment-specific settings
type EnvSettings struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   Metadata        `yaml:"metadata"`
	Spec       EnvSettingsSpec `yaml:"spec"`
}

// EnvSettingsSpec defines environment settings
type EnvSettingsSpec struct {
	Owner          ComponentRef           `yaml:"owner,omitempty"`
	ComponentRef   ComponentRef           `yaml:"componentRef,omitempty"`
	Environment    string                 `yaml:"environment"`
	Overrides      map[string]interface{} `yaml:"overrides,omitempty"`
	AddonOverrides map[string]map[string]interface{} `yaml:"addonOverrides,omitempty"`
}

// ComponentRef references a component
type ComponentRef struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

// Stage represents a rendering stage
type Stage struct {
	Name       string
	AddonCount int
}
