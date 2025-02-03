package memory_test

import (
	"context"
	"os"
	"testing"
	"time"

	"agentai/pkg/agents"
	"agentai/pkg/core"
	"agentai/pkg/llm"
	"agentai/pkg/memory"
)

func TestMemoryIntegration(t *testing.T) {
	ctx := context.Background()

	// Get Gemini API key from environment
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		t.Fatalf("GOOGLE_API_KEY environment variable not set")
	}

	// Initialize LLM provider
	llmConfig := &core.LLMConfig{
		Model:       "gemini-2.0-flash-thinking-exp-01-21",
		Temperature: 0.7,
		MaxTokens:   2000,
		TopP:        1.0,
		APIKey:      apiKey,
	}

	llmProvider, err := llm.NewGeminiStudioProvider(ctx, llmConfig)
	if err != nil {
		t.Fatalf("Failed to create Gemini provider: %v", err)
	}

	// Create memory components
	embedder := memory.NewSimpleEmbedder(64)
	memConfig := &memory.MemoryConfig{
		DefaultTTL:          24 * time.Hour,
		MaxResults:          10,
		RelevanceThreshold:  0.7,
		EnableNotifications: true,
	}

	shortTermStorage := memory.NewRAGStorage(embedder)
	shortTermMem := memory.NewShortTermMemory(shortTermStorage, memConfig)

	longTermStorage := memory.NewRAGStorage(embedder)
	longTermMem := memory.NewLongTermMemory(longTermStorage, memConfig)

	// Create agents with different roles
	codeReviewer := agents.NewAgent(
		"CodeReviewer",
		"Senior Code Reviewer",
		[]string{"Review code for best practices, security, and performance"},
		llmProvider,
	)
	codeReviewer.Config.AllowCodeExecution = true
	codeReviewer.Config.CodeExecutionMode = "safe"
	codeReviewer.Memory = shortTermMem.BaseMemory

	securityExpert := agents.NewAgent(
		"SecurityExpert",
		"Security Specialist",
		[]string{"Identify and address security vulnerabilities in code"},
		llmProvider,
	)
	securityExpert.Config.AllowCodeExecution = true
	securityExpert.Config.CodeExecutionMode = "safe"
	securityExpert.Memory = shortTermMem.BaseMemory

	performanceEngineer := agents.NewAgent(
		"PerformanceEngineer",
		"Performance Optimization Expert",
		[]string{"Analyze and improve code performance and efficiency"},
		llmProvider,
	)
	performanceEngineer.Config.AllowCodeExecution = true
	performanceEngineer.Config.CodeExecutionMode = "safe"
	performanceEngineer.Memory = shortTermMem.BaseMemory

	t.Run("TestAgentMemorySharing", func(t *testing.T) {
		// Test memory sharing between agents
		testValue := "Initial code review findings: Potential security risks in password handling"
		metadata := map[string]interface{}{
			"task_id": "code_review_123",
			"agent":   "CodeReviewer",
			"phase":   "initial_review",
			"type":    "code_review",
		}

		// Save to short-term memory
		err := shortTermMem.Save(testValue, metadata, "CodeReviewer", memory.TaskScope, nil)
		if err != nil {
			t.Fatalf("Failed to save to short-term memory: %v", err)
		}

		// Search in short-term memory
		results, err := shortTermMem.Search(&memory.MemoryQuery{
			Query:      "security risks password handling",
			MaxResults: 5,
			MinScore:   0.7,
			Metadata: map[string]interface{}{
				"task_id": "code_review_123",
				"type":    "code_review",
			},
		})
		if err != nil {
			t.Fatalf("Failed to search short-term memory: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected to find memory results, got none")
		}

		// Save to long-term memory
		err = longTermMem.Save(testValue, metadata, "CodeReviewer", memory.GlobalScope, nil)
		if err != nil {
			t.Fatalf("Failed to save to long-term memory: %v", err)
		}

		// Search in long-term memory
		results, err = longTermMem.Search(&memory.MemoryQuery{
			Query:      "code review findings",
			MaxResults: 5,
			MinScore:   0.7,
			Metadata: map[string]interface{}{
				"task_id": "code_review_123",
				"phase":   "initial_review",
			},
		})
		if err != nil {
			t.Fatalf("Failed to search long-term memory: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected to find memory results, got none")
		}
	})

	t.Run("TestMemoryRetrieval", func(t *testing.T) {
		// Test memory retrieval with different scopes and metadata
		memories := []struct {
			value    string
			metadata map[string]interface{}
			scope    memory.MemoryScope
		}{
			{
				value: "Task specific memory",
				metadata: map[string]interface{}{
					"task_id": "task1",
					"type":    "task",
				},
				scope: memory.TaskScope,
			},
			{
				value: "Global insight",
				metadata: map[string]interface{}{
					"type": "insight",
				},
				scope: memory.GlobalScope,
			},
		}

		// Save memories
		for _, m := range memories {
			err := shortTermMem.Save(m.value, m.metadata, "CodeReviewer", m.scope, nil)
			if err != nil {
				t.Fatalf("Failed to save memory: %v", err)
			}
		}

		// Test retrieval by scope
		for _, m := range memories {
			results, err := shortTermMem.Search(&memory.MemoryQuery{
				Query:      m.value,
				MaxResults: 5,
				MinScore:   0.7,
				Metadata:   m.metadata,
			})
			if err != nil {
				t.Fatalf("Failed to search memory: %v", err)
			}

			if len(results) == 0 {
				t.Errorf("Expected to find memory results for scope %v, got none", m.scope)
			}
		}
	})
}
