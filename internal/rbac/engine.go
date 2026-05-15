package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
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
	// maxClassLevel is the highest numeric classification rank this role may access.
	// -1 means classification is inactive (role has no classification_level set).
	maxClassLevel int
}

// Engine enforces tool-level RBAC for incoming MCP requests.
// It implements the middleware.Middleware interface via Process().
type Engine struct {
	// permissions maps role name -> compiled permission rules.
	permissions map[string]*permission

	// levelRank maps classification level name → numeric rank (index in levels slice).
	// nil means classification is inactive.
	levelRank map[string]int

	// toolClassification maps tool name → classification level name.
	// nil means classification is inactive.
	toolClassification map[string]string

	// sessionFloors maps session ID → minimum allowed classification rank.
	// When a user calls a higher-classification tool, their floor rises and
	// they can no longer call tools below that floor (prevents information leakage).
	sessionFloors  map[string]int
	sessionFloorsMu sync.RWMutex
}

// NewEngine builds an Engine from a slice of RoleConfig entries and the
// classification configuration. Classification is opt-in: if ClassificationConfig
// has no levels or no tool assignments, classification checks are skipped.
func NewEngine(roles []config.RoleConfig, classification config.ClassificationConfig) *Engine {
	e := &Engine{
		permissions: make(map[string]*permission, len(roles)),
	}

	// Build classification maps. Classification is inactive until both
	// levels and tool assignments are configured.
	classificationActive := len(classification.Levels) > 0 && len(classification.ToolAssignments) > 0
	if classificationActive {
		e.levelRank = make(map[string]int, len(classification.Levels))
		for i, lvl := range classification.Levels {
			e.levelRank[lvl] = i
		}
		e.toolClassification = make(map[string]string, len(classification.ToolAssignments))
		for toolName, lvl := range classification.ToolAssignments {
			e.toolClassification[toolName] = lvl
		}
		e.sessionFloors = make(map[string]int)
	}

	for _, rc := range roles {
		perm := compileRole(rc, e.levelRank)
		e.permissions[rc.Name] = perm
	}
	return e
}

// compileRole translates a RoleConfig into a permission struct.
func compileRole(rc config.RoleConfig, levelRank map[string]int) *permission {
	perm := &permission{
		maxClassLevel: -1, // inactive by default
	}

	// Resolve classification level rank.
	if levelRank != nil && rc.ClassificationLevel != "" {
		if rank, ok := levelRank[rc.ClassificationLevel]; ok {
			perm.maxClassLevel = rank
		}
	}

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
		toolName := proxy.ExtractToolName(req)

		// ReadOnly: deny all tool calls.
		if perm.denyToolsCall {
			return nil, fmt.Errorf("rbac: role %q cannot execute tools (tools/call is denied)", identity.Role)
		}

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

		// Classification check (runs after allow/deny RBAC).
		if e.levelRank != nil && e.toolClassification != nil {
			_, err := e.checkClassification(identity, toolName)
			if err != nil {
				return nil, err
			}
		}
	}

	return req, nil
}

// checkClassification validates that the user's session can access the requested
// tool at its classification level, and updates the session floor if needed.
func (e *Engine) checkClassification(identity *proxy.ClientIdentity, toolName string) (*proxy.MCPRequest, error) {
	// Look up tool's classification level. If unassigned, defaults to
	// the lowest level (index 0).
	toolLevel, hasLevel := e.toolClassification[toolName]
	toolRank := 0
	if hasLevel {
		if rank, ok := e.levelRank[toolLevel]; ok {
			toolRank = rank
		}
	}

	// Check role's maximum classification level. If the role has no
	// classification level assigned, allow access (defaults to lowest).
	perm := e.permissions[identity.Role]
	if perm.maxClassLevel >= 0 && toolRank > perm.maxClassLevel {
		return nil, fmt.Errorf("rbac: tool %q at classification %q exceeds role %q maximum level", toolName, toolLevel, identity.Role)
	}

	// Enforce session floor: if the user has previously called a higher-
	// classification tool, they cannot call tools below that floor.
	e.sessionFloorsMu.RLock()
	currentFloor, hasFloor := e.sessionFloors[identity.SessionID]
	e.sessionFloorsMu.RUnlock()

	if hasFloor && toolRank < currentFloor {
		return nil, fmt.Errorf("rbac: tool %q at classification %q is below session floor (declassification prevented)", toolName, toolLevel)
	}

	// Update floor if this tool's rank is higher than current floor.
	e.sessionFloorsMu.Lock()
	if !hasFloor || toolRank > e.sessionFloors[identity.SessionID] {
		e.sessionFloors[identity.SessionID] = toolRank
	}
	e.sessionFloorsMu.Unlock()

	return nil, nil
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

// FilterToolsList filters the tools/list result to only include tools the
// identity's role can access. Admin sees all tools. ReadOnly sees all tools
// (can list but not call). Restricted sees only the tools in their
// allowed_tools list. Classification-based and floor-based filtering are
// also applied when classification is active.
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

	// Parse the tools list.
	var result toolsListResult
	if err := json.Unmarshal(toolsList, &result); err != nil {
		return nil, fmt.Errorf("rbac: failed to parse tools/list result: %w", err)
	}

	// Get current session floor for filtering.
	e.sessionFloorsMu.RLock()
	currentFloor, hasFloor := e.sessionFloors[identity.SessionID]
	e.sessionFloorsMu.RUnlock()

	filtered := make([]toolEntry, 0, len(result.Tools))
	for _, tool := range result.Tools {
		// Check allowed_tools list.
		if !perm.allowedTools[tool.Name] {
			continue
		}

		// Check classification level.
		if e.levelRank != nil && e.toolClassification != nil {
			toolLevel, hasLevel := e.toolClassification[tool.Name]
			toolRank := 0
			if hasLevel {
				if rank, ok := e.levelRank[toolLevel]; ok {
					toolRank = rank
				}
			}

			// Role maximum level check.
			if perm.maxClassLevel >= 0 && toolRank > perm.maxClassLevel {
				continue
			}

			// Session floor check: hide tools below the session floor
			// (user has accessed higher-classification data).
			if hasFloor && toolRank < currentFloor {
				continue
			}
		}

		filtered = append(filtered, tool)
	}
	result.Tools = filtered

	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("rbac: failed to marshal filtered tools/list: %w", err)
	}
	return json.RawMessage(out), nil
}
