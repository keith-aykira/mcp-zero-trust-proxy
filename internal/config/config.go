package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// envVarPattern matches ${ENV_VAR} syntax for secret injection.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// builtInRoleNames is the set of built-in RBAC role names that are always present.
var builtInRoleNames = map[string]bool{
	"admin":      true,
	"readonly":   true,
	"restricted": true,
}

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

	// Validate role names and collect all role names from config
	allRoleNames := make(map[string]bool)
	for _, role := range cfg.Roles {
		if role.Name == "" {
			errs = append(errs, "role.name is required")
		} else {
			allRoleNames[role.Name] = true
		}
	}

	// Add user_roles roles to the set
	if cfg.UserRoles.Default != "" {
		allRoleNames[cfg.UserRoles.Default] = true
	}
	for _, role := range cfg.UserRoles.Mapping {
		if role != "" {
			allRoleNames[role] = true
		}
	}
	for _, rule := range cfg.UserRoles.ClaimMapping {
		if rule.Role != "" {
			allRoleNames[rule.Role] = true
		}
	}

	// Validate that all referenced roles are defined in roles section
	for roleName := range allRoleNames {
		if !builtInRoleNames[roleName] {
			found := false
			for _, role := range cfg.Roles {
				if role.Name == roleName {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, fmt.Sprintf("role %q referenced in user_roles but not defined in roles section", roleName))
			}
		}
	}

	// Validate claim mapping rules
	validOperators := map[string]bool{"equals": true, "contains": true, "starts_with": true, "ends_with": true, "regex": true}
	for i, rule := range cfg.UserRoles.ClaimMapping {
		ruleIndex := i + 1
		if rule.Claim == "" {
			errs = append(errs, fmt.Sprintf("claim_mapping[%d]: claim name is required", ruleIndex))
		}
		if !validOperators[rule.Operator] {
			errs = append(errs, fmt.Sprintf("claim_mapping[%d]: unknown operator %q: valid operators are equals, contains, starts_with, ends_with, regex", ruleIndex, rule.Operator))
		}
		// Validate regex syntax if operator is "regex"
		if rule.Operator == "regex" {
			_, err := regexp.Compile(rule.Value)
			if err != nil {
				errs = append(errs, fmt.Sprintf("claim_mapping[%d]: invalid regex %q: %v", ruleIndex, rule.Value, err))
			}
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

	// Validate user restrictions regex patterns
	if cfg.UserRestrictions.AllowRegex != "" {
		_, err := regexp.Compile(cfg.UserRestrictions.AllowRegex)
		if err != nil {
			errs = append(errs, fmt.Sprintf("user_restrictions.allow_regex is invalid: %v", err))
		}
	}
	if cfg.UserRestrictions.DenyRegex != "" {
		_, err := regexp.Compile(cfg.UserRestrictions.DenyRegex)
		if err != nil {
			errs = append(errs, fmt.Sprintf("user_restrictions.deny_regex is invalid: %v", err))
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
	if cfg.Server.MaxBodySize == 0 {
		cfg.Server.MaxBodySize = 1048576 // 1MB
	}

	// UserRoles defaults
	if cfg.UserRoles.Default == "" {
		cfg.UserRoles.Default = "readonly"
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

	// Merge built-in roles with user-defined roles
	mergeRoles(cfg)
}

// mergeRoles ensures built-in roles are always present, merging with user-defined roles.
// User-defined roles with built-in names override the built-in defaults.
// Custom roles are appended to preserve user configuration order.
func mergeRoles(cfg *Config) {
	// Create a map of user-defined roles for quick lookup
	userRoles := make(map[string]RoleConfig)
	for _, role := range cfg.Roles {
		userRoles[role.Name] = role
	}

	// Start with built-in roles
	defaultRoles := defaultRoles()
	result := make([]RoleConfig, 0, len(defaultRoles)+len(userRoles))

	// Add built-in roles, allowing user overrides
	for _, builtin := range defaultRoles {
		if userRole, ok := userRoles[builtin.Name]; ok {
			result = append(result, userRole)
		} else {
			result = append(result, builtin)
		}
	}

	// Append custom roles (user-defined roles with non-built-in names)
	for _, role := range cfg.Roles {
		if !builtInRoleNames[role.Name] {
			result = append(result, role)
		}
	}

	cfg.Roles = result
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
