package pii

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/config"
	"github.com/keith-aykira/mcp-zero-trust-proxy/internal/proxy"
)

const defaultFullMask = "***REDACTED***"

// compiledPattern holds a pre-compiled regex and its masking configuration.
type compiledPattern struct {
	name        string
	regexp      *regexp.Regexp
	mask        string
	partialMask *partialMaskConfig
}

// partialMaskConfig holds partial masking parameters.
type partialMaskConfig struct {
	showFirst int
	showLast  int
	filler    string
}

// Masker applies PII masking to MCP requests and responses based on sensitivity class configuration.
type Masker struct {
	globalEnabled          bool
	compiledPatterns       map[string]*compiledPattern
	compiledClasses        map[string][]*compiledPattern
	toolClassAssignments   map[string]string
	toolClassCompilers     []*toolClassCompiler
}

// toolClassCompiler matches tool names to sensitivity class assignments.
type toolClassCompiler struct {
	pattern   *regexp.Regexp
	className string
}

// New constructs a PII masker from the configuration.
// All regex patterns are pre-compiled at startup for runtime performance.
// If the config is empty or not set, returns nil (no masking).
func New(cfg config.PIIMaskingConfig) (*Masker, error) {
	if len(cfg.Patterns) == 0 && len(cfg.SensitivityClasses) == 0 && len(cfg.ToolClassAssignments) == 0 {
		return nil, nil
	}

	compiledPatterns := make(map[string]*compiledPattern)

	for _, p := range cfg.Patterns {
		re, err := regexp.Compile(p.Pattern)
		if err != nil {
			return nil, fmt.Errorf("compiling pattern %q: %w", p.Name, err)
		}

		cp := &compiledPattern{
			name:   p.Name,
			regexp: re,
			mask:   resolveMask(p.Mask, p.PartialMask),
		}

		if p.PartialMask != nil && (p.PartialMask.ShowFirst > 0 || p.PartialMask.ShowLast > 0) {
			cp.partialMask = &partialMaskConfig{
				showFirst: p.PartialMask.ShowFirst,
				showLast:  p.PartialMask.ShowLast,
				filler:    resolveFiller(p.PartialMask.Filler),
			}
		}

		compiledPatterns[p.Name] = cp
	}

	compiledClasses := make(map[string][]*compiledPattern)

	for _, c := range cfg.SensitivityClasses {
		var patterns []*compiledPattern

		for _, pn := range c.PatternNames {
			if cp, ok := compiledPatterns[pn]; ok {
				patterns = append(patterns, cp)
			}
		}

		if len(patterns) > 0 {
			compiledClasses[c.Name] = patterns
		}
	}

	var toolClassCompilers []*toolClassCompiler

	for toolPattern, className := range cfg.ToolClassAssignments {
		if _, ok := compiledClasses[className]; !ok {
			continue
		}

		if strings.Contains(toolPattern, "*") {
			reStr := "(?s)^" + regexp.QuoteMeta(toolPattern) + "$"
			reStr = strings.ReplaceAll(reStr, "\\*", ".*")
			re, err := regexp.Compile(reStr)
			if err != nil {
				return nil, fmt.Errorf("compiling tool pattern %q: %w", toolPattern, err)
			}
			toolClassCompilers = append(toolClassCompilers, &toolClassCompiler{
				pattern:   re,
				className: className,
			})
		} else {
			toolClassCompilers = append(toolClassCompilers, &toolClassCompiler{
				pattern:   regexp.MustCompile(fmt.Sprintf(`^%s$`, regexp.QuoteMeta(toolPattern))),
				className: className,
			})
		}
	}

	return &Masker{
		globalEnabled:          cfg.Enabled,
		compiledPatterns:       compiledPatterns,
		compiledClasses:        compiledClasses,
		toolClassAssignments:   cfg.ToolClassAssignments,
		toolClassCompilers:     toolClassCompilers,
	}, nil
}

// resolveMask returns the mask string, defaulting to defaultFullMask if empty.
func resolveMask(mask string, partial *config.PartialMaskConfig) string {
	if mask != "" {
		return mask
	}
	if partial != nil && partial.Filler != "" {
		return strings.Repeat(partial.Filler, 4)
	}
	return defaultFullMask
}

// resolveFiller returns the filler character, defaulting to "*" if empty.
func resolveFiller(filler string) string {
	if len(filler) != 1 {
		return "*"
	}
	return filler
}

// ShouldMaskTool returns true if the given tool name should have PII masking applied.
// A tool is matched if:
//   - exact match in tool_class_assignments, or
//   - regex match against wildcard patterns in tool_class_assignments.
func (m *Masker) ShouldMaskTool(toolName string) bool {
	if m == nil {
		return false
	}

	for _, tc := range m.toolClassCompilers {
		if tc.pattern.MatchString(toolName) {
			return true
		}
	}

	return false
}

// GetSensitiveFields returns the list of sensitive field names that should be masked for the given tool.
func (m *Masker) GetSensitiveFields(toolName string) []string {
	if m == nil {
		return nil
	}

	var classNames []string

	for _, tc := range m.toolClassCompilers {
		if tc.pattern.MatchString(toolName) {
			classNames = append(classNames, tc.className)
		}
	}

	if len(classNames) == 0 {
		return nil
	}

	sensitiveFields := make(map[string]bool)

	for _, cn := range classNames {
		if patterns, ok := m.compiledClasses[cn]; ok {
			for _, p := range patterns {
				sensitiveFields[p.name] = true
			}
		}
	}

	fields := make([]string, 0, len(sensitiveFields))

	for field := range sensitiveFields {
		fields = append(fields, field)
	}

	return fields
}

// MaskRequest applies PII masking to incoming request parameters before forwarding upstream.
// Only masks params for tools/call methods on tools assigned to a sensitivity class.
func (m *Masker) MaskRequest(req *proxy.MCPRequest) (*proxy.MCPRequest, error) {
	if m == nil || req == nil || req.Method != proxy.MethodToolsCall {
		return req, nil
	}

	toolName := proxy.ExtractToolName(req)

	if toolName == "" || !m.ShouldMaskTool(toolName) {
		return req, nil
	}

	sensitiveFields := m.GetSensitiveFields(toolName)

	if len(sensitiveFields) == 0 {
		return req, nil
	}

	maskedParams, err := m.maskJSONValue(req.Params, sensitiveFields)

	if err != nil {
		return req, nil
	}

	maskedReq := *req
	maskedReq.Params = maskedParams

	return &maskedReq, nil
}

// MaskResponse applies PII masking to upstream responses before sending downstream.
// Masks result fields for tools assigned to a sensitivity class.
// toolOrResource is the tool name for tools/call, or the resource URI for resources/read.
func (m *Masker) MaskResponse(method string, toolOrResource string, response json.RawMessage) (json.RawMessage, error) {
	if m == nil {
		return response, nil
	}

	switch method {
	case proxy.MethodToolsCall:
		return m.maskToolsCallResponse(toolOrResource, response)
	case proxy.MethodResourcesRead:
		return m.maskResourceSReadResponse(toolOrResource, response)
	}

	return response, nil
}

// maskToolsCallResponse applies PII masking to a tools/call response.
func (m *Masker) maskToolsCallResponse(toolName string, response json.RawMessage) (json.RawMessage, error) {
	var fullResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *proxy.RPCError `json:"error,omitempty"`
	}

	if err := json.Unmarshal(response, &fullResp); err != nil {
		return response, nil
	}

	if fullResp.Result == nil || fullResp.Error != nil {
		return response, nil
	}

	var result map[string]interface{}

	if err := json.Unmarshal(fullResp.Result, &result); err != nil {
		return response, nil
	}

	sensitiveFields := m.GetSensitiveFields(toolName)

	if len(sensitiveFields) == 0 {
		return response, nil
	}

	if content, ok := result["content"].([]interface{}); ok {
		for i := range content {
			if contentItem, ok := content[i].(map[string]interface{}); ok {
				if text, ok := contentItem["text"].(string); ok {
					maskedText := m.applyPatterns(text, sensitiveFields)
					contentItem["text"] = maskedText
				}

				if annotations, ok := contentItem["annotations"].(map[string]interface{}); ok {
					if anthropicUtil, ok := annotations["anthropic_utility"].([]interface{}); ok {
						for j := range anthropicUtil {
							if utilItem, ok := anthropicUtil[j].(map[string]interface{}); ok {
								m.maskUtilityFields(utilItem, sensitiveFields)
							}
						}
					}
				}
			}
		}
	}

	maskedResult, err := json.Marshal(result)

	if err != nil {
		return response, nil
	}

	fullResp.Result = maskedResult

	return json.Marshal(fullResp)
}

// maskResourceSReadResponse applies PII masking to a resources/read response.
func (m *Masker) maskResourceSReadResponse(resourceURI string, response json.RawMessage) (json.RawMessage, error) {
	var fullResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *proxy.RPCError `json:"error,omitempty"`
	}

	if err := json.Unmarshal(response, &fullResp); err != nil {
		return response, nil
	}

	if fullResp.Result == nil || fullResp.Error != nil {
		return response, nil
	}

	var result map[string]interface{}

	if err := json.Unmarshal(fullResp.Result, &result); err != nil {
		return response, nil
	}

	// Extract sensitivity class from resource URI (strip scheme and host, keep path)
	// e.g., "file:///home/user/documents/passwd" -> "/documents/passwd"
	// Then use as key to look up assigned sensitivity class.
	sensitiveFields := m.GetSensitiveFields(resourceURI)

	if len(sensitiveFields) == 0 {
		return response, nil
	}

	// Apply masking to the resource content.
	if content, ok := result["contents"].([]interface{}); ok {
		for i := range content {
			if contentItem, ok := content[i].(map[string]interface{}); ok {
				if text, ok := contentItem["text"].(string); ok {
					maskedText := m.applyPatterns(text, sensitiveFields)
					contentItem["text"] = maskedText
				}

				if blob, ok := contentItem["blob"].(string); ok {
					maskedBlob := m.applyPatterns(blob, sensitiveFields)
					contentItem["blob"] = maskedBlob
				}
			}
		}
	}

	maskedResult, err := json.Marshal(result)

	if err != nil {
		return response, nil
	}

	fullResp.Result = maskedResult

	return json.Marshal(fullResp)
}

// maskUtilityFields masks common utility field names in anthropic_utility annotations.
func (m *Masker) maskUtilityFields(utilItem map[string]interface{}, sensitiveFields []string) {
	for _, field := range []string{"url", "path", "key", "token", "password", "api_key", "access_token", "secret"} {
		if val, ok := utilItem[field].(string); ok {
			utilItem[field] = m.applyPatterns(val, sensitiveFields)
		}
	}
}

// maskJSONValue recursively traverses JSON data and applies PII patterns to string values.
func (m *Masker) maskJSONValue(value json.RawMessage, sensitiveFields []string) ([]byte, error) {
	var data interface{}

	if err := json.Unmarshal(value, &data); err != nil {
		return nil, err
	}

	masked, err := m.traverseAndMask(data, sensitiveFields)

	if err != nil {
		return nil, err
	}

	return json.Marshal(masked)
}

// traverseAndMask recursively walks the JSON data tree and masks string values.
func (m *Masker) traverseAndMask(data interface{}, sensitiveFields []string) (interface{}, error) {
	switch v := data.(type) {
	case string:
		return m.applyPatterns(v, sensitiveFields), nil

	case []interface{}:
		result := make([]interface{}, len(v))

		for i, item := range v {
			masked, err := m.traverseAndMask(item, sensitiveFields)

			if err != nil {
				return nil, err
			}

			result[i] = masked
		}

		return result, nil

	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))

		for key, val := range v {
			masked, err := m.traverseAndMask(val, sensitiveFields)

			if err != nil {
				return nil, err
			}

			result[key] = masked
		}

		return result, nil

	default:
		return data, nil
	}
}

// applyPatterns applies all PII patterns to the given string.
func (m *Masker) applyPatterns(text string, sensitiveFields []string) string {
	result := text

	for _, fieldName := range sensitiveFields {
		if pattern, ok := m.compiledPatterns[fieldName]; ok {
			result = m.applyPattern(result, pattern)
		}
	}

	return result
}

// applyPattern applies a single PII pattern to the given string.
func (m *Masker) applyPattern(text string, pattern *compiledPattern) string {
	allMatches := pattern.regexp.FindAllStringSubmatchIndex(text, -1)

	if len(allMatches) == 0 {
		return text
	}

	allSubmatch := pattern.regexp.FindAllStringSubmatch(text, -1)

	var sb strings.Builder
	sb.Grow(len(text))
	start := 0

	for i, indices := range allMatches {
		matchStart := indices[0]
		matchEnd := indices[1]

		sb.WriteString(text[start:matchStart])
		sb.WriteString(m.replaceMatch(text[matchStart:matchEnd], pattern, allSubmatch[i]))
		start = matchEnd
	}

	sb.WriteString(text[start:])

	return sb.String()
}

// replaceMatch generates the replacement string for a single match.
func (m *Masker) replaceMatch(match string, pattern *compiledPattern, submatch []string) string {
	if pattern.partialMask != nil && (pattern.partialMask.showFirst > 0 || pattern.partialMask.showLast > 0) {
		showFirst := pattern.partialMask.showFirst
		showLast := pattern.partialMask.showLast

		if showFirst > len(match) {
			showFirst = len(match)
		}

		if showLast > len(match) {
			showLast = len(match)
		}

		firstPart := ""

		if showFirst > 0 {
			firstPart = match[:showFirst]
		}

		lastPart := ""

		if showLast > 0 {
			if len(match) > showFirst {
				lastPart = match[len(match)-showLast:]
			} else {
				lastPart = match
			}
		}

		middleLen := len(match) - showFirst - showLast

		if middleLen <= 0 {
			if showFirst+showLast >= len(match) {
				return match
			}
		}

		middleMask := strings.Repeat(pattern.partialMask.filler, middleLen)

		return firstPart + middleMask + lastPart
	}

	mask := pattern.mask

	for i, submatchStr := range submatch {
		mask = strings.ReplaceAll(mask, fmt.Sprintf("${%d}", i), submatchStr)
	}

	return mask
}
