package multimodal

import (
	"agentai/pkg/core"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// AddImageTool allows agents to add images to their context
type AddImageTool struct {
	workDir string
}

// NewAddImageTool creates a new add image tool instance
func NewAddImageTool(workDir string) *AddImageTool {
	return &AddImageTool{
		workDir: workDir,
	}
}

func (t *AddImageTool) Name() string {
	return "add_image"
}

func (t *AddImageTool) Description() string {
	return "Add an image to the conversation context. Accepts image path or base64 encoded image data."
}

type ImageInput struct {
	Source   string `json:"source"`    // Path to image file or base64 encoded data
	IsBase64 bool   `json:"is_base64"` // Whether the source is base64 encoded
}

func (t *AddImageTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"source": {
			Type:        "string",
			Description: "Path to image file or base64 encoded image data",
			Required:    true,
		},
		"is_base64": {
			Type:        "boolean",
			Description: "Whether the source is base64 encoded",
			Required:    true,
		},
	}
}

func (t *AddImageTool) Execute(input string) (string, error) {
	var params ImageInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %v", err)
	}

	if params.Source == "" {
		return "", fmt.Errorf("source parameter is required")
	}

	// Create images directory if it doesn't exist
	imagesDir := filepath.Join(t.workDir, "images")
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create images directory: %v", err)
	}

	var imagePath string
	if params.IsBase64 {
		// Decode base64 data and save to file
		imageData, err := base64.StdEncoding.DecodeString(params.Source)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 image: %v", err)
		}

		imagePath = filepath.Join(imagesDir, fmt.Sprintf("image_%d.png", time.Now().UnixNano()))
		if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
			return "", fmt.Errorf("failed to write image file: %v", err)
		}
	} else {
		// Copy image file to images directory
		sourceFile, err := os.Open(params.Source)
		if err != nil {
			return "", fmt.Errorf("failed to open source image: %v", err)
		}
		defer sourceFile.Close()

		imagePath = filepath.Join(imagesDir, filepath.Base(params.Source))
		destFile, err := os.Create(imagePath)
		if err != nil {
			return "", fmt.Errorf("failed to create destination file: %v", err)
		}
		defer destFile.Close()

		if _, err := io.Copy(destFile, sourceFile); err != nil {
			return "", fmt.Errorf("failed to copy image: %v", err)
		}
	}

	// Return the path to the saved image
	return imagePath, nil
}
