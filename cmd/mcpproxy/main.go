package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/audit"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/auth"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/proxy"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/rbac"
	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/ratelimit"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// version is injected at build time via -ldflags "-X main.version=..."
var version = "dev"

func main() {
	// Parse CLI flags
	configPath := flag.String("config", "./config.yaml", "Path to YAML configuration file")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("mcpproxy version %s\n", version)
		os.Exit(0)
	}

	// Configure logger (console output during startup; switches to JSON for production)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Str("config", *configPath).Msg("Failed to load configuration")
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
	// Start background goroutine for periodic cache cleanup (expired tokens/state entries)
	authenticator.StartCleanup()
	defer authenticator.StopCleanup()

	// Step 3: RBAC engine
	rbacEngine := rbac.NewEngine(cfg.Roles)

	// Step 4: Rate limiter
	rateLimiter := ratelimit.NewLimiter(&cfg.RateLimit)

	// Start periodic cleanup for stale rate limiter entries (every 10 minutes)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimiter.Cleanup(1 * time.Hour)
		}
	}()

	// Step 5: Audit logger
	auditLogger, err := audit.NewLogger(&cfg.Audit)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize audit logger")
	}
	defer auditLogger.Close() //nolint:errcheck

	// Step 6: Proxy handler
	handler, err := proxy.NewHandler(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize proxy handler")
	}

	// Step 7: Pipeline — wires all middleware together
	pipeline := proxy.NewPipeline(
		handler,
		authenticator,
		rateLimiter,
		rbacEngine,
		auditLogger,
		authenticator, // Authenticator also implements AuthHandler (HandleAuthStart, HandleCallback)
	)

	// Register routes
	mux := http.NewServeMux()
	mux.Handle("/", pipeline)

	// HTTP server with timeouts
	server := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	tlsEnabled := cfg.Server.TLS.CertFile != "" && cfg.Server.TLS.KeyFile != ""
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
