package tools

import (
	"agentai/pkg/core"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Calculator is a tool that performs basic arithmetic operations
type Calculator struct{}

func (t *Calculator) Name() string {
	return "calculator"
}

func (t *Calculator) Description() string {
	return "Perform basic arithmetic operations (add, subtract, multiply, divide)"
}

type CalculatorInput struct {
	Operation string  `json:"operation"`
	Number1   float64 `json:"number1"`
	Number2   float64 `json:"number2"`
}

func (t *Calculator) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"operation": {
			Type:        "string",
			Description: "The arithmetic operation to perform",
			Required:    true,
			Enum:        []string{"add", "subtract", "multiply", "divide"},
		},
		"number1": {
			Type:        "number",
			Description: "The first number",
			Required:    true,
		},
		"number2": {
			Type:        "number",
			Description: "The second number",
			Required:    true,
		},
	}
}

func (t *Calculator) Execute(input string) (string, error) {
	// Try parsing as JSON first
	var params CalculatorInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		// If JSON parsing fails, try old space-separated format
		return t.executeOldFormat(input)
	}

	return t.calculate(params.Operation, params.Number1, params.Number2)
}

func (t *Calculator) executeOldFormat(input string) (string, error) {
	parts := strings.Fields(input)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid input format: expected 'operation number1 number2'")
	}

	operation := parts[0]
	number1, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return "", fmt.Errorf("invalid number1: %v", err)
	}

	number2, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return "", fmt.Errorf("invalid number2: %v", err)
	}

	return t.calculate(operation, number1, number2)
}

func (t *Calculator) calculate(operation string, number1, number2 float64) (string, error) {
	var result float64

	switch operation {
	case "add":
		result = number1 + number2
	case "subtract":
		result = number1 - number2
	case "multiply":
		result = number1 * number2
	case "divide":
		if number2 == 0 {
			return "", fmt.Errorf("division by zero")
		}
		result = number1 / number2
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}

	return fmt.Sprintf("%.2f", result), nil
}
