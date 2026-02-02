// Package main 演示 Dify 平台的基础非流式对话。
//
// 功能特性：
//   - 支持环境变量或 YAML 读取 API Key
//   - Dify Chat Messages API 集成
//   - 支持多轮对话（conversation_id）
//
// 前置条件：
//   - 设置环境变量 DIFY_API_KEY
//   - 或在 examples/dify/config.yaml 中配置凭证
//
// 使用方法：
//
//	go run examples/dify/basic/main.go
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

	// 注册所有 Provider（包括 dify）。
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	apiKey := os.Getenv("DIFY_API_KEY")
	if apiKey == "" {
		cfg, err := config.LoadConfig("examples/dify/config.yaml")
		if err == nil && len(cfg.Credentials) > 0 {
			apiKey = cfg.Credentials[0].APIKey
		}
	}

	if apiKey == "" {
		log.Fatal("请设置 DIFY_API_KEY 环境变量或配置 examples/dify/config.yaml")
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

	fmt.Println("=== Dify 非流式对话 ===")
	resp, err := cli.Chat(context.Background(), "dify", base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "你好，请介绍一下 Dify"}},
	})
	if err != nil {
		log.Fatalf("请求失败: %v", err)
	}

	fmt.Printf("响应: %s\n", resp.Text)
	if resp.SessionID != "" {
		fmt.Printf("会话 ID: %s\n", resp.SessionID)
	}
	if resp.Usage != nil {
		fmt.Printf("Token 使用: %d (输入: %d, 输出: %d)\n",
			resp.Usage.TotalTokens,
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
		)
	}
}
