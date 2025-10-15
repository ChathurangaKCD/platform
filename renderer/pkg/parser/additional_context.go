package parser

import (
	"encoding/json"
	"fmt"
	"os"
)

// AdditionalContext represents platform-injected context
type AdditionalContext struct {
	PodSelectors   map[string]string `json:"podSelectors"`
	Build          BuildData         `json:"build"`
	Configurations ConfigurationData `json:"configurations"`
	Secrets        SecretData        `json:"secrets"`
}

// BuildData represents build information
type BuildData struct {
	Image string `json:"image"`
}

// ConfigurationData represents configuration data (envs and files)
type ConfigurationData struct {
	Envs  []EnvVar       `json:"envs"`
	Files []ConfigFile   `json:"files"`
}

// SecretData represents secret references (envs and files)
type SecretData struct {
	Envs  []SecretEnvVar  `json:"envs"`
	Files []SecretFile    `json:"files"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ConfigFile represents a config file to mount
type ConfigFile struct {
	MountPath string `json:"mountPath"`
	Content   string `json:"content"`
}

// SecretEnvVar represents a secret environment variable reference
type SecretEnvVar struct {
	Name     string `json:"name"`
	ValueRef string `json:"valueRef"`
}

// SecretFile represents a secret file reference to mount
type SecretFile struct {
	MountPath string `json:"mountPath"`
	ValueRef  string `json:"valueRef"`
}

// LoadAdditionalContext loads additional context from a JSON file
func LoadAdditionalContext(path string) (*AdditionalContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read additional context file: %w", err)
	}

	var ctx AdditionalContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal additional context: %w", err)
	}

	return &ctx, nil
}
