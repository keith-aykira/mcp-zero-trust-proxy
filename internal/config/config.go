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

	// Validate audit sinks
	for i, sink := range cfg.Audit.Sinks {
		sinkIndex := i + 1
		if sink.Type == "" {
			errs = append(errs, fmt.Sprintf("audit.sinks[%d]: type is required", sinkIndex))
			continue
		}
		validSinkTypes := map[string]bool{"ocsf": true, "cef": true, "json_http": true}
		if !validSinkTypes[sink.Type] {
			errs = append(errs, fmt.Sprintf("audit.sinks[%d]: unknown type %q: valid types are ocsf, cef, json_http", sinkIndex, sink.Type))
			continue
		}
		// Validate type-specific config
		switch sink.Type {
		case "ocsf":
			if sink.OCSF == nil {
				errs = append(errs, fmt.Sprintf("audit.sinks[%d]: ocsf config is required for type \"ocsf\"", sinkIndex))
			} else {
				if strings.TrimSpace(sink.OCSF.WorkspaceID) == "" {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: ocsf.workspace_id is required", sinkIndex))
				}
				if strings.TrimSpace(sink.OCSF.APIKey) == "" {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: ocsf.api_key is required (can use ${ENV_VAR} syntax)", sinkIndex))
				}
				if sink.OCSF.BatchSize < 1 {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: ocsf.batch_size must be >= 1", sinkIndex))
				}
			}
		case "cef":
			if sink.CEF == nil {
				errs = append(errs, fmt.Sprintf("audit.sinks[%d]: cef config is required for type \"cef\"", sinkIndex))
			} else {
				if strings.TrimSpace(sink.CEF.Host) == "" {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: cef.host is required", sinkIndex))
				}
				validTransports := map[string]bool{"udp": true, "tcp": true, "tcp_tls": true, "https": true}
				if sink.CEF.Transport != "" && !validTransports[sink.CEF.Transport] {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: unknown cef.transport %q: valid transports are udp, tcp, tcp_tls, https", sinkIndex, sink.CEF.Transport))
				}
				if sink.CEF.Port < 1 || sink.CEF.Port > 65535 {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: cef.port must be between 1 and 65535", sinkIndex))
				}
			}
		case "json_http":
			if sink.JSONHTTP == nil {
				errs = append(errs, fmt.Sprintf("audit.sinks[%d]: json_http config is required for type \"json_http\"", sinkIndex))
			} else {
				if strings.TrimSpace(sink.JSONHTTP.Endpoint) == "" {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: json_http.endpoint is required", sinkIndex))
				}
				// Validate endpoint is HTTPS
				if !strings.HasPrefix(sink.JSONHTTP.Endpoint, "https://") {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: json_http.endpoint must use HTTPS protocol", sinkIndex))
				}
			}
		}
		// Validate filter config
		if sink.Filter != nil {
			for _, result := range sink.Filter.Results {
				validResults := map[string]bool{"allowed": true, "denied": true}
				if !validResults[result] {
					errs = append(errs, fmt.Sprintf("audit.sinks[%d]: unknown filter result %q: valid values are allowed, denied", sinkIndex, result))
				}
			}
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

	// Validate PII masking configuration
	validatePIIMasking(cfg, &errs)

	if len(errs) > 0 {
		return fmt.Errorf("configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// validatePIIMasking validates the PII masking configuration.
func validatePIIMasking(cfg *Config, errs *[]string) {
	if len(cfg.PIIMasking.Patterns) == 0 && len(cfg.PIIMasking.SensitivityClasses) == 0 && len(cfg.PIIMasking.ToolClassAssignments) == 0 {
		return
	}

	// Build a map of defined pattern names for validation
	definedPatterns := make(map[string]bool)
	for _, pattern := range cfg.PIIMasking.Patterns {
		if pattern.Name == "" {
			*errs = append(*errs, "pii_masking.patterns: pattern name is required")
		} else {
			if definedPatterns[pattern.Name] {
				*errs = append(*errs, fmt.Sprintf("pii_masking.patterns: duplicate pattern name %q", pattern.Name))
			}
			definedPatterns[pattern.Name] = true
		}

		// Pattern regex is required
		if strings.TrimSpace(pattern.Pattern) == "" {
			*errs = append(*errs, fmt.Sprintf("pii_masking.patterns[%q]: pattern regex is required", pattern.Name))
		} else {
			// Validate regex syntax
			if _, err := regexp.Compile(pattern.Pattern); err != nil {
				*errs = append(*errs, fmt.Sprintf("pii_masking.patterns[%q]: invalid regex %q: %v", pattern.Name, pattern.Pattern, err))
			}
		}

		// Validate partial_mask.filler is exactly one character if set
		if pattern.PartialMask != nil && pattern.PartialMask.Filler != "" {
			if len(pattern.PartialMask.Filler) != 1 {
				*errs = append(*errs, fmt.Sprintf("pii_masking.patterns[%q]: partial_mask.filler must be exactly one character", pattern.Name))
			}
		}
	}

	// Build a map of defined sensitivity class names for validation
	definedClasses := make(map[string]bool)
	for _, class := range cfg.PIIMasking.SensitivityClasses {
		if class.Name == "" {
			*errs = append(*errs, "pii_masking.sensitivity_classes: class name is required")
		} else {
			if definedClasses[class.Name] {
				*errs = append(*errs, fmt.Sprintf("pii_masking.sensitivity_classes: duplicate class name %q", class.Name))
			}
			definedClasses[class.Name] = true
		}

		// Validate all pattern references exist
		for _, patternName := range class.PatternNames {
			if !definedPatterns[patternName] {
				*errs = append(*errs, fmt.Sprintf("pii_masking.sensitivity_classes[%q]: references undefined pattern %q", class.Name, patternName))
			}
		}
	}

	// Validate tool class assignments reference defined classes
	for toolPattern, className := range cfg.PIIMasking.ToolClassAssignments {
		if toolPattern == "" {
			*errs = append(*errs, "pii_masking.tool_class_assignments: tool pattern cannot be empty")
			continue
		}
		if !definedClasses[className] {
			*errs = append(*errs, fmt.Sprintf("pii_masking.tool_class_assignments[%q]: references undefined class %q", toolPattern, className))
		}
	}
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

	// Apply defaults to sink configurations
	for i := range cfg.Audit.Sinks {
		sink := &cfg.Audit.Sinks[i]
		// Default name to type if not set
		if sink.Name == "" {
			sink.Name = sink.Type
		}
		// Default enabled to true
		if sink.Enabled {
			// Already true, nothing to do
		} else if sink.Type != "" {
			// Type is set but enabled is false - respect user choice
			// But if we want true as default when not explicitly set, we'd need to track that
			// For now, false means disabled, true (or absent with default) means enabled
			sink.Enabled = true
		}
		// Apply OCSF defaults
		if sink.Type == "ocsf" && sink.OCSF != nil {
			if sink.OCSF.BatchSize == 0 {
				sink.OCSF.BatchSize = 100
			}
			if sink.OCSF.FlushInterval == 0 {
				sink.OCSF.FlushInterval = 5
			}
			if sink.OCSF.Timeout == 0 {
				sink.OCSF.Timeout = 30
			}
			if sink.OCSF.BufferSize == 0 {
				sink.OCSF.BufferSize = 1000
			}
		}
		// Apply CEF defaults
		if sink.Type == "cef" && sink.CEF != nil {
			if sink.CEF.Transport == "" {
				sink.CEF.Transport = "udp"
			}
			if sink.CEF.Port == 0 {
				switch sink.CEF.Transport {
				case "tcp_tls":
					sink.CEF.Port = 6514
				case "udp", "tcp":
					sink.CEF.Port = 514
				case "https":
					sink.CEF.Port = 443
				}
			}
			if sink.CEF.Facility == "" {
				sink.CEF.Facility = "local0"
			}
			if sink.CEF.BatchSize == 0 && sink.CEF.Transport == "https" {
				sink.CEF.BatchSize = 10
			}
			if sink.CEF.FlushInterval == 0 && sink.CEF.Transport == "https" {
				sink.CEF.FlushInterval = 1
			}
			if sink.CEF.Timeout == 0 {
				sink.CEF.Timeout = 5
			}
			if sink.CEF.BufferSize == 0 {
				sink.CEF.BufferSize = 1000
			}
		}
		// Apply JSON-HTTP defaults
		if sink.Type == "json_http" && sink.JSONHTTP != nil {
			if sink.JSONHTTP.BatchSize == 0 {
				sink.JSONHTTP.BatchSize = 50
			}
			if sink.JSONHTTP.FlushInterval == 0 {
				sink.JSONHTTP.FlushInterval = 10
			}
			if sink.JSONHTTP.Timeout == 0 {
				sink.JSONHTTP.Timeout = 15
			}
			if sink.JSONHTTP.BufferSize == 0 {
				sink.JSONHTTP.BufferSize = 1000
			}
		}
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
