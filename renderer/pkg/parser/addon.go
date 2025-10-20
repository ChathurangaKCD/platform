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
	// Map addon names to file names
	nameToFile := map[string]string{
		"persistent-volume-claim": "pvc-addon.yaml",
		"sidecar-container":       "sidecar-addon.yaml",
		"emptydir-volume":         "emptydir-addon.yaml",
	}

	addons := make(map[string]*types.Addon)

	for _, name := range addonNames {
		var addonPath string

		// Check if there's a mapping for this addon name
		if fileName, ok := nameToFile[name]; ok {
			addonPath = filepath.Join(addonDir, fileName)
		} else {
			// Try with -addon.yaml suffix
			addonPath = filepath.Join(addonDir, name+"-addon.yaml")
		}

		addon, err := LoadAddon(addonPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load addon %s from %s: %w", name, addonPath, err)
		}
		addons[name] = addon
	}

	return addons, nil
}
