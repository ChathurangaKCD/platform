package parser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chathurangada/cel_playground/renderer/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadAddon loads an Addon definition from a YAML file
func LoadAddon(path string) (*types.Addon, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read addon file: %w", err)
	}

	var addon types.Addon
	if err := yaml.Unmarshal(data, &addon); err != nil {
		return nil, fmt.Errorf("failed to parse addon: %w", err)
	}

	return &addon, nil
}

// LoadAddons loads multiple addon definitions from a directory
func LoadAddons(addonDir string, addonNames []string) (map[string]*types.Addon, error) {
	addons := make(map[string]*types.Addon)

	for _, name := range addonNames {
		// Try with -addon.yaml suffix first
		addonPath := filepath.Join(addonDir, name+"-addon.yaml")
		addon, err := LoadAddon(addonPath)
		if err != nil {
			// Try without suffix
			addonPath = filepath.Join(addonDir, name+".yaml")
			addon, err = LoadAddon(addonPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load addon %s: %w", name, err)
			}
		}
		addons[name] = addon
	}

	return addons, nil
}
