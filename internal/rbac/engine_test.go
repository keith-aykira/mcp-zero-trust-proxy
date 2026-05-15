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
	return rbac.NewEngine(roles, config.ClassificationConfig{})
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
	engine := rbac.NewEngine(roles, config.ClassificationConfig{})
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

// makeClassifiedEngine builds an Engine with classification enabled.
func makeClassifiedEngine(roles []config.RoleConfig, classification config.ClassificationConfig) *rbac.Engine {
	return rbac.NewEngine(roles, classification)
}

func makeIdentityWithSession(role string, sessionID string) *proxy.ClientIdentity {
	return &proxy.ClientIdentity{
		ClientID:  "test-client",
		Role:      role,
		SessionID: sessionID,
	}
}

// ---- Classification tests ----

func TestClassification_AllowsEqualLevel(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"secret_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)
	req := makeToolsCallRequest("secret_tool")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-1")
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected classified admin to access Confidential tool, got: %v", err)
	}
}

func TestClassification_DeniesHigherLevelTool(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"secret_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Sensitive"},
	}
	engine := makeClassifiedEngine(roles, c)
	req := makeToolsCallRequest("secret_tool")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-2")
	_, err := engine.Process(context.Background(), req, identity)
	if err == nil {
		t.Error("expected Sensitive-level user to be denied Confidential tool, but it was allowed")
	}
}

func TestClassification_AllowsLowerLevelTool(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"public_tool": "Public",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)
	req := makeToolsCallRequest("public_tool")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-3")
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected Confidential-level user to access Public tool, got: %v", err)
	}
}

func TestClassification_UnassignedToolDefaultsToPublic(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"secret_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Sensitive"},
	}
	engine := makeClassifiedEngine(roles, c)
	req := makeToolsCallRequest("unknown_tool")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-4")
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected unassigned tool (defaults Public) to be accessible, got: %v", err)
	}
}

func TestClassification_RoleNoLevel(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"secret_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin}, // no classification_level
	}
	engine := makeClassifiedEngine(roles, c)
	req := makeToolsCallRequest("secret_tool")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-5")
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected unclassified role to access tool, got: %v", err)
	}
}

func TestClassification_InactiveWhenNoToolAssignments(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		// No tool assignments — classification should be inactive
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Sensitive"},
	}
	engine := makeClassifiedEngine(roles, c)
	req := makeToolsCallRequest("anything")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-6")
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected classification inactive, got: %v", err)
	}
}

// ---- Declassification prevention tests ----

func TestPrevention_DeniesDowngradeAfterHigherTool(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"high_tool": "Confidential",
			"low_tool":  "Public",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-prevention-1")

	// First call a high-classification tool (floor rises).
	reqHigh := makeToolsCallRequest("high_tool")
	_, err := engine.Process(context.Background(), reqHigh, identity)
	if err != nil {
		t.Fatalf("expected high tool to succeed, got: %v", err)
	}

	// Now try a low-classification tool — should fail (declassification prevention).
	reqLow := makeToolsCallRequest("low_tool")
	_, err = engine.Process(context.Background(), reqLow, identity)
	if err == nil {
		t.Error("expected downgrade to Public tool to be denied after Confidential call")
	}
}

func TestPrevention_AllowsSameFloorTool(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"high_tool":  "Confidential",
			"high_tool2": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-prevention-2")

	req1 := makeToolsCallRequest("high_tool")
	_, err := engine.Process(context.Background(), req1, identity)
	if err != nil {
		t.Fatalf("expected first tool to succeed, got: %v", err)
	}

	req2 := makeToolsCallRequest("high_tool2")
	_, err = engine.Process(context.Background(), req2, identity)
	if err != nil {
		t.Errorf("expected same-floor tool to still be allowed, got: %v", err)
	}
}

func TestPrevention_AllowsEscalation(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"low_tool":  "Public",
			"mid_tool":  "Sensitive",
			"high_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-prevention-3")

	reqLow := makeToolsCallRequest("low_tool")
	_, err := engine.Process(context.Background(), reqLow, identity)
	if err != nil {
		t.Fatalf("expected low tool to succeed, got: %v", err)
	}

	reqMid := makeToolsCallRequest("mid_tool")
	_, err = engine.Process(context.Background(), reqMid, identity)
	if err != nil {
		t.Fatalf("expected escalation from Public to Sensitive to succeed, got: %v", err)
	}

	reqHigh := makeToolsCallRequest("high_tool")
	_, err = engine.Process(context.Background(), reqHigh, identity)
	if err != nil {
		t.Fatalf("expected escalation from Sensitive to Confidential to succeed, got: %v", err)
	}
}

func TestPrevention_SeparateSessions(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"high_tool": "Confidential",
			"low_tool":  "Public",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)

	// Session A calls a high tool.
	sessA := makeIdentityWithSession(rbac.RoleAdmin, "sess-a")
	reqHigh := makeToolsCallRequest("high_tool")
	_, _ = engine.Process(context.Background(), reqHigh, sessA)

	// Session B should be unaffected.
	sessB := makeIdentityWithSession(rbac.RoleAdmin, "sess-b")
	reqLow := makeToolsCallRequest("low_tool")
	_, err := engine.Process(context.Background(), reqLow, sessB)
	if err != nil {
		t.Errorf("expected separate session to be unaffected, got: %v", err)
	}
}

// ---- Classification FilterToolsList tests ----

func TestFilterToolsList_ClassificationFilters(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"pub_tool": "Public",
			"sens_tool": "Sensitive",
			"conf_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleRestricted, AllowedTools: []string{"pub_tool", "sens_tool", "conf_tool"}, ClassificationLevel: "Sensitive"},
	}
	engine := makeClassifiedEngine(roles, c)
	identity := makeIdentityWithSession(rbac.RoleRestricted, "sess-filter-1")

	allTools := makeToolsListResult([]string{"pub_tool", "sens_tool", "conf_tool"})
	filtered, err := engine.FilterToolsList(identity, allTools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := extractToolNames(filtered)
	if len(names) != 2 {
		t.Errorf("expected 2 tools (conf_tool excluded), got %d: %v", len(names), names)
	}
}

func TestFilterToolsList_ClassificationFloorFilters(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Public", "Sensitive", "Confidential"},
		ToolAssignments: map[string]string{
			"pub_tool": "Public",
			"conf_tool": "Confidential",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Confidential"},
	}
	engine := makeClassifiedEngine(roles, c)
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-filter-2")

	// Raising floor
	allTools := makeToolsListResult([]string{"pub_tool", "conf_tool"})
	filtered, _ := engine.FilterToolsList(identity, allTools)
	// Admin has unrestricted allowed_tools (nil), so FilterToolsList returns all unchanged
	if filtered == nil {
		t.Fatal("expected non-nil result")
	}
	// Now let a restricted (allowed_tools non-nil) user call a high tool, then filter.
	roles2 := []config.RoleConfig{
		{Name: rbac.RoleRestricted, AllowedTools: []string{"pub_tool", "conf_tool"}, ClassificationLevel: "Confidential"},
	}
	engine2 := makeClassifiedEngine(roles2, c)
	identity2 := makeIdentityWithSession(rbac.RoleRestricted, "sess-filter-3")

	// First call the high tool to raise floor.
	reqHigh := makeToolsCallRequest("conf_tool")
	_, _ = engine2.Process(context.Background(), reqHigh, identity2)

	// Now filter the tools list — pub_tool should be hidden.
	filtered2, err := engine2.FilterToolsList(identity2, allTools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := extractToolNames(filtered2)
	if len(names) != 1 || names[0] != "conf_tool" {
		t.Errorf("expected only conf_tool after floor raised, got: %v", names)
	}
}

// ---- Custom hierarchy tests ----

func TestClassification_CustomHierarchy(t *testing.T) {
	c := config.ClassificationConfig{
		Levels: []string{"Unclassified", "Restricted", "Secret", "TopSecret"},
		ToolAssignments: map[string]string{
			"ts_tool": "TopSecret",
			"s_tool":  "Secret",
		},
	}
	roles := []config.RoleConfig{
		{Name: rbac.RoleAdmin, ClassificationLevel: "Secret"},
	}
	engine := makeClassifiedEngine(roles, c)

	// Secret user can access Secret tool.
	req := makeToolsCallRequest("s_tool")
	identity := makeIdentityWithSession(rbac.RoleAdmin, "sess-custom")
	_, err := engine.Process(context.Background(), req, identity)
	if err != nil {
		t.Errorf("expected Secret user to access Secret tool, got: %v", err)
	}

	// Secret user cannot access TopSecret tool.
	req2 := makeToolsCallRequest("ts_tool")
	_, err = engine.Process(context.Background(), req2, identity)
	if err == nil {
		t.Error("expected Secret user to be denied TopSecret tool")
	}
}
