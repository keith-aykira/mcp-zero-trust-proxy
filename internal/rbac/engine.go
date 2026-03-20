package rbac

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
)

// permission holds the compiled access rules for a single role.
type permission struct {
	// allowedMethods is the set of JSON-RPC methods this role may call.
	// nil means all methods are allowed (admin).
	allowedMethods map[string]bool
	// allowedTools is the set of tool names this role may call via tools/call.
	// nil means all tools are allowed (admin).
	allowedTools map[string]bool
	// denyTools is the set of tool names explicitly denied (overrides allow).
	denyTools map[string]bool
	// denyToolsCall denies ALL tools/call invocations regardless of tool name (readonly).
	denyToolsCall bool
}

// Engine enforces tool-level RBAC for incoming MCP requests.
// It implements the middleware.Middleware interface via Process().
type Engine struct {
	// permissions maps role name -> compiled permission rules.
	permissions map[string]*permission
}

// NewEngine builds an Engine from a slice of RoleConfig entries.
// It compiles each role's rules into an efficient lookup structure.
func NewEngine(roles []config.RoleConfig) *Engine {
	e := &Engine{
		permissions: make(map[string]*permission, len(roles)),
	}
	for _, rc := range roles {
		perm := compileRole(rc)
		e.permissions[rc.Name] = perm
	}
	return e
}

// compileRole translates a RoleConfig into a permission struct.
func compileRole(rc config.RoleConfig) *permission {
	perm := &permission{}

	switch rc.Name {
	case RoleAdmin:
		// Admin: all methods and all tools allowed.
		perm.allowedMethods = nil // nil = all allowed
		perm.allowedTools = nil   // nil = all allowed

	case RoleReadOnly:
		// ReadOnly: can call list/read methods but NOT tools/call.
		perm.allowedMethods = map[string]bool{
			proxy.MethodInitialize:    true,
			proxy.MethodToolsList:     true,
			proxy.MethodResourcesRead: true,
			proxy.MethodPromptsGet:    true,
		}
		perm.denyToolsCall = true // blanket deny on tools/call

	case RoleRestricted:
		// Restricted: can call tools/call but only for explicitly allowed tools.
		perm.allowedMethods = map[string]bool{
			proxy.MethodInitialize:    true,
			proxy.MethodToolsList:     true,
			proxy.MethodToolsCall:     true,
			proxy.MethodResourcesRead: true,
			proxy.MethodPromptsGet:    true,
		}
		// Build allowed_tools set from config.
		if len(rc.AllowedTools) > 0 {
			perm.allowedTools = make(map[string]bool, len(rc.AllowedTools))
			for _, t := range rc.AllowedTools {
				perm.allowedTools[t] = true
			}
		}
	default:
		// Unknown roles get deny-all (empty allowed sets, no nil "all allowed").
		perm.allowedMethods = map[string]bool{}
		perm.allowedTools = map[string]bool{}
	}

	// Build deny_tools set (applies to all roles that can call tools).
	if len(rc.DenyTools) > 0 {
		perm.denyTools = make(map[string]bool, len(rc.DenyTools))
		for _, t := range rc.DenyTools {
			perm.denyTools[t] = true
		}
	}

	return perm
}

// Process evaluates req against the identity's RBAC policy.
// It implements the middleware.Middleware interface.
// Returns a non-nil error to deny the request, nil to allow it.
func (e *Engine) Process(ctx context.Context, req *proxy.MCPRequest, identity *proxy.ClientIdentity) (*proxy.MCPRequest, error) {
	// initialize is always allowed regardless of role — required for MCP handshake.
	if req.Method == proxy.MethodInitialize {
		return req, nil
	}

	perm, ok := e.permissions[identity.Role]
	if !ok {
		return nil, fmt.Errorf("rbac: unknown role %q — access denied", identity.Role)
	}

	// Check method-level permission.
	if perm.allowedMethods != nil {
		if !perm.allowedMethods[req.Method] {
			return nil, fmt.Errorf("rbac: role %q is not allowed to call method %q", identity.Role, req.Method)
		}
	}

	// For tools/call, enforce tool-level restrictions.
	if req.Method == proxy.MethodToolsCall {
		// ReadOnly: deny all tool calls.
		if perm.denyToolsCall {
			return nil, fmt.Errorf("rbac: role %q cannot execute tools (tools/call is denied)", identity.Role)
		}

		toolName := proxy.ExtractToolName(req)

		// Check deny list first (deny takes precedence over allow).
		if perm.denyTools != nil && perm.denyTools[toolName] {
			return nil, fmt.Errorf("rbac: tool %q is explicitly denied for role %q", toolName, identity.Role)
		}

		// If allowedTools is non-nil, tool must be in the set (restricted role).
		if perm.allowedTools != nil {
			if !perm.allowedTools[toolName] {
				return nil, fmt.Errorf("rbac: tool %q is not in the allowed list for role %q", toolName, identity.Role)
			}
		}
	}

	return req, nil
}

// toolsListResult is the shape of the tools/list result payload.
type toolsListResult struct {
	Tools []toolEntry `json:"tools"`
}

// toolEntry represents a single tool in a tools/list response.
type toolEntry struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// FilterToolsList filters the tools/list result to only include tools the identity's role can access.
// Admin sees all tools. ReadOnly sees all tools (can list but not call).
// Restricted sees only the tools in their allowed_tools list.
// Returns the filtered JSON-encoded result.
func (e *Engine) FilterToolsList(identity *proxy.ClientIdentity, toolsList json.RawMessage) (json.RawMessage, error) {
	perm, ok := e.permissions[identity.Role]
	if !ok {
		// Unknown role: return empty list.
		empty, _ := json.Marshal(toolsListResult{Tools: []toolEntry{}})
		return json.RawMessage(empty), nil
	}

	// Admin and ReadOnly see all tools (nil allowedTools = unrestricted listing).
	if perm.allowedTools == nil && !perm.denyToolsCall {
		return toolsList, nil
	}
	// ReadOnly: can list all tools (even though they can't call them).
	if perm.denyToolsCall {
		return toolsList, nil
	}

	// Restricted: filter to only allowed tools.
	var result toolsListResult
	if err := json.Unmarshal(toolsList, &result); err != nil {
		return nil, fmt.Errorf("rbac: failed to parse tools/list result: %w", err)
	}

	filtered := make([]toolEntry, 0, len(result.Tools))
	for _, tool := range result.Tools {
		if perm.allowedTools[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	result.Tools = filtered

	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("rbac: failed to marshal filtered tools/list: %w", err)
	}
	return json.RawMessage(out), nil
}
