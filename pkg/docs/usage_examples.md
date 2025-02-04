# Agent Go Usage Examples

## Core Components

### Agents

Agents are autonomous entities that can perform tasks and collaborate with other agents:

```go
// Create an agent
agentConfig := &agents.AgentConfig{
    ID: "agent1",
    Name: "Code Reviewer",
    Description: "Specialized in code review and security analysis",
    Capabilities: map[capabilities.Capability]capabilities.CapabilityLevel{
        capabilities.CodeReview: capabilities.Expert,
        capabilities.Security:   capabilities.Advanced,
    },
    DelegationPrefs: &agents.DelegationPreference{
        Strategy: agents.DelegateOnExpertise,
        SupervisorRoles: []string{"tech-lead", "senior-dev"},
        MaxDelegations: 3,
    },
}
agent := agents.NewAgent(agentConfig)

// Configure agent memory
memoryConfig := &memory.MemoryConfig{
    MaxResults: 10,
    RelevanceThreshold: 0.7,
    DefaultTTL: 24 * time.Hour,
    EnableNotifications: true,
}
agent.InitializeMemory(memoryConfig)

// Add tools to agent
codeReviewTool := tools.NewCodeReviewTool()
securityScanTool := tools.NewSecurityScanTool()
agent.AddTools(codeReviewTool, securityScanTool)
```

### Teams

Teams manage groups of agents working together:

```go
// Create a team
teamConfig := &team.Config{
    Name: "Development Team",
    ProcessType: team.Sequential,
    MaxConcurrentTasks: 5,
    EnableMemorySharing: true,
}
devTeam := team.NewTeam(teamConfig)

// Add agents to team with roles
devTeam.AddAgent(agent1, []string{"code-reviewer", "security-expert"})
devTeam.AddAgent(agent2, []string{"architect", "tech-lead"})
devTeam.AddAgent(agent3, []string{"developer"})

// Configure team-wide shared memory
sharedMemory := memory.NewSharedMemory(memory.DefaultMemoryConfig())
devTeam.SetSharedMemory(sharedMemory)

// Create and assign task to team
task := tasks.NewTask("Review PR #123", tasks.CodeReviewType)
result, err := devTeam.ExecuteTask(ctx, task)
```

### Tasks

Tasks represent units of work that can be executed by agents:

```go
// Create a task with options
task := tasks.NewTask("Implement Authentication", tasks.ImplementationType,
    tasks.WithPriority(tasks.High),
    tasks.WithDeadline(time.Now().Add(24 * time.Hour)),
    tasks.WithDependencies(["design-review", "security-review"]),
    tasks.WithValidation(func(result *tasks.Result) error {
        // Custom validation logic
        return nil
    }),
)

// Add task context and requirements
task.SetContext(map[string]interface{}{
    "repository": "auth-service",
    "branch":     "feature/oauth2",
    "reviewer":   "tech-lead",
})

task.SetRequirements(tasks.Requirements{
    Capabilities: []capabilities.Capability{
        capabilities.Authentication,
        capabilities.Security,
    },
    MinCapabilityLevel: capabilities.Advanced,
    RequiredRoles: []string{"senior-dev", "security-expert"},
})

// Execute task with monitoring
progress := make(chan tasks.Progress)
go task.ExecuteWithProgress(ctx, progress)

for p := range progress {
    fmt.Printf("Task progress: %d%%, Status: %s\n", p.Percentage, p.Status)
}
```

### Memory Management

Memory enables agents to store and retrieve information:

```go
// Create memory systems
personalMemory := memory.NewBaseMemory(memory.DefaultMemoryConfig())
sharedMemory := memory.NewSharedMemory(memory.DefaultMemoryConfig())

// Store task-related memory
taskMemory := &memory.StorageItem{
    Value: "Security vulnerability found in authentication flow",
    Metadata: map[string]interface{}{
        "severity": "high",
        "component": "oauth2",
    },
    Source: "agent1",
    Scope: memory.TaskScope,
    Type: memory.TaskMemory,
    Timestamp: time.Now(),
}
personalMemory.Save(ctx, taskMemory)

// Query memories
query := &memory.MemoryQuery{
    Query: "security vulnerabilities",
    Type: memory.TaskMemory,
    Scope: memory.TaskScope,
    MinScore: 0.7,
    MaxResults: 5,
}
results, err := personalMemory.Search(query)

// Share memory between agents
sharedContext := &memory.SharedMemoryContext{
    Source: "agent1",
    Target: []string{"agent2", "agent3"},
    Value: "Updated security best practices",
    Scope: memory.ProcessScope,
    Relevance: 0.9,
}
sharedMemory.Share(ctx, sharedContext)

// Subscribe to memory updates
callback := func(notification memory.MemoryNotification) {
    fmt.Printf("New memory from %s: %s\n", notification.Source, notification.Value)
}
personalMemory.Subscribe("agent1", callback)
```

### Role-Based Access Control

Configure and manage agent roles and permissions:

```go
// Define roles and permissions
roles := rbac.NewRoleManager()

roles.DefineRole("tech-lead", rbac.RoleConfig{
    Permissions: []string{
        "approve.code",
        "assign.tasks",
        "modify.architecture",
    },
    MaxDelegations: 5,
    CanEscalate: true,
})

roles.DefineRole("security-expert", rbac.RoleConfig{
    Permissions: []string{
        "review.security",
        "approve.security",
        "modify.security",
    },
    RequiredCapabilities: []capabilities.Capability{
        capabilities.Security,
    },
})

// Assign roles to agents
roles.AssignRole("agent1", "tech-lead")
roles.AssignRole("agent2", "security-expert")

// Check permissions
if roles.HasPermission("agent1", "approve.code") {
    // Proceed with code approval
}

// Role-based task execution
task := tasks.NewTask("Security Review", tasks.SecurityReviewType)
task.RequireRole("security-expert")

if err := roles.ValidateTaskExecution(agent, task); err != nil {
    // Handle unauthorized execution attempt
}
```

### Integration Example

Here's how all components work together:

```go
// Create a development team with specialized agents
devTeam := team.NewTeam(&team.Config{
    Name: "Feature Team",
    ProcessType: team.Hierarchical,
})

// Add agents with different roles
techLead := agents.NewAgent(agents.AgentConfig{
    ID: "tech-lead",
    Capabilities: map[capabilities.Capability]capabilities.CapabilityLevel{
        capabilities.Architecture: capabilities.Expert,
        capabilities.CodeReview:   capabilities.Expert,
    },
})
devTeam.AddAgent(techLead, []string{"tech-lead"})

securityExpert := agents.NewAgent(agents.AgentConfig{
    ID: "security",
    Capabilities: map[capabilities.Capability]capabilities.CapabilityLevel{
        capabilities.Security: capabilities.Expert,
    },
})
devTeam.AddAgent(securityExpert, []string{"security-expert"})

// Create a complex task
task := tasks.NewTask("Implement OAuth2",
    tasks.WithPriority(tasks.Critical),
    tasks.WithRoles([]string{"tech-lead", "security-expert"}),
)

// Execute with collaboration
result, err := devTeam.ExecuteTask(ctx, task)
if err != nil {
    // Handle execution error
}

// Share learnings across team
experience := &agents.Experience{
    ID: "oauth2-impl",
    Capability: capabilities.Authentication,
    Outcome: "Successfully implemented OAuth2 with enhanced security",
    Confidence: 0.95,
}
devTeam.ShareExperience(experience)

// Monitor and adapt
metrics := devTeam.GetMetrics()
if metrics.SuccessRate < 0.8 {
    // Adjust team strategy or composition
    devTeam.OptimizeConfiguration()
}
```

This example demonstrates:
- Agent creation with specialized capabilities
- Role-based team organization
- Task execution with requirements
- Memory sharing and learning
- Team-wide monitoring and adaptation

## Agent Communication

The Agent Communication system enables agents to exchange messages and collaborate effectively. Here's how to use it:

```go
// Initialize communication channel with shared memory
memory := memory.NewSharedMemory()
channel := agents.NewCommunicationChannel(memory)

// Send a direct message
message := &agents.Message{
    ID:          "msg-123",
    Type:        agents.MessageTypeDirect,
    FromAgent:   "agent1",
    ToAgent:     "agent2",
    Content:     "Can you help review this code?",
    Context:     map[string]interface{}{"taskId": "task-456"},
    Priority:    1,
    Timestamp:   time.Now(),
    RequiresAck: true,
}
err := channel.SendMessage(ctx, message)

// Send a broadcast message
broadcast := &agents.Message{
    ID:        "broadcast-789",
    Type:      agents.MessageTypeBroadcast,
    FromAgent: "agent1",
    Content:   "Task completed successfully",
    Priority:  2,
    Timestamp: time.Now(),
}
err = channel.SendMessage(ctx, broadcast)

// Check for new messages
messages := channel.GetMessages("agent2", true) // true for unread only

// Acknowledge a message
err = channel.AcknowledgeMessage("msg-123")

// Clean up expired messages
channel.ClearExpiredMessages()
```

### Message Types
- `MessageTypeDirect`: One-to-one communication
- `MessageTypeBroadcast`: One-to-many communication
- `MessageTypeCollaboration`: Request for collaboration
- `MessageTypeResponse`: Response to collaboration request
- `MessageTypeFeedback`: Feedback on tasks/collaboration

## Agent Learning

The Learning system enables agents to learn from experiences and adapt their behavior. Here's how to use it:

```go
// Configure learning system
config := &agents.LearningConfig{
    Strategy:               agents.ReinforcementLearning,
    EnableExperienceSharing: true,
    MinConfidenceThreshold:  0.7,
    MaxExperienceAge:        24 * time.Hour,
    MemoryScope:            memory.AgentScope,
}
learningSystem := agents.NewLearningSystem(config)

// Record a learning experience
experience := &agents.Experience{
    ID:         "exp-123",
    AgentID:    "agent1",
    TaskID:     "task-456",
    Capability: capabilities.CodeReview,
    Outcome:    "Successfully identified security vulnerabilities",
    Confidence: 0.85,
    Metadata: map[string]interface{}{
        "vulnerabilities_found": 3,
        "review_duration":      "10m",
    },
    Timestamp: time.Now(),
}
err := learningSystem.RecordExperience(experience)

// Query experiences with filters
filters := map[string]interface{}{
    "capability": capabilities.CodeReview,
    "min_confidence": 0.8,
}
relevantExperiences := learningSystem.GetExperiences(filters)

// Access learning metrics
metrics := learningSystem.Metrics
fmt.Printf("Total experiences: %d\n", metrics.ExperienceCount)
fmt.Printf("Success rate: %.2f\n", metrics.SuccessRate)
fmt.Printf("Average confidence: %.2f\n", metrics.ConfidenceAvg)
```

### Learning Strategies
- `ReinforcementLearning`: Learn through rewards/penalties
- `SupervisedLearning`: Learn from labeled examples
- `UnsupervisedLearning`: Discover patterns without labels

### Best Practices

1. **Communication**:
   - Set appropriate message priorities
   - Use message context for additional metadata
   - Clean up expired messages regularly
   - Handle acknowledgments for important messages

2. **Learning**:
   - Choose appropriate learning strategy for your use case
   - Set reasonable confidence thresholds
   - Include relevant metadata in experiences
   - Regularly check learning metrics
   - Configure appropriate memory scope for experience sharing

3. **Integration**:
   - Combine communication and learning for collaborative learning
   - Use shared memory for persistent knowledge
   - Monitor metrics to evaluate agent improvement
   - Adjust configurations based on performance

### Example: Collaborative Learning

```go
// Agent 1 learns something and shares it
experience := &agents.Experience{
    ID:         "exp-789",
    AgentID:    "agent1",
    Capability: capabilities.ProblemSolving,
    Outcome:    "Found efficient solution pattern",
    Confidence: 0.9,
}
learningSystem.RecordExperience(experience)

// Share the learning with other agents
message := &agents.Message{
    Type:      agents.MessageTypeCollaboration,
    FromAgent: "agent1",
    Content:   "New solution pattern discovered",
    Context: map[string]interface{}{
        "experience_id": experience.ID,
        "capability":   string(capabilities.ProblemSolving),
    },
}
channel.SendMessage(ctx, message)
```

This enables agents to:
- Share learning experiences
- Collaborate on complex tasks
- Build collective knowledge
- Adapt based on shared insights 