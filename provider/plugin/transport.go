package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// PluginTransport 通过浏览器插件进行通信的Transport
type PluginTransport struct {
	wsConn    *websocket.Conn
	locators  *ElementLocators
	configID  string
	callbacks map[string]chan *Message
	mu        sync.RWMutex

	client         *Client
	requestTimeout time.Duration
	callbackBuffer int
}

var _ base.ProviderTransportSpec = (*PluginTransport)(nil)

// NewPluginTransport 创建传输层
func NewPluginTransport(cfg Config) (*PluginTransport, error) {
	cfg = cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &PluginTransport{
		locators:       cfg.Locators,
		configID:       cfg.ConfigID,
		callbacks:      make(map[string]chan *Message),
		client:         NewClient(cfg),
		requestTimeout: cfg.RequestTimeout,
		callbackBuffer: cfg.CallbackBuffer,
	}, nil
}

// Connect 建立WebSocket连接
func (t *PluginTransport) Connect(ctx context.Context) error {
	if t.client == nil {
		return errors.New("plugin: client not initialized")
	}
	err := t.client.Connect(ctx, t.handleMessage)
	if err != nil {
		return err
	}
	t.wsConn = t.client.Conn()
	return nil
}

// Close 断开连接
func (t *PluginTransport) Close() error {
	if t.client == nil {
		return nil
	}
	return t.client.Close()
}

// RoundTrip 实现 ProviderTransportSpec 接口
func (t *PluginTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("plugin: request is nil")
	}
	if err := t.Connect(req.Context()); err != nil {
		return nil, err
	}

	body, err := readRequestBody(req)
	if err != nil {
		return nil, err
	}
	text, sessionID, stream, startNewChat, err := extractRequest(body)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, errors.New("plugin: empty message")
	}
	if t.configID == "" {
		return nil, errors.New("plugin: missing configID")
	}

	msgID := uuid.NewString()
	payload := sendMessagePayload{
		ConfigID:     t.configID,
		SessionID:    sessionID,
		Text:         text,
		Locators:     t.locators,
		StartNewChat: startNewChat,
		Stream:       stream,
		PlatformURL:  "",
	}
	if t.locators != nil {
		payload.PlatformURL = t.locators.PlatformURL
	}
	msg := &Message{
		ID:        msgID,
		Type:      MsgSendMessage,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	}

	if stream {
		return t.roundTripStream(req, msgID, msg)
	}

	ch := make(chan *Message, t.callbackBuffer)
	t.addCallback(msgID, ch)
	defer t.removeCallback(msgID, ch)

	if err := t.client.Send(msg); err != nil {
		return nil, err
	}

	ctx := req.Context()
	if _, ok := ctx.Deadline(); !ok && t.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.requestTimeout)
		defer cancel()
	}

	replyText, duration, err := t.waitForReply(ctx, ch)
	if err != nil {
		return nil, err
	}
	return buildResponse(req, replyText, duration)
}

type streamEvent struct {
	Text      string `json:"text,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Done      bool   `json:"done,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (t *PluginTransport) roundTripStream(req *http.Request, msgID string, msg *Message) (*http.Response, error) {
	ch := make(chan *Message, t.callbackBuffer)
	t.addCallback(msgID, ch)

	if err := t.client.Send(msg); err != nil {
		t.removeCallback(msgID, ch)
		return nil, err
	}

	ctx := req.Context()
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok && t.requestTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, t.requestTimeout)
	}

	pr, pw := io.Pipe()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       pr,
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/x-ndjson")

	go t.streamToPipe(ctx, cancel, msgID, ch, pw)

	return resp, nil
}

func (t *PluginTransport) streamToPipe(
	ctx context.Context,
	cancel context.CancelFunc,
	msgID string,
	ch chan *Message,
	pw *io.PipeWriter,
) {
	defer func() {
		if cancel != nil {
			cancel()
		}
		t.removeCallback(msgID, ch)
		_ = pw.Close()
	}()

	writeEvent := func(ev streamEvent) bool {
		data, err := json.Marshal(ev)
		if err != nil {
			return false
		}
		data = append(data, '\n')
		if _, err := pw.Write(data); err != nil {
			return false
		}
		return true
	}

	var sentText string

	for {
		select {
		case <-ctx.Done():
			_ = writeEvent(streamEvent{Error: ctx.Err().Error(), Done: true})
			return
		case msg, ok := <-ch:
			if !ok {
				_ = writeEvent(streamEvent{Error: "plugin: connection closed", Done: true})
				return
			}
			switch msg.Type {
			case MsgReplyChunk:
				var payload replyChunkPayload
				if err := decodePayload(msg.Payload, &payload); err != nil {
					_ = writeEvent(streamEvent{Error: err.Error(), Done: true})
					return
				}
				text := payload.Chunk
				if text == "" && payload.FullText != "" {
					if strings.HasPrefix(payload.FullText, sentText) {
						text = payload.FullText[len(sentText):]
					} else {
						text = payload.FullText
					}
				}
				if text != "" {
					if !writeEvent(streamEvent{Text: text}) {
						return
					}
					sentText += text
				}
			case MsgReplyDone:
				var payload replyDonePayload
				if err := decodePayload(msg.Payload, &payload); err != nil {
					_ = writeEvent(streamEvent{Error: err.Error(), Done: true})
					return
				}
				if payload.FullText != "" {
					if strings.HasPrefix(payload.FullText, sentText) {
						diff := payload.FullText[len(sentText):]
						if diff != "" {
							_ = writeEvent(streamEvent{Text: diff})
						}
					} else {
						_ = writeEvent(streamEvent{Text: payload.FullText})
					}
					sentText = payload.FullText
				}
				_ = writeEvent(streamEvent{Done: true})
				return
			case MsgError:
				var payload errorPayload
				if err := decodePayload(msg.Payload, &payload); err != nil {
					_ = writeEvent(streamEvent{Error: err.Error(), Done: true})
					return
				}
				msgText := payload.Error
				if msgText == "" {
					msgText = payload.Message
				}
				if msgText == "" {
					msgText = "plugin: unknown error"
				}
				_ = writeEvent(streamEvent{Error: msgText, Done: true})
				return
			}
		}
	}
}

func (t *PluginTransport) handleMessage(msg *Message) {
	if msg == nil || msg.ID == "" {
		return
	}
	ch := t.getCallback(msg.ID)
	if ch == nil {
		return
	}
	safeSend(ch, msg)
}

func (t *PluginTransport) addCallback(id string, ch chan *Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callbacks[id] = ch
}

func (t *PluginTransport) removeCallback(id string, ch chan *Message) {
	t.mu.Lock()
	delete(t.callbacks, id)
	t.mu.Unlock()
	close(ch)
}

func (t *PluginTransport) getCallback(id string) chan *Message {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.callbacks[id]
}

func safeSend(ch chan *Message, msg *Message) {
	defer func() {
		_ = recover()
	}()
	ch <- msg
}

type sendMessagePayload struct {
	ConfigID     string           `json:"configId"`
	SessionID    string           `json:"sessionId,omitempty"`
	Text         string           `json:"text"`
	Locators     *ElementLocators `json:"locators,omitempty"`
	StartNewChat bool             `json:"startNewChat,omitempty"`
	Stream       bool             `json:"stream,omitempty"`
	PlatformURL  string           `json:"platformUrl,omitempty"`
}

type replyChunkPayload struct {
	Chunk    string `json:"chunk"`
	FullText string `json:"fullText"`
}

type replyDonePayload struct {
	FullText string `json:"fullText"`
	Duration int64  `json:"duration"`
}

type errorPayload struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type responsePayload struct {
	Text     string `json:"text"`
	Duration int64  `json:"duration,omitempty"`
}

func (t *PluginTransport) waitForReply(ctx context.Context, ch <-chan *Message) (string, int64, error) {
	var fullText string
	var duration int64
	for {
		select {
		case <-ctx.Done():
			return fullText, duration, ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return fullText, duration, errors.New("plugin: connection closed")
			}
			switch msg.Type {
			case MsgReplyChunk:
				var payload replyChunkPayload
				if err := decodePayload(msg.Payload, &payload); err != nil {
					return fullText, duration, err
				}
				if payload.FullText != "" {
					fullText = payload.FullText
				} else {
					fullText += payload.Chunk
				}
			case MsgReplyDone:
				var payload replyDonePayload
				if err := decodePayload(msg.Payload, &payload); err != nil {
					return fullText, duration, err
				}
				if payload.FullText != "" {
					fullText = payload.FullText
				}
				duration = payload.Duration
				return fullText, duration, nil
			case MsgError:
				var payload errorPayload
				if err := decodePayload(msg.Payload, &payload); err != nil {
					return fullText, duration, err
				}
				if payload.Error != "" {
					return fullText, duration, errors.New(payload.Error)
				}
				if payload.Message != "" {
					return fullText, duration, errors.New(payload.Message)
				}
				return fullText, duration, errors.New("plugin: unknown error")
			}
		}
	}
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, errors.New("plugin: request body is empty")
	}
	defer func() { _ = req.Body.Close() }()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

type requestPayload struct {
	Messages     []requestMessage `json:"messages"`
	Prompt       string           `json:"prompt"`
	Text         string           `json:"text"`
	Input        string           `json:"input"`
	Stream       bool             `json:"stream"`
	SessionID    string           `json:"session_id"`
	SessionAlt   string           `json:"sessionId"`
	StartNewChat bool             `json:"startNewChat"`
}

type requestMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func extractRequest(body []byte) (string, string, bool, bool, error) {
	var payload requestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", false, false, err
	}
	text := extractText(payload.Messages)
	if text == "" {
		if payload.Text != "" {
			text = payload.Text
		} else if payload.Input != "" {
			text = payload.Input
		} else if payload.Prompt != "" {
			text = payload.Prompt
		}
	}
	sessionID := payload.SessionID
	if sessionID == "" {
		sessionID = payload.SessionAlt
	}
	return text, sessionID, payload.Stream, payload.StartNewChat, nil
}

func extractText(messages []requestMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" || messages[i].Role == "" {
			if text := coerceToString(messages[i].Content); text != "" {
				return text
			}
		}
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if text := coerceToString(messages[i].Content); text != "" {
			return text
		}
	}
	return ""
}

func coerceToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []interface{}:
		var buf bytes.Buffer
		for _, item := range v {
			switch part := item.(type) {
			case string:
				buf.WriteString(part)
			case map[string]interface{}:
				if text, ok := part["text"].(string); ok {
					buf.WriteString(text)
				}
			}
		}
		return buf.String()
	case map[string]interface{}:
		if text, ok := v["text"].(string); ok {
			return text
		}
	}
	return ""
}

func decodePayload(payload interface{}, out interface{}) error {
	switch v := payload.(type) {
	case json.RawMessage:
		return json.Unmarshal(v, out)
	case []byte:
		return json.Unmarshal(v, out)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, out)
	}
}

func buildResponse(req *http.Request, text string, duration int64) (*http.Response, error) {
	payload := responsePayload{Text: text, Duration: duration}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	status := http.StatusOK
	resp := &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		Request:       req,
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}
