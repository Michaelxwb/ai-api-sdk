// Package main 演示 Dify 平台的流式对话。
//
// 前置条件：
//   - 设置环境变量 DIFY_API_KEY
//
// 使用方法：
//
//	go run examples/dify/streaming/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	apiKey := os.Getenv("DIFY_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置 DIFY_API_KEY 环境变量")
	}

	cfg := &config.Config{
		Providers: []config.ProviderConfig{
			{
				Name:    "dify",
				Type:    "dify",
				BaseURL: "https://api.dify.ai/v1",
				AuthRef: "dify_cred",
			},
		},
		Credentials: []*auth.Credential{
			{
				ID:       "dify_cred",
				Provider: "dify",
				AuthType: auth.AuthTypeAPIKey,
				APIKey:   apiKey,
			},
		},
	}

	mgr, err := auth.NewManager(nil, &auth.RoundRobinSelector{})
	if err != nil {
		log.Fatalf("创建认证管理器失败: %v", err)
	}
	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	cli := client.NewClient(cfg, mgr)

	fmt.Println("=== Dify 流式对话 ===")
	fmt.Print("响应: ")

	stream, err := cli.ChatStream(context.Background(), "dify", base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "用三句话介绍 Dify"}},
	})
	if err != nil {
		log.Fatalf("流式请求失败: %v", err)
	}

	var sessionID string
	var usage *base.Usage
	for chunk := range stream {
		if chunk.Error != nil {
			log.Printf("\n流式错误: %v", chunk.Error)
			break
		}
		if chunk.SessionID != "" {
			sessionID = chunk.SessionID
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		fmt.Print(chunk.Text)
	}
	fmt.Println()

	if sessionID != "" {
		fmt.Printf("会话 ID: %s\n", sessionID)
	}
	if usage != nil {
		fmt.Printf("Token 使用: %d (输入: %d, 输出: %d)\n",
			usage.TotalTokens,
			usage.PromptTokens,
			usage.CompletionTokens,
		)
	}
}
