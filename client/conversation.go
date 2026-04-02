package client

// ConversationMode defines how multi-turn conversation state is managed.
type ConversationMode string

const (
	// ConversationModeRemoteSession indicates session state is managed by the remote provider.
	// The SDK extracts a session ID from provider responses and injects it in subsequent requests.
	// No local history is loaded or sent; the provider maintains conversation context.
	ConversationModeRemoteSession ConversationMode = "remote_session"

	// ConversationModeLocalHistory indicates session state is managed locally.
	// The SDK loads conversation history from SessionStore and prepends it to each request.
	// No session ID is injected into provider requests.
	ConversationModeLocalHistory ConversationMode = "local_history"
)

// ResolveConversationMode returns the default conversation mode for a provider.
// Callers should prefer an explicitly specified mode over this default.
func ResolveConversationMode(provider string) ConversationMode {
	switch provider {
	case "openai", "claude", "gemini", "ollama",
		"deepseek", "moonshot", "dashscope", "volcengine", "qianfan", "openai_compat", "bailian_app":
		return ConversationModeLocalHistory
	case "dify", "ragflow":
		return ConversationModeRemoteSession
	default: // fastgpt, generic, plugin, etc.
		return ""
	}
}

// ResolveDefaultStream returns the default streaming mode for a provider.
// Most providers stream by default; some providers are synchronous-first.
func ResolveDefaultStream(provider string) bool {
	switch provider {
	case "bailian_app":
		return false
	}
	return true
}

// OnErrorStrategy defines how multi-turn errors are handled.
type OnErrorStrategy string

const (
	// OnErrorAbort stops the conversation on first error (default).
	OnErrorAbort OnErrorStrategy = "abort"
	// OnErrorContinue allows the conversation to proceed after a single-turn failure.
	OnErrorContinue OnErrorStrategy = "continue"
)
