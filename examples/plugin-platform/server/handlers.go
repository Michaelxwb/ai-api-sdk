package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Michaelxwb/ai-api-sdk/examples/plugin-platform/storage"
)

type Config struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

type ConfigResponse struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Platform  string                   `json:"platform"`
	URL       string                   `json:"url"`
	CreatedAt time.Time                `json:"createdAt"`
	Connected bool                     `json:"connected"`
	Locators  *storage.ElementLocators `json:"locators,omitempty"`
}

type CreateConfigRequest struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type SendMessagePayload struct {
	ConfigID     string                  `json:"configId"`
	SessionID    string                  `json:"sessionId,omitempty"`
	Text         string                  `json:"text"`
	Locators     storage.ElementLocators `json:"locators"`
	PlatformURL  string                  `json:"platformUrl,omitempty"`
	Stream       bool                    `json:"stream,omitempty"`
	StartNewChat bool                    `json:"startNewChat,omitempty"`
}

type ChatRequest struct {
	ConfigID     string `json:"configId"`
	Text         string `json:"text"`
	SessionID    string `json:"sessionId,omitempty"`
	Stream       bool   `json:"stream,omitempty"`
	StartNewChat bool   `json:"startNewChat,omitempty"`
}

type ChatResponse struct {
	ID         string   `json:"id"`
	Content    string   `json:"content"`
	Chunks     []string `json:"chunks,omitempty"`
	DurationMs int64    `json:"durationMs"`
	Error      string   `json:"error,omitempty"`
}

type API struct {
	ws       *WebSocketServer
	locators *storage.LocatorStore
	configs  *ConfigStore
}

func NewAPI(ws *WebSocketServer, locators *storage.LocatorStore, configs *ConfigStore) *API {
	return &API{
		ws:       ws,
		locators: locators,
		configs:  configs,
	}
}

func (a *API) HandleConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		configs := a.configs.List()
		responses := make([]ConfigResponse, 0, len(configs))
		for _, cfg := range configs {
			response := ConfigResponse{
				ID:        cfg.ID,
				Name:      cfg.Name,
				Platform:  cfg.Platform,
				URL:       cfg.URL,
				CreatedAt: cfg.CreatedAt,
				Connected: a.ws.IsConnected(cfg.ID) || a.ws.HasAnyConnection(),
			}
			if locators, ok := a.locators.Get(cfg.ID); ok {
				response.Locators = &locators
			}
			responses = append(responses, response)
		}
		writeJSON(w, http.StatusOK, responses)
	case http.MethodPost:
		var req CreateConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		cfg := a.configs.Create(req)
		response := ConfigResponse{
			ID:        cfg.ID,
			Name:      cfg.Name,
			Platform:  cfg.Platform,
			URL:       cfg.URL,
			CreatedAt: cfg.CreatedAt,
			Connected: false,
		}
		writeJSON(w, http.StatusCreated, response)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *API) HandleConfigAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/configs/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "locate" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	configID := parts[0]
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if _, ok := a.configs.Get(configID); !ok {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}
	// 检查是否有任意插件连接
	if !a.ws.HasAnyConnection() {
		writeError(w, http.StatusBadRequest, "plugin not connected")
		return
	}

	cfg, _ := a.configs.Get(configID)
	pending := a.ws.NewPending(pendingLocate, configID)
	payload, err := encodePayload(map[string]string{
		"configId":    configID,
		"platformUrl": cfg.URL, // 包含URL以便插件找到正确的Tab
	})
	if err != nil {
		a.ws.RemovePending(pending.id)
		writeError(w, http.StatusInternalServerError, "failed to build payload")
		return
	}
	msg := Message{
		ID:        pending.id,
		Type:      MessageTypeStartLocating,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
	if err := a.ws.SendToConfig(configID, msg); err != nil {
		a.ws.RemovePending(pending.id)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := pending.wait(ctx); err != nil {
		a.ws.RemovePending(pending.id)
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "locate timed out")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	locators := pending.Locators()
	if isLocatorsEmpty(locators) {
		if stored, ok := a.locators.Get(configID); ok {
			locators = stored
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       configID,
		"locators": locators,
	})
}

func (a *API) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.ConfigID) == "" {
		writeError(w, http.StatusBadRequest, "configId is required")
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}
	if _, ok := a.configs.Get(req.ConfigID); !ok {
		writeError(w, http.StatusNotFound, "config not found")
		return
	}
	// 检查是否有任意插件连接
	if !a.ws.HasAnyConnection() {
		writeError(w, http.StatusBadRequest, "plugin not connected")
		return
	}

	locators, ok := a.locators.Get(req.ConfigID)
	if !ok {
		writeError(w, http.StatusBadRequest, "locators not set")
		return
	}

	cfg, _ := a.configs.Get(req.ConfigID)
	if req.Stream {
		a.handleChatStream(w, r, req, cfg, locators)
		return
	}
	pending := a.ws.NewPending(pendingChat, req.ConfigID)
	payload, err := encodePayload(SendMessagePayload{
		ConfigID:     req.ConfigID,
		SessionID:    req.SessionID,
		Text:         req.Text,
		Locators:     locators,
		PlatformURL:  cfg.URL, // 包含URL以便插件找到正确的Tab
		Stream:       req.Stream,
		StartNewChat: req.StartNewChat,
	})
	if err != nil {
		a.ws.RemovePending(pending.id)
		writeError(w, http.StatusInternalServerError, "failed to build payload")
		return
	}
	msg := Message{
		ID:        pending.id,
		Type:      MessageTypeSendMessage,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
	if err := a.ws.SendToConfig(req.ConfigID, msg); err != nil {
		a.ws.RemovePending(pending.id)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	if err := pending.wait(ctx); err != nil {
		a.ws.RemovePending(pending.id)
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "chat timed out")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	response := ChatResponse{
		ID:         pending.id,
		Content:    pending.Content(),
		Chunks:     pending.Chunks(),
		DurationMs: pending.Duration().Milliseconds(),
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleChatStream(w http.ResponseWriter, r *http.Request, req ChatRequest, cfg *Config, locators storage.ElementLocators) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	pending := a.ws.NewPending(pendingChat, req.ConfigID)
	payload, err := encodePayload(SendMessagePayload{
		ConfigID:     req.ConfigID,
		SessionID:    req.SessionID,
		Text:         req.Text,
		Locators:     locators,
		PlatformURL:  cfg.URL,
		Stream:       true,
		StartNewChat: req.StartNewChat,
	})
	if err != nil {
		a.ws.RemovePending(pending.id)
		writeError(w, http.StatusInternalServerError, "failed to build payload")
		return
	}
	msg := Message{
		ID:        pending.id,
		Type:      MessageTypeSendMessage,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}
	if err := a.ws.SendToConfig(req.ConfigID, msg); err != nil {
		a.ws.RemovePending(pending.id)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[stream] starting SSE stream for configID=%s, pendingID=%s", req.ConfigID, pending.id)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	defer a.ws.RemovePending(pending.id)

	for {
		select {
		case chunk, ok := <-pending.ChunkCh():
			if !ok {
				log.Printf("[stream] chunkCh closed, pendingID=%s, err=%v, content_len=%d", pending.id, pending.Err(), len(pending.Content()))
				if err := pending.Err(); err != nil {
					writeSSE(w, "error", map[string]any{"error": err.Error()})
				} else {
					writeSSE(w, "done", map[string]any{
						"content":    pending.Content(),
						"durationMs": pending.Duration().Milliseconds(),
					})
				}
				flusher.Flush()
				return
			}
			log.Printf("[stream] chunk received, pendingID=%s, chunk_len=%d", pending.id, len(chunk))
			writeSSE(w, "chunk", map[string]any{"chunk": chunk})
			flusher.Flush()
		case <-ctx.Done():
			log.Printf("[stream] context timeout, pendingID=%s", pending.id)
			writeSSE(w, "error", map[string]any{"error": "timeout"})
			flusher.Flush()
			return
		}
	}
}

func writeSSE(w http.ResponseWriter, event string, data any) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
}

type ConfigStore struct {
	mu    sync.RWMutex
	items map[string]*Config
}

func NewConfigStore() *ConfigStore {
	return &ConfigStore{items: make(map[string]*Config)}
}

func (s *ConfigStore) Create(req CreateConfigRequest) *Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := &Config{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(req.Name),
		Platform:  strings.TrimSpace(req.Platform),
		URL:       strings.TrimSpace(req.URL),
		CreatedAt: time.Now(),
	}
	s.items[cfg.ID] = cfg
	return cfg
}

func (s *ConfigStore) List() []*Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Config, 0, len(s.items))
	for _, cfg := range s.items {
		copyCfg := *cfg
		result = append(result, &copyCfg)
	}
	return result
}

func (s *ConfigStore) Get(id string) (*Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.items[id]
	if !ok {
		return nil, false
	}
	copyCfg := *cfg
	return &copyCfg, true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func isLocatorsEmpty(locators storage.ElementLocators) bool {
	return isLocatorEmpty(locators.Input) &&
		isLocatorEmpty(locators.SendButton) &&
		isLocatorEmpty(locators.ReplyArea)
}

func isLocatorEmpty(locator storage.ElementLocator) bool {
	return strings.TrimSpace(locator.Selector) == "" && strings.TrimSpace(locator.XPath) == ""
}
