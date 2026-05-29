package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/catalog"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

// mockUpstreamServer creates a test HTTP server that simulates an MCP server.
func mockUpstreamServer(tools []config.ToolInfo) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Read request body.
		body, _ := io.ReadAll(r.Body)

		// Parse JSON-RPC request.
		var req struct {
			Method string `json:"method"`
		}
		json.Unmarshal(body, &req) //nolint:errcheck

		if req.Method != "tools/list" {
			http.Error(w, "Unknown method", http.StatusNotFound)
			return
		}

		// Return tools list.
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": tools,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// TestCatalog_Integration tests catalog with real HTTP upstream server.
func TestCatalog_Integration(t *testing.T) {
	// Create mock upstream server.
	upstreamTools := []config.ToolInfo{
		{Name: "read_file", Description: "Read a file from disk"},
		{Name: "write_file", Description: "Write content to a file"},
		{Name: "list_dir", Description: "List directory contents"},
	}

	server := mockUpstreamServer(upstreamTools)
	defer server.Close()

	// Create catalog with upstream server.
	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: ":memory:",
		UpstreamURLs: map[string]string{"files": server.URL},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer cat.Stop()

	// Manually sync the server (since automatic sync requires start).
	// This would be done by the background goroutine in production.
	internalCat := cat.(*catalog.ToolCatalog)
	if err := internalCat.SyncServerForTest("files", server.URL); err != nil {
		t.Fatalf("failed to sync server: %v", err)
	}

	// Verify tools were cataloged.
	count, err := cat.GetToolCount()
	if err != nil {
		t.Fatalf("failed to get tool count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 tools in catalog, got %d", count)
	}

	// Verify tools can be retrieved.
	filtered, err := cat.GetFilteredTools("files", nil)
	if err != nil {
		t.Fatalf("failed to get filtered tools: %v", err)
	}
	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered tools, got %d", len(filtered))
	}

	// Verify tool names.
	expectedNames := map[string]bool{
		"read_file":  true,
		"write_file": true,
		"list_dir": true,
	}
	for _, tool := range filtered {
		if !expectedNames[tool.Name] {
			t.Errorf("unexpected tool %q", tool.Name)
		}
	}
}

// TestCatalog_MultipleServers tests cataloging multiple upstream servers.
func TestCatalog_MultipleServers(t *testing.T) {
	// Create two mock upstream servers.
	filesServer := mockUpstreamServer([]config.ToolInfo{
		{Name: "read_file", Description: "Read file"},
		{Name: "write_file", Description: "Write file"},
	})
	defer filesServer.Close()

	dbServer := mockUpstreamServer([]config.ToolInfo{
		{Name: "query", Description: "Query database"},
		{Name: "insert", Description: "Insert row"},
	})
	defer dbServer.Close()

	// Create catalog with multiple upstream servers.
	cfg := &catalog.CatalogConfig{
		Enabled: true,
		DatabasePath: ":memory:",
		UpstreamURLs: map[string]string{
			"files":  filesServer.URL,
			"database": dbServer.URL,
		},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer cat.Stop()

	internalCat := cat.(*catalog.ToolCatalog)

	// Sync both servers.
	if err := internalCat.SyncServerForTest("files", filesServer.URL); err != nil {
		t.Fatalf("failed to sync files server: %v", err)
	}
	if err := internalCat.SyncServerForTest("database", dbServer.URL); err != nil {
		t.Fatalf("failed to sync database server: %v", err)
	}

	// Verify total tool count.
	count, err := cat.GetToolCount()
	if err != nil {
		t.Fatalf("failed to get tool count: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 total tools, got %d", count)
	}

	// Verify server-specific filtering.
	filesTools, err := cat.GetFilteredTools("files", nil)
	if err != nil {
		t.Fatalf("failed to get files tools: %v", err)
	}
	if len(filesTools) != 2 {
		t.Errorf("expected 2 files tools, got %d", len(filesTools))
	}

	dbTools, err := cat.GetFilteredTools("database", nil)
	if err != nil {
		t.Fatalf("failed to get database tools: %v", err)
	}
	if len(dbTools) != 2 {
		t.Errorf("expected 2 database tools, got %d", len(dbTools))
	}

	// Verify aggregated tools.
	aggregated, err := cat.GetAggregatedTools()
	if err != nil {
		t.Fatalf("failed to get aggregated tools: %v", err)
	}
	if len(aggregated) != 4 {
		t.Errorf("expected 4 aggregated tools, got %d", len(aggregated))
	}
}

// TestCatalog_RBACFiltering tests catalog filtering with RBAC allowed tools.
func TestCatalog_RBACFiltering(t *testing.T) {
	// Create mock upstream server with many tools.
	upstreamTools := []config.ToolInfo{
		{Name: "read_file", Description: "Read file"},
		{Name: "write_file", Description: "Write file"},
		{Name: "delete_file", Description: "Delete file"},
		{Name: "list_dir", Description: "List directory"},
	}

	server := mockUpstreamServer(upstreamTools)
	defer server.Close()

	// Create catalog.
	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: ":memory:",
		UpstreamURLs: map[string]string{"files": server.URL},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer cat.Stop()

	internalCat := cat.(*catalog.ToolCatalog)
	if err := internalCat.SyncServerForTest("files", server.URL); err != nil {
		t.Fatalf("failed to sync server: %v", err)
	}

	// Simulate RBAC filtering: user only allowed to use read_file and list_dir.
	allowedTools := map[string]bool{
		"read_file": true,
		"list_dir": true,
	}

	filtered, err := cat.GetFilteredTools("files", allowedTools)
	if err != nil {
		t.Fatalf("failed to get filtered tools: %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("expected 2 allowed tools, got %d", len(filtered))
	}

	// Verify only allowed tools are returned.
	for _, tool := range filtered {
		if !allowedTools[tool.Name] {
			t.Errorf("tool %q should not be in filtered results (not allowed by RBAC)", tool.Name)
		}
	}
}

// TestCatalog_ToolUpdate tests that tools are updated when upstream changes.
func TestCatalog_ToolUpdate(t *testing.T) {
	// Create mock upstream server with dynamic tools.
	var currentTools []config.ToolInfo
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return current tools.
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": currentTools,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Initial tools.
	currentTools = []config.ToolInfo{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
	}

	// Create catalog.
	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: ":memory:",
		UpstreamURLs: map[string]string{"dynamic": server.URL},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer cat.Stop()

	internalCat := cat.(*catalog.ToolCatalog)

	// Initial sync.
	if err := internalCat.SyncServerForTest("dynamic", server.URL); err != nil {
		t.Fatalf("failed to initial sync: %v", err)
	}

	count, _ := cat.GetToolCount()
	if count != 2 {
		t.Errorf("expected 2 tools initially, got %d", count)
	}

	// Update tools on upstream.
	currentTools = []config.ToolInfo{
		{Name: "tool1", Description: "Updated first tool"},
		{Name: "tool3", Description: "Third tool (new)"},
	}

	// Sync again.
	if err := internalCat.SyncServerForTest("dynamic", server.URL); err != nil {
		t.Fatalf("failed to second sync: %v", err)
	}

	// Verify update.
	count, _ = cat.GetToolCount()
	if count != 2 {
		t.Errorf("expected 2 tools after update, got %d", count)
	}

	tools, _ := cat.GetFilteredTools("dynamic", nil)
_toolLoop:
	for _, tool := range tools {
		switch tool.Name {
		case "tool1":
			if tool.Description != "Updated first tool" {
				t.Errorf("expected tool1 description to be updated, got %q", tool.Description)
			}
		case "tool2":
			t.Error("tool2 should have been removed")
		case "tool3":
			// Expected new tool.
			continue
		default:
			t.Errorf("unexpected tool %q", tool.Name)
		}
		// Check that we didn't return tool2.
		for _, expected := range []string{"tool1", "tool3"} {
			if tool.Name == expected {
				continue_toolLoop = true
				break
			}
		}
	}
}

// TestCatalog_PipelineIntegration tests catalog integrated with proxy pipeline.
func TestCatalog_PipelineIntegration(t *testing.T) {
	// Create mock upstream server.
	upstreamTools := []config.ToolInfo{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
	}

	server := mockUpstreamServer(upstreamTools)
	defer server.Close()

	// Create catalog.
	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: ":memory:",
		UpstreamURLs: map[string]string{"test": server.URL},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer cat.Stop()

	internalCat := cat.(*catalog.ToolCatalog)
	if err := internalCat.SyncServerForTest("test", server.URL); err != nil {
		t.Fatalf("failed to sync server: %v", err)
	}

	// Simulate a tools/list request being handled by pipeline.
	// The pipeline would call GetToolsListResponse() with server name and RBAC filters.
	tools, err := cat.GetToolsListResponse("test", nil)
	if err != nil {
		t.Fatalf("failed to get tools list response: %v", err)
	}

	if len(tools) != 2 {
		t.Errorf("expected 2 tools in response, got %d", len(tools))
	}

	// Verify response can be marshaled to JSON.
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]interface{}{
			"tools": tools,
		},
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// Verify JSON structure.
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	result := parsed["result"].(map[string]interface{})
	toolsList := result["tools"].([]interface{})
	if len(toolsList) != 2 {
		t.Errorf("expected 2 tools in JSON response, got %d", len(toolsList))
	}
}

// TestCatalog_StartStop tests catalog lifecycle.
func TestCatalog_StartStop(t *testing.T) {
	cfg := &catalog.CatalogConfig{
		Enabled:        true,
		DatabasePath:   ":memory:",
		UpstreamURLs:   map[string]string{},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}

	// Start should not block or panic.
	done := make(chan bool)
	go func() {
		cat.Start()
		done <- true
	}()

	// Give it time to start.
	select {
	case <-done:
		// Started successfully.
	case <-time.After(1 * time.Second):
		t.Error("catalog.Start() blocked for more than 1 second")
	}

	// Stop should not block or panic.
	go func() {
		cat.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Stopped successfully.
	case <-time.After(1 * time.Second):
		t.Error("catalog.Stop() blocked for more than 1 second")
	}
}

// TestCatalog_GetAggregatedTools tests namespacing of tools from multiple servers.
func TestCatalog_GetAggregatedTools(t *testing.T) {
	// Create servers with overlapping tool names.
	server1 := mockUpstreamServer([]config.ToolInfo{
		{Name: "read", Description: "Read from A"},
		{Name: "write", Description: "Write to A"},
	})
	defer server1.Close()

	server2 := mockUpstreamServer([]config.ToolInfo{
		{Name: "read", Description: "Read from B"}, // Same name
		{Name: "query", Description: "Query B"},
	})
	defer server2.Close()

	cfg := &catalog.CatalogConfig{
		Enabled: true,
		DatabasePath: ":memory:",
		UpstreamURLs: map[string]string{
			"server_a": server1.URL,
			"server_b": server2.URL,
		},
	}

	cat, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer cat.Stop()

	internalCat := cat.(*catalog.ToolCatalog)

	if err := internalCat.SyncServerForTest("server_a", server1.URL); err != nil {
		t.Fatalf("failed to sync server_a: %v", err)
	}
	if err := internalCat.SyncServerForTest("server_b", server2.URL); err != nil {
		t.Fatalf("failed to sync server_b: %v", err)
	}

	aggregated, err := cat.GetAggregatedTools()
	if err != nil {
		t.Fatalf("failed to get aggregated tools: %v", err)
	}

	if len(aggregated) != 4 {
		t.Errorf("expected 4 aggregated tools, got %d", len(aggregated))
	}

	// Check for expected namespaced names.
	nameSet := make(map[string]bool)
	for _, tool := range aggregated {
		nameSet[tool.Name] = true
	}

	// server_a tools should be namespaced as server_a.name
	if !nameSet["server_a.read"] {
		t.Error("expected server_a.read in aggregated tools")
	}
	if !nameSet["server_a.write"] {
		t.Error("expected server_a.write in aggregated tools")
	}

	// server_b.read should be renamed due to collision with server_a.read
	// Format: read.server_b
	if !nameSet["read.server_b"] {
		t.Error("expected read.server_b (renamed due to collision) in aggregated tools")
	}
	if !nameSet["server_b.query"] {
		t.Error("expected server_b.query in aggregated tools")
	}
}

// TestCatalog_EdgeCases tests various edge cases.
func TestCatalog_EdgeCases(t *testing.T) {
	t.Run("empty_upstream_tools", func(t *testing.T) {
		server := mockUpstreamServer([]config.ToolInfo{})
		defer server.Close()

		cfg := &catalog.CatalogConfig{
			Enabled:      true,
			DatabasePath: ":memory:",
			UpstreamURLs: map[string]string{"empty": server.URL},
		}

		cat, err := catalog.NewToolCatalog(cfg)
		if err != nil {
			t.Fatalf("failed to create catalog: %v", err)
		}
		defer cat.Stop()

		internalCat := cat.(*catalog.ToolCatalog)
		if err := internalCat.SyncServerForTest("empty", server.URL); err != nil {
			t.Fatalf("failed to sync empty server: %v", err)
		}

		count, _ := cat.GetToolCount()
		if count != 0 {
			t.Errorf("expected 0 tools from empty server, got %d", count)
		}
	})

	t.Run("very_long_tool_name", func(t *testing.T) {
		longName := fmt.Sprintf("tool_%s", string(bytes.Repeat([]byte("a"), 1000)))
		server := mockUpstreamServer([]config.ToolInfo{
			{Name: longName, Description: "Very long name"},
		})
		defer server.Close()

		cfg := &catalog.CatalogConfig{
			Enabled:      true,
			DatabasePath: ":memory:",
			UpstreamURLs: map[string]string{"long": server.URL},
		}

		cat, err := catalog.NewToolCatalog(cfg)
		if err != nil {
			t.Fatalf("failed to create catalog: %v", err)
		}
		defer cat.Stop()

		internalCat := cat.(*catalog.ToolCatalog)
		if err := internalCat.SyncServerForTest("long", server.URL); err != nil {
			t.Fatalf("failed to sync server with long name: %v", err)
		}

		count, _ := cat.GetToolCount()
		if count != 1 {
			t.Errorf("expected 1 tool with long name, got %d", count)
		}
	})

	t.Run("special_characters_in_description", func(t *testing.T) {
		server := mockUpstreamServer([]config.ToolInfo{
			{Name: "special", Description: "Description with \"quotes\", newlines\n, and unicode: 你好"},
		})
		defer server.Close()

		cfg := &catalog.CatalogConfig{
			Enabled:      true,
			DatabasePath: ":memory:",
			UpstreamURLs: map[string]string{"special": server.URL},
		}

		cat, err := catalog.NewToolCatalog(cfg)
		if err != nil {
			t.Fatalf("failed to create catalog: %v", err)
		}
		defer cat.Stop()

		internalCat := cat.(*catalog.ToolCatalog)
		if err := internalCat.SyncServerForTest("special", server.URL); err != nil {
			t.Fatalf("failed to sync server with special chars: %v", err)
		}

		tools, _ := cat.GetFilteredTools("special", nil)
		if len(tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(tools))
		}
	})
}
