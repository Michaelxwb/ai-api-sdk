package client

import (
	"context"
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

	// 内部状态
	mu          sync.Mutex
	initialized bool
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

// Chat 发送非流式对话请求
func (s *Session) Chat(ctx context.Context, req base.ChatRequest) (base.ChatResponse, error) {
	startNewChat := req.StartNewChat || s.startNewChat
	// 1. 懒生成SessionID
	if !startNewChat && s.store != nil && s.id == "" && !s.isDifyProvider() {
		s.mu.Lock()
		if s.id == "" {
			s.id = uuid.New().String()
		}
		s.mu.Unlock()
	}

	// 2. 加载历史（如果HistoryAuto）
	var historyMsgs []base.Message
	if !startNewChat && s.mode == HistoryAuto && s.store != nil && s.id != "" {
		state, err := s.store.Get(ctx, s.id)
		if err == nil && state != nil {
			historyMsgs = state.Messages
		}
		// 失败降级：继续执行，无历史
	}

	// 3. 合并消息
	if !startNewChat {
		allMsgs := append(historyMsgs, req.Messages...)
		req.Messages = allMsgs
	}

	// 4. 发送请求（使用Client内部方法）
	if req.SessionID == "" && !startNewChat {
		req.SessionID = s.id
	}
	var resp base.ChatResponse
	var err error
	if s.cred != nil || s.pc != nil {
		if s.cred == nil || s.pc == nil {
			return base.ChatResponse{}, errors.New("client: missing credential or provider config")
		}
		resp, err = s.client.chatWith(ctx, s.cred, s.pc, req)
	} else {
		resolved, resolveErr := s.client.resolveChatInputs(s.provider)
		if resolveErr != nil {
			return base.ChatResponse{}, resolveErr
		}
		resp, err = s.client.chatWith(ctx, resolved.cred, resolved.pc, req)
		if err != nil {
			if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
				s.client.AuthMgr.MarkFailed(resolved.cred.ID)
			}
			return base.ChatResponse{}, err
		}
		if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
			s.client.AuthMgr.MarkSuccess(resolved.cred.ID)
		}
	}
	if err != nil {
		return base.ChatResponse{}, err
	}

	// 5. 提取Dify conversation_id
	if !startNewChat && resp.SessionID != "" && s.id == "" {
		s.mu.Lock()
		if s.id == "" {
			s.id = resp.SessionID
		}
		s.mu.Unlock()
	}

	// 6. 保存历史
	sessionID := s.ID()
	if !startNewChat && s.store != nil && sessionID != "" {
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

	return resp, nil
}

// ChatStream 发送流式对话请求
func (s *Session) ChatStream(ctx context.Context, req base.ChatRequest) (<-chan streaming.StreamChunk, error) {
	startNewChat := req.StartNewChat || s.startNewChat
	// 1. 懒生成SessionID
	if !startNewChat && s.store != nil && s.id == "" && !s.isDifyProvider() {
		s.mu.Lock()
		if s.id == "" {
			s.id = uuid.New().String()
		}
		s.mu.Unlock()
	}

	// 2. 加载历史（如果HistoryAuto）
	var historyMsgs []base.Message
	if !startNewChat && s.mode == HistoryAuto && s.store != nil && s.id != "" {
		state, err := s.store.Get(ctx, s.id)
		if err == nil && state != nil {
			historyMsgs = state.Messages
		}
		// 失败降级：继续执行，无历史
	}

	// 3. 合并消息
	if !startNewChat {
		allMsgs := append(historyMsgs, req.Messages...)
		req.Messages = allMsgs
	}

	// 4. 发送请求（使用Client内部方法）
	if req.SessionID == "" && !startNewChat {
		req.SessionID = s.id
	}
	var stream <-chan streaming.StreamChunk
	var err error
	if s.cred != nil || s.pc != nil {
		if s.cred == nil || s.pc == nil {
			return nil, errors.New("client: missing credential or provider config")
		}
		stream, err = s.client.chatWithStream(ctx, s.cred, s.pc, req)
	} else {
		resolved, resolveErr := s.client.resolveChatInputs(s.provider)
		if resolveErr != nil {
			return nil, resolveErr
		}
		stream, err = s.client.chatWithStream(ctx, resolved.cred, resolved.pc, req)
		if err != nil {
			if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
				s.client.AuthMgr.MarkFailed(resolved.cred.ID)
			}
			return nil, err
		}
		if s.client != nil && s.client.AuthMgr != nil && resolved.cred != nil {
			s.client.AuthMgr.MarkSuccess(resolved.cred.ID)
		}
	}
	if err != nil {
		return nil, err
	}

	// 5. 包装Stream以拦截conversation_id，流结束后保存历史
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

// ID 获取SessionID
func (s *Session) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

func (s *Session) isDifyProvider() bool {
	if s == nil {
		return false
	}
	if s.pc != nil {
		if isDifyName(s.pc.Type) || isDifyName(s.pc.Name) {
			return true
		}
	}
	if isDifyName(s.provider) {
		return true
	}
	if s.client != nil && s.client.Config != nil && s.provider != "" {
		if pc := s.client.Config.FindProvider(s.provider); pc != nil {
			name := pc.Type
			if name == "" {
				name = pc.Name
			}
			if isDifyName(name) {
				return true
			}
		}
	}
	return false
}

func isDifyName(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "dify")
}
