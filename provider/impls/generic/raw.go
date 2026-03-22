package generic

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/auth"
)

// RawIntegrationSpec represents a raw, user-provided API integration specification.
// This is the input format before compilation into a GenericProfile.
type RawIntegrationSpec struct {
	// Name is a human-readable label for this integration.
	Name string `json:"name" yaml:"name"`
	// URL is the full endpoint URL. If scheme is missing, https:// is prepended.
	URL string `json:"url" yaml:"url"`
	// Method is the HTTP method (default: POST).
	Method string `json:"method" yaml:"method"`
	// Headers are HTTP headers to include. Used for auth extraction.
	Headers map[string]string `json:"headers" yaml:"headers"`
	// Body is the request body template. Use "$$$" for input placeholder,
	// "$$$SESSION_ID$$$" for session ID placeholder,
	// "$$$NAME$$$" for chain field placeholders (any NAME except SESSION_ID).
	Body map[string]any `json:"body" yaml:"body"`
	// Response configures response parsing.
	Response RawResponseSpec `json:"response" yaml:"response"`
	// Conversation defines conversation mode.
	Conversation ConversationProfile `json:"conversation" yaml:"conversation"`
	// ChainFields 描述多轮字段链路传递规则，可不传。
	// 传了则每条的 Placeholder、ResponsePath、ExtractOnEvent 均必填。
	ChainFields []ChainField `json:"chain_fields" yaml:"chain_fields"`
}

// RawResponseSpec defines how to parse responses in the raw spec format.
type RawResponseSpec struct {
	// TextPath is the JSON path to extract text from non-streaming response.
	TextPath string `json:"text_path" yaml:"text_path"`
	// RemoteIDPath is the JSON path to extract remote session ID.
	RemoteIDPath string `json:"remote_id_path" yaml:"remote_id_path"`
	// Stream configures streaming response parsing.
	Stream StreamProfile `json:"stream" yaml:"stream"`
}

// CompiledIntegration is the result of compiling a RawIntegrationSpec.
// It contains a GenericProfile ready for use, plus extracted auth credentials.
type CompiledIntegration struct {
	// Profile is the compiled generic adapter profile.
	Profile GenericProfile
	// Credential is the extracted authentication credential, or nil if none detected.
	Credential *auth.Credential
	// BaseURL is the parsed base URL (scheme + host).
	BaseURL string
	// ExtraHeaders are non-auth headers to inject.
	ExtraHeaders map[string]string
}

// ParseRawIntegration compiles a RawIntegrationSpec into a CompiledIntegration.
func ParseRawIntegration(raw RawIntegrationSpec) (*CompiledIntegration, error) {
	if raw.Conversation.Mode == "" {
		return nil, fmt.Errorf("generic: conversation mode is required")
	}
	if raw.Conversation.Mode != "remote_session" && raw.Conversation.Mode != "local_history" {
		return nil, fmt.Errorf("generic: invalid conversation mode %q, must be remote_session or local_history", raw.Conversation.Mode)
	}

	// Validate ChainFields
	if err := validateChainFields(raw.ChainFields); err != nil {
		return nil, err
	}

	// Parse and normalize URL
	rawURL := strings.TrimSpace(raw.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("generic: URL is required")
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("generic: invalid URL %q: %w", raw.URL, err)
	}
	baseURL := parsed.Scheme + "://" + parsed.Host
	path := parsed.Path
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}

	// Method
	method := strings.ToUpper(strings.TrimSpace(raw.Method))
	if method == "" {
		method = "POST"
	}

	// Convert body: replace $$$ placeholders
	body := convertBodyPlaceholders(raw.Body)

	// Detect session_id field
	sessionIDField := findSessionIDField(body)

	// Extract credential from headers
	cred, extraHeaders, err := ExtractCredential(raw.Headers)
	if err != nil {
		return nil, err
	}

	// Separate dynamic headers (containing {{uuid}}) from static ones.
	// Dynamic headers are stored in the profile and rendered per-request.
	var dynamicHeaders map[string]string
	if len(extraHeaders) > 0 {
		for k, v := range extraHeaders {
			if strings.Contains(v, "{{uuid}}") {
				if dynamicHeaders == nil {
					dynamicHeaders = make(map[string]string)
				}
				dynamicHeaders[k] = v
				delete(extraHeaders, k)
			}
		}
	}

	profile := GenericProfile{
		Request: RequestProfile{
			Method:         method,
			Path:           path,
			DynamicHeaders: dynamicHeaders,
			BodyTemplate:   body,
			SessionIDField: sessionIDField,
		},
		Response: ResponseProfile{
			TextPath:     raw.Response.TextPath,
			RemoteIDPath: raw.Response.RemoteIDPath,
			Stream: StreamProfile{
				Protocol:    raw.Response.Stream.Protocol,
				DeltaPaths:  raw.Response.Stream.DeltaPaths,
				DonePath:    raw.Response.Stream.DonePath,
				DoneValue:   raw.Response.Stream.DoneValue,
				DoneMarker:  raw.Response.Stream.DoneMarker,
				ChainFields: raw.ChainFields,
			},
		},
		Conversation: raw.Conversation,
	}

	return &CompiledIntegration{
		Profile:      profile,
		Credential:   cred,
		BaseURL:      baseURL,
		ExtraHeaders: extraHeaders,
	}, nil
}

// convertBodyPlaceholders replaces $$$ and $$$SESSION_ID$$$ in body template.
func convertBodyPlaceholders(body map[string]any) map[string]any {
	if body == nil {
		return nil
	}
	result := make(map[string]any, len(body))
	for k, v := range body {
		result[k] = convertValue(v)
	}
	return result
}

// chainPlaceholderRe 匹配 $$$NAME$$$ 形式的链路占位符（NAME 为大写字母、数字、下划线）。
var chainPlaceholderRe = regexp.MustCompile(`\$\$\$[A-Z0-9_]+\$\$\$`)

func convertValue(v any) any {
	switch val := v.(type) {
	case string:
		// 1. 先替换 $$$SESSION_ID$$$
		s := strings.ReplaceAll(val, "$$$SESSION_ID$$$", "{{session_id}}")
		// 2. 保护剩余 $$$NAME$$$ 链路占位符，防止被 $$$ → {{input}} 误替换
		matches := chainPlaceholderRe.FindAllString(s, -1)
		for i, m := range matches {
			s = strings.ReplaceAll(s, m, fmt.Sprintf("\x00CHAIN%d\x00", i))
		}
		// 3. 替换独立的 $$$（用户输入占位符）
		s = strings.ReplaceAll(s, "$$$", "{{input}}")
		// 4. 还原链路占位符
		for i, m := range matches {
			s = strings.ReplaceAll(s, fmt.Sprintf("\x00CHAIN%d\x00", i), m)
		}
		return s
	case map[string]any:
		return convertBodyPlaceholders(val)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = convertValue(item)
		}
		return result
	default:
		return v
	}
}

// findSessionIDField finds the field name whose value is "{{session_id}}".
func findSessionIDField(body map[string]any) string {
	for k, v := range body {
		if s, ok := v.(string); ok && s == "{{session_id}}" {
			return k
		}
		if m, ok := v.(map[string]any); ok {
			if f := findSessionIDField(m); f != "" {
				return f
			}
		}
	}
	return ""
}

// ExtractCredential extracts authentication from HTTP headers.
// Priority: Bearer Token > Basic Auth > Cookie/X-* as custom headers.
// Returns error if both Bearer and Basic are present simultaneously.
func ExtractCredential(headers map[string]string) (*auth.Credential, map[string]string, error) {
	if len(headers) == 0 {
		return nil, nil, nil
	}

	var bearerToken string
	var basicAuth string
	var hasBoth bool
	extraHeaders := make(map[string]string)

	for k, v := range headers {
		key := strings.ToLower(strings.TrimSpace(k))
		switch key {
		case "authorization":
			val := strings.TrimSpace(v)
			if strings.HasPrefix(strings.ToLower(val), "bearer ") {
				if basicAuth != "" {
					hasBoth = true
				}
				bearerToken = strings.TrimSpace(val[7:])
			} else if strings.HasPrefix(strings.ToLower(val), "basic ") {
				if bearerToken != "" {
					hasBoth = true
				}
				basicAuth = strings.TrimSpace(val[6:])
			}
		default:
			extraHeaders[k] = v
		}
	}

	if hasBoth {
		return nil, nil, fmt.Errorf("generic: conflicting auth: both Bearer and Basic detected in headers")
	}

	if bearerToken != "" {
		cred := &auth.Credential{
			AuthType:    auth.AuthTypeBearerToken,
			AccessToken: bearerToken,
		}
		return cred, extraHeaders, nil
	}

	if basicAuth != "" {
		cred := &auth.Credential{
			AuthType:    auth.AuthTypeBasic,
			AccessToken: basicAuth,
			Headers:     map[string]string{"Authorization": "Basic " + basicAuth},
		}
		return cred, extraHeaders, nil
	}

	// No standard auth detected; put all headers as custom
	if len(headers) > 0 {
		cred := &auth.Credential{
			AuthType: auth.AuthTypeNone,
			Headers:  headers,
		}
		return cred, nil, nil
	}

	return nil, extraHeaders, nil
}

// RawPacket represents a raw HTTP request or response packet.
type RawPacket struct {
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string            `json:"body" yaml:"body"`
}

// MultiRoundSpec provides 2~5 rounds of raw request/response packets for auto-inference.
// Each round must contain both request and response. Default recommendation: 3~5 rounds.
type MultiRoundSpec struct {
	// URL is the full endpoint URL.
	URL string `json:"url" yaml:"url"`
	// Rounds contains 2~5 complete request/response pairs.
	Rounds []RoundPair `json:"rounds" yaml:"rounds"`
	// Conversation defines conversation mode (required).
	Conversation ConversationProfile `json:"conversation" yaml:"conversation"`
}

// RoundPair is a single round's request + response pair.
type RoundPair struct {
	Request  RawPacket `json:"request" yaml:"request"`
	Response RawPacket `json:"response" yaml:"response"`
}

// TwoRoundSpec is a backward-compatible alias for MultiRoundSpec with exactly 2 rounds.
// Internally converted to MultiRoundSpec for processing.
type TwoRoundSpec struct {
	URL    string `json:"url" yaml:"url"`
	Round1 struct {
		Request  RawPacket `json:"request" yaml:"request"`
		Response RawPacket `json:"response" yaml:"response"`
	} `json:"round1" yaml:"round1"`
	Round2 struct {
		Request  RawPacket `json:"request" yaml:"request"`
		Response RawPacket `json:"response" yaml:"response"`
	} `json:"round2" yaml:"round2"`
	Conversation ConversationProfile `json:"conversation" yaml:"conversation"`
}

// InferIntegrationByTwoRound is a backward-compatible entry point.
// It converts TwoRoundSpec to MultiRoundSpec and delegates to InferIntegrationByMultiRound.
func InferIntegrationByTwoRound(spec TwoRoundSpec) (*InferredIntegration, error) {
	if spec.Round1.Request.Body == "" || spec.Round1.Response.Body == "" ||
		spec.Round2.Request.Body == "" || spec.Round2.Response.Body == "" {
		return nil, fmt.Errorf("generic: two_round_spec requires 4 packets")
	}
	multi := MultiRoundSpec{
		URL: spec.URL,
		Rounds: []RoundPair{
			{Request: spec.Round1.Request, Response: spec.Round1.Response},
			{Request: spec.Round2.Request, Response: spec.Round2.Response},
		},
		Conversation: spec.Conversation,
	}
	return InferIntegrationByMultiRound(multi)
}

// chainPlaceholderFormatRe 校验占位符格式：$$$NAME$$$ 其中 NAME 为非空大写字母/数字/下划线。
var chainPlaceholderFormatRe = regexp.MustCompile(`^\$\$\$[A-Z0-9_]+\$\$\$$`)

// validateChainFields 校验 ChainFields 列表：非空时每条的三个字段均必填，格式合法。
func validateChainFields(fields []ChainField) error {
	for i, cf := range fields {
		if cf.Placeholder == "" {
			return fmt.Errorf("generic: chain_fields[%d].placeholder is required", i)
		}
		if !chainPlaceholderFormatRe.MatchString(cf.Placeholder) {
			return fmt.Errorf("generic: chain_fields[%d].placeholder %q is invalid, must match $$$NAME$$$ where NAME is [A-Z0-9_]+", i, cf.Placeholder)
		}
		if cf.Placeholder == "$$$SESSION_ID$$$" {
			return fmt.Errorf("generic: chain_fields[%d].placeholder cannot be $$$SESSION_ID$$$ (reserved)", i)
		}
		if cf.ResponsePath == "" {
			return fmt.Errorf("generic: chain_fields[%d].response_path is required", i)
		}
	}
	return nil
}
