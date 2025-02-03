package core

// Default role names
const (
	RoleAdmin       = "admin"
	RoleManager     = "manager"
	RoleWorker      = "worker"
	RoleObserver    = "observer"
	RoleTaskCreator = "task_creator"
)

// DefaultRoles returns a set of predefined roles with their permissions
func DefaultRoles() []*Role {
	return []*Role{
		{
			Name:        RoleAdmin,
			Description: "Full system access with all permissions",
			Hierarchy:   100,
			Permissions: []Permission{
				{Resource: ResourceTask, Action: ActionRead},
				{Resource: ResourceTask, Action: ActionWrite},
				{Resource: ResourceTask, Action: ActionExecute},
				{Resource: ResourceTask, Action: ActionDelete},
				{Resource: ResourceMemory, Action: ActionRead},
				{Resource: ResourceMemory, Action: ActionWrite},
				{Resource: ResourceTool, Action: ActionRead},
				{Resource: ResourceTool, Action: ActionExecute},
				{Resource: ResourceAgent, Action: ActionRead},
				{Resource: ResourceAgent, Action: ActionWrite},
				{Resource: ResourceTeam, Action: ActionRead},
				{Resource: ResourceTeam, Action: ActionWrite},
				{Resource: ResourceTeam, Action: ActionExecute},
			},
		},
		{
			Name:        RoleManager,
			Description: "Manages team and oversees task execution",
			Hierarchy:   80,
			ParentRole:  RoleTaskCreator,
			Permissions: []Permission{
				{Resource: ResourceAgent, Action: ActionRead},
				{Resource: ResourceAgent, Action: ActionWrite},
				{Resource: ResourceTeam, Action: ActionRead},
				{Resource: ResourceTeam, Action: ActionWrite},
				{Resource: ResourceTeam, Action: ActionExecute},
				{Resource: ResourceMemory, Action: ActionRead},
				{Resource: ResourceMemory, Action: ActionWrite},
			},
		},
		{
			Name:        RoleTaskCreator,
			Description: "Can create and manage tasks",
			Hierarchy:   60,
			ParentRole:  RoleWorker,
			Permissions: []Permission{
				{Resource: ResourceTask, Action: ActionRead},
				{Resource: ResourceTask, Action: ActionWrite},
				{Resource: ResourceTask, Action: ActionExecute},
				{Resource: ResourceTool, Action: ActionRead},
				{Resource: ResourceTool, Action: ActionExecute},
			},
		},
		{
			Name:        RoleWorker,
			Description: "Standard worker with task execution permissions",
			Hierarchy:   40,
			ParentRole:  RoleObserver,
			Permissions: []Permission{
				{Resource: ResourceTask, Action: ActionExecute},
				{Resource: ResourceTool, Action: ActionExecute},
				{Resource: ResourceMemory, Action: ActionRead},
				{Resource: ResourceMemory, Action: ActionWrite},
			},
		},
		{
			Name:        RoleObserver,
			Description: "Read-only access to tasks and results",
			Hierarchy:   20,
			Permissions: []Permission{
				{Resource: ResourceTask, Action: ActionRead},
				{Resource: ResourceMemory, Action: ActionRead},
				{Resource: ResourceTool, Action: ActionRead},
				{Resource: ResourceAgent, Action: ActionRead},
				{Resource: ResourceTeam, Action: ActionRead},
			},
		},
	}
}

// InitializeDefaultRoles sets up the default roles in the RBAC manager
func InitializeDefaultRoles(rbac *RBACManager) error {
	// Add roles in order of hierarchy (lowest first) to handle inheritance
	roles := DefaultRoles()
	for i := len(roles) - 1; i >= 0; i-- {
		if err := rbac.AddRole(roles[i]); err != nil {
			return err
		}
	}
	return nil
}
