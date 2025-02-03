package core

import (
	"fmt"
	"sync"
)

// PermissionAction defines allowed actions on resources
type PermissionAction string

const (
	ActionRead    PermissionAction = "read"
	ActionWrite   PermissionAction = "write"
	ActionExecute PermissionAction = "execute"
	ActionDelete  PermissionAction = "delete"
	ActionGrant   PermissionAction = "grant"
)

// ResourceType defines types of resources that can be protected
type ResourceType string

const (
	ResourceTask   ResourceType = "task"
	ResourceMemory ResourceType = "memory"
	ResourceTool   ResourceType = "tool"
	ResourceAgent  ResourceType = "agent"
	ResourceTeam   ResourceType = "team"
)

// Permission defines a single permission on a resource
type Permission struct {
	Resource   ResourceType
	Action     PermissionAction
	Conditions map[string]interface{} // Additional constraints like time windows, resource tags
}

// Role represents a collection of permissions
type Role struct {
	Name        string
	Description string
	Permissions []Permission
	Hierarchy   int    // Role level in hierarchy (higher number = more privileges)
	ParentRole  string // Name of parent role for inheritance
}

// RBACManager handles role-based access control
type RBACManager struct {
	mu          sync.RWMutex
	roles       map[string]*Role
	userRoles   map[string][]string // AgentID -> Role names
	inheritance map[string][]string // Role -> Child roles
}

// NewRBACManager creates a new RBAC manager instance
func NewRBACManager() *RBACManager {
	return &RBACManager{
		roles:       make(map[string]*Role),
		userRoles:   make(map[string][]string),
		inheritance: make(map[string][]string),
	}
}

// AddRole adds a new role to the system
func (rm *RBACManager) AddRole(role *Role) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roles[role.Name]; exists {
		return fmt.Errorf("role %s already exists", role.Name)
	}

	rm.roles[role.Name] = role

	// Set up inheritance
	if role.ParentRole != "" {
		if _, exists := rm.roles[role.ParentRole]; !exists {
			return fmt.Errorf("parent role %s does not exist", role.ParentRole)
		}
		rm.inheritance[role.ParentRole] = append(rm.inheritance[role.ParentRole], role.Name)
	}

	return nil
}

// AssignRole assigns a role to an agent
func (rm *RBACManager) AssignRole(agentID, roleName string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roles[roleName]; !exists {
		return fmt.Errorf("role %s does not exist", roleName)
	}

	rm.userRoles[agentID] = append(rm.userRoles[agentID], roleName)
	return nil
}

// HasPermission checks if an agent has a specific permission
func (rm *RBACManager) HasPermission(agentID string, resource ResourceType, action PermissionAction) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	roles := rm.userRoles[agentID]
	for _, roleName := range roles {
		if rm.roleHasPermission(roleName, resource, action) {
			return true
		}
	}
	return false
}

// roleHasPermission checks if a role has a specific permission (including inherited permissions)
func (rm *RBACManager) roleHasPermission(roleName string, resource ResourceType, action PermissionAction) bool {
	role, exists := rm.roles[roleName]
	if !exists {
		return false
	}

	// Check direct permissions
	for _, perm := range role.Permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
	}

	// Check parent role if exists
	if role.ParentRole != "" {
		return rm.roleHasPermission(role.ParentRole, resource, action)
	}

	return false
}

// GetAgentRoles returns all roles assigned to an agent
func (rm *RBACManager) GetAgentRoles(agentID string) []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.userRoles[agentID]
}

// GetEffectivePermissions returns all permissions an agent has (including inherited)
func (rm *RBACManager) GetEffectivePermissions(agentID string) []Permission {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	permissions := make(map[string]Permission) // Use string key to deduplicate
	roles := rm.userRoles[agentID]

	for _, roleName := range roles {
		rm.collectRolePermissions(roleName, permissions)
	}

	// Convert map to slice
	result := make([]Permission, 0, len(permissions))
	for _, perm := range permissions {
		result = append(result, perm)
	}
	return result
}

// collectRolePermissions recursively collects permissions from a role and its parents
func (rm *RBACManager) collectRolePermissions(roleName string, permissions map[string]Permission) {
	role, exists := rm.roles[roleName]
	if !exists {
		return
	}

	// Add direct permissions
	for _, perm := range role.Permissions {
		key := fmt.Sprintf("%s:%s", perm.Resource, perm.Action)
		permissions[key] = perm
	}

	// Add parent permissions
	if role.ParentRole != "" {
		rm.collectRolePermissions(role.ParentRole, permissions)
	}
}
