package parser

import (
	"fmt"
	"os"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadEnvSettings reads EnvSettings from YAML.
func LoadEnvSettings(path string) (*types.EnvSettings, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read env settings: %w", err)
	}

	var env types.EnvSettings
	if err := yaml.Unmarshal(content, &env); err != nil {
		return nil, fmt.Errorf("failed to parse env settings: %w", err)
	}

	return &env, nil
}
