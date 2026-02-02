package session

func cloneMessages(msgs []Message) []Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out
}

func cloneMeta(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]string, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func cloneState(state *SessionState) *SessionState {
	if state == nil {
		return nil
	}
	cloned := *state
	cloned.Messages = cloneMessages(state.Messages)
	cloned.Meta = cloneMeta(state.Meta)
	return &cloned
}
