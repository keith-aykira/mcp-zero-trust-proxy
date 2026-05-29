package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// SyncServerForTest directly syncs a single server (for testing).
func (tc *ToolCatalog) SyncServerForTest(serverName, serverURL string) error {
	return tc.syncServer(serverName, serverURL)
}

// ToolCatalog manages tool metadata from upstream MCP servers.
type ToolCatalog struct {
	db         *sql.DB
	mu         sync.RWMutex
	httpClient *http.Client
	stopChan   chan struct{}
	config     *CatalogConfig
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewToolCatalog creates a new tool catalog with SQLite storage.
func NewToolCatalog(cfg *CatalogConfig) (*ToolCatalog, error) {
	// Open SQLite database with WAL mode for better concurrency.
	connStr := cfg.DatabasePath
	if !strings.Contains(connStr, "_busy_timeout") {
		connStr += "?_busy_timeout=5000&_journal_mode=WAL"
	}

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Initialize schema.
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	ctx, cancel := context.CancelFunc(func() {})
	ctx, cancel = context.WithCancel(context.Background())

	tc := &ToolCatalog{
		db: db,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: createHTTPTransport(),
		},
		stopChan: make(chan struct{}),
		config:   cfg,
		ctx:      ctx,
		cancel:   cancel,
	}

	return tc, nil
}

// Start begins the periodic update loop for cataloging MCP server tools.
func (tc *ToolCatalog) Start() {
	log.Info().Str("db", tc.config.DatabasePath).
		Dur("interval", tc.config.RefreshInterval).
		Msg("Starting tool catalog background updater")

	// Initial sync.
	tc.syncAllServers()

	go tc.updateLoop()
}

// Stop halts the periodic update loop and closes the database.
func (tc *ToolCatalog) Stop() {
	log.Info().Msg("Stopping tool catalog")
	tc.cancel()
	close(tc.stopChan)
	if tc.db != nil {
		if err := tc.db.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close catalog database")
		}
	}
}

// updateLoop runs periodically to fetch and update tool metadata.
func (tc *ToolCatalog) updateLoop() {
	ticker := time.NewTicker(tc.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tc.stopChan:
			return
		case <-tc.ctx.Done():
			return
		case <-ticker.C:
			log.Debug().Msg("Running scheduled tool catalog sync")
			tc.syncAllServers()
		}
	}
}

// syncAllServers fetches tools/list from all configured servers.
func (tc *ToolCatalog) syncAllServers() {
	servers := tc.getUpstreamServers()
	if len(servers) == 0 {
		log.Debug().Msg("No upstream servers configured for catalog sync")
		return
	}

	log.Debug().Int("servers", len(servers)).Msg("Syncing tools catalog")

	var wg sync.WaitGroup
	errChan := make(chan error, len(servers))

	for name, url := range servers {
		wg.Add(1)
		go func(serverName, serverURL string) {
			defer wg.Done()
			if err := tc.syncServer(serverName, serverURL); err != nil {
				errChan <- fmt.Errorf("%s: %w", serverName, err)
			}
		}(name, url)
	}

	wg.Wait()
	close(errChan)

	// Collect and log errors.
	errCount := 0
	for err := range errChan {
		errCount++
		log.Warn().Err(err).Msg("Failed to sync server tools")
	}

	log.Info().Int("servers", len(servers)).Int("errors", errCount).
		Msg("Tool catalog sync completed")
}

// syncServer fetches tools/list from a single server.
func (tc *ToolCatalog) syncServer(serverName, serverURL string) error {
	// Build tools/list JSON-RPC request.
	reqBody := []byte(`{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}`)

	req, err := http.NewRequest(tc.ctx, http.MethodPost, serverURL, strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	// Parse JSON-RPC response.
	var rpcResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("upstream error: %s", rpcResp.Error.Message)
	}

	// Parse tools list.
	var toolsList struct {
		Tools []config.ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(rpcResp.Result, &toolsList); err != nil {
		return fmt.Errorf("parse tools: %w", err)
	}

	// Store in database.
	if err := tc.upsertTools(serverName, toolsList.Tools); err != nil {
		return fmt.Errorf("upsert tools: %w", err)
	}

	log.Debug().Str("server", serverName).Int("tools", len(toolsList.Tools)).
		Msg("Synced server tools")
	return nil
}

// upsertTools inserts or updates tools for a server.
func (tc *ToolCatalog) upsertTools(serverName string, tools []config.ToolInfo) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tx, err := tc.db.BeginTx(tc.ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing tools for this server.
	if _, err := tx.ExecContext(tc.ctx, "DELETE FROM tools WHERE server_name = ?", serverName); err != nil {
		return fmt.Errorf("delete existing: %w", err)
	}

	if len(tools) == 0 {
		return tx.Commit()
	}

	// Insert new tools.
	stmt, err := tx.PrepareContext(tc.ctx,
		"INSERT INTO tools (server_name, name, description, input_schema, updated_at) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, tool := range tools {
		inputSchemaJSON, err := json.Marshal(tool.InputSchema)
		if err != nil {
			inputSchemaJSON = []byte("{}")
		}

		_, err = stmt.ExecContext(tc.ctx, serverName, tool.Name, tool.Description, inputSchemaJSON, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("exec insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetFilteredTools returns tools from the catalog filtered by server and allowed tools.
func (tc *ToolCatalog) GetFilteredTools(serverName string, allowedTools map[string]bool) ([]config.ToolInfo, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	var query string
	var args []interface{}

	if serverName != "" {
		query = "SELECT name, description, input_schema FROM tools WHERE server_name = ?"
		args = append(args, serverName)
	} else {
		query = "SELECT name, description, input_schema FROM tools"
	}

	// Apply allowed tools filter if provided (for restricted roles).
	if allowedTools != nil && len(allowedTools) > 0 {
		toolNames := make([]string, 0, len(allowedTools))
		for name := range allowedTools {
			toolNames = append(toolNames, name)
		}

		placeholders := make([]string, len(toolNames))
		for i := range toolNames {
			placeholders[i] = "?"
			args = append(args, toolNames[i])
		}

		query += " AND name IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += " ORDER BY name"

	rows, err := tc.db.QueryContext(tc.ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tools: %w", err)
	}
	defer rows.Close()

	tools := make([]config.ToolInfo, 0)
	for rows.Next() {
		var t config.ToolInfo
		var inputSchemaJSON []byte
		if err := rows.Scan(&t.Name, &t.Description, &inputSchemaJSON); err != nil {
			log.Warn().Err(err).Msg("Failed to scan tool row")
			continue
		}
		if len(inputSchemaJSON) > 0 {
			if err := json.Unmarshal(inputSchemaJSON, &t.InputSchema); err != nil {
				log.Warn().Err(err).Str("tool", t.Name).Msg("Failed to unmarshal input schema")
				t.InputSchema = nil
			}
		}
		tools = append(tools, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return tools, nil
}

// GetAggregatedTools returns tools from all servers with server-name. prefix.
func (tc *ToolCatalog) GetAggregatedTools() ([]config.ToolInfo, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	rows, err := tc.db.QueryContext(tc.ctx,
		"SELECT server_name, name, description, input_schema FROM tools ORDER BY server_name, name")
	if err != nil {
		return nil, fmt.Errorf("query tools: %w", err)
	}
	defer rows.Close()

	tools := make([]config.ToolInfo, 0)
	nameSet := make(map[string]bool)

	for rows.Next() {
		var t config.ToolInfo
		var inputSchemaJSON []byte
		if err := rows.Scan(&t.ServerName, &t.Name, &t.Description, &inputSchemaJSON); err != nil {
			log.Warn().Err(err).Msg("Failed to scan tool row")
			continue
		}

		// Create namespaced name.
		namespacedName := fmt.Sprintf("%s.%s", t.ServerName, t.Name)

		// Check for collision.
		if nameSet[t.Name] {
			// Collision: rename to {name}.{server}.
			namespacedName = fmt.Sprintf("%s.%s", t.Name, t.ServerName)
			log.Debug().
				Str("original", t.Name).
				Str("renamed", namespacedName).
				Str("server", t.ServerName).
				Msg("Tool name collision, renaming")
		}
		nameSet[t.Name] = true

		t.Name = namespacedName
		if len(inputSchemaJSON) > 0 {
			if err := json.Unmarshal(inputSchemaJSON, &t.InputSchema); err != nil {
				t.InputSchema = nil
			}
		}
		tools = append(tools, t)
	}

	return tools, rows.Err()
}

// GetServerList returns the list of all servers in the catalog.
func (tc *ToolCatalog) GetServerList() ([]string, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	rows, err := tc.db.QueryContext(tc.ctx, "SELECT DISTINCT server_name FROM tools ORDER BY server_name")
	if err != nil {
		return nil, fmt.Errorf("query servers: %w", err)
	}
	defer rows.Close()

	servers := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		servers = append(servers, name)
	}

	return servers, rows.Err()
}

// GetToolsListResponse builds a tools/list response structure.
func (tc *ToolCatalog) GetToolsListResponse(serverName string, allowedTools map[string]bool) ([]config.ToolInfo, error) {
	tools, err := tc.GetFilteredTools(serverName, allowedTools)
	if err != nil {
		return nil, err
	}
	return tools, nil
}

// Clear removes all cataloged tools from the database.
func (tc *ToolCatalog) Clear() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	_, err := tc.db.ExecContext(tc.ctx, "DELETE FROM tools")
	return err
}

// GetToolCount returns the total number of tools in the catalog.
func (tc *ToolCatalog) GetToolCount() (int, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	var count int
	err := tc.db.QueryRowContext(tc.ctx, "SELECT COUNT(*) FROM tools").Scan(&count)
	return count, err
}

// ForceRefresh triggers an immediate sync of all servers.
func (tc *ToolCatalog) ForceRefresh() {
	tc.syncAllServers()
}

// GetUpstreamServers returns the configured upstream server URLs.
// Exported for testing.
func (tc *ToolCatalog) GetUpstreamServers() map[string]string {
	return tc.config.UpstreamURLs
}

// UpsertToolsForTest directly inserts/updates tools without HTTP fetch.
// Exported for testing.
func (tc *ToolCatalog) UpsertToolsForTest(serverName string, tools []config.ToolInfo) error {
	return tc.upsertTools(serverName, tools)
}

// getUpstreamServers returns the configured upstream server URLs.
func (tc *ToolCatalog) getUpstreamServers() map[string]string {
	return tc.config.UpstreamURLs
}

// SetUpstreamServers updates the upstream server URLs.
func (tc *ToolCatalog) SetUpstreamServers(servers map[string]string) {
	tc.config.UpstreamURLs = servers
}

// initSchema creates the database tables.
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tools (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		server_name TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		input_schema TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(server_name, name)
	);
	CREATE INDEX IF NOT EXISTS idx_tools_server ON tools(server_name);
	CREATE INDEX IF NOT EXISTS idx_tools_name ON tools(name);
	`
	_, err := db.Exec(schema)
	return err
}

// createHTTPTransport creates a secure HTTP transport.
func createHTTPTransport() http.RoundTripper {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}
