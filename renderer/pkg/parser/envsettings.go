package parser

import (
	"fmt"
	"os"

	"github.com/chathurangada/cel_playground/renderer/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadEnvSettings loads EnvSettings from a YAML file
func LoadEnvSettings(path string) (*types.EnvSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read env settings file: %w", err)
	}

	var envSettings types.EnvSettings
	if err := yaml.Unmarshal(data, &envSettings); err != nil {
		return nil, fmt.Errorf("failed to parse env settings: %w", err)
	}

	return &envSettings, nil
}
