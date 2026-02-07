package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Michaelxwb/ai-api-sdk/examples/plugin-platform/storage"
)

const (
	MessageTypeAuth           = "auth"
	MessageTypeHeartbeat      = "heartbeat"
	MessageTypeStartLocating  = "start_locating"
	MessageTypeSendMessage    = "send_message"
	MessageTypeStopGeneration = "stop_generation"
	MessageTypeLocatingResult = "locating_result"
	MessageTypeReplyChunk     = "reply_chunk"
	MessageTypeReplyDone      = "reply_done"
	MessageTypeError          = "error"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 25 * time.Second
	maxMessageSize = 1 << 20
)

type Message struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type AuthPayload struct {
	ConfigID      string `json:"configId,omitempty"`
	Token         string `json:"token,omitempty"`
	PluginVersion string `json:"pluginVersion,omitempty"`
	BrowserInfo   string `json:"browserInfo,omitempty"`
}

type LocateResultPayload struct {
	ConfigID string                  `json:"configId"`
	Locators storage.ElementLocators `json:"locators"`
}

type ReplyChunkPayload struct {
	ConfigID string `json:"configId"`
	Chunk    string `json:"chunk"`
	Content  string `json:"content"`
	FullText string `json:"fullText"`
}

type ReplyDonePayload struct {
	ConfigID string `json:"configId"`
	Content  string `json:"content"`
	FullText string `json:"fullText"`
	Error    string `json:"error"`
}

type ErrorPayload struct {
	Message string `json:"message"`
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	configID string
	role     ClientRole
	mu       sync.Mutex
	closed   bool
}

func (c *Client) close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()
	close(c.send)
	_ = c.conn.Close()
}

type pendingKind string

const (
	pendingLocate pendingKind = "locate"
	pendingChat   pendingKind = "chat"
)

type ClientRole string

const (
	rolePlugin ClientRole = "plugin"
	roleClient ClientRole = "client"
)

type relayRequest struct {
	client   *Client
	configID string
}

type pendingRequest struct {
	id        string
	kind      pendingKind
	configID  string
	createdAt time.Time

	mu       sync.Mutex
	chunks   []string
	content  string
	locators storage.ElementLocators
	err      error
	done     bool
	chunkCh  chan string
	doneCh   chan struct{}
}

func newPending(kind pendingKind, configID string) *pendingRequest {
	return &pendingRequest{
		id:        uuid.NewString(),
		kind:      kind,
		configID:  configID,
		createdAt: time.Now(),
		chunkCh:   make(chan string, 64),
		doneCh:    make(chan struct{}),
	}
}

func (p *pendingRequest) addChunk(chunk string) {
	if strings.TrimSpace(chunk) == "" {
		return
	}
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	p.chunks = append(p.chunks, chunk)
	select {
	case p.chunkCh <- chunk:
	default:
	}
	p.mu.Unlock()
}

func (p *pendingRequest) setContent(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	p.mu.Lock()
	p.content = content
	p.mu.Unlock()
}

func (p *pendingRequest) setLocators(locators storage.ElementLocators) {
	p.mu.Lock()
	p.locators = locators
	p.mu.Unlock()
}

func (p *pendingRequest) fail(err error) {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	p.err = err
	p.done = true
	close(p.chunkCh)
	p.mu.Unlock()
	close(p.doneCh)
}

func (p *pendingRequest) finish() {
	p.mu.Lock()
	if p.done {
		p.mu.Unlock()
		return
	}
	p.done = true
	close(p.chunkCh)
	p.mu.Unlock()
	close(p.doneCh)
}

func (p *pendingRequest) wait(ctx context.Context) error {
	select {
	case <-p.doneCh:
		return p.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *pendingRequest) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *pendingRequest) ChunkCh() <-chan string {
	return p.chunkCh
}

func (p *pendingRequest) Content() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.content != "" {
		return p.content
	}
	return strings.Join(p.chunks, "")
}

func (p *pendingRequest) Chunks() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	copyChunks := make([]string, len(p.chunks))
	copy(copyChunks, p.chunks)
	return copyChunks
}

func (p *pendingRequest) Locators() storage.ElementLocators {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.locators
}

func (p *pendingRequest) Duration() time.Duration {
	return time.Since(p.createdAt)
}

type WebSocketServer struct {
	upgrader websocket.Upgrader

	mu      sync.RWMutex
	clients map[string]*Client

	pendingMu sync.Mutex
	pending   map[string]*pendingRequest

	relayMu sync.Mutex
	relay   map[string]relayRequest

	locators *storage.LocatorStore
}

func NewWebSocketServer(locators *storage.LocatorStore) *WebSocketServer {
	return &WebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		clients:  make(map[string]*Client),
		pending:  make(map[string]*pendingRequest),
		relay:    make(map[string]relayRequest),
		locators: locators,
	}
}

func (s *WebSocketServer) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	role := parseRole(r)
	client := &Client{
		conn: conn,
		send: make(chan []byte, 16),
		role: role,
	}

	configID := r.URL.Query().Get("configId")
	if role == rolePlugin {
		if configID == "" {
			// 为没有configId的连接分配一个默认ID
			configID = "_default_" + uuid.NewString()[:8]
		}
		s.registerClient(configID, client)
		log.Printf("plugin connected: %s", configID)
	} else {
		client.configID = configID
		if configID == "" {
			log.Printf("sdk client connected")
		} else {
			log.Printf("sdk client connected: %s", configID)
		}
	}

	go s.writePump(client)
	s.readPump(client)
}

func (s *WebSocketServer) IsConnected(configID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.clients[configID]
	return ok
}

func (s *WebSocketServer) HasAnyConnection() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients) > 0
}

func (s *WebSocketServer) SendToConfig(configID string, message Message) error {
	client := s.getClient(configID)
	// 如果没有找到指定configID的client，尝试获取任意一个已连接的client
	if client == nil {
		client = s.getAnyClient()
	}
	if client == nil {
		return errors.New("plugin not connected")
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	select {
	case client.send <- payload:
		return nil
	default:
		return errors.New("plugin send buffer full")
	}
}

func (s *WebSocketServer) getAnyClient() *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		return c
	}
	return nil
}

func (s *WebSocketServer) NewPending(kind pendingKind, configID string) *pendingRequest {
	pending := newPending(kind, configID)
	s.pendingMu.Lock()
	s.pending[pending.id] = pending
	s.pendingMu.Unlock()
	return pending
}

func (s *WebSocketServer) RemovePending(id string) {
	s.pendingMu.Lock()
	delete(s.pending, id)
	s.pendingMu.Unlock()
}

func (s *WebSocketServer) getPending(id string) *pendingRequest {
	s.pendingMu.Lock()
	pending := s.pending[id]
	s.pendingMu.Unlock()
	return pending
}

func (s *WebSocketServer) writePump(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.close()
		s.unregisterClient(client)
	}()

	for {
		select {
		case message, ok := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *WebSocketServer) readPump(client *Client) {
	defer func() {
		client.close()
		s.unregisterClient(client)
	}()

	client.conn.SetReadLimit(maxMessageSize)
	_ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		_ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := client.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("invalid message: %v", err)
			continue
		}
		if msg.Timestamp == 0 {
			msg.Timestamp = time.Now().UnixMilli()
		}

		s.handleMessage(client, msg)
	}
}

func (s *WebSocketServer) handleMessage(client *Client, msg Message) {
	if s.forwardRelay(msg) && msg.Type != MessageTypeLocatingResult {
		return
	}

	if client.role == roleClient {
		if msg.Type == MessageTypeStartLocating || msg.Type == MessageTypeSendMessage {
			s.handleRelayRequest(client, msg)
			return
		}
	}

	switch msg.Type {
	case MessageTypeAuth:
		if client.role != rolePlugin {
			return
		}
		var payload AuthPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			s.sendError(client, msg.ID, "invalid auth payload")
			return
		}
		if payload.ConfigID == "" {
			// 插件可能不会带 configId，保持已有的连接标识
			return
		}
		s.registerClient(payload.ConfigID, client)
	case MessageTypeHeartbeat:
		// no-op: pong handler already refreshes deadlines
	case MessageTypeLocatingResult:
		var payload LocateResultPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			s.sendError(client, msg.ID, "invalid locating payload")
			return
		}
		configID := payload.ConfigID
		if configID == "" {
			configID = client.configID
		}
		if configID == "" {
			s.sendError(client, msg.ID, "configId required")
			return
		}
		payload.Locators.UpdatedAt = time.Now()
		s.locators.Set(configID, payload.Locators)
		if pending := s.getPending(msg.ID); pending != nil {
			pending.setLocators(payload.Locators)
			pending.finish()
			s.RemovePending(msg.ID)
		}
	case MessageTypeReplyChunk:
		log.Printf("[ws] received reply_chunk, msgID=%s", msg.ID)
		var payload ReplyChunkPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			s.sendError(client, msg.ID, "invalid reply chunk payload")
			return
		}
		chunk := payload.Chunk
		if chunk == "" {
			chunk = payload.Content
		}
		if chunk == "" {
			chunk = payload.FullText
		}
		if pending := s.getPending(msg.ID); pending != nil {
			pending.addChunk(chunk)
		}
	case MessageTypeReplyDone:
		log.Printf("[ws] received reply_done, msgID=%s", msg.ID)
		var payload ReplyDonePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			s.sendError(client, msg.ID, "invalid reply done payload")
			return
		}
		if pending := s.getPending(msg.ID); pending != nil {
			if payload.Error != "" {
				pending.fail(errors.New(payload.Error))
			} else {
				content := payload.Content
				if content == "" {
					content = payload.FullText
				}
				pending.setContent(content)
				pending.finish()
			}
			s.RemovePending(msg.ID)
		}
	case MessageTypeError:
		log.Printf("[ws] received error, msgID=%s", msg.ID)
		var payload ErrorPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			payload.Message = "unknown error"
		}
		if payload.Message == "" {
			payload.Message = payload.Error
		}
		if payload.Message == "" {
			payload.Message = payload.Details
		}
		if payload.Message == "" {
			payload.Message = "unknown error"
		}
		if pending := s.getPending(msg.ID); pending != nil {
			pending.fail(errors.New(payload.Message))
			s.RemovePending(msg.ID)
		}
	case MessageTypeSendMessage:
		log.Printf("[ws] received send_message from plugin, msgID=%s (not handled in WS path)", msg.ID)
	default:
		log.Printf("unhandled message type: %s", msg.Type)
	}
}

func (s *WebSocketServer) sendError(client *Client, id, message string) {
	payload, _ := json.Marshal(ErrorPayload{Message: message})
	_ = s.SendToClient(client, Message{
		ID:        id,
		Type:      MessageTypeError,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
}

func (s *WebSocketServer) SendToClient(client *Client, message Message) error {
	if client == nil {
		return errors.New("client not connected")
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	select {
	case client.send <- payload:
		return nil
	default:
		return errors.New("client send buffer full")
	}
}

func (s *WebSocketServer) registerClient(configID string, client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.clients[configID]; existing != nil && existing != client {
		existing.close()
	}
	client.configID = configID
	s.clients[configID] = client
}

func (s *WebSocketServer) unregisterClient(client *Client) {
	if client == nil {
		return
	}
	if client.role == roleClient {
		s.removeRelayByClient(client)
		return
	}
	configID := client.configID
	if configID == "" {
		return
	}
	s.mu.Lock()
	existing := s.clients[configID]
	if existing == client {
		delete(s.clients, configID)
	}
	s.mu.Unlock()
	if existing != nil {
		s.failPendingByConfig(configID, errors.New("connection closed"))
		s.failRelayByConfig(configID, errors.New("connection closed"))
	}
}

func (s *WebSocketServer) failPendingByConfig(configID string, err error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for id, pending := range s.pending {
		if pending.configID == configID {
			pending.fail(err)
			delete(s.pending, id)
		}
	}
}

func (s *WebSocketServer) getClient(configID string) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[configID]
}

func parseRole(r *http.Request) ClientRole {
	role := strings.ToLower(r.URL.Query().Get("role"))
	if role == "client" || role == "sdk" || role == "api" {
		return roleClient
	}
	if r.URL.Query().Get("client") == "1" {
		return roleClient
	}
	return rolePlugin
}

func (s *WebSocketServer) addRelay(id string, req relayRequest) {
	s.relayMu.Lock()
	s.relay[id] = req
	s.relayMu.Unlock()
}

func (s *WebSocketServer) getRelay(id string) (relayRequest, bool) {
	s.relayMu.Lock()
	relay, ok := s.relay[id]
	s.relayMu.Unlock()
	return relay, ok
}

func (s *WebSocketServer) removeRelay(id string) {
	s.relayMu.Lock()
	delete(s.relay, id)
	s.relayMu.Unlock()
}

func (s *WebSocketServer) removeRelayByClient(client *Client) {
	s.relayMu.Lock()
	for id, relay := range s.relay {
		if relay.client == client {
			delete(s.relay, id)
		}
	}
	s.relayMu.Unlock()
}

func (s *WebSocketServer) failRelayByConfig(configID string, err error) {
	type relayItem struct {
		id     string
		client *Client
	}
	var items []relayItem
	s.relayMu.Lock()
	for id, relay := range s.relay {
		if relay.configID != configID {
			continue
		}
		delete(s.relay, id)
		items = append(items, relayItem{id: id, client: relay.client})
	}
	s.relayMu.Unlock()
	for _, item := range items {
		s.sendError(item.client, item.id, err.Error())
	}
}

func (s *WebSocketServer) forwardRelay(msg Message) bool {
	relay, ok := s.getRelay(msg.ID)
	if !ok {
		return false
	}
	_ = s.SendToClient(relay.client, msg)
	switch msg.Type {
	case MessageTypeLocatingResult, MessageTypeReplyDone, MessageTypeError:
		s.removeRelay(msg.ID)
	}
	return true
}

func (s *WebSocketServer) handleRelayRequest(client *Client, msg Message) {
	configID := extractConfigID(msg.Payload)
	if configID == "" {
		configID = client.configID
	}
	if configID == "" {
		s.sendError(client, msg.ID, "configId required")
		return
	}
	msg = s.injectPlatformURL(configID, msg)
	s.addRelay(msg.ID, relayRequest{client: client, configID: configID})
	if err := s.SendToConfig(configID, msg); err != nil {
		s.removeRelay(msg.ID)
		s.sendError(client, msg.ID, err.Error())
	}
}

func extractConfigID(payload json.RawMessage) string {
	var result struct {
		ConfigID string `json:"configId"`
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return ""
	}
	return result.ConfigID
}

func (s *WebSocketServer) injectPlatformURL(configID string, msg Message) Message {
	if len(msg.Payload) == 0 {
		return msg
	}
	var payload map[string]any
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return msg
	}
	if _, ok := payload["platformUrl"]; ok {
		return msg
	}
	platformURL := ""
	if locMap, ok := payload["locators"].(map[string]any); ok {
		if raw, ok := locMap["platformUrl"].(string); ok && strings.TrimSpace(raw) != "" {
			platformURL = raw
		}
	}
	if platformURL == "" {
		locators, ok := s.locators.Get(configID)
		if ok {
			platformURL = strings.TrimSpace(locators.PlatformURL)
		}
	}
	if platformURL == "" {
		return msg
	}
	payload["platformUrl"] = platformURL
	if locMap, ok := payload["locators"].(map[string]any); ok {
		if _, ok := locMap["platformUrl"]; !ok {
			locMap["platformUrl"] = platformURL
			payload["locators"] = locMap
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return msg
	}
	msg.Payload = raw
	return msg
}

func encodePayload(value any) (json.RawMessage, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
