package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/AnobleSCM/mcp-zero-trust-proxy/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Parse CLI flags
	configPath := flag.String("config", "./config.yaml", "Path to YAML configuration file")
	flag.Parse()

	// Configure logger (console output for startup)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config from %q: %v\n", *configPath, err)
		os.Exit(1)
	}

	// Validate configuration
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Print loaded config summary
	fmt.Printf("MCP Zero-Trust Proxy\n")
	fmt.Printf("  Upstream URL:  %s\n", cfg.Server.UpstreamURL)
	fmt.Printf("  Listen Addr:   %s\n", cfg.Server.ListenAddr)
	fmt.Printf("  Auth Provider: %s\n", cfg.Auth.Provider)
	fmt.Printf("  Roles:         %d defined\n", len(cfg.Roles))
	fmt.Printf("  Rate Limit:    %d req/min (burst: %d)\n", cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.BurstSize)
	fmt.Printf("  Audit:         enabled=%v, output=%s\n", cfg.Audit.Enabled, cfg.Audit.Output)
	fmt.Printf("  Log Level:     %s\n", cfg.Logging.Level)

	// TODO(Plan 06): Wire proxy server startup here
	// server := proxy.NewServer(cfg)
	// log.Info().Str("addr", cfg.Server.ListenAddr).Msg("Starting proxy server")
	// if err := server.ListenAndServe(); err != nil {
	//     log.Fatal().Err(err).Msg("Server error")
	// }

	log.Info().
		Str("upstream", cfg.Server.UpstreamURL).
		Str("listen", cfg.Server.ListenAddr).
		Int("roles", len(cfg.Roles)).
		Msg("Configuration loaded successfully (proxy startup not yet wired)")
}
