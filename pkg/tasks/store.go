package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TaskOutput represents the complete output of a task execution
type TaskOutput struct {
	TaskID      string                 `json:"task_id"`
	Description string                 `json:"description"`
	Status      TaskStatus             `json:"status"`
	Result      string                 `json:"result"`
	Context     map[string]interface{} `json:"context"`
	Metadata    map[string]interface{} `json:"metadata"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
}

// OutputStore manages task output persistence
type OutputStore struct {
	baseDir     string
	mutex       sync.RWMutex
	outputs     map[string]*TaskOutput
	initialized bool
}

// NewOutputStore creates a new output store
func NewOutputStore(baseDir string) (*OutputStore, error) {
	store := &OutputStore{
		baseDir: baseDir,
		outputs: make(map[string]*TaskOutput),
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load existing outputs
	if err := store.loadOutputs(); err != nil {
		return nil, err
	}

	store.initialized = true
	return store, nil
}

// SaveOutput saves a task output
func (s *OutputStore) SaveOutput(output *TaskOutput) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Update in-memory cache
	s.outputs[output.TaskID] = output

	// Save to file
	filename := filepath.Join(s.baseDir, output.TaskID+".json")
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

// GetOutput retrieves a task output
func (s *OutputStore) GetOutput(taskID string) (*TaskOutput, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	output, ok := s.outputs[taskID]
	if !ok {
		return nil, fmt.Errorf("output not found for task %s", taskID)
	}

	return output, nil
}

// ListOutputs returns all stored task outputs
func (s *OutputStore) ListOutputs() []*TaskOutput {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	outputs := make([]*TaskOutput, 0, len(s.outputs))
	for _, output := range s.outputs {
		outputs = append(outputs, output)
	}

	return outputs
}

// DeleteOutput removes a task output
func (s *OutputStore) DeleteOutput(taskID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Remove from memory
	delete(s.outputs, taskID)

	// Remove file
	filename := filepath.Join(s.baseDir, taskID+".json")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete output file: %w", err)
	}

	return nil
}

// loadOutputs loads all outputs from disk
func (s *OutputStore) loadOutputs() error {
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read output directory: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filename := filepath.Join(s.baseDir, file.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read output file %s: %w", file.Name(), err)
		}

		var output TaskOutput
		if err := json.Unmarshal(data, &output); err != nil {
			return fmt.Errorf("failed to unmarshal output from %s: %w", file.Name(), err)
		}

		s.outputs[output.TaskID] = &output
	}

	return nil
}

// QueryOutputs searches for outputs matching criteria
func (s *OutputStore) QueryOutputs(criteria map[string]interface{}) []*TaskOutput {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var results []*TaskOutput
	for _, output := range s.outputs {
		if matchesCriteria(output, criteria) {
			results = append(results, output)
		}
	}

	return results
}

// matchesCriteria checks if an output matches search criteria
func matchesCriteria(output *TaskOutput, criteria map[string]interface{}) bool {
	for key, value := range criteria {
		switch key {
		case "status":
			if output.Status != value.(TaskStatus) {
				return false
			}
		case "task_id":
			if output.TaskID != value.(string) {
				return false
			}
			// Add more criteria matching as needed
		}
	}
	return true
}
