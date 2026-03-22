// Package generic implements the Generic Adapter provider for non-standard API integration.
// It supports template-driven request building, response parsing, SSE/NDJSON streaming,
// and raw integration spec compilation.
package generic

// ChainField 描述一条"响应字段 → 下一轮请求字段"的链路传递规则。
// 三个字段在 ChainFields 列表非空时均为必填。
type ChainField struct {
	// Placeholder 是请求体中的占位符，格式必须为 $$$NAME$$$，e.g. "$$$PARENT_MSG$$$"。
	Placeholder string `json:"placeholder" yaml:"placeholder"`
	// ResponsePath 是从响应 JSON 中提取值的路径，e.g. "message_id"。
	ResponsePath string `json:"response_path" yaml:"response_path"`
	// ExtractOnEvent 指定只从哪个 SSE event 类型提取，e.g. "message_end"。
	// 可不填；为空时从任意含目标字段的 SSE 帧提取。
	ExtractOnEvent string `json:"extract_on_event" yaml:"extract_on_event"`
}

// GenericProfile defines the request/response mapping for a non-standard API provider.
type GenericProfile struct {
	Request  RequestProfile  `json:"request" yaml:"request"`
	Response ResponseProfile `json:"response" yaml:"response"`
	// Conversation defines conversation behavior for this generic provider.
	Conversation ConversationProfile `json:"conversation" yaml:"conversation"`
}

// RequestProfile configures how to build the HTTP request.
type RequestProfile struct {
	// Method is the HTTP method (default: POST).
	Method string `json:"method" yaml:"method"`
	// Path is the URL path appended to BaseURL.
	Path string `json:"path" yaml:"path"`
	// DynamicHeaders are headers containing {{uuid}} placeholders, rendered per-request.
	DynamicHeaders map[string]string `json:"dynamic_headers,omitempty" yaml:"dynamic_headers,omitempty"`
	// BodyTemplate is the JSON body template with placeholders like {{input}}, {{session_id}}.
	BodyTemplate map[string]any `json:"body_template" yaml:"body_template"`
	// SessionIDField is the JSON field name for injecting session_id (e.g. "conversation_id").
	SessionIDField string `json:"session_id_field" yaml:"session_id_field"`
}

// ResponseProfile configures how to parse the HTTP response.
type ResponseProfile struct {
	// TextPath is the JSON path to extract text content (e.g. "choices.0.message.content").
	TextPath string `json:"text_path" yaml:"text_path"`
	// RemoteIDPath is the JSON path to extract remote session ID (e.g. "conversation_id").
	RemoteIDPath string `json:"remote_id_path" yaml:"remote_id_path"`
	// Stream configures streaming response parsing.
	Stream StreamProfile `json:"stream" yaml:"stream"`
}

// StreamProfile configures streaming response parsing.
type StreamProfile struct {
	// Protocol is "sse" or "ndjson".
	Protocol string `json:"protocol" yaml:"protocol"`
	// DeltaPaths are JSON paths to extract incremental text from each chunk.
	DeltaPaths []string `json:"delta_paths" yaml:"delta_paths"`
	// DonePath is the JSON path to check for stream completion.
	DonePath string `json:"done_path" yaml:"done_path"`
	// DoneValue is the value at DonePath that signals completion.
	DoneValue string `json:"done_value" yaml:"done_value"`
	// DoneMarker is the raw SSE data string that signals stream end (e.g. "[DONE]").
	DoneMarker string `json:"done_marker" yaml:"done_marker"`
	// ChainFields 描述多轮字段链路传递规则，可为空。
	// 非空时每条的三个子字段均必填。
	ChainFields []ChainField `json:"chain_fields" yaml:"chain_fields"`
	// ErrorPath is the JSON path to extract error messages from stream frames (e.g. "error.message").
	ErrorPath string `json:"error_path" yaml:"error_path"`
}

// ConversationProfile configures conversation behavior.
type ConversationProfile struct {
	// Mode is the conversation mode: "remote_session" or "local_history".
	Mode string `json:"mode" yaml:"mode"`
}

// InferredField describes an inferred field classification from MultiRoundSpec analysis.
type InferredField struct {
	// RequestPath is the JSON path in the request body (e.g. "session_id").
	RequestPath string `json:"request_path"`
	// ResponsePath is the JSON path in the response body (e.g. "conversation_id").
	ResponsePath string `json:"response_path,omitempty"`
	// Class is the field classification: input, session_id, chain, dynamic, static.
	Class string `json:"class"`
	// Placeholder is the template placeholder (e.g. "{{input}}", "{{session_id}}", "$$$NAME$$$").
	Placeholder string `json:"placeholder"`
	// Confidence is the classification confidence score in [0, 1].
	Confidence float64 `json:"confidence"`
	// ConflictWith lists alternative candidate classifications when ambiguous.
	ConflictWith []string `json:"conflict_with,omitempty"`
	// Reason is a human-readable justification for the classification.
	Reason string `json:"reason"`
}

// InferenceReport summarizes the result of MultiRoundSpec auto-inference.
type InferenceReport struct {
	// OverallConfidence is the aggregated confidence score.
	OverallConfidence float64 `json:"overall_confidence"`
	// Status is one of: auto_confirmed, pending_confirm, failed.
	Status string `json:"status"`
	// Fields lists all inferred field classifications.
	Fields []InferredField `json:"fields"`
	// Warnings lists non-fatal issues encountered during inference.
	Warnings []string `json:"warnings,omitempty"`
	// FallbackSuggested indicates whether manual RawIntegrationSpec is recommended.
	FallbackSuggested bool `json:"fallback_suggested"`
	// Suggestions lists actionable modification suggestions for the user.
	Suggestions []Suggestion `json:"suggestions,omitempty"`
	// FlowSpecMeta carries forward-compatible metadata for future FlowSpec extensions.
	FlowSpecMeta FlowSpecMeta `json:"flow_spec_meta"`
}

// FlowSpecMeta provides forward-compatible metadata for future FlowSpec extensions.
type FlowSpecMeta struct {
	// Version is the spec version, fixed to "v1alpha1" for MultiRoundSpec.
	Version string `json:"version"`
	// Source identifies the input type, e.g. "MultiRoundSpec".
	Source string `json:"source"`
}

// Suggestion describes an actionable modification recommendation.
type Suggestion struct {
	// Target is the field path or config key this suggestion applies to.
	Target string `json:"target"`
	// Action is the suggested operation: replace, add, remove, review.
	Action string `json:"action"`
	// Value is the suggested value or candidate value.
	Value string `json:"value"`
	// Reason explains why this suggestion is made.
	Reason string `json:"reason"`
	// Priority is high, medium, or low.
	Priority string `json:"priority"`
}

// InferredIntegration is the output of MultiRoundSpec auto-inference.
// It always contains a GenericProfile and an InferenceReport.
type InferredIntegration struct {
	// Profile is the inferred GenericProfile (may be partial if status != auto_confirmed).
	Profile *GenericProfile `json:"profile"`
	// Report contains inference details, confidence, and suggestions.
	Report *InferenceReport `json:"report"`
	// Credential is the extracted authentication credential, or nil.
	Credential interface{} `json:"credential,omitempty"`
	// BaseURL is the parsed base URL (scheme + host).
	BaseURL string `json:"base_url"`
	// ExtraHeaders are non-auth headers to inject.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
}
