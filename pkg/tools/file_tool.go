package tools

import (
	"agentai/pkg/core"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileTool provides file system operation capabilities
type FileTool struct {
	workDir string
}

// NewFileTool creates a new file operations tool instance
func NewFileTool(workDir string) *FileTool {
	return &FileTool{
		workDir: workDir,
	}
}

func (t *FileTool) Name() string {
	return "file"
}

func (t *FileTool) Description() string {
	return "Perform file system operations like reading, writing, and managing files and directories"
}

type FileOperation struct {
	Operation string      `json:"operation"`
	Path      string      `json:"path"`
	Content   string      `json:"content,omitempty"`
	NewPath   string      `json:"new_path,omitempty"`
	Recursive bool        `json:"recursive,omitempty"`
	Mode      os.FileMode `json:"mode,omitempty"`
}

func (t *FileTool) Parameters() map[string]core.ParameterDefinition {
	return map[string]core.ParameterDefinition{
		"operation": {
			Type:        "string",
			Description: "File operation to perform (read, write, delete, move, copy, list, mkdir)",
			Required:    true,
			Enum:        []string{"read", "write", "delete", "move", "copy", "list", "mkdir"},
		},
		"path": {
			Type:        "string",
			Description: "Path to the file or directory",
			Required:    true,
		},
		"content": {
			Type:        "string",
			Description: "Content to write to file (for write operation)",
			Required:    false,
		},
		"new_path": {
			Type:        "string",
			Description: "New path for move/copy operations",
			Required:    false,
		},
		"recursive": {
			Type:        "boolean",
			Description: "Whether to perform operation recursively",
			Required:    false,
		},
		"mode": {
			Type:        "integer",
			Description: "File mode for creation (e.g., 0644)",
			Required:    false,
		},
	}
}

func (t *FileTool) Execute(input string) (string, error) {
	var op FileOperation
	if err := json.Unmarshal([]byte(input), &op); err != nil {
		return "", fmt.Errorf("failed to parse operation: %w", err)
	}

	// Resolve path relative to work directory
	path := filepath.Join(t.workDir, op.Path)

	switch op.Operation {
	case "read":
		return t.readFile(path)
	case "write":
		return t.writeFile(path, op.Content, op.Mode)
	case "delete":
		return t.deleteFile(path, op.Recursive)
	case "move":
		newPath := filepath.Join(t.workDir, op.NewPath)
		return t.moveFile(path, newPath)
	case "copy":
		newPath := filepath.Join(t.workDir, op.NewPath)
		return t.copyFile(path, newPath)
	case "list":
		return t.listDir(path, op.Recursive)
	case "mkdir":
		return t.makeDir(path, op.Mode)
	default:
		return "", fmt.Errorf("unknown operation: %s", op.Operation)
	}
}

func (t *FileTool) readFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

func (t *FileTool) writeFile(path string, content string, mode os.FileMode) (string, error) {
	if mode == 0 {
		mode = 0644
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return "File written successfully", nil
}

func (t *FileTool) deleteFile(path string, recursive bool) (string, error) {
	if recursive {
		if err := os.RemoveAll(path); err != nil {
			return "", fmt.Errorf("failed to delete recursively: %w", err)
		}
	} else {
		if err := os.Remove(path); err != nil {
			return "", fmt.Errorf("failed to delete: %w", err)
		}
	}
	return "Deleted successfully", nil
}

func (t *FileTool) moveFile(oldPath, newPath string) (string, error) {
	if err := os.Rename(oldPath, newPath); err != nil {
		return "", fmt.Errorf("failed to move: %w", err)
	}
	return "Moved successfully", nil
}

func (t *FileTool) copyFile(src, dst string) (string, error) {
	sourceFile, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("failed to open source: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("failed to create destination: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return "", fmt.Errorf("failed to copy: %w", err)
	}

	return "Copied successfully", nil
}

func (t *FileTool) listDir(path string, recursive bool) (string, error) {
	var files []string
	var walkFn filepath.WalkFunc

	if recursive {
		walkFn = func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(t.workDir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
			return nil
		}
		if err := filepath.Walk(path, walkFn); err != nil {
			return "", fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", fmt.Errorf("failed to read directory: %w", err)
		}
		for _, entry := range entries {
			files = append(files, entry.Name())
		}
	}

	result, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format file list: %w", err)
	}
	return string(result), nil
}

func (t *FileTool) makeDir(path string, mode os.FileMode) (string, error) {
	if mode == 0 {
		mode = 0755
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	return "Directory created successfully", nil
}
