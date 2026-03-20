package client

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
	"github.com/Michaelxwb/ai-api-sdk/session"
	"github.com/google/uuid"
)

// Session 会话对象
type Session struct {
	client   *Client
	provider string
	cred     *auth.Credential
	pc       *config.ProviderConfig

	// 可选配置
	store session.SessionStore
	id    string
	mode  HistoryMode
	meta  map[string]string
	// 默认是否为新对话
	startNewChat bool

	// conversationMode 定义会话模式。
	// 空字符串表示未设置（向后兼容过渡期，保持旧行为）。
	conversationMode ConversationMode

	// onError 定义错误处理策略，默认 abort。
	onError OnErrorStrategy

	// historyWindow 定义本地历史裁剪窗口。
	historyWindow HistoryWindow

	// timeout 对话超时时间，0 表示使用 Client.HTTP.Timeout 默认值
	timeout time.Duration

	// 内部状态
	mu          sync.Mutex
	initialized bool
	chainValues map[string]string // 上一轮提取的链路字段值（remote_session 模式），key=占位符，val=值
}

// HistoryMode 历史消息加载模式
type HistoryMode int

const (
	HistoryAuto HistoryMode = iota // 自动加载历史
	HistoryNone                    // 仅持久化，不加载
)

// SessionOption 配置Session的可选参数
type SessionOption func(*Session)

// WithStore 配置SessionStore
func WithStore(store session.SessionStore) SessionOption {
	return func(s *Session) {
		s.store = store
	}
}

// WithAutoID 自动生成SessionID（若为空）
func WithAutoID() SessionOption {
	return func(s *Session) {
		if s.id == "" {
			s.id = uuid.New().String()
		}
	}
}

// WithID 指定SessionID
func WithID(id string) SessionOption {
	return func(s *Session) {
		s.id = id
	}
}

// WithHistoryMode 设置历史加载模式
func WithHistoryMode(mode HistoryMode) SessionOption {
	return func(s *Session) {
		s.mode = mode
	}
}

// WithMeta 设置会话的自定义元数据字段，每次保存时自动写入
func WithMeta(kv map[string]string) SessionOption {
	return func(s *Session) {
		s.meta = kv
	}
}

// WithStartNewChat 设置是否每次都开启新对话（默认 false）
func WithStartNewChat(enabled bool) SessionOption {
	return func(s *Session) {
		s.startNewChat = enabled
	}
}

// WithConversationMode 设置会话模式。
// remote_session: 由远端管理会话，SDK 从响应提取 session_id 并注入续轮请求。
// local_history: 由本地 SessionStore 管理历史，不向 Provider 传递 session_id。
func WithConversationMode(mode ConversationMode) SessionOption {
	return func(s *Session) {
		s.conversationMode = mode
	}
}

// WithOnError 设置错误处理策略。
func WithOnError(strategy OnErrorStrategy) SessionOption {
	return func(s *Session) {
		s.onError = strategy
	}
}

// WithHistoryWindow 设置本地历史裁剪窗口。
func WithHistoryWindow(hw HistoryWindow) SessionOption {
	return func(s *Session) {
		s.historyWindow = hw
	}
}

// WithTimeout 设置对话请求的超时时间，应用于 Chat 和 ChatStream。
// 0 或不设置时使用 Client.HTTP.Timeout 默认值（60s）。
func WithTimeout(d time.Duration) SessionOption {
	return func(s *Session) {
		s.timeout = d
	}
}

// Chat 发送非流式对话请求
func (s *Session) Chat(ctx context.Context, req base.ChatRequest) (base.ChatResponse, error) {
	if s.timeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.timeout)
			defer cancel()
		}
	}

	startNewChat := req.StartNewChat || s.startNewChat

	switch s.conversationMode {
	case ConversationModeRemoteSession:
		return s.chatRemoteSession(ctx, req, startNewChat)
	case ConversationModeLocalHistory:
		return s.chatLocalHistory(ctx, req, startNewChat)
	default:
		// Legacy behavior (backward compatible)
		return s.chatLegacy(ctx, req, startNewChat)
	}
}

// chatRemoteSession handles remote_session mode for Chat.
// No local history is loaded. session_id is injected only when s.id != "".
func (s *Session) chatRemoteSession(ctx context.Context, req base.ChatRequest, startNewChat bool) (base.ChatResponse, error) {
	// For remote_session, auto-generate session ID on first turn.
	if s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
		s.mu.Unlock()
	}

	if !startNewChat {
		req.SessionID = s.id
	}

	resp, err := s.doChat(ctx, req)
	if err != nil {
		return base.ChatResponse{}, err
	}

	// Extract remote session_id from response (always update to use server-provided ID)
	if !startNewChat && resp.SessionID != "" {
		s.mu.Lock()
		s.id = resp.SessionID
		s.mu.Unlock()

		// Persist immediately (accumulate history from store)
		if s.store != nil {
			accReq := req
			accReq.Messages = s.prependStoredMessages(ctx, req.Messages)
			s.saveState(ctx, accReq, resp)
		}
	} else if !startNewChat && s.store != nil && s.id != "" {
		accReq := req
		accReq.Messages = s.prependStoredMessages(ctx, req.Messages)
		s.saveState(ctx, accReq, resp)
	}

	return resp, nil
}

// chatLocalHistory handles local_history mode for Chat.
// Loads history from store, applies truncation, does NOT inject session_id to provider.
func (s *Session) chatLocalHistory(ctx context.Context, req base.ChatRequest, startNewChat bool) (base.ChatResponse, error) {
	// Ensure we have a local session ID
	if !startNewChat && s.store != nil && s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = uuid.New().String()
		}
		s.mu.Unlock()
	}

	// Load and truncate history
	var historyMsgs []base.Message
	if !startNewChat && s.mode == HistoryAuto && s.store != nil && s.id != "" {
		state, err := s.store.Get(ctx, s.id)
		if err == nil && state != nil {
			historyMsgs = state.Messages
		}
	}

	// Apply history window truncation
	if len(historyMsgs) > 0 {
		historyMsgs = s.truncateHistory(historyMsgs)
	}

	// Merge history with current messages
	if !startNewChat && len(historyMsgs) > 0 {
		allMsgs := append(historyMsgs, req.Messages...)
		req.Messages = allMsgs
	}

	// Do NOT inject session_id for local_history mode
	req.SessionID = ""

	resp, err := s.doChat(ctx, req)
	if err != nil {
		return base.ChatResponse{}, err
	}

	// Save to store
	if !startNewChat && s.store != nil && s.id != "" {
		s.saveState(ctx, req, resp)
	}

	return resp, nil
}

// chatLegacy preserves the original behavior for backward compatibility (no mode set).
func (s *Session) chatLegacy(ctx context.Context, req base.ChatRequest, startNewChat bool) (base.ChatResponse, error) {
	// 1. Lazy generate SessionID
	if !startNewChat && s.store != nil && s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = uuid.New().String()
		}
		s.mu.Unlock()
	}

	// 2. Load history
	var historyMsgs []base.Message
	if !startNewChat && s.mode == HistoryAuto && s.store != nil && s.id != "" {
		state, err := s.store.Get(ctx, s.id)
		if err == nil && state != nil {
			historyMsgs = state.Messages
		}
	}

	// 3. Merge messages
	if !startNewChat {
		allMsgs := append(historyMsgs, req.Messages...)
		req.Messages = allMsgs
	}

	// 4. Inject session_id
	if req.SessionID == "" && !startNewChat {
		req.SessionID = s.id
	}

	resp, err := s.doChat(ctx, req)
	if err != nil {
		return base.ChatResponse{}, err
	}

	// 5. Extract session_id from response
	if !startNewChat && resp.SessionID != "" && s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = resp.SessionID
		}
		s.mu.Unlock()
	}

	// 6. Save history
	if !startNewChat && s.store != nil && s.ID() != "" {
		s.saveState(ctx, req, resp)
	}

	return resp, nil
}

// doChat executes the actual chat request through the client.
func (s *Session) doChat(ctx context.Context, req base.ChatRequest) (base.ChatResponse, error) {
	if s.cred != nil || s.pc != nil {
		if s.cred == nil || s.pc == nil {
			return base.ChatResponse{}, errors.New("client: missing credential or provider config")
		}
		return s.client.chatWith(ctx, s.cred, s.pc, req)
	}

	resolved, resolveErr := s.client.resolveChatInputs(s.provider)
	if resolveErr != nil {
		return base.ChatResponse{}, resolveErr
	}
	resp, err := s.client.chatWith(ctx, resolved.cred, resolved.pc, req)
	if err != nil {
		if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
			s.client.AuthMgr.MarkFailed(resolved.cred.ID)
		}
		return base.ChatResponse{}, err
	}
	if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
		s.client.AuthMgr.MarkSuccess(resolved.cred.ID)
	}
	return resp, nil
}

// saveState persists session state to the store.
func (s *Session) saveState(ctx context.Context, req base.ChatRequest, resp base.ChatResponse) {
	sessionID := s.ID()
	if sessionID == "" {
		return
	}
	newMsgs := append(req.Messages, base.Message{
		Role:    "assistant",
		Content: resp.Text,
	})

	state := &session.SessionState{
		ID:        sessionID,
		Provider:  s.provider,
		Messages:  newMsgs,
		UpdatedAt: time.Now(),
	}

	m := make(map[string]string, len(s.meta)+3)
	for k, v := range s.meta {
		m[k] = v
	}
	if req.Model != "" {
		m["model"] = req.Model
	}
	if s.conversationMode != "" {
		m["mode"] = string(s.conversationMode)
	}
	if s.onError != "" {
		m["on_error"] = string(s.onError)
	}
	if len(m) > 0 {
		state.Meta = m
	}

	if err := s.store.Save(ctx, state); err != nil {
		if s.client.SessionConfig.OnStoreError != nil {
			s.client.SessionConfig.OnStoreError(ctx, err)
		}
	}
}

// prependStoredMessages loads existing messages from store and prepends them to currentMsgs.
// Used by remote_session mode to accumulate conversation history for local persistence,
// since remote_session does not load history into req.Messages before sending.
func (s *Session) prependStoredMessages(ctx context.Context, currentMsgs []base.Message) []base.Message {
	if s.store == nil || s.ID() == "" {
		return currentMsgs
	}
	state, err := s.store.Get(ctx, s.ID())
	if err != nil || state == nil || len(state.Messages) == 0 {
		return currentMsgs
	}
	result := make([]base.Message, 0, len(state.Messages)+len(currentMsgs))
	result = append(result, state.Messages...)
	result = append(result, currentMsgs...)
	return result
}

// truncateHistory applies history window truncation.
func (s *Session) truncateHistory(msgs []base.Message) []base.Message {
	hw := s.historyWindow
	// Fall back to client-level SessionConfig
	if hw.MaxMessages == 0 && hw.MaxTokens == 0 {
		hw = s.client.SessionConfig.HistoryWindow
	}
	if hw.MaxMessages == 0 && hw.MaxTokens == 0 {
		return msgs
	}
	return session.Truncate(msgs, hw.MaxMessages, hw.MaxTokens)
}

// ChatStream 发送流式对话请求
func (s *Session) ChatStream(ctx context.Context, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
	if s.timeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			// 只设 deadline，不在此 defer cancel。
			// chatWithStream 内部检测到 ctx 已有 deadline 后不会再覆盖，
			// 其 goroutine 会在 stream 关闭时负责清理。
			ctx, _ = context.WithTimeout(ctx, s.timeout)
		}
	}

	startNewChat := req.StartNewChat || s.startNewChat

	switch s.conversationMode {
	case ConversationModeRemoteSession:
		return s.chatStreamRemoteSession(ctx, req, startNewChat)
	case ConversationModeLocalHistory:
		return s.chatStreamLocalHistory(ctx, req, startNewChat)
	default:
		return s.chatStreamLegacy(ctx, req, startNewChat)
	}
}

func (s *Session) chatStreamRemoteSession(ctx context.Context, req base.ChatRequest, startNewChat bool) (<-chan streaming.StreamChunk, error) {
	// For remote_session, auto-generate session ID on first turn.
	// Some APIs (e.g., Qianwen) require session_id on every request.
	if s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = strings.ReplaceAll(uuid.New().String(), "-", "")
		}
		s.mu.Unlock()
	}

	if !startNewChat {
		req.SessionID = s.id
	}

	// 传入上一轮收集的链路字段值
	s.mu.Lock()
	if len(s.chainValues) > 0 && !startNewChat {
		req.ChainValues = s.chainValues
	}
	s.mu.Unlock()

	stream, err := s.doChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)

		var fullText string
		var streamErr error
		var sessionIDPersisted bool
		latestChainValues := make(map[string]string)

		for chunk := range stream {
			// Immediately persist session_id on first sight
			if !startNewChat && chunk.SessionID != "" && !sessionIDPersisted {
				s.mu.Lock()
				s.id = chunk.SessionID
				s.mu.Unlock()

				// Immediately save placeholder with session ID, preserving existing messages.
				// Per design: remote_session first-round save failure is critical.
				if s.store != nil {
					existingMsgs := s.prependStoredMessages(ctx, nil)
					placeholder := &session.SessionState{
						ID:        s.ID(),
						Provider:  s.provider,
						Messages:  existingMsgs,
						UpdatedAt: time.Now(),
					}
					if s.conversationMode != "" {
						placeholder.Meta = map[string]string{"mode": string(s.conversationMode)}
					}
					if saveErr := s.store.Save(ctx, placeholder); saveErr != nil {
						if s.client.SessionConfig.OnStoreError != nil {
							s.client.SessionConfig.OnStoreError(ctx, saveErr)
						}
					}
				}
				sessionIDPersisted = true
			}
			// 累积链路字段值
			for k, v := range chunk.ChainValues {
				latestChainValues[k] = v
			}
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
			if chunk.Text != "" {
				fullText += chunk.Text
			}
			out <- chunk
		}

		// 流结束后更新 Session 链路字段值（供下一轮使用）
		if len(latestChainValues) > 0 {
			s.mu.Lock()
			s.chainValues = latestChainValues
			s.mu.Unlock()
		}

		if streamErr != nil {
			return
		}

		// Full save after stream completes (accumulate history from store)
		sessionID := s.ID()
		if !startNewChat && s.store != nil && sessionID != "" {
			allMsgs := s.prependStoredMessages(ctx, req.Messages)
			allMsgs = append(allMsgs, base.Message{
				Role:    "assistant",
				Content: fullText,
			})
			state := &session.SessionState{
				ID:        sessionID,
				Provider:  s.provider,
				Messages:  allMsgs,
				UpdatedAt: time.Now(),
			}
			m := make(map[string]string, len(s.meta)+2)
			for k, v := range s.meta {
				m[k] = v
			}
			if req.Model != "" {
				m["model"] = req.Model
			}
			if s.conversationMode != "" {
				m["mode"] = string(s.conversationMode)
			}
			if len(m) > 0 {
				state.Meta = m
			}
			if err := s.store.Save(ctx, state); err != nil {
				if s.client.SessionConfig.OnStoreError != nil {
					s.client.SessionConfig.OnStoreError(ctx, err)
				}
			}
		}
	}()

	return out, nil
}

func (s *Session) chatStreamLocalHistory(ctx context.Context, req base.ChatRequest, startNewChat bool) (<-chan streaming.StreamChunk, error) {
	// Ensure local session ID
	if !startNewChat && s.store != nil && s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = uuid.New().String()
		}
		s.mu.Unlock()
	}

	// Load history and restore chain values
	var historyMsgs []base.Message
	if !startNewChat && s.mode == HistoryAuto && s.store != nil && s.id != "" {
		state, err := s.store.Get(ctx, s.id)
		if err == nil && state != nil {
			historyMsgs = state.Messages
			// Restore persisted chain values (if not already set in memory)
			if state.Meta != nil {
				if chainJSON, ok := state.Meta["chain_values"]; ok && chainJSON != "" {
					var cv map[string]string
					if jsonErr := json.Unmarshal([]byte(chainJSON), &cv); jsonErr == nil {
						s.mu.Lock()
						if len(s.chainValues) == 0 {
							s.chainValues = cv
						}
						s.mu.Unlock()
					}
				}
			}
		}
	}
	if len(historyMsgs) > 0 {
		historyMsgs = s.truncateHistory(historyMsgs)
	}
	if !startNewChat && len(historyMsgs) > 0 {
		allMsgs := append(historyMsgs, req.Messages...)
		req.Messages = allMsgs
	}

	// If we have a previous tool_call_id, convert the current user input to a tool message
	if !startNewChat {
		s.mu.Lock()
		prevChain := s.chainValues
		s.mu.Unlock()
		if toolCallID, ok := prevChain["$$$TOOL_CALL_ID$$$"]; ok && toolCallID != "" {
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if strings.EqualFold(req.Messages[i].Role, "user") {
					req.Messages[i].Role = "tool"
					req.Messages[i].ToolCallID = toolCallID
					break
				}
			}
		}
		// Pass chain values to provider for template rendering
		if len(prevChain) > 0 {
			req.ChainValues = prevChain
		}
	}

	// Do NOT inject session_id
	req.SessionID = ""

	stream, err := s.doChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)
		var fullText string
		var streamErr error
		latestChainValues := make(map[string]string)

		for chunk := range stream {
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
			if chunk.Text != "" {
				fullText += chunk.Text
			}
			for k, v := range chunk.ChainValues {
				latestChainValues[k] = v
			}
			out <- chunk
		}

		// Update in-memory chain values for next turn
		if len(latestChainValues) > 0 {
			s.mu.Lock()
			s.chainValues = latestChainValues
			s.mu.Unlock()
		}

		if streamErr != nil {
			return
		}

		sessionID := s.ID()
		if !startNewChat && s.store != nil && sessionID != "" {
			newMsgs := append(req.Messages, base.Message{
				Role:    "assistant",
				Content: fullText,
			})
			state := &session.SessionState{
				ID:        sessionID,
				Provider:  s.provider,
				Messages:  newMsgs,
				UpdatedAt: time.Now(),
			}
			m := make(map[string]string, len(s.meta)+4)
			for k, v := range s.meta {
				m[k] = v
			}
			if req.Model != "" {
				m["model"] = req.Model
			}
			m["mode"] = string(ConversationModeLocalHistory)
			// Persist chain values for next session restore
			s.mu.Lock()
			cv := s.chainValues
			s.mu.Unlock()
			if len(cv) > 0 {
				if chainJSON, err := json.Marshal(cv); err == nil {
					m["chain_values"] = string(chainJSON)
				}
			}
			state.Meta = m
			if err := s.store.Save(ctx, state); err != nil {
				if s.client.SessionConfig.OnStoreError != nil {
					s.client.SessionConfig.OnStoreError(ctx, err)
				}
			}
		}
	}()

	return out, nil
}

func (s *Session) chatStreamLegacy(ctx context.Context, req base.ChatRequest, startNewChat bool) (<-chan streaming.StreamChunk, error) {
	// 1. Lazy generate SessionID
	if !startNewChat && s.store != nil && s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = uuid.New().String()
		}
		s.mu.Unlock()
	}

	// 2. Load history
	var historyMsgs []base.Message
	if !startNewChat && s.mode == HistoryAuto && s.store != nil && s.id != "" {
		state, err := s.store.Get(ctx, s.id)
		if err == nil && state != nil {
			historyMsgs = state.Messages
		}
	}

	// 3. Merge messages
	if !startNewChat {
		allMsgs := append(historyMsgs, req.Messages...)
		req.Messages = allMsgs
	}

	// 4. Inject session_id
	if req.SessionID == "" && !startNewChat {
		req.SessionID = s.id
	}

	stream, err := s.doChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	// 5. Wrap stream
	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)

		var fullText string
		var streamErr error
		var sessionIDExtracted bool

		for chunk := range stream {
			if !startNewChat && chunk.SessionID != "" && !sessionIDExtracted {
				if s.id == "" {
					s.mu.Lock()
					if s.id == "" {
						s.id = chunk.SessionID
					}
					s.mu.Unlock()
				}
				sessionIDExtracted = true
			}
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
			if chunk.Text != "" {
				fullText += chunk.Text
			}
			out <- chunk
		}

		if streamErr != nil {
			return
		}

		sessionID := s.ID()
		if !startNewChat && s.store != nil && sessionID != "" {
			newMsgs := append(req.Messages, base.Message{
				Role:    "assistant",
				Content: fullText,
			})

			state := &session.SessionState{
				ID:        sessionID,
				Provider:  s.provider,
				Messages:  newMsgs,
				UpdatedAt: time.Now(),
			}
			if len(s.meta) > 0 || req.Model != "" {
				m := make(map[string]string, len(s.meta)+1)
				for k, v := range s.meta {
					m[k] = v
				}
				if req.Model != "" {
					m["model"] = req.Model
				}
				state.Meta = m
			}

			if err := s.store.Save(ctx, state); err != nil {
				if s.client.SessionConfig.OnStoreError != nil {
					s.client.SessionConfig.OnStoreError(ctx, err)
				}
			}
		}
	}()

	return out, nil
}

// doChatStream executes the actual stream request through the client.
func (s *Session) doChatStream(ctx context.Context, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
	if s.cred != nil || s.pc != nil {
		if s.cred == nil || s.pc == nil {
			return nil, errors.New("client: missing credential or provider config")
		}
		return s.client.chatWithStream(ctx, s.cred, s.pc, req)
	}

	resolved, resolveErr := s.client.resolveChatInputs(s.provider)
	if resolveErr != nil {
		return nil, resolveErr
	}
	stream, err := s.client.chatWithStream(ctx, resolved.cred, resolved.pc, req)
	if err != nil {
		if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
			s.client.AuthMgr.MarkFailed(resolved.cred.ID)
		}
		return nil, err
	}
	if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
		s.client.AuthMgr.MarkSuccess(resolved.cred.ID)
	}
	return stream, nil
}

// ID 获取SessionID
func (s *Session) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}
