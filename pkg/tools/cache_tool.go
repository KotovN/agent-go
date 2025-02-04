package tools

import (
	"encoding/json"
	"fmt"

	"github.com/KotovN/agent-go/pkg/agents/cache"
	"github.com/KotovN/agent-go/pkg/core"
)

// CacheTool provides direct access to the cache
type CacheTool struct {
	cacheHandler *cache.CacheHandler
}

// NewCacheTool creates a new cache tool instance
func NewCacheTool(cacheHandler *cache.CacheHandler) *CacheTool {
	return &CacheTool{
		cacheHandler: cacheHandler,
	}
}

func (t *CacheTool) Name() string {
	return "cache"
}

func (t *CacheTool) Description() string {
	return "Read cached results from previous tool executions"
}

type CacheInput struct {
	Tool  string `json:"tool"`
	Input string `json:"input"`
}

func (t *CacheTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"tool": {
			Type:        "string",
			Description: "The name of the tool whose cache to read",
			Required:    true,
		},
		"input": {
			Type:        "string",
			Description: "The input that was used with the tool",
			Required:    true,
		},
	}
}

func (t *CacheTool) Execute(input string) (string, error) {
	var params CacheInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %v", err)
	}

	if params.Tool == "" || params.Input == "" {
		return "", fmt.Errorf("both tool and input parameters are required")
	}

	result, exists := t.cacheHandler.Read(params.Tool, params.Input)
	if !exists {
		return "", fmt.Errorf("no cached result found for tool '%s' with input '%s'", params.Tool, params.Input)
	}

	return result, nil
}
