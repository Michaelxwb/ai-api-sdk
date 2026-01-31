package session

import "strings"

// TruncatePolicy applies truncation to a message slice.
// Implementations should return a new slice when truncation occurs.
type TruncatePolicy interface {
	Truncate(messages []Message) []Message
}

// WindowPolicy keeps the most recent N messages.
// If KeepSystemPrompt is true, leading system messages are preserved.
type WindowPolicy struct {
	MaxMessages      int
	KeepSystemPrompt bool
}

// Options returns GetOptions hints for stores.
func (p WindowPolicy) Options() GetOptions {
	return GetOptions{
		MaxMessages:      p.MaxMessages,
		KeepSystemPrompt: p.KeepSystemPrompt,
	}
}

// Truncate keeps the most recent N messages, optionally preserving system prompts.
func (p WindowPolicy) Truncate(messages []Message) []Message {
	if p.MaxMessages <= 0 || len(messages) <= p.MaxMessages {
		return messages
	}

	if !p.KeepSystemPrompt {
		start := len(messages) - p.MaxMessages
		if start < 0 {
			start = 0
		}
		return append([]Message(nil), messages[start:]...)
	}

	// Keep leading system messages, apply window to the rest.
	prefixEnd := 0
	for prefixEnd < len(messages) && isSystemRole(messages[prefixEnd].Role) {
		prefixEnd++
	}

	remaining := messages[prefixEnd:]
	if len(remaining) <= p.MaxMessages {
		return messages
	}

	start := len(remaining) - p.MaxMessages
	if start < 0 {
		start = 0
	}

	truncated := make([]Message, 0, prefixEnd+len(remaining)-start)
	truncated = append(truncated, messages[:prefixEnd]...)
	truncated = append(truncated, remaining[start:]...)
	return truncated
}

func isSystemRole(role string) bool {
	return strings.EqualFold(role, "system")
}
