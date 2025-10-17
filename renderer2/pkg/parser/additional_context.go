package parser

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chathurangada/cel_playground/renderer2/pkg/types"
)

// LoadAdditionalContext loads additional renderer context data from JSON.
func LoadAdditionalContext(path string) (*types.AdditionalContext, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read additional context: %w", err)
	}

	var ctx types.AdditionalContext
	if err := json.Unmarshal(content, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse additional context: %w", err)
	}

	return &ctx, nil
}
