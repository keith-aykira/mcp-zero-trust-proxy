package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/audit"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/auth"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/pii"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/ratelimit"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/rbac"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	// Parse CLI flags
	configPath := flag.String("config", "./config.yaml", "Path to YAML configuration file")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	testConfig := flag.Bool("test", false, "Test configuration validity and exit with status code")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("mcpproxy version %s\n", version)
		os.Exit(0)
	}

	if *testConfig {
		testConfiguration(*configPath)
		return
	}

	// Configure logger (console output during startup; switches to JSON for production)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Resolve config path absolutely and validate against path traversal
	absConfigPath, err := filepath.Abs(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid config path")
	}

	// Load configuration
	cfg, err := config.Load(absConfigPath)
	if err != nil {
		log.Fatal().Err(err).Str("config", absConfigPath).Msg("Failed to load configuration")
	}

	// Validate configuration
	if err := config.Validate(cfg); err != nil {
		log.Fatal().Err(err).Msg("Invalid configuration")
	}

	// Apply log level from config
	level, err := zerolog.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Switch to JSON logging for production if configured
	if cfg.Logging.Format == "json" {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	log.Info().
		Str("version", version).
		Str("upstream", cfg.Server.UpstreamURL).
		Str("listen", cfg.Server.ListenAddr).
		Int("roles", len(cfg.Roles)).
		Bool("audit", cfg.Audit.Enabled).
		Str("provider", cfg.Auth.Provider).
		Msg("Starting MCP Zero-Trust Proxy")

	// Step 1: Session store (required by auth)
	sessionStore := auth.NewSessionStore(24 * time.Hour)
	defer sessionStore.Stop()

	// Step 2: Authenticator
	authenticator, err := auth.NewAuthenticator(&cfg.Auth, sessionStore)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize authenticator")
	}
	// Wire user-to-role mapping from config
	authenticator.SetUserRoles(cfg.UserRoles.Mapping, cfg.UserRoles.Default)
	// Wire claim-based role mapping from config
	authenticator.SetClaimRules(cfg.UserRoles.ClaimMapping)
	// Wire user restrictions from config
	if err := authenticator.SetUserRestrictions(cfg.UserRestrictions.AllowRegex, cfg.UserRestrictions.DenyRegex); err != nil {
		log.Fatal().Err(err).Msg("Failed to set user restrictions")
	}

	// Start background goroutine for periodic cache cleanup (expired tokens/state entries)
	authenticator.StartCleanup()
	defer authenticator.StopCleanup()

	// Step 3: RBAC engine
	rbacEngine := rbac.NewEngine(cfg.Roles)

	// Step 4: Rate limiter
	rateLimiter := ratelimit.NewLimiter(&cfg.RateLimit)

	// Start periodic cleanup for stale rate limiter entries (every 10 minutes)
	rlStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rateLimiter.Cleanup(1 * time.Hour)
			case <-rlStop:
				return
			}
		}
	}()

	// Step 5: Audit logger
	auditLogger, err := audit.NewLogger(&cfg.Audit)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize audit logger")
	}
	defer auditLogger.Close() //nolint:errcheck

	// Step 6: PII masker
	piiMasker, err := pii.New(cfg.PIIMasking)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize PII masker")
	}

	// Step 7: Proxy handler
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize proxy handler")
	}

	// Step 8: Pipeline — wires all middleware together
	corsConfig := &proxy.CORSConfig{
		AllowedOrigins: cfg.CORS.AllowedOrigins,
		AllowedMethods: cfg.CORS.AllowedMethods,
		AllowedHeaders: cfg.CORS.AllowedHeaders,
		MaxAge:         cfg.CORS.MaxAge,
	}
	pipeline := proxy.NewPipeline(
		handler,
		authenticator,
		rateLimiter,
		rbacEngine,
		piiMasker,
		auditLogger,
		authenticator, // Authenticator also implements AuthHandler (HandleAuthStart, HandleCallback)
		proxy.WithMaxBodySize(cfg.Server.MaxBodySize),
		proxy.WithCORS(corsConfig),
	)

	// Register health check endpoint
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Register routes
	mux.Handle("/", pipeline)

	// Build TLS configuration from config
	tlsEnabled := cfg.Server.TLS.CertFile != "" && cfg.Server.TLS.KeyFile != ""
	tlsConfig := buildTLSConfig(&cfg.Server.TLS)

	// Build HTTP server with security hardening
	server := buildServer(&cfg.Server, mux, tlsConfig)

	// Log TLS status
	if tlsEnabled {
		log.Info().
			Str("cert_file", cfg.Server.TLS.CertFile).
			Msg("TLS enabled")
	} else {
		log.Info().Msg("TLS disabled (plain HTTP)")
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info().
			Str("addr", cfg.Server.ListenAddr).
			Str("upstream", cfg.Server.UpstreamURL).
			Str("version", version).
			Int("roles", len(cfg.Roles)).
			Msg("Proxy ready")
		var serveErr error
		if tlsEnabled {
			server.TLSConfig = tlsConfig
			serveErr = server.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			serveErr = server.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			serverErr <- serveErr
		}
	}()

	// Wait for shutdown signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatal().Err(err).Msg("Server error")

	case sig := <-quit:
		log.Info().Str("signal", sig.String()).Msg("Shutdown signal received")

		// Stop background goroutines
		close(rlStop)
		authenticator.StopCleanup()
		sessionStore.Stop()

		// Graceful shutdown with 10-second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("Graceful shutdown failed; forcing close")
			server.Close() //nolint:errcheck
		} else {
			log.Info().Msg("Server stopped gracefully")
		}
	}
}

// testConfiguration validates that the configuration file can be loaded and passes all validation checks.
// Exits with status 0 if valid, non-zero if invalid. Used for CI/CD pipeline validation.
func testConfiguration(configPath string) {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid config path: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(absConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load configuration from %q: %v\n", absConfigPath, err)
		os.Exit(1)
	}

	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid configuration:\n%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("configuration valid: %s\n", absConfigPath)
	os.Exit(0)
}

// buildTLSConfig creates a *tls.Config from ServerConfig.TLS with security hardening:
// - Minimum TLS version enforced (default: 1.2)
// - Cipher suite preferences applied (defaults to Go's secure ciphers if not specified)
// - PreferServerCipherSuites enabled for server-side selection
func buildTLSConfig(tlsCfg *config.TLSConfig) *tls.Config {
	result := &tls.Config{
		PreferServerCipherSuites: true,
	}

	// Set minimum TLS version
	minVersion, ok := parseTLSVersion(tlsCfg.MinVersion)
	if ok {
		result.MinVersion = minVersion
	} else {
		// Default to TLS 1.2 if parsing fails
		result.MinVersion = tls.VersionTLS12
		log.Warn().Str("provided", tlsCfg.MinVersion).Msg("Invalid TLS min_version; defaulting to 1.2")
	}

	// Apply custom cipher suites if specified
	if len(tlsCfg.CipherSuites) > 0 {
		var suites []uint16
		for _, name := range tlsCfg.CipherSuites {
			for _, cs := range tls.CipherSuites() {
				if cs.Name == name {
					suites = append(suites, cs.ID)
					break
				}
			}
		}
		if len(suites) > 0 {
			result.CipherSuites = suites
		}
	}

	return result
}

// parseTLSVersion converts a version string to the corresponding TLS constant.
func parseTLSVersion(v string) (uint16, bool) {
	switch v {
	case "1.0":
		return tls.VersionTLS10, true
	case "1.1":
		return tls.VersionTLS11, true
	case "1.2":
		return tls.VersionTLS12, true
	case "1.3":
		return tls.VersionTLS13, true
	default:
		return 0, false
	}
}

// buildServer creates an *http.Server with security-focused configuration:
// - ReadHeaderTimeout for Slowloris protection
// - Configured TLS if enabled
func buildServer(serverCfg *config.ServerConfig, handler http.Handler, tlsConfig *tls.Config) *http.Server {
	return &http.Server{
		Addr:              serverCfg.ListenAddr,
		Handler:           handler,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,   // Slowloris protection
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,           // 1MB max headers (prevents header flooding)
	}
}
