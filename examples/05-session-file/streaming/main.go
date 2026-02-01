// Package main 演示使用 File 会话存储的流式多轮对话。
//
// 功能特性：
//   - 基于 YAML 的配置和凭证管理
//   - 通过 ChatSessionStream 进行流式会话对话
//   - 打字机效果输出
//
// 前置条件：
//   - 无。若 JSON 文件不存在将自动创建。
//   - 在 examples/config.example.yaml 中配置有效凭证
//
// 使用方法：
//
//	go run streaming.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
	"github.com/Michaelxwb/ai-api-sdk/session"

	// 注册所有 Provider（触发 init 副作用）。
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	// 步骤 1: 加载 YAML 配置文件（providers、认证存储、凭证）
	cfg, err := config.LoadConfig("examples/config.example.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 步骤 2: 创建认证管理器并注册凭证
	authStore := auth.NewFileStore(cfg.Auth.Store.Path)
	applyAuthStoreConfig(authStore, cfg)

	mgr, err := auth.NewManager(authStore, &auth.RoundRobinSelector{})
	if err != nil {
		log.Fatalf("Failed to create auth manager: %v", err)
	}
	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	// 步骤 3: 创建客户端和会话存储
	cli := client.NewClient(cfg, mgr)
	store, err := sessionstore.NewFileStore("examples/sessions.json")
	if err != nil {
		log.Fatalf("Failed to create file store: %v", err)
	}

	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate:     true,
		TruncatePolicy: session.WindowPolicy{MaxMessages: 20, KeepSystemPrompt: true},
	}

	// 步骤 4: 执行三轮对话会话并输出流式结果
	sessionID := "file-session-003"
	providerName := "vllm_local"

	fmt.Println("=== Turn 1 ===")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel1()
	stream1, err := cli.ChatSessionStream(ctx1, providerName, sessionID, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "用一段话介绍“Rust”语言。"}},
	})
	if err != nil {
		log.Fatalf("Turn 1 error: %v", err)
	}
	if err := printStream(stream1); err != nil {
		log.Fatalf("Turn 1 stream failed: %v", err)
	}
	time.Sleep(30 * time.Second)

	fmt.Println("=== Turn 2 ===")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()
	stream2, err := cli.ChatSessionStream(ctx2, providerName, sessionID, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "它的并发模型是什么？"}},
	})
	if err != nil {
		log.Fatalf("Turn 2 error: %v", err)
	}
	if err := printStream(stream2); err != nil {
		log.Fatalf("Turn 2 stream failed: %v", err)
	}
	time.Sleep(30 * time.Second)

	fmt.Println("=== Turn 3 ===")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel3()
	stream3, err := cli.ChatSessionStream(ctx3, providerName, sessionID, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "请准确地重复我的第一个问题。"}},
	})
	if err != nil {
		log.Fatalf("Turn 3 error: %v", err)
	}
	if err := printStream(stream3); err != nil {
		log.Fatalf("Turn 3 stream failed: %v", err)
	}
}

func printStream(stream <-chan streaming.StreamChunk) error {
	for chunk := range stream {
		if chunk.Error != nil {
			return chunk.Error
		}
		for _, r := range chunk.Text {
			fmt.Printf("%c", r)
			time.Sleep(10 * time.Millisecond)
		}
		if chunk.Done {
			fmt.Println()
		}
	}
	return nil
}

func applyAuthStoreConfig(store *auth.FileStore, cfg *config.Config) {
	store.Encrypted = cfg.Auth.Store.Encryption.Enabled || cfg.Auth.Store.Encrypted
	store.MasterKeyEnv = cfg.Auth.Store.Encryption.MasterKeyEnv
	store.MasterKeyFile = cfg.Auth.Store.Encryption.MasterKeyFile
	if cfg.Auth.Store.Encryption.KDFParams.N > 0 {
		store.ScryptParams.N = cfg.Auth.Store.Encryption.KDFParams.N
		store.ScryptParams.R = cfg.Auth.Store.Encryption.KDFParams.R
		store.ScryptParams.P = cfg.Auth.Store.Encryption.KDFParams.P
		store.ScryptParams.KeyLen = cfg.Auth.Store.Encryption.KDFParams.KeyLen
	}
}
