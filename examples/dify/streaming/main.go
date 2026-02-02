// Package main 演示使用 YAML 认证的流式对话（Dify）。
//
// 功能特性：
//   - 基于 YAML 的配置和凭证管理
//   - 通过 ChatStream 获取流式响应
//   - 打字机效果输出
//   - 多轮流式对话演示（使用 conversation_id）
//   - 显示 Token 使用统计
//   - 配置 SessionStore（会话管理）
//
// Dify 会话管理说明：
//   - Dify 的 conversation_id 由服务端生成和管理
//   - 首次对话不传 conversation_id（不设置 SessionID），从响应中获取
//   - 后续对话使用返回的 conversation_id 作为 SessionID
//   - 这是 Dify 的标准流程，与客户端生成 SessionID 的模式不同
//
// 前置条件：
//   - 在 examples/config.example.yaml 中配置有效的 Dify 凭证
//
// 使用方法：
//
//	go run main.go
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

	// 步骤 3: 创建客户端并配置会话存储
	cli := client.NewClient(cfg, mgr)
	store := sessionstore.NewMemoryStore()
	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate: true, // 自动创建会话
	}

	// 步骤 4: 第一轮流式对话
	fmt.Println("=== 第一轮流式对话 ===")
	fmt.Print("用户: 介绍一下 Dify\n助手: ")

	ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel1()

	stream1, err := cli.ChatStream(ctx1, "dify", base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "介绍一下 Dify"}},
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	conversationID, usage1, err := printStream(stream1)
	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}
	fmt.Println()
	if conversationID != "" {
		fmt.Printf("会话 ID: %s\n", conversationID)
	}
	printTokenUsage(usage1)
	fmt.Println()

	// 步骤 5: 如果获取到 conversation_id，进行第二轮对话
	if conversationID != "" {
		time.Sleep(2 * time.Second)

		fmt.Println("=== 第二轮流式对话（续接上下文） ===")
		fmt.Print("用户: 它的主要功能是什么？\n助手: ")

		ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel2()

		// 使用 ChatSessionStream 进行会话管理
		stream2, err := cli.ChatSessionStream(ctx2, "dify", conversationID, base.ChatRequest{
			Messages: []base.Message{{Role: "user", Content: "它的主要功能是什么？"}},
		})
		if err != nil {
			log.Fatalf("Stream error: %v", err)
		}

		_, usage2, err := printStream(stream2)
		if err != nil {
			log.Fatalf("Stream failed: %v", err)
		}
		fmt.Println()
		printTokenUsage(usage2)

		fmt.Println("\n=== 流式对话演示完成 ===")
		fmt.Printf("会话 ID: %s\n", conversationID)
		fmt.Println("多轮流式对话演示完成，上下文记忆功能正常工作。")
	}
}

func printStream(stream <-chan streaming.StreamChunk) (string, *base.Usage, error) {
	var conversationID string
	var usage *base.Usage

	for chunk := range stream {
		if chunk.Error != nil {
			return "", nil, chunk.Error
		}

		// 提取 conversation_id（只在第一次获取时记录）
		if conversationID == "" && chunk.SessionID != "" {
			conversationID = chunk.SessionID
		}

		// 提取 Token 使用信息
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		// 打字机效果输出文本
		for _, r := range chunk.Text {
			fmt.Printf("%c", r)
			time.Sleep(10 * time.Millisecond)
		}
	}

	return conversationID, usage, nil
}

func printTokenUsage(usage *base.Usage) {
	if usage != nil && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		fmt.Printf("Token 使用: 输入=%d, 输出=%d, 总计=%d\n",
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens)
	}
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
