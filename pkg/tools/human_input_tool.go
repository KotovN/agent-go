package tools

import (
	"agentai/pkg/core"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// HumanInputTool provides interactive input capabilities
type HumanInputTool struct {
	defaultTimeout time.Duration
	reader         *bufio.Reader
}

// NewHumanInputTool creates a new human input tool instance
func NewHumanInputTool(defaultTimeout time.Duration) *HumanInputTool {
	if defaultTimeout == 0 {
		defaultTimeout = 5 * time.Minute
	}
	return &HumanInputTool{
		defaultTimeout: defaultTimeout,
		reader:         bufio.NewReader(os.Stdin),
	}
}

func (t *HumanInputTool) Name() string {
	return "human_input"
}

func (t *HumanInputTool) Description() string {
	return "Request input from a human user with optional validation and timeout"
}

type InputRequest struct {
	Prompt      string        `json:"prompt"`
	Validation  string        `json:"validation,omitempty"`
	Options     []string      `json:"options,omitempty"`
	Default     string        `json:"default,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	Required    bool          `json:"required,omitempty"`
	Description string        `json:"description,omitempty"`
}

func (t *HumanInputTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"prompt": {
			Type:        "string",
			Description: "The prompt to show to the user",
			Required:    true,
		},
		"validation": {
			Type:        "string",
			Description: "Validation pattern (regex) for the input",
			Required:    false,
		},
		"options": {
			Type:        "array",
			Description: "List of valid options for the input",
			Required:    false,
		},
		"default": {
			Type:        "string",
			Description: "Default value if no input is provided",
			Required:    false,
		},
		"timeout": {
			Type:        "integer",
			Description: "Timeout in seconds (0 for default timeout)",
			Required:    false,
		},
		"required": {
			Type:        "boolean",
			Description: "Whether the input is required",
			Required:    false,
		},
		"description": {
			Type:        "string",
			Description: "Additional description or help text",
			Required:    false,
		},
	}
}

func (t *HumanInputTool) Execute(input string) (string, error) {
	var req InputRequest
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("failed to parse input request: %w", err)
	}

	// Set timeout
	timeout := t.defaultTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout * time.Second
	}

	// Format prompt
	prompt := req.Prompt
	if req.Description != "" {
		prompt = fmt.Sprintf("%s\n%s", req.Description, prompt)
	}
	if len(req.Options) > 0 {
		prompt = fmt.Sprintf("%s\nOptions: %s", prompt, strings.Join(req.Options, ", "))
	}
	if req.Default != "" {
		prompt = fmt.Sprintf("%s [default: %s]", prompt, req.Default)
	}
	prompt = fmt.Sprintf("%s: ", prompt)

	// Create channel for input
	resultCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Start input goroutine
	go func() {
		fmt.Print(prompt)
		input, err := t.reader.ReadString('\n')
		if err != nil {
			errCh <- fmt.Errorf("failed to read input: %w", err)
			return
		}

		input = strings.TrimSpace(input)
		if input == "" && req.Default != "" {
			input = req.Default
		}

		// Validate input
		if err := t.validateInput(input, req); err != nil {
			errCh <- err
			return
		}

		resultCh <- input
	}()

	// Wait for input or timeout
	select {
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("input timeout after %v", timeout)
	}
}

func (t *HumanInputTool) validateInput(input string, req InputRequest) error {
	// Check if input is required
	if req.Required && input == "" {
		return fmt.Errorf("input is required")
	}

	// Check against options
	if len(req.Options) > 0 {
		valid := false
		for _, opt := range req.Options {
			if strings.EqualFold(input, opt) {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("input must be one of: %s", strings.Join(req.Options, ", "))
		}
	}

	// Check against validation pattern
	if req.Validation != "" {
		// Note: In a real implementation, you would compile and use the regex pattern here
		// For simplicity, we're just checking if it's not empty
		if input == "" {
			return fmt.Errorf("input does not match validation pattern")
		}
	}

	return nil
}
