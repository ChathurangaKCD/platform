package parser

import (
	"fmt"
	"os"

	"github.com/chathurangada/cel_playground/renderer/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadComponentTypeDefinition loads a ComponentTypeDefinition from a YAML file
func LoadComponentTypeDefinition(path string) (*types.ComponentTypeDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read component type definition file: %w", err)
	}

	var ctd types.ComponentTypeDefinition
	if err := yaml.Unmarshal(data, &ctd); err != nil {
		return nil, fmt.Errorf("failed to parse component type definition: %w", err)
	}

	return &ctd, nil
}

// LoadComponent loads a Component from a YAML file
func LoadComponent(path string) (*types.Component, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read component file: %w", err)
	}

	var component types.Component
	if err := yaml.Unmarshal(data, &component); err != nil {
		return nil, fmt.Errorf("failed to parse component: %w", err)
	}

	return &component, nil
}
