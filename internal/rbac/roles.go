package rbac

import "github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"

// Built-in role name constants.
const (
	RoleAdmin      = "admin"
	RoleReadOnly   = "readonly"
	RoleRestricted = "restricted"
)

// DefaultRoles returns the three built-in role configurations.
// Admin has unrestricted access. ReadOnly can list/read but not call tools.
// Restricted has an empty allowed_tools list — callers must override this.
func DefaultRoles() []config.RoleConfig {
	return []config.RoleConfig{
		{
			Name:         RoleAdmin,
			AllowedTools: []string{}, // empty = all tools allowed
			DenyTools:    []string{},
		},
		{
			Name:         RoleReadOnly,
			AllowedTools: []string{}, // tools/call is denied at engine level, not per-tool
			DenyTools:    []string{},
		},
		{
			Name:         RoleRestricted,
			AllowedTools: []string{}, // must be explicitly configured
			DenyTools:    []string{},
		},
	}
}
