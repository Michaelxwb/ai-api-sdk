package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var errNotConnected = errors.New("plugin: websocket not connected")

type wireMessage struct {
	ID        string          `json:"id"`
	Type      MessageType     `json:"type"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// Client WebSocket客户端
type Client struct {
	cfg     Config
	dialer  *websocket.Dialer
	conn    *websocket.Conn
	handler func(*Message)
	closed  chan struct{}

	mu      sync.RWMutex
	writeMu sync.Mutex
	closeMu sync.Once
}

// NewClient 创建客户端
func NewClient(cfg Config) *Client {
	cfg = cfg.normalize()
	dialer := &websocket.Dialer{HandshakeTimeout: cfg.DialTimeout}
	return &Client{cfg: cfg, dialer: dialer, closed: make(chan struct{})}
}

// Connect 建立连接
func (c *Client) Connect(ctx context.Context, handler func(*Message)) error {
	if err := c.cfg.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.conn != nil {
		if handler != nil {
			c.handler = handler
		}
		c.mu.Unlock()
		return nil
	}
	headers := http.Header{}
	for k, v := range c.cfg.Headers {
		headers.Set(k, v)
	}
	if c.cfg.AuthToken != "" && headers.Get("Authorization") == "" {
		headers.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	}
	conn, _, err := c.dialer.DialContext(ctx, c.cfg.Endpoint, headers)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	c.conn = conn
	if handler != nil {
		c.handler = handler
	}
	c.mu.Unlock()
	go c.readLoop()
	return nil
}

// Close 断开连接
func (c *Client) Close() error {
	var err error
	c.closeMu.Do(func() {
		c.mu.Lock()
		conn := c.conn
		c.conn = nil
		c.mu.Unlock()
		close(c.closed)
		if conn != nil {
			err = conn.Close()
		}
	})
	return err
}

// Send 发送消息
func (c *Client) Send(message *Message) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errNotConnected
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteJSON(message)
}

// Conn 返回底层连接
func (c *Client) Conn() *websocket.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

func (c *Client) readLoop() {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		_ = c.Close()
		return
	}
	for {
		var wire wireMessage
		if err := conn.ReadJSON(&wire); err != nil {
			_ = c.Close()
			return
		}
		msg := &Message{ID: wire.ID, Type: wire.Type, Timestamp: wire.Timestamp, Payload: wire.Payload}
		c.mu.RLock()
		handler := c.handler
		c.mu.RUnlock()
		if handler != nil {
			handler(msg)
		}
	}
}
