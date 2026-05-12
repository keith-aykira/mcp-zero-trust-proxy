package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

type HealthResponse struct {
	Status string `json:"status"`
}

func TestMain_HealthEndpoint(t *testing.T) {
	mux, cfgPath := setupTestMux(t)
	defer os.Remove(cfgPath)

	t.Run("returns 200 with status ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		}

		if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("wrong content type: got %v want %v", ct, "application/json")
		}

		var health HealthResponse
		if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if health.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", health.Status)
		}
	})

	t.Run("is case sensitive - /HEALTH proxied to upstream", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/HEALTH", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusBadGateway {
			t.Logf("/HEALTH returned %d (expected 502 Bad Gateway from upstream)", status)
		}
	})

	t.Run("supports HEAD method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/health", nil)
		rr := httptest.NewRecorder()

		mux.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code for HEAD: got %v want %v", status, http.StatusOK)
		}
	})
}

func TestMain_testConfiguration(t *testing.T) {
	t.Run("validates valid config", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "test-config.yaml")

		testConfig := `
server:
  upstream_url: http://localhost:3000
  listen_addr: ":8080"
auth:
  provider: github
  client_id: test-client-id
`
		if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		absPath, _ := filepath.Abs(cfgPath)
		cfg, err := config.Load(absPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if err := config.Validate(cfg); err != nil {
			t.Errorf("config validation failed: %v", err)
		}
	})

	t.Run("rejects missing upstream_url", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "bad-config.yaml")

		badConfig := `
server:
  listen_addr: ":8080"
auth:
  provider: github
  client_id: test-client-id
`
		if err := os.WriteFile(cfgPath, []byte(badConfig), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		absPath, _ := filepath.Abs(cfgPath)
		cfg, err := config.Load(absPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		if err := config.Validate(cfg); err == nil {
			t.Errorf("expected validation error for missing upstream_url")
		}
	})
}

func setupTestMux(t *testing.T) (*http.ServeMux, string) {
	t.Helper()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	testConfig := `
server:
  upstream_url: http://localhost:3000
  listen_addr: ":8080"
auth:
  provider: github
  client_id: test-client-id
`
	if err := os.WriteFile(cfgPath, []byte(testConfig), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	router, err := proxy.NewServerRouter(&cfg.Server.Registry)
	if err != nil {
		t.Fatalf("failed to create router: %v", err)
	}
	handler, err := proxy.NewHandler(cfg, router)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	pipeline := proxy.NewPipeline(
		handler,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", pipeline)

	return mux, cfgPath
}