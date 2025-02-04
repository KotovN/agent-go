package tools

import (
	"agent-go/pkg/core"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CodeInterpreterTool allows agents to execute code
type CodeInterpreterTool struct {
	unsafeMode bool
	workDir    string
}

// NewCodeInterpreterTool creates a new code interpreter tool instance
func NewCodeInterpreterTool(unsafeMode bool, workDir string) *CodeInterpreterTool {
	return &CodeInterpreterTool{
		unsafeMode: unsafeMode,
		workDir:    workDir,
	}
}

func (t *CodeInterpreterTool) Name() string {
	return "code_interpreter"
}

func (t *CodeInterpreterTool) Description() string {
	return "Execute code in a controlled environment. Code can be in Python, JavaScript, or shell script."
}

type CodeInput struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

func (t *CodeInterpreterTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"language": {
			Type:        "string",
			Description: "Programming language of the code (python, javascript, or shell)",
			Required:    true,
			Enum:        []string{"python", "javascript", "shell"},
		},
		"code": {
			Type:        "string",
			Description: "The code to execute",
			Required:    true,
		},
	}
}

func (t *CodeInterpreterTool) Execute(input string) (string, error) {
	var params CodeInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %v", err)
	}

	if params.Language == "" || params.Code == "" {
		return "", fmt.Errorf("both language and code parameters are required")
	}

	if t.unsafeMode {
		return t.executeUnsafe(params.Language, params.Code)
	}
	return t.executeSafe(params.Language, params.Code)
}

func (t *CodeInterpreterTool) executeSafe(language, code string) (string, error) {
	// Check if Docker is installed and running
	if err := t.checkDocker(); err != nil {
		return "", err
	}

	// Create a temporary directory for the code
	tempDir, err := os.MkdirTemp(t.workDir, "code-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write the code to a file
	filename := t.getFilename(language)
	codePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write code file: %v", err)
	}

	// Build Docker command arguments
	args := []string{"run", "--rm"}
	args = append(args, "-v", fmt.Sprintf("%s:/code", tempDir))
	args = append(args, "--network", "none")
	args = append(args, "--memory", "512m")
	args = append(args, "--cpus", "1")
	args = append(args, t.getDockerImage(language))
	args = append(args, t.getRunCommand(language, filename)...)

	// Execute the code
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("code execution failed: %v\nOutput: %s", err, output)
	}

	return string(output), nil
}

func (t *CodeInterpreterTool) executeUnsafe(language, code string) (string, error) {
	// Create a temporary directory for the code
	tempDir, err := os.MkdirTemp(t.workDir, "code-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write the code to a file
	filename := t.getFilename(language)
	codePath := filepath.Join(tempDir, filename)
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write code file: %v", err)
	}

	// Prepare command based on language
	var cmd *exec.Cmd
	switch language {
	case "python":
		cmd = exec.Command("python", codePath)
	case "javascript":
		cmd = exec.Command("node", codePath)
	case "shell":
		cmd = exec.Command("sh", codePath)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	// Execute the code
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("code execution failed: %v\nOutput: %s", err, output)
	}

	return string(output), nil
}

func (t *CodeInterpreterTool) checkDocker() error {
	// Check if Docker is installed
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed")
	}

	// Check if Docker daemon is running
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not running")
	}

	return nil
}

func (t *CodeInterpreterTool) getDockerImage(language string) string {
	switch language {
	case "python":
		return "python:3.9-slim"
	case "javascript":
		return "node:16-slim"
	case "shell":
		return "alpine:latest"
	default:
		return ""
	}
}

func (t *CodeInterpreterTool) getFilename(language string) string {
	switch language {
	case "python":
		return "code.py"
	case "javascript":
		return "code.js"
	case "shell":
		return "code.sh"
	default:
		return "code.txt"
	}
}

func (t *CodeInterpreterTool) getRunCommand(language, filename string) []string {
	switch language {
	case "python":
		return []string{"python", fmt.Sprintf("/code/%s", filename)}
	case "javascript":
		return []string{"node", fmt.Sprintf("/code/%s", filename)}
	case "shell":
		return []string{"sh", fmt.Sprintf("/code/%s", filename)}
	default:
		return []string{}
	}
}
