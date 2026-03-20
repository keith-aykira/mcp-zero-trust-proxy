package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// validRoleNames is the set of accepted RBAC role names.
var validRoleNames = map[string]bool{
	"admin":      true,
	"readonly":   true,
	"restricted": true,
}

// envVarPattern matches ${ENV_VAR} syntax for secret injection.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Load reads the YAML configuration file at path, applies defaults, and returns a validated Config.
// Environment variables referenced with ${VAR} syntax are substituted before parsing.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	// Substitute ${ENV_VAR} references with their environment variable values.
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	applyDefaults(&cfg)

	return &cfg, nil
}

// Validate checks the configuration for required fields, valid values, and consistency.
// It collects all validation errors and returns them as a single combined error.
func Validate(cfg *Config) error {
	var errs []string

	// Required: server.upstream_url
	if strings.TrimSpace(cfg.Server.UpstreamURL) == "" {
		errs = append(errs, "server.upstream_url is required")
	}

	// Validate role names
	for _, role := range cfg.Roles {
		if !validRoleNames[role.Name] {
			errs = append(errs, fmt.Sprintf("unknown role name %q: valid roles are admin, readonly, restricted", role.Name))
		}
	}

	// Validate auth provider if set
	if cfg.Auth.Provider != "" {
		validProviders := map[string]bool{"github": true, "google": true, "oidc": true}
		if !validProviders[cfg.Auth.Provider] {
			errs = append(errs, fmt.Sprintf("unknown auth provider %q: valid providers are github, google, oidc", cfg.Auth.Provider))
		}
		// OIDC requires an issuer URL
		if cfg.Auth.Provider == "oidc" && strings.TrimSpace(cfg.Auth.IssuerURL) == "" {
			errs = append(errs, "auth.issuer_url is required when provider is \"oidc\"")
		}
	}

	// Validate audit output
	if cfg.Audit.Output != "" {
		validOutputs := map[string]bool{"stdout": true, "file": true, "both": true}
		if !validOutputs[cfg.Audit.Output] {
			errs = append(errs, fmt.Sprintf("unknown audit output %q: valid outputs are stdout, file, both", cfg.Audit.Output))
		}
		// File output requires a file path
		if (cfg.Audit.Output == "file" || cfg.Audit.Output == "both") && strings.TrimSpace(cfg.Audit.FilePath) == "" {
			errs = append(errs, "audit.file_path is required when audit.output is \"file\" or \"both\"")
		}
	}

	// Validate log level
	if cfg.Logging.Level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[cfg.Logging.Level] {
			errs = append(errs, fmt.Sprintf("unknown log level %q: valid levels are debug, info, warn, error", cfg.Logging.Level))
		}
	}

	// Validate log format
	if cfg.Logging.Format != "" {
		validFormats := map[string]bool{"json": true, "text": true}
		if !validFormats[cfg.Logging.Format] {
			errs = append(errs, fmt.Sprintf("unknown log format %q: valid formats are json, text", cfg.Logging.Format))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// applyDefaults fills in default values for optional configuration fields.
func applyDefaults(cfg *Config) {
	// Server defaults
	if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = ":8080"
	}

	// Rate limit defaults
	if cfg.RateLimit.RequestsPerMinute == 0 {
		cfg.RateLimit.RequestsPerMinute = 100
	}
	if cfg.RateLimit.BurstSize == 0 {
		cfg.RateLimit.BurstSize = 10
	}

	// Audit defaults
	// Note: bool zero-value is false, but we want default enabled=true.
	// We detect "not set" by checking if output is also empty — if a user explicitly
	// sets enabled: false that will be respected. We default to true only when
	// the entire audit block is absent (output is also at its zero value).
	if !cfg.Audit.Enabled && cfg.Audit.Output == "" {
		cfg.Audit.Enabled = true
	}
	if cfg.Audit.Output == "" {
		cfg.Audit.Output = "stdout"
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	// Default roles: if no roles are configured, add the 3 built-in roles
	if len(cfg.Roles) == 0 {
		cfg.Roles = defaultRoles()
	}
}

// defaultRoles returns the 3 built-in RBAC role configurations.
func defaultRoles() []RoleConfig {
	return []RoleConfig{
		{
			Name:         "admin",
			AllowedTools: []string{}, // empty = all tools allowed
			DenyTools:    []string{},
		},
		{
			Name: "readonly",
			AllowedTools: []string{
				"tools/list",
				"resources/list",
				"resources/read",
				"prompts/list",
				"prompts/get",
			},
			DenyTools: []string{},
		},
		{
			Name:         "restricted",
			AllowedTools: []string{"tools/list"},
			DenyTools:    []string{},
		},
	}
}

// expandEnvVars replaces ${ENV_VAR} references in the YAML content with environment variable values.
// If an environment variable is not set, the placeholder is left as-is.
func expandEnvVars(content string) string {
	return envVarPattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := match[2 : len(match)-1]
		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		// Leave unresolved references as-is
		return match
	})
}
