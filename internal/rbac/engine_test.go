package rbac_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/rbac"
)

// makeEngine builds an Engine from default roles plus optional overrides.
func makeEngine(roles []config.RoleConfig) *rbac.Engine {
	return rbac.NewEngine(roles)
}

func makeRequest(method string) *proxy.MCPRequest {
	return &proxy.MCPRequest{
		JSONRPC: "2.0",
		Method:  method,
		ID:      1,
	}
}

func makeToolsCallRequest(toolName string) *proxy.MCPRequest {
	params, _ := json.Marshal(map[string]string{"name": toolName})
	return &proxy.MCPRequest{
		JSONRPC: "2.0",
		Method:  proxy.MethodToolsCall,
		Params:  json.RawMessage(params),
		ID:      1,
	}
}

func makeIdentity(role string) *proxy.ClientIdentity {
	return &proxy.ClientIdentity{
		ClientID:  "test-client",
		Role:      role,
		SessionID: "test-session",
	}
}

// ---- Admin role tests ----

func TestAdmin_AllowsToolsCall_AnyTool(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeToolsCallRequest("any_tool_whatsoever")
	identity := makeIdentity(rbac.RoleAdmin)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected admin to allow tools/call, got error: %v", err)
	}
}

func TestAdmin_AllowsToolsList(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodToolsList)
	identity := makeIdentity(rbac.RoleAdmin)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected admin to allow tools/list, got error: %v", err)
	}
}

func TestAdmin_AllowsResourcesRead(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodResourcesRead)
	identity := makeIdentity(rbac.RoleAdmin)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected admin to allow resources/read, got error: %v", err)
	}
}

func TestAdmin_AllowsPromptsGet(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodPromptsGet)
	identity := makeIdentity(rbac.RoleAdmin)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected admin to allow prompts/get, got error: %v", err)
	}
}

func TestAdmin_AllowsInitialize(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodInitialize)
	identity := makeIdentity(rbac.RoleAdmin)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected admin to allow initialize, got error: %v", err)
	}
}

// ---- ReadOnly role tests ----

func TestReadOnly_AllowsToolsList(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodToolsList)
	identity := makeIdentity(rbac.RoleReadOnly)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected readonly to allow tools/list, got error: %v", err)
	}
}

func TestReadOnly_DeniesToolsCall(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeToolsCallRequest("any_tool")
	identity := makeIdentity(rbac.RoleReadOnly)
	_, err := engine.Process(context.Background(), req, identity)
	if err == nil {
		t.Error("expected readonly to deny tools/call, but it was allowed")
	}
}

func TestReadOnly_AllowsResourcesRead(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodResourcesRead)
	identity := makeIdentity(rbac.RoleReadOnly)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected readonly to allow resources/read, got error: %v", err)
	}
}

func TestReadOnly_AllowsPromptsGet(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodPromptsGet)
	identity := makeIdentity(rbac.RoleReadOnly)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected readonly to allow prompts/get, got error: %v", err)
	}
}

func TestReadOnly_AllowsInitialize(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodInitialize)
	identity := makeIdentity(rbac.RoleReadOnly)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected initialize to be allowed for readonly, got error: %v", err)
	}
}

// ---- Restricted role tests ----

func TestRestricted_AllowsToolsCall_AllowedTool(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin},
		{Name: rbac.RoleReadOnly},
		{Name: rbac.RoleRestricted, AllowedTools: []string{"read_file", "list_dir"}},
	}
	engine := makeEngine(roles)
	req := makeToolsCallRequest("read_file")
	identity := makeIdentity(rbac.RoleRestricted)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected restricted to allow tools/call for 'read_file', got error: %v", err)
	}
}

func TestRestricted_DeniesToolsCall_NotAllowedTool(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin},
		{Name: rbac.RoleReadOnly},
		{Name: rbac.RoleRestricted, AllowedTools: []string{"read_file"}},
	}
	engine := makeEngine(roles)
	req := makeToolsCallRequest("write_file")
	identity := makeIdentity(rbac.RoleRestricted)
	_, err := engine.Process(context.Background(), req, identity)
	if err == nil {
		t.Error("expected restricted to deny tools/call for 'write_file', but it was allowed")
	}
}

func TestRestricted_AllowsInitialize(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin},
		{Name: rbac.RoleReadOnly},
		{Name: rbac.RoleRestricted, AllowedTools: []string{"read_file"}},
	}
	engine := makeEngine(roles)
	req := makeRequest(proxy.MethodInitialize)
	identity := makeIdentity(rbac.RoleRestricted)
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected initialize to always be allowed, got error: %v", err)
	}
}

// ---- Unknown role tests ----

func TestUnknownRole_DeniesAll(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodToolsList)
	identity := makeIdentity("unknown-role-xyz")
	_, err := engine.Process(context.Background(), req, identity)
	if err == nil {
		t.Error("expected unknown role to be denied, but it was allowed")
	}
}

// ---- Initialize always allowed test ----

func TestInitialize_AllowedForAllRoles(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	req := makeRequest(proxy.MethodInitialize)
	for _, role := range []string{rbac.RoleAdmin, rbac.RoleReadOnly, rbac.RoleRestricted} {
		identity := makeIdentity(role)
		_, err := engine.Process(context.Background(), req, identity)
		if err != nil {
			t.Errorf("expected initialize to be allowed for role %q, got error: %v", role, err)
		}
	}
}

// ---- FilterToolsList tests ----

func makeToolsListResult(tools []string) json.RawMessage {
	type toolDef struct {
		Name string `json:"name"`
	}
	type result struct {
		Tools []toolDef `json:"tools"`
	}
	defs := make([]toolDef, len(tools))
	for i, name := range tools {
		defs[i] = toolDef{Name: name}
	}
	raw, _ := json.Marshal(result{Tools: defs})
	return json.RawMessage(raw)
}

func extractToolNames(raw json.RawMessage) []string {
	var result struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	names := make([]string, len(result.Tools))
	for i, t := range result.Tools {
		names[i] = t.Name
	}
	return names
}

func TestFilterToolsList_AdminSeesAll(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	identity := makeIdentity(rbac.RoleAdmin)
	allTools := makeToolsListResult([]string{"read_file", "write_file", "exec_cmd"})
	filtered, err := engine.FilterToolsList(identity, allTools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := extractToolNames(filtered)
	if len(names) != 3 {
		t.Errorf("admin should see all 3 tools, got %d: %v", len(names), names)
	}
}

func TestFilterToolsList_ReadOnlySeesAll(t *testing.T) {
	engine := makeEngine(rbac.DefaultRoles())
	identity := makeIdentity(rbac.RoleReadOnly)
	allTools := makeToolsListResult([]string{"read_file", "write_file", "exec_cmd"})
	filtered, err := engine.FilterToolsList(identity, allTools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := extractToolNames(filtered)
	if len(names) != 3 {
		t.Errorf("readonly should see all 3 tools (can list but not call), got %d: %v", len(names), names)
	}
}

func TestFilterToolsList_RestrictedSeesOnlyAllowed(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin},
		{Name: rbac.RoleReadOnly},
		{Name: rbac.RoleRestricted, AllowedTools: []string{"read_file", "list_dir"}},
	}
	engine := makeEngine(roles)
	identity := makeIdentity(rbac.RoleRestricted)
	allTools := makeToolsListResult([]string{"read_file", "write_file", "list_dir", "exec_cmd"})
	filtered, err := engine.FilterToolsList(identity, allTools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := extractToolNames(filtered)
	if len(names) != 2 {
		t.Errorf("restricted should see only 2 tools (read_file, list_dir), got %d: %v", len(names), names)
	}
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["read_file"] || !nameSet["list_dir"] {
		t.Errorf("restricted should see read_file and list_dir, got: %v", names)
	}
}

// ---- Engine loaded from config test ----

func TestEngine_LoadedFromConfig(t *testing.T) {
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin},
		{Name: rbac.RoleReadOnly},
		{Name: rbac.RoleRestricted, AllowedTools: []string{"read_file"}},
	}
	engine := rbac.NewEngine(roles)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	// Verify admin role works
	req := makeToolsCallRequest("anything")
	_, err := engine.Process(context.Background(), req, makeIdentity(rbac.RoleAdmin))
	if err != nil {
		t.Errorf("config-loaded engine: admin should allow tools/call, got: %v", err)
	}
}
