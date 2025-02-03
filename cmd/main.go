package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"agentai/pkg/agents"
	"agentai/pkg/core"
	"agentai/pkg/llm"
	"agentai/pkg/memory"
	"agentai/pkg/tasks"
	"agentai/pkg/team"
)

func main() {
	ctx := context.Background()

	// Initialize logger with debug level
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting application")

	// Get Gemini API key from environment
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		logger.Error("GOOGLE_API_KEY environment variable is not set")
		os.Exit(1)
	}
	logger.Debug("API key loaded")

	// Initialize Gemini LLM provider
	llmConfig := &core.LLMConfig{
		Model:       "gemini-2.0-flash-thinking-exp-01-21",
		Temperature: 0.7,
		MaxTokens:   2000,
		TopP:        1.0,
		APIKey:      apiKey,
	}
	logger.Debug("Initializing LLM provider")

	llmProvider, err := llm.NewGeminiStudioProvider(ctx, llmConfig)
	if err != nil {
		logger.Error("Failed to create Gemini provider", "error", err)
		os.Exit(1)
	}
	logger.Info("LLM provider initialized")

	logger.Debug("Creating manager agent")
	manager := agents.NewAgent(
		"Team Manager",
		"Task Coordinator",
		[]string{"Coordinate and delegate tasks to specialized agents"},
		llmProvider,
	)
	manager.Config.AllowDelegation = true
	manager.Config.AllowCodeExecution = false
	logger.Info("Manager agent created")

	logger.Debug("Creating math specialist agent")
	// Create math specialist agent
	mathSpecialist := agents.NewAgent(
		"Math Assistant",
		"Math Problem Solver",
		[]string{"Help users solve mathematical problems"},
		llmProvider,
	)
	mathSpecialist.Config.AllowCodeExecution = true
	mathSpecialist.Config.CodeExecutionMode = "safe"
	mathSpecialist.Config.MaxRetryLimit = 3
	logger.Info("Math specialist agent created")

	// Setup team working directory
	workDir := "team_workspace"
	logger.Debug("Creating work directory", "path", workDir)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		logger.Error("Failed to create work directory", "error", err)
		os.Exit(1)
	}

	// Configure memory
	memoryConfig := memory.MemoryConfig{
		DefaultTTL:          24 * time.Hour,
		EnableNotifications: true,
		ShortTermStorage: &memory.StorageConfig{
			Provider: memory.ProviderRAG,
			MaxSize:  1000,
			Options: map[string]interface{}{
				"dimension": 64,
			},
		},
		LongTermStorage: &memory.StorageConfig{
			Provider: memory.ProviderLocal,
			Path:     filepath.Join(workDir, "long_term.db"),
			MaxSize:  10000,
		},
		EntityStorage: &memory.StorageConfig{
			Provider: memory.ProviderRAG,
			MaxSize:  1000,
			Options: map[string]interface{}{
				"dimension": 64,
			},
		},
	}
	logger.Debug("Memory config created")

	// Initialize memory for manager
	logger.Debug("Initializing manager memory")
	if err := manager.InitializeMemory(workDir, &memoryConfig); err != nil {
		logger.Error("Failed to initialize manager memory", "error", err)
		os.Exit(1)
	}
	logger.Info("Manager memory initialized")

	// Initialize memory for math specialist
	logger.Debug("Initializing math specialist memory")
	if err := mathSpecialist.InitializeMemory(workDir, &memoryConfig); err != nil {
		logger.Error("Failed to initialize math specialist memory", "error", err)
		os.Exit(1)
	}
	logger.Info("Math specialist memory initialized")

	logger.Debug("Creating team")
	// Create the team
	tm, err := team.NewTeam(
		"Math Team",
		"A team specialized in solving math problems",
		team.Hierarchical,
		workDir,
		memoryConfig,
		team.WithManager(manager),
		team.WithMemory(true),
	)
	if err != nil {
		logger.Error("Failed to create team", "error", err)
		os.Exit(1)
	}
	logger.Info("Team created successfully")

	logger.Debug("Adding manager to team")
	// Add agents to team
	if err := tm.AddAgent(manager); err != nil {
		logger.Error("Failed to add manager", "error", err)
		os.Exit(1)
	}
	logger.Info("Manager added to team")

	logger.Debug("Adding math specialist to team")
	if err := tm.AddAgent(mathSpecialist); err != nil {
		logger.Error("Failed to add math specialist", "error", err)
		os.Exit(1)
	}
	logger.Info("Math specialist added to team")

	// Create a sample task
	taskID := "math-task-1"
	taskPrompt := `Solve this math problem step by step:
1. Multiply 17.5 by 3.2
2. Take that result and add 10
3. Finally, divide the total by 2
`
	logger.Debug("Creating task", "id", taskID)

	// Create a task
	task, err := tasks.NewTask(taskPrompt, "Explain each step and show the final result.")
	if err != nil {
		logger.Error("Failed to create task", "error", err)
		os.Exit(1)
	}
	logger.Info("Task created", "id", taskID)

	// Add timeout to context
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	logger.Debug("Starting task execution", "id", taskID)
	// Execute the task using the team
	err = tm.ExecuteTask(ctxWithTimeout, task)
	if err != nil {
		logger.Error("Failed to execute task", "error", err)
		os.Exit(1)
	}

	// Log successful execution
	logger.Info("Task executed successfully",
		"team_name", tm.Name,
		"task_id", taskID,
	)

	fmt.Println("\nTask Result:")
	fmt.Println(task.Result)
}
