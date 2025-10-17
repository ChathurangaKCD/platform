package parser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
	"gopkg.in/yaml.v3"
)

// LoadAddons loads addon definitions by name from a directory.
func LoadAddons(dir string, names []string) (map[string]*types.Addon, error) {
	result := make(map[string]*types.Addon, len(names))

	for _, name := range names {
		var (
			content []byte
			err     error
			path    string
		)

		candidates := []string{
			fmt.Sprintf("%s-addon.yaml", name),
			fmt.Sprintf("%s.yaml", name),
		}

		for _, candidate := range candidates {
			path = filepath.Join(dir, candidate)
			content, err = os.ReadFile(path)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read addon %s: %w", name, err)
		}

		var addon types.Addon
		if err := yaml.Unmarshal(content, &addon); err != nil {
			return nil, fmt.Errorf("failed to parse addon %s: %w", name, err)
		}
		result[name] = &addon
	}

	return result, nil
}
