// Package main 演示使用 MySQL 会话存储的程序化认证多轮对话。
//
// 功能特性：
//   - 手动 Credential + ProviderConfig (no YAML)
//   - 通过 ChatSessionStreamSync 进行非流式会话对话
//   - 在 MySQL 存储中自动持久化历史记录
//
// 前置条件：
//   - 确保 MySQL 正在运行并更新 DSN。
//   - 在代码中更新 BaseURL 和 AccessToken
//
// 使用方法：
//
//	go run programmatic.go
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
	"github.com/Michaelxwb/ai-api-sdk/session"

	// 注册所有 Provider（触发 init 副作用）。
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	// 步骤 1: 在代码中创建凭证
	cred := &auth.Credential{
		ID:          "local-token",
		Provider:    "openai_compat",
		AuthType:    auth.AuthTypeBearerToken,
		AccessToken: "REPLACE_ME",
	}

	// 步骤 2: 在代码中构建 Provider 配置和 SDK 配置
	pc := config.ProviderConfig{
		Name:    "vllm_local",
		Type:    "openai_compat",
		BaseURL: "http://127.0.0.1:8000/v1",
		AuthRef: cred.ID,
	}
	cfg := &config.Config{Providers: []config.ProviderConfig{pc}}

	mgr, err := auth.NewManager(nil, &auth.RoundRobinSelector{})
	if err != nil {
		log.Fatalf("Failed to create auth manager: %v", err)
	}
	mgr.Register(cred)

	// 步骤 3: 创建客户端和会话存储
	cli := client.NewClient(cfg, mgr)
	dsn := "user:pass@tcp(localhost:3306)/sessions?parseTime=true"
	store, err := sessionstore.NewMySQLStore(dsn)
	if err != nil {
		log.Fatalf("Failed to create MySQL store: %v", err)
	}
	defer func() { _ = store.Close() }()
	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate:     true,
		TruncatePolicy: session.WindowPolicy{MaxMessages: 20, KeepSystemPrompt: true},
	}

	// 步骤 4: 执行三轮对话会话以演示记忆
	sessionID := "mysql-session-001"
	providerName := "vllm_local"

	fmt.Println("=== Turn 1 ===")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel1()
	resp1, err := cli.ChatSessionStreamSync(ctx1, providerName, sessionID, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "Introduce Go in one paragraph."}},
	})
	if err != nil {
		log.Fatalf("Turn 1 error: %v", err)
	}
	fmt.Printf("Response: %s\n\n", resp1.Text)

	fmt.Println("=== Turn 2 ===")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel2()
	resp2, err := cli.ChatSessionStreamSync(ctx2, providerName, sessionID, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "What is its concurrency model?"}},
	})
	if err != nil {
		log.Fatalf("Turn 2 error: %v", err)
	}
	fmt.Printf("Response: %s\n\n", resp2.Text)

	fmt.Println("=== Turn 3 ===")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel3()
	resp3, err := cli.ChatSessionStreamSync(ctx3, providerName, sessionID, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "Repeat my first question exactly."}},
	})
	if err != nil {
		log.Fatalf("Turn 3 error: %v", err)
	}
	fmt.Printf("Response: %s\n", resp3.Text)
}
