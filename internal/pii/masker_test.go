package pii

import (
	"encoding/json"
	"testing"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

func TestNew(t *testing.T) {
	t.Run("empty config returns nil", func(t *testing.T) {
		cfg := config.PIIMaskingConfig{}
		masker, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if masker != nil {
			t.Error("expected nil masker for empty config")
		}
	})

	t.Run("valid config returns masker", func(t *testing.T) {
		cfg := config.PIIMaskingConfig{
			Enabled: true,
			Patterns: []config.PIIPatternConfig{
				{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
			},
			SensitivityClasses: []config.PIISensitivityClassConfig{
				{Name: "medium", PatternNames: []string{"email"}},
			},
			ToolClassAssignments: map[string]string{"user_tool": "medium"},
		}
		masker, err := New(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if masker == nil {
			t.Error("expected non-nil masker for valid config")
		}
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		cfg := config.PIIMaskingConfig{
			Patterns: []config.PIIPatternConfig{
				{Name: "invalid", Pattern: `[invalid(regex`},
			},
		}
		_, err := New(cfg)
		if err == nil {
			t.Error("expected error for invalid regex")
		}
	})
}

func TestShouldMaskTool(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email"}},
		},
		ToolClassAssignments: map[string]string{
			"user_tool":    "medium",
			"data_*":       "medium",
			"export_users": "medium",
		},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name       string
		toolName   string
		shouldMask bool
	}{
		{"exact match", "user_tool", true},
		{"wildcard match prefix", "data_export", true},
		{"wildcard match suffix", "data_import", true},
		{"wildcard match middle", "data_test_data", true},
		{"exact export_users", "export_users", true},
		{"no match", "other_tool", false},
		{"partial no match", "user_tools", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.ShouldMaskTool(tt.toolName)
			if result != tt.shouldMask {
				t.Errorf("ShouldMaskTool(%q) = %v, want %v", tt.toolName, result, tt.shouldMask)
			}
		})
	}

	t.Run("nil masker returns false", func(t *testing.T) {
		var m *Masker
		if m.ShouldMaskTool("anything") {
			t.Error("nil masker should return false")
		}
	})
}

func TestGetSensitiveFields(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
			{Name: "ssn", Pattern: `\b\d{3}-\d{2}-\d{4}\b`},
			{Name: "credit_card", Pattern: `\b\d{4}-\d{4}-\d{4}-\d{4}\b`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email", "ssn"}},
			{Name: "high", PatternNames: []string{"credit_card"}},
		},
		ToolClassAssignments: map[string]string{
			"user_tool": "medium",
			"payment_tool": "high",
		},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("medium sensitivity returns correct fields", func(t *testing.T) {
		fields := masker.GetSensitiveFields("user_tool")
		expected := map[string]bool{"email": true, "ssn": true}
		if len(fields) != 2 {
			t.Fatalf("expected 2 fields, got %d: %v", len(fields), fields)
		}
		for _, f := range fields {
			if !expected[f] {
				t.Errorf("unexpected field: %s", f)
			}
		}
	})

	t.Run("high sensitivity returns correct fields", func(t *testing.T) {
		fields := masker.GetSensitiveFields("payment_tool")
		if len(fields) != 1 {
			t.Fatalf("expected 1 field, got %d: %v", len(fields), fields)
		}
		if fields[0] != "credit_card" {
			t.Errorf("expected credit_card, got %s", fields[0])
		}
	})

	t.Run("unassigned tool returns empty", func(t *testing.T) {
		fields := masker.GetSensitiveFields("unknown_tool")
		if len(fields) != 0 {
			t.Errorf("expected empty fields, got %v", fields)
		}
	})

	t.Run("nil masker returns nil", func(t *testing.T) {
		var m *Masker
		fields := m.GetSensitiveFields("anything")
		if fields != nil {
			t.Error("nil masker should return nil")
		}
	})
}

func TestMaskRequest(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email"}},
		},
		ToolClassAssignments: map[string]string{"user_tool": "medium"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("masks email in tools/call request", func(t *testing.T) {
		req := &proxy.MCPRequest{
			Method: proxy.MethodToolsCall,
			Params: json.RawMessage(`{"name": "user_tool", "arguments": {"email": "test@example.com"}}`),
		}
		maskedReq, err := masker.MaskRequest(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var params map[string]interface{}
		if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
			t.Fatalf("failed to unmarshal params: %v", err)
		}
		args, ok := params["arguments"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected arguments to be map, got %T", params["arguments"])
		}
		email := args["email"]
		if email == "test@example.com" {
			t.Error("email was not masked")
		}
		if email != "***REDACTED***" {
			t.Errorf("expected ***REDACTED***, got %v", email)
		}
	})

	t.Run("does not mask unassigned tool", func(t *testing.T) {
		req := &proxy.MCPRequest{
			Method: proxy.MethodToolsCall,
			Params: json.RawMessage(`{"name": "other_tool", "arguments": {"email": "test@example.com"}}`),
		}
		maskedReq, err := masker.MaskRequest(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var params map[string]interface{}
		if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
			t.Fatalf("failed to unmarshal params: %v", err)
		}
		args, ok := params["arguments"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected arguments to be map, got %T", params["arguments"])
		}
		email := args["email"]
		if email != "test@example.com" {
			t.Errorf("email should not be masked for unassigned tool, got %v", email)
		}
	})

	t.Run("does not mask non-tools/call method", func(t *testing.T) {
		req := &proxy.MCPRequest{
			Method: "tools/list",
			Params: json.RawMessage(`{"email": "test@example.com"}`),
		}
		maskedReq, err := masker.MaskRequest(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var params map[string]interface{}
		if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
			t.Fatalf("failed to unmarshal params: %v", err)
		}
		email := params["email"]
		if email != "test@example.com" {
			t.Errorf("email should not be masked for non-tools/call method, got %v", email)
		}
	})

	t.Run("nil masker passes through", func(t *testing.T) {
		var m *Masker
		req := &proxy.MCPRequest{
			Method: proxy.MethodToolsCall,
			Params: json.RawMessage(`{"name": "user_tool", "arguments": {"email": "test@example.com"}}`),
		}
		maskedReq, err := m.MaskRequest(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(maskedReq.Params) != string(req.Params) {
			t.Error("nil masker should pass through unchanged")
		}
	})
}

func TestMaskResponse(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email"}},
		},
		ToolClassAssignments: map[string]string{"get_user": "medium"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("masks email in tools/call response", func(t *testing.T) {
		response := json.RawMessage(`{
			"jsonrpc": "2.0",
			"id": 1,
			"result": {
				"content": [
					{"type": "text", "text": "User email: test@example.com"}
				]
			}
		}`)
		maskedResp, err := masker.MaskResponse(proxy.MethodToolsCall, "get_user", response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(maskedResp) == string(response) {
			t.Error("response was not modified")
		}
		var fullResp map[string]interface{}
		if err := json.Unmarshal(maskedResp, &fullResp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		result, ok := fullResp["result"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected result object")
		}
		content := result["content"].([]interface{})
		textItem := content[0].(map[string]interface{})
		text := textItem["text"].(string)
		if text == "User email: test@example.com" {
			t.Error("email was not masked in response")
		}
		if text != "User email: ***REDACTED***" {
			t.Errorf("expected 'User email: ***REDACTED***', got %q", text)
		}
	})

	t.Run("nil masker passes through", func(t *testing.T) {
		var m *Masker
		response := json.RawMessage(`{"jsonrpc": "2.0", "id": 1, "result": {}}`)
		maskedResp, err := m.MaskResponse(proxy.MethodToolsCall, "any_tool", response)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(maskedResp) != string(response) {
			t.Error("nil masker should pass through unchanged")
		}
	})
}

func TestPartialMasking(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{
				Name:    "email",
				Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
				PartialMask: &config.PartialMaskConfig{
					ShowFirst: 2,
					ShowLast:  3,
					Filler:    "*",
				},
			},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email"}},
		},
		ToolClassAssignments: map[string]string{"user_tool": "medium"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &proxy.MCPRequest{
		Method: proxy.MethodToolsCall,
		Params: json.RawMessage(`{"name": "user_tool", "arguments": {"email": "john.doe@example.com"}}`),
	}
	maskedReq, err := masker.MaskRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments to be map, got %T", params["arguments"])
	}
		email := args["email"]
		expected := "jo***************com"
		if email != expected {
			t.Errorf("expected %q, got %q", expected, email)
		}
	}

func TestCustomMask(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{
				Name:    "ssn",
				Pattern: `(\d{3})-(\d{2})-(\d{4})`,
				Mask:    "***-**-${2}",
			},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "high", PatternNames: []string{"ssn"}},
		},
		ToolClassAssignments: map[string]string{"benefits_tool": "high"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &proxy.MCPRequest{
		Method: proxy.MethodToolsCall,
		Params: json.RawMessage(`{"name": "benefits_tool", "arguments": {"ssn": "123-45-6789"}}`),
	}
	maskedReq, err := masker.MaskRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments to be map, got %T", params["arguments"])
	}
	ssn := args["ssn"]
	expected := "***-**-45"
	if ssn != expected {
		t.Errorf("expected %q, got %q", expected, ssn)
	}
}

func TestMultiplePatterns(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
			{Name: "phone", Pattern: `\(\d{3}\) \d{3}-\d{4}`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email", "phone"}},
		},
		ToolClassAssignments: map[string]string{"contact_tool": "medium"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &proxy.MCPRequest{
		Method: proxy.MethodToolsCall,
		Params: json.RawMessage(`{"name": "contact_tool", "arguments": {"contact": "Email: user@test.com, Phone: (555) 123-4567"}}`),
	}
	maskedReq, err := masker.MaskRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments to be map, got %T", params["arguments"])
	}
	contact := args["contact"].(string)
	expected := "Email: ***REDACTED***, Phone: ***REDACTED***"
	if contact != expected {
		t.Errorf("expected %q, got %q", expected, contact)
	}
}

func TestMaskedNestedStructure(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "email", Pattern: `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"email"}},
		},
		ToolClassAssignments: map[string]string{"user_tool": "medium"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := &proxy.MCPRequest{
		Method: proxy.MethodToolsCall,
		Params: json.RawMessage(`{
			"name": "user_tool",
			"arguments": {
				"users": [
					{"name": "Alice", "email": "alice@example.com"},
					{"name": "Bob", "email": "bob@example.com"}
				]
			}
		}`),
	}
	maskedReq, err := masker.MaskRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(maskedReq.Params, &params); err != nil {
		t.Fatalf("failed to unmarshal params: %v", err)
	}
	args, ok := params["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments to be map, got %T", params["arguments"])
	}
	users := args["users"].([]interface{})
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	for i, user := range users {
		userMap := user.(map[string]interface{})
		email := userMap["email"]
		if email == "alice@example.com" || email == "bob@example.com" {
			t.Errorf("user %d email was not masked: %v", i, email)
		}
	}
}

func TestResponseWithAnthropicUtility(t *testing.T) {
	cfg := config.PIIMaskingConfig{
		Enabled: true,
		Patterns: []config.PIIPatternConfig{
			{Name: "url", Pattern: `https?://[^\s]+`},
		},
		SensitivityClasses: []config.PIISensitivityClassConfig{
			{Name: "medium", PatternNames: []string{"url"}},
		},
		ToolClassAssignments: map[string]string{"search": "medium"},
	}
	masker, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	response := json.RawMessage(`{
		"jsonrpc": "2.0",
		"id": 1,
		"result": {
			"content": [
				{
					"type": "text",
					"text": "Found link: https://example.com/secret/path",
					"annotations": {
						"anthropic_utility": [
							{"url": "https://example.com/secret/path"}
						]
					}
				}
			]
		}
	}`)
	maskedResp, err := masker.MaskResponse(proxy.MethodToolsCall, "search", response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var fullResp map[string]interface{}
	if err := json.Unmarshal(maskedResp, &fullResp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	result, ok := fullResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result object")
	}
	content := result["content"].([]interface{})
	item := content[0].(map[string]interface{})
	text := item["text"].(string)
	if text == "Found link: https://example.com/secret/path" {
		t.Error("text was not masked")
	}
}
