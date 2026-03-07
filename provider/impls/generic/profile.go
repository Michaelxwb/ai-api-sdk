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
