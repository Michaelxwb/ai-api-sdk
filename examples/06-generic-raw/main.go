package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
	"github.com/Michaelxwb/ai-api-sdk/session"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

// debugTransport prints the request body before forwarding.
type debugTransport struct {
	base http.RoundTripper
}

func (d *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		fmt.Printf("[DEBUG] 请求体: %s\n", string(body))
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return d.base.RoundTrip(req)
}

// memStore 是一个简单的内存 SessionStore，仅供 example 演示多轮对话使用。
type memStore struct {
	mu   sync.Mutex
	data map[string]*session.SessionState
}

func newMemStore() *memStore { return &memStore{data: make(map[string]*session.SessionState)} }

func (m *memStore) Get(_ context.Context, id string) (*session.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.data[id]; ok {
		return s, nil
	}
	return nil, session.ErrSessionNotFound
}

func (m *memStore) Save(_ context.Context, state *session.SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[state.ID] = state
	return nil
}

func (m *memStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
}

func main() {
	fmt.Println("=== Generic Raw API - 从 request_json.json 读取配置（支持 remote_session / local_history）===")
	fmt.Println()

	c, configs := setup()
	ctx := context.Background()

	//for i, spec := range configs["remote_session"] {
	//	fmt.Printf("\n【remote_session[%d]】model=%s\n", i, spec.Model)
	//	demoRemoteSession(ctx, c, spec)
	//}
	for i, spec := range configs["local_history"] {
		fmt.Printf("\n【local_history[%d]】model=%s\n", i, spec.Model)
		demoLocalHistory(ctx, c, spec)
	}
}

// setup 读取配置文件并初始化 HTTP 客户端。
func setup() (*client.Client, map[string][]generic.RawHTTPSpec) {
	_, srcFile, _, _ := runtime.Caller(0)
	jsonPath := filepath.Join(filepath.Dir(srcFile), "request_json.json")

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	var configs map[string][]generic.RawHTTPSpec
	if err := json.Unmarshal(data, &configs); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	c := client.New()
	innerTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
	c.HTTP = &http.Client{
		Timeout:   120 * time.Second,
		Transport: &debugTransport{base: innerTransport},
	}

	return c, configs
}

// demoRemoteSession 演示 remote_session 模式（服务端维护会话，多轮对话）。
func demoRemoteSession(ctx context.Context, c *client.Client, spec generic.RawHTTPSpec) {
	fmt.Println("=== remote_session 模式 ===")

	sess, err := c.NewSessionFromHTTPSpec(spec)
	if err != nil {
		log.Printf("创建 remote_session Session 失败: %v", err)
		return
	}

	fmt.Println("\n── 第一轮 ──")
	fmt.Printf("发送前 session ID: %q（空 = 首轮，服务端创建新会话）\n", sess.ID())
	answer1, err := chatStream(ctx, sess, "你好，请用20个字简单介绍一下你自己")
	if err != nil {
		log.Printf("第一轮请求失败: %v", err)
		return
	}
	fmt.Printf("\n服务端返回 conversation_id: %q\n", sess.ID())
	_ = answer1

	fmt.Println("\n── 第二轮 ──")
	fmt.Printf("发送前 session ID: %q（注入 conversation_id + parent_message_id）\n", sess.ID())
	answer2, err := chatStream(ctx, sess, "你刚才说了什么？")
	if err != nil {
		log.Printf("第二轮请求失败: %v", err)
		return
	}
	_ = answer2
}

// demoLocalHistory 演示 local_history 模式（本地维护历史，多轮对话）。
// SDK 在每轮请求前自动从 memStore 加载历史消息并拼入 messages，无需业务层手动管理。
func demoLocalHistory(ctx context.Context, c *client.Client, spec generic.RawHTTPSpec) {
	fmt.Println("\n=== local_history 模式 ===")

	store := newMemStore()
	sess, err := c.NewSessionFromHTTPSpec(spec,
		client.WithStore(store),
		client.WithAutoID(),
	)
	if err != nil {
		log.Printf("创建 local_history Session 失败: %v", err)
		return
	}
	fmt.Printf("本地 session ID: %q\n", sess.ID())

	fmt.Println("\n── 第一轮 ──")
	answer1, err := chatStream(ctx, sess, "请用20个字介绍一下Go语言")
	if err != nil {
		log.Printf("第一轮请求失败: %v", err)
		return
	}
	fmt.Println()
	_ = answer1

	fmt.Println("\n── 第二轮（基于上文继续追问）──")
	answer2, err := chatStream(ctx, sess, "你刚才说的特点中，最重要的是哪一个？请用20个字回答总结一下")
	if err != nil {
		log.Printf("第二轮请求失败: %v", err)
		return
	}
	fmt.Println()
	_ = answer2

	fmt.Println("\n── 第三轮 ──")
	answer3, err := chatStream(ctx, sess, "我问的第一个问题是什么？")
	if err != nil {
		log.Printf("第三轮请求失败: %v", err)
		return
	}
	fmt.Println()
	_ = answer3
}

// chatStream 发送流式请求并打印 delta，返回完整回答
func chatStream(ctx context.Context, sess *client.Session, question string) (string, error) {
	fmt.Printf("问：%s\n答：", question)

	stream, err := sess.ChatStream(ctx, base.ChatRequest{
		Messages: []base.Message{
			{Role: "user", Content: question},
		},
	})
	if err != nil {
		return "", fmt.Errorf("发起流式请求失败: %w", err)
	}

	var full string
	for chunk := range stream {
		if chunk.Error != nil {
			return full, fmt.Errorf("流式接收错误: %w", chunk.Error)
		}
		if chunk.Text != "" {
			fmt.Print(chunk.Text)
			full += chunk.Text
		}
	}
	return full, nil
}
