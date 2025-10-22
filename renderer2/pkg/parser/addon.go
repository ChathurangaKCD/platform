package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadAddons loads addon definitions from the provided directory. If names are supplied,
// the returned map only includes those addons; otherwise, all discovered addons are returned.
func LoadAddons(dir string, names []string) (map[string]*types.Addon, error) {
	discovered, err := loadAllAddons(dir)
	if err != nil {
		return nil, err
	}

	if len(names) == 0 {
		return discovered, nil
	}

	result := make(map[string]*types.Addon, len(names))
	for _, name := range names {
		addon, ok := discovered[name]
		if !ok {
			return nil, fmt.Errorf("addon %s not found in %s", name, dir)
		}
		result[name] = addon
	}
	return result, nil
}

func loadAllAddons(dir string) (map[string]*types.Addon, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read addon directory %s: %w", dir, err)
	}

	addons := make(map[string]*types.Addon)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read addon file %s: %w", path, err)
		}

		var addon types.Addon
		if err := yaml.Unmarshal(content, &addon); err != nil {
			return nil, fmt.Errorf("failed to parse addon file %s: %w", path, err)
		}

		if addon.Metadata.Name == "" {
			return nil, fmt.Errorf("addon file %s missing metadata.name", path)
		}

		addons[addon.Metadata.Name] = &addon
	}

	return addons, nil
}
