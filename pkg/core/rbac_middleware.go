package core

import (
	"context"
	"fmt"
	"net/http"

	"agentai/pkg/tasks"
)

// ContextKey type for context values
type ContextKey string

const (
	// ContextKeyAgent is the key for agent ID in context
	ContextKeyAgent ContextKey = "agent_id"
)

// RBACMiddleware provides middleware functions for RBAC
type RBACMiddleware struct {
	rbac *RBACManager
}

// NewRBACMiddleware creates a new RBAC middleware instance
func NewRBACMiddleware(rbac *RBACManager) *RBACMiddleware {
	return &RBACMiddleware{rbac: rbac}
}

// RequirePermission creates middleware that checks for specific permission
func (m *RBACMiddleware) RequirePermission(resource ResourceType, action PermissionAction) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agentID := r.Context().Value(ContextKeyAgent).(string)
			if !m.rbac.HasPermission(agentID, resource, action) {
				http.Error(w, "Permission denied", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// TaskExecutionMiddleware checks permissions before task execution
func (m *RBACMiddleware) TaskExecutionMiddleware(next func(context.Context, *tasks.Task) error) func(context.Context, *tasks.Task) error {
	return func(ctx context.Context, task *tasks.Task) error {
		agentID := ctx.Value(ContextKeyAgent).(string)
		if !m.rbac.HasPermission(agentID, ResourceTask, ActionExecute) {
			return fmt.Errorf("agent %s does not have permission to execute tasks", agentID)
		}
		return next(ctx, task)
	}
}

// ToolExecutionMiddleware checks permissions before tool execution
func (m *RBACMiddleware) ToolExecutionMiddleware(next func(context.Context, Tool) error) func(context.Context, Tool) error {
	return func(ctx context.Context, tool Tool) error {
		agentID := ctx.Value(ContextKeyAgent).(string)
		if !m.rbac.HasPermission(agentID, ResourceTool, ActionExecute) {
			return fmt.Errorf("agent %s does not have permission to execute tools", agentID)
		}
		return next(ctx, tool)
	}
}

// MemoryAccessMiddleware checks permissions for memory operations
func (m *RBACMiddleware) MemoryAccessMiddleware(action PermissionAction) func(context.Context, string) error {
	return func(ctx context.Context, memoryKey string) error {
		agentID := ctx.Value(ContextKeyAgent).(string)
		if !m.rbac.HasPermission(agentID, ResourceMemory, action) {
			return fmt.Errorf("agent %s does not have permission for memory %s operation", agentID, action)
		}
		return nil
	}
}

// TeamManagementMiddleware checks permissions for team operations
func (m *RBACMiddleware) TeamManagementMiddleware(action PermissionAction) func(context.Context, string) error {
	return func(ctx context.Context, teamID string) error {
		agentID := ctx.Value(ContextKeyAgent).(string)
		if !m.rbac.HasPermission(agentID, ResourceTeam, action) {
			return fmt.Errorf("agent %s does not have permission for team %s operation", agentID, action)
		}
		return nil
	}
}

// AgentManagementMiddleware checks permissions for agent operations
func (m *RBACMiddleware) AgentManagementMiddleware(action PermissionAction) func(context.Context, string) error {
	return func(ctx context.Context, targetAgentID string) error {
		agentID := ctx.Value(ContextKeyAgent).(string)
		if !m.rbac.HasPermission(agentID, ResourceAgent, action) {
			return fmt.Errorf("agent %s does not have permission for agent %s operation", agentID, action)
		}
		return nil
	}
}

// WithAgent adds agent ID to context
func WithAgent(agentID string) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, ContextKeyAgent, agentID)
	}
}

// GetAgentFromContext retrieves agent ID from context
func GetAgentFromContext(ctx context.Context) (string, error) {
	agentID, ok := ctx.Value(ContextKeyAgent).(string)
	if !ok {
		return "", fmt.Errorf("agent ID not found in context")
	}
	return agentID, nil
}
