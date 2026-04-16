package generic

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// GenericSpec implements base.ProviderSpec and streaming.ProviderStreamSpec
// for non-standard API providers configured via GenericProfile.
type GenericSpec struct {
	profile GenericProfile
}

// NewGenericSpec creates a GenericSpec from a compiled profile.
func NewGenericSpec(profile GenericProfile) *GenericSpec {
	return &GenericSpec{profile: profile}
}

func init() {
	base.Register("generic", &GenericSpec{})
}

// Name returns the provider name.
func (s *GenericSpec) Name() string { return "generic" }

// DefaultBaseURL returns empty since generic providers have no default URL.
func (s *GenericSpec) DefaultBaseURL() string { return "" }

// SupportedAuthTypes returns all common auth types.
func (s *GenericSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{
		auth.AuthTypeBearerToken,
		auth.AuthTypeAPIKey,
		auth.AuthTypeBasic,
		auth.AuthTypeNone,
	}
}

// BuildRequest constructs an HTTP request from the profile template.
func (s *GenericSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	if err := base.ErrResponseFormatUnsupported("generic", req.ResponseFormat); err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		path = s.profile.Request.Path
	}

	url := strings.TrimRight(baseURL, "/")
	if path != "" {
		if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "?") {
			url += "/"
		}
		url += path
	}

	method := s.profile.Request.Method
	if method == "" {
		method = "POST"
	}

	// Extract input from messages
	input := extractInput(req.Messages)

	// Determine session_id to inject
	sessionID := req.SessionID

	// Render body template
	tmpl := s.profile.Request.BodyTemplate
	if tmpl == nil {
		// Build a minimal default body
		tmpl = map[string]any{"input": "{{input}}"}
	}

	// local_history 多轮：按模板消息结构展开完整历史
	if s.profile.Conversation.Mode == "local_history" {
		tmpl = expandHistoryMessages(tmpl, req.Messages)
	}

	// Pre-generate UUID so body and dynamic headers share the same value.
	uuid := generateHexUUID()

	bodyStr, err := renderTemplate(tmpl, input, sessionID, req.ChainValues, uuid)
	if err != nil {
		return nil, fmt.Errorf("generic: template render failed: %w", err)
	}

	// Merge ExtraBody
	if len(opts.ExtraBody) > 0 {
		var bodyMap map[string]any
		if err := json.Unmarshal([]byte(bodyStr), &bodyMap); err == nil {
			for k, v := range opts.ExtraBody {
				bodyMap[k] = v
			}
			merged, err := json.Marshal(bodyMap)
			if err == nil {
				bodyStr = string(merged)
			}
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader([]byte(bodyStr)))
	if err != nil {
		return nil, fmt.Errorf("generic: request creation failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}

	// Apply dynamic headers with per-request UUID.
	for k, v := range s.profile.Request.DynamicHeaders {
		httpReq.Header.Set(k, strings.ReplaceAll(v, "{{uuid}}", uuid))
	}

	return httpReq, nil
}

// ParseResponse parses a non-streaming HTTP response.
func (s *GenericSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("generic: response is nil")
	}

	var bodyReader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err == nil {
			bodyReader = gr
			defer func() { _ = gr.Close() }()
		}
	}

	data, err := io.ReadAll(io.LimitReader(bodyReader, 4<<20))
	if err != nil {
		return base.ChatResponse{}, fmt.Errorf("generic: read response failed: %w", err)
	}

	result := base.ChatResponse{Raw: data}

	// Extract text
	if s.profile.Response.TextPath != "" {
		text, err := streaming.ExtractJSONPath(data, s.profile.Response.TextPath)
		if err == nil {
			result.Text = text
		}
	}

	// Extract remote session ID
	if s.profile.Response.RemoteIDPath != "" {
		sid, err := streaming.ExtractJSONPath(data, s.profile.Response.RemoteIDPath)
		if err == nil {
			result.SessionID = sid
		}
	}

	// Extract chain values (reuse stream chain field definitions)
	chainFields := s.profile.Response.Stream.ChainFields
	if len(chainFields) > 0 {
		cv := make(map[string]string)
		for _, cf := range chainFields {
			val, err := streaming.ExtractJSONPath(data, cf.ResponsePath)
			if err == nil && val != "" {
				cv[cf.Placeholder] = val
			}
		}
		if len(cv) > 0 {
			result.ChainValues = cv
		}
	}

	return result, nil
}

// AuthStrategyOverride returns no override; auth is handled externally.
func (s *GenericSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	return nil, false
}

// ParseStreamResponse parses a streaming HTTP response (SSE or NDJSON).
func (s *GenericSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("generic: response is nil")
	}

	// Handle gzip on the stream body
	var bodyReader io.ReadCloser = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("generic: failed to decompress gzip stream: %w", err)
		}
		bodyReader = gzipReadCloser{gr, resp.Body}
		resp.Body = bodyReader
	}

	ctx := streaming.StreamContext(resp)

	cfg := streaming.StreamConfig{
		Protocol:   streaming.StreamProtocol(s.profile.Response.Stream.Protocol),
		DeltaPaths: s.profile.Response.Stream.DeltaPaths,
		DonePath:   s.profile.Response.Stream.DonePath,
		DoneValue:  s.profile.Response.Stream.DoneValue,
		DoneMarker: s.profile.Response.Stream.DoneMarker,
		ErrorPath:  s.profile.Response.Stream.ErrorPath,
	}

	extractor := streaming.MakeJSONPathMultiExtractor(
		cfg.DeltaPaths,
		cfg.DonePath,
		cfg.DoneValue,
		cfg.ErrorPath,
	)

	var inner <-chan streaming.StreamChunk
	var err error
	protocol := strings.ToLower(s.profile.Response.Stream.Protocol)
	switch protocol {
	case "ndjson":
		parser := &streaming.NDJSONParser{Config: cfg}
		inner, err = parser.Parse(ctx, resp, extractor)
	case "json":
		parser := &streaming.JSONParser{Config: cfg}
		inner, err = parser.Parse(ctx, resp, extractor)
	default: // "sse" or empty defaults to SSE
		parser := &streaming.SSEParser{Config: cfg}
		inner, err = parser.Parse(ctx, resp, extractor)
	}
	if err != nil {
		return nil, err
	}

	remoteIDPath := s.profile.Response.RemoteIDPath
	chainFields := s.profile.Response.Stream.ChainFields
	if remoteIDPath == "" && len(chainFields) == 0 {
		return inner, nil
	}

	// Wrap the inner channel to extract RemoteIDPath and ChainField values.
	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)
		// accChainValues persists across all chunks. A placeholder's value is written
		// only when a non-empty extraction succeeds and never overwritten with empty.
		accChainValues := make(map[string]string)
		for chunk := range inner {
			if remoteIDPath != "" && chunk.SessionID == "" && len(chunk.Raw) > 0 {
				if sid, e := streaming.ExtractJSONPath(chunk.Raw, remoteIDPath); e == nil && sid != "" {
					chunk.SessionID = sid
				}
			}
			if len(chainFields) > 0 {
				if len(chunk.Raw) > 0 {
					for _, cf := range chainFields {
						// If ExtractOnEvent is empty, extract from any frame containing the field.
						// Otherwise, match SSE protocol-level event first; fall back to JSON-embedded
						// event field (e.g. Dify embeds {"event":"message_end"} in data).
						eventMatch := cf.ExtractOnEvent == ""
						if !eventMatch {
							eventMatch = chunk.Event == cf.ExtractOnEvent
						}
						if !eventMatch {
							if jsonEvent, e := streaming.ExtractJSONPath(chunk.Raw, "event"); e == nil {
								eventMatch = jsonEvent == cf.ExtractOnEvent
							}
						}
						if !eventMatch {
							continue
						}
						val, e := streaming.ExtractJSONPath(chunk.Raw, cf.ResponsePath)
						if e == nil && val != "" {
							accChainValues[cf.Placeholder] = val
						}
					}
				}
				// Attach the current accumulated chain values to every outgoing chunk.
				// Chunks after the first successful extraction always carry the full
				// accumulated state, so downstream consumers never miss a value that
				// was extracted in an earlier chunk.
				if len(accChainValues) > 0 {
					copied := make(map[string]string, len(accChainValues))
					for k, v := range accChainValues {
						copied[k] = v
					}
					chunk.ChainValues = copied
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}

// gzipReadCloser wraps a gzip reader and the underlying body.
type gzipReadCloser struct {
	*gzip.Reader
	underlying io.ReadCloser
}

func (g gzipReadCloser) Close() error {
	_ = g.Reader.Close()
	return g.underlying.Close()
}

// extractInput gets the last user message content as input.
func extractInput(messages []base.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		role := strings.ToLower(messages[i].Role)
		if role == "user" || role == "tool" {
			return messages[i].Content
		}
	}
	if len(messages) > 0 {
		return messages[len(messages)-1].Content
	}
	return ""
}

// expandHistoryMessages rebuilds the "messages" field using role templates extracted from the body template.
// The template should contain message templates for each role (user, assistant, tool, etc.).
// System messages are preserved as-is; other role messages are used as format templates.
// Called for local_history mode for every request (single turn or multi-turn).
func expandHistoryMessages(tmpl map[string]any, messages []base.Message) map[string]any {
	if len(messages) == 0 {
		return tmpl
	}
	msgArr, ok := tmpl["messages"].([]any)
	if !ok || len(msgArr) == 0 {
		return tmpl
	}

	// Collect fixed messages (system) and per-role templates (last occurrence wins).
	var fixed []any
	roleTmpl := make(map[string]map[string]any)
	for _, item := range msgArr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		role = strings.ToLower(role)
		if role == "system" {
			fixed = append(fixed, m)
		} else if role != "" {
			roleTmpl[role] = m
		}
	}

	expanded := make([]any, 0, len(fixed)+len(messages))
	expanded = append(expanded, fixed...)

	for i, msg := range messages {
		role := strings.ToLower(msg.Role)
		isCurrentTurn := i == len(messages)-1 && (role == "user" || role == "tool")

		t, hasTmpl := roleTmpl[role]
		if !hasTmpl {
			expanded = append(expanded, map[string]any{"role": msg.Role, "content": msg.Content})
			continue
		}
		if isCurrentTurn {
			// Current input: keep {{input}} and chain placeholders for renderTemplate
			expanded = append(expanded, t)
		} else if role == "tool" && msg.ToolCallID != "" {
			// Historical tool message: fill both content and tool_call_id
			expanded = append(expanded, fillMsgTemplateWithChain(t, msg.Content, map[string]string{
				"$$$TOOL_CALL_ID$$$": msg.ToolCallID,
			}))
		} else if role == "assistant" && i+1 < len(messages) && strings.ToLower(messages[i+1].Role) == "tool" && messages[i+1].ToolCallID != "" {
			// Historical assistant preceding a tool message: fill content and tool_call_id from next message
			expanded = append(expanded, fillMsgTemplateWithChain(t, msg.Content, map[string]string{
				"$$$TOOL_CALL_ID$$$": messages[i+1].ToolCallID,
			}))
		} else {
			expanded = append(expanded, fillMsgTemplate(t, msg.Content))
		}
	}

	result := make(map[string]any, len(tmpl))
	for k, v := range tmpl {
		result[k] = v
	}
	result["messages"] = expanded
	return result
}

// cloneMap deep-clones a map[string]any via JSON round-trip.
func cloneMap(m map[string]any) map[string]any {
	data, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// replaceInAny recursively replaces placeholder in a Go value tree.
// Detects inner-JSON context: if the placeholder appears as `"placeholder"` inside
// a Go string (meaning the string value itself is inner JSON), applies jsonEscapeString
// before substitution so json.Marshal produces correct double-encoding.
func replaceInAny(v any, placeholder, value string) any {
	switch val := v.(type) {
	case string:
		if !strings.Contains(val, placeholder) {
			return val
		}
		if strings.Contains(val, `"`+placeholder+`"`) {
			escaped, err := jsonEscapeString(value)
			if err != nil {
				return strings.ReplaceAll(val, placeholder, value)
			}
			return strings.ReplaceAll(val, placeholder, escaped)
		}
		return strings.ReplaceAll(val, placeholder, value)
	case map[string]any:
		for k, child := range val {
			val[k] = replaceInAny(child, placeholder, value)
		}
		return val
	case []any:
		for i, item := range val {
			val[i] = replaceInAny(item, placeholder, value)
		}
		return val
	default:
		return v
	}
}

// fillMsgTemplate deep-clones a message template and replaces {{input}} with actual content.
func fillMsgTemplate(template map[string]any, content string) map[string]any {
	role, _ := template["role"].(string)
	fallback := map[string]any{"role": role, "content": content}
	cloned := cloneMap(template)
	if cloned == nil {
		return fallback
	}
	replaceInAny(cloned, "{{input}}", content)
	return cloned
}

// fillMsgTemplateWithChain deep-clones a message template, replaces {{input}} with content,
// and replaces the given chain placeholders with their values.
func fillMsgTemplateWithChain(template map[string]any, content string, chainOverride map[string]string) map[string]any {
	role, _ := template["role"].(string)
	fallback := map[string]any{"role": role, "content": content}
	cloned := cloneMap(template)
	if cloned == nil {
		return fallback
	}
	replaceInAny(cloned, "{{input}}", content)
	for ph, val := range chainOverride {
		replaceInAny(cloned, ph, val)
	}
	return cloned
}
