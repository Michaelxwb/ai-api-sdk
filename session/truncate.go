package session

import "strings"

// TruncatePolicy applies truncation to a message slice.
// Implementations should return a new slice when truncation occurs.
type TruncatePolicy interface {
	Truncate(messages []Message) []Message
}

// WindowPolicy keeps the most recent N messages and/or limits total token budget.
// If KeepSystemPrompt is true, leading system messages are preserved.
type WindowPolicy struct {
	MaxMessages      int
	MaxTokens        int // approximate token budget; each message is estimated as len(Content)/4
	KeepSystemPrompt bool
}

// Options returns GetOptions hints for stores.
func (p WindowPolicy) Options() GetOptions {
	return GetOptions{
		MaxMessages:      p.MaxMessages,
		MaxTokens:        p.MaxTokens,
		KeepSystemPrompt: p.KeepSystemPrompt,
	}
}

// Truncate keeps the most recent N messages, optionally preserving system prompts,
// and further trims to fit within MaxTokens if set.
func (p WindowPolicy) Truncate(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	// Step 1: apply MaxMessages window
	msgs := p.truncateByCount(messages)

	// Step 2: apply MaxTokens budget
	if p.MaxTokens > 0 {
		msgs = p.truncateByTokens(msgs)
	}

	return msgs
}

func (p WindowPolicy) truncateByCount(messages []Message) []Message {
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

func (p WindowPolicy) truncateByTokens(messages []Message) []Message {
	if p.MaxTokens <= 0 || len(messages) == 0 {
		return messages
	}

	// Identify system prefix to preserve
	prefixEnd := 0
	if p.KeepSystemPrompt {
		for prefixEnd < len(messages) && isSystemRole(messages[prefixEnd].Role) {
			prefixEnd++
		}
	}

	// Calculate prefix token cost
	prefixTokens := 0
	for i := 0; i < prefixEnd; i++ {
		prefixTokens += estimateTokens(messages[i].Content)
	}

	// Walk from the end, accumulating tokens
	remaining := messages[prefixEnd:]
	budget := p.MaxTokens - prefixTokens
	if budget <= 0 {
		// Even system prompts exceed budget; return system prompts only
		return append([]Message(nil), messages[:prefixEnd]...)
	}

	startIdx := len(remaining)
	used := 0
	for i := len(remaining) - 1; i >= 0; i-- {
		cost := estimateTokens(remaining[i].Content)
		if used+cost > budget {
			break
		}
		used += cost
		startIdx = i
	}

	if prefixEnd == 0 && startIdx == 0 {
		return messages
	}

	truncated := make([]Message, 0, prefixEnd+len(remaining)-startIdx)
	truncated = append(truncated, messages[:prefixEnd]...)
	truncated = append(truncated, remaining[startIdx:]...)
	return truncated
}

// Truncate is a standalone function that truncates messages by both count and token limits.
func Truncate(msgs []Message, maxMessages int, maxTokens int) []Message {
	p := WindowPolicy{MaxMessages: maxMessages, MaxTokens: maxTokens}
	return p.Truncate(msgs)
}

// estimateTokens provides a rough token estimate: len(content) / 4.
func estimateTokens(content string) int {
	n := len(content) / 4
	if n == 0 && len(content) > 0 {
		n = 1
	}
	return n
}

func isSystemRole(role string) bool {
	return strings.EqualFold(role, "system")
}
