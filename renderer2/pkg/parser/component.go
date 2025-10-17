package parser

import (
	"fmt"
	"os"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadComponentTypeDefinition reads a ComponentTypeDefinition YAML file.
func LoadComponentTypeDefinition(path string) (*types.ComponentTypeDefinition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read component type definition: %w", err)
	}

	var ctd types.ComponentTypeDefinition
	if err := yaml.Unmarshal(content, &ctd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal component type definition: %w", err)
	}

	return &ctd, nil
}

// LoadComponent reads a Component YAML file.
func LoadComponent(path string) (*types.Component, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read component: %w", err)
	}

	var component types.Component
	if err := yaml.Unmarshal(content, &component); err != nil {
		return nil, fmt.Errorf("failed to unmarshal component: %w", err)
	}

	return &component, nil
}
