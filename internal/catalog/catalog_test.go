package catalog_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/catalog"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
)

// setupTempDB creates a temporary database file for testing.
func setupTempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_catalog.db")
	return dbPath
}

// cleanupDB removes the database file.
func cleanupDB(dbPath string) {
	os.Remove(dbPath) //nolint:errcheck
}

// TestNewToolCatalog tests catalog creation.
func TestNewToolCatalog(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	catalog, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer catalog.Stop()

	if catalog == nil {
		t.Fatal("expected catalog to be non-nil")
	}
}

// TestNewToolCatalog_InvalidPath tests error handling for invalid database path.
func TestNewToolCatalog_InvalidPath(t *testing.T) {
	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: "/nonexistent/path/catalog.db",
		UpstreamURLs: map[string]string{},
	}

	_, err := catalog.NewToolCatalog(cfg)
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

// TestUpsertTools tests inserting and updating tools in the catalog.
func TestUpsertTools(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	serverName := "test-server"
	tools := []config.ToolInfo{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
		{Name: "tool3", Description: "Third tool"},
	}

	err = tc.(*catalog.ToolCatalog).UpsertToolsForTest(serverName, tools)
	if err != nil {
		t.Fatalf("failed to upsert tools: %v", err)
	}

	count, err := tc.GetToolCount()
	if err != nil {
		t.Fatalf("failed to get tool count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 tools, got %d", count)
	}
}

// TestGetFilteredTools tests filtering tools by server name.
func TestGetFilteredTools(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	// Insert tools for two servers.
	server1 := "server-a"
	server2 := "server-b"

	tools1 := []config.ToolInfo{
		{Name: "read_file", Description: "Read a file"},
		{Name: "write_file", Description: "Write a file"},
	}

	tools2 := []config.ToolInfo{
		{Name: "query_db", Description: "Query database"},
		{Name: "insert_row", Description: "Insert a row"},
	}

	tc.(*catalog.ToolCatalog).UpsertToolsForTest(server1, tools1) //nolint:errcheck
	tc.(*catalog.ToolCatalog).UpsertToolsForTest(server2, tools2) //nolint:errcheck

	// Test filtering by server-a.
	filtered, err := tc.GetFilteredTools(server1, nil)
	if err != nil {
		t.Fatalf("failed to get filtered tools: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 tools for %s, got %d", server1, len(filtered))
	}

	// Verify tool names.
	expectedNames := map[string]bool{"read_file": true, "write_file": true}
	for _, tool := range filtered {
		if !expectedNames[tool.Name] {
			t.Errorf("unexpected tool %q in %s results", tool.Name, server1)
		}
	}
}

// TestGetFilteredTools_AllowedTools tests filtering by allowed tools map.
func TestGetFilteredTools_AllowedTools(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	serverName := "test-server"
	tools := []config.ToolInfo{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
		{Name: "tool3", Description: "Third tool"},
	}

	tc.(*catalog.ToolCatalog).UpsertToolsForTest(serverName, tools) //nolint:errcheck

	// Filter to only allow tool1 and tool3.
	allowedTools := map[string]bool{"tool1": true, "tool3": true}
	filtered, err := tc.GetFilteredTools(serverName, allowedTools)
	if err != nil {
		t.Fatalf("failed to get filtered tools: %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("expected 2 allowed tools, got %d", len(filtered))
	}

	// Verify only allowed tools are present.
	for _, tool := range filtered {
		if !allowedTools[tool.Name] {
			t.Errorf("tool %q should not be in filtered results", tool.Name)
		}
	}
}

// TestGetAggregatedTools tests merging tools from all servers with namespacing.
func TestGetAggregatedTools(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	// Insert tools with same names in different servers.
	server1 := "files"
	server2 := "database"

	tools1 := []config.ToolInfo{
		{Name: "read", Description: "Read file"},
		{Name: "write", Description: "Write file"},
	}

	tools2 := []config.ToolInfo{
		{Name: "read", Description: "Read from DB"}, // Same name as in files
		{Name: "query", Description: "Query DB"},
	}

	tc.(*catalog.ToolCatalog).UpsertToolsForTest(server1, tools1) //nolint:errcheck
	tc.(*catalog.ToolCatalog).UpsertToolsForTest(server2, tools2) //nolint:errcheck

	aggregated, err := tc.GetAggregatedTools()
	if err != nil {
		t.Fatalf("failed to get aggregated tools: %v", err)
	}

	if len(aggregated) != 4 {
		t.Errorf("expected 4 aggregated tools, got %d", len(aggregated))
	}

	// Check namespacing: server.tool_name format.
	nameSet := make(map[string]bool)
	for _, tool := range aggregated {
		nameSet[tool.Name] = true
	}

	// Check for expected namespaced names.
	if !nameSet["files.read"] {
		t.Error("expected files.read in aggregated tools")
	}
	if !nameSet["files.write"] {
		t.Error("expected files.write in aggregated tools")
	}
	if !nameSet["database.read"] {
		t.Error("expected database.read in aggregated tools (renamed due to collision)")
	}
	if !nameSet["database.query"] {
		t.Error("expected database.query in aggregated tools")
	}
}

// TestGetServerList tests retrieving list of servers in catalog.
func TestGetServerList(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	server1 := "server-a"
	server2 := "server-b"

	tc.(*catalog.ToolCatalog).UpsertToolsForTest(server1, []config.ToolInfo{{Name: "tool1"}}) //nolint:errcheck
	tc.(*catalog.ToolCatalog).UpsertToolsForTest(server2, []config.ToolInfo{{Name: "tool2"}}) //nolint:errcheck

	servers, err := tc.GetServerList()
	if err != nil {
		t.Fatalf("failed to get server list: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}

	expectedServers := map[string]bool{server1: true, server2: true}
	for _, server := range servers {
		if !expectedServers[server] {
			t.Errorf("unexpected server %q in list", server)
		}
	}
}

// TestClear tests clearing all tools from catalog.
func TestClear(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	// Insert some tools.
	tc.(*catalog.ToolCatalog).UpsertToolsForTest("server", []config.ToolInfo{
		{Name: "tool1"},
		{Name: "tool2"},
	}) //nolint:errcheck

	count, _ := tc.GetToolCount()
	if count != 2 {
		t.Errorf("expected 2 tools before clear, got %d", count)
	}

	// Clear catalog.
	if err := tc.Clear(); err != nil {
		t.Fatalf("failed to clear catalog: %v", err)
	}

	count, _ = tc.GetToolCount()
	if count != 0 {
		t.Errorf("expected 0 tools after clear, got %d", count)
	}
}

// TestGetToolsListResponse tests building a tools/list response.
func TestGetToolsListResponse(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	serverName := "test-server"
	tools := []config.ToolInfo{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
	}

	tc.(*catalog.ToolCatalog).UpsertToolsForTest(serverName, tools) //nolint:errcheck

	response, err := tc.GetToolsListResponse(serverName, nil)
	if err != nil {
		t.Fatalf("failed to get tools list response: %v", err)
	}

	if len(response) != 2 {
		t.Errorf("expected 2 tools in response, got %d", len(response))
	}
}

// TestSetUpstreamServers tests setting upstream server URLs.
func TestSetUpstreamServers(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	servers := map[string]string{
		"server-a": "http://localhost:3001",
		"server-b": "http://localhost:3002",
	}

	tc.SetUpstreamServers(servers)

	// Verify servers can be used in sync (we won't actually connect,
	// just verify they're stored).
	internalTC := tc.(*catalog.ToolCatalog)
	storedServers := internalTC.GetUpstreamServersForTest()

	if len(storedServers) != 2 {
		t.Errorf("expected 2 upstream servers, got %d", len(storedServers))
	}

	if storedServers["server-a"] != "http://localhost:3001" {
		t.Errorf("expected server-a URL to be http://localhost:3001, got %s", storedServers["server-a"])
	}
}

// TestStartAndStop tests catalog start and stop lifecycle.
func TestStartAndStop(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled: true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}

	// Start catalog (will try to sync, but no servers configured).
	tc.Start()

	// Give it a moment to initialize.
	time.Sleep(100 * time.Millisecond)

	// Stop should not panic.
	tc.Stop()
}

// TestEmptyCatalog tests operations on an empty catalog.
func TestEmptyCatalog(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	// Get tools from empty catalog should return empty slice.
	tools, err := tc.GetFilteredTools("nonexistent", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools from empty catalog, got %d", len(tools))
	}

	// Get tool count should return 0.
	count, err := tc.GetToolCount()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 tool count, got %d", count)
	}

	// Get server list should return empty slice.
	servers, err := tc.GetServerList()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}
}

// TestToolUpdate tests that upserting tools updates existing entries.
func TestToolUpdate(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	serverName := "test-server"

	// Initial tools.
	initialTools := []config.ToolInfo{
		{Name: "tool1", Description: "Original description"},
		{Name: "tool2", Description: "Tool two"},
	}
	tc.(*catalog.ToolCatalog).UpsertToolsForTest(serverName, initialTools) //nolint:errcheck

	// Updated tools (tool1 description changed, tool3 added, tool2 removed).
	updatedTools := []config.ToolInfo{
		{Name: "tool1", Description: "Updated description"},
		{Name: "tool3", Description: "New tool"},
	}
	tc.(*catalog.ToolCatalog).UpsertToolsForTest(serverName, updatedTools) //nolint:errcheck

	// Verify update.
	filtered, err := tc.GetFilteredTools(serverName, nil)
	if err != nil {
		t.Fatalf("failed to get filtered tools: %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("expected 2 tools after update, got %d", len(filtered))
	}

	// Check tool1 has updated description.
	for _, tool := range filtered {
		if tool.Name == "tool1" && tool.Description != "Updated description" {
			t.Errorf("expected tool1 description to be 'Updated description', got %q", tool.Description)
		}
		if tool.Name == "tool2" {
			t.Error("tool2 should have been removed")
		}
	}
}

// TestConcurrentAccess tests concurrent reads and writes to catalog.
func TestConcurrentAccess(t *testing.T) {
	dbPath := setupTempDB(t)
	defer cleanupDB(dbPath)

	cfg := &catalog.CatalogConfig{
		Enabled:      true,
		DatabasePath: dbPath,
		UpstreamURLs: map[string]string{},
	}

	tc, err := catalog.NewToolCatalog(cfg)
	if err != nil {
		t.Fatalf("failed to create catalog: %v", err)
	}
	defer tc.Stop()

	serverName := "test-server"
	done := make(chan bool)

	// Writer goroutine.
	go func() {
		for i := 0; i < 10; i++ {
			tools := []config.ToolInfo{
				{Name: fmt.Sprintf("tool_%d", i), Description: fmt.Sprintf("Tool %d", i)},
			}
			tc.(*catalog.ToolCatalog).UpsertToolsForTest(serverName, tools) //nolint:errcheck
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Reader goroutine.
	go func() {
		for i := 0; i < 10; i++ {
			_, _ = tc.GetFilteredTools(serverName, nil)
			_, _ = tc.GetAggregatedTools()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines.
	<-done
	<-done
}
