package tasks

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ContextManager handles shared state and context between tasks
type ContextManager struct {
	mutex       sync.RWMutex
	contextData map[string]interface{}
	taskData    map[string]map[string]interface{}
}

// NewContextManager creates a new context manager instance
func NewContextManager() *ContextManager {
	return &ContextManager{
		contextData: make(map[string]interface{}),
		taskData:    make(map[string]map[string]interface{}),
	}
}

// SetGlobalContext sets a value in the global context
func (c *ContextManager) SetGlobalContext(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.contextData[key] = value
}

// GetGlobalContext gets a value from the global context
func (c *ContextManager) GetGlobalContext(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	value, exists := c.contextData[key]
	return value, exists
}

// SetTaskContext sets a value in a task's context
func (c *ContextManager) SetTaskContext(taskID, key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.taskData[taskID]; !exists {
		c.taskData[taskID] = make(map[string]interface{})
	}
	c.taskData[taskID][key] = value
}

// GetTaskContext gets a value from a task's context
func (c *ContextManager) GetTaskContext(taskID, key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if taskContext, exists := c.taskData[taskID]; exists {
		value, exists := taskContext[key]
		return value, exists
	}
	return nil, false
}

// ShareTaskContext shares context data between tasks
func (c *ContextManager) ShareTaskContext(sourceTaskID, targetTaskID string, keys ...string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	sourceContext, exists := c.taskData[sourceTaskID]
	if !exists {
		return fmt.Errorf("source task %s context not found", sourceTaskID)
	}

	if _, exists := c.taskData[targetTaskID]; !exists {
		c.taskData[targetTaskID] = make(map[string]interface{})
	}

	if len(keys) == 0 {
		// Share all context data if no specific keys provided
		for k, v := range sourceContext {
			c.taskData[targetTaskID][k] = v
		}
	} else {
		// Share only specified keys
		for _, key := range keys {
			if value, exists := sourceContext[key]; exists {
				c.taskData[targetTaskID][key] = value
			}
		}
	}

	return nil
}

// ExportTaskContext exports a task's context as JSON
func (c *ContextManager) ExportTaskContext(taskID string) (string, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	taskContext, exists := c.taskData[taskID]
	if !exists {
		return "", fmt.Errorf("task %s context not found", taskID)
	}

	data, err := json.Marshal(taskContext)
	if err != nil {
		return "", fmt.Errorf("failed to marshal task context: %w", err)
	}

	return string(data), nil
}

// ImportTaskContext imports JSON data into a task's context
func (c *ContextManager) ImportTaskContext(taskID string, jsonData string) error {
	var contextData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &contextData); err != nil {
		return fmt.Errorf("failed to unmarshal task context: %w", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.taskData[taskID]; !exists {
		c.taskData[taskID] = make(map[string]interface{})
	}

	for k, v := range contextData {
		c.taskData[taskID][k] = v
	}

	return nil
}

// ClearTaskContext removes all context data for a task
func (c *ContextManager) ClearTaskContext(taskID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.taskData, taskID)
}

// ClearGlobalContext removes all global context data
func (c *ContextManager) ClearGlobalContext() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.contextData = make(map[string]interface{})
}
