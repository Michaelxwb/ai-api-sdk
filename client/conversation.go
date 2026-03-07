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

// OnErrorStrategy defines how multi-turn errors are handled.
type OnErrorStrategy string

const (
	// OnErrorAbort stops the conversation on first error (default).
	OnErrorAbort OnErrorStrategy = "abort"
	// OnErrorContinue allows the conversation to proceed after a single-turn failure.
	OnErrorContinue OnErrorStrategy = "continue"
)
