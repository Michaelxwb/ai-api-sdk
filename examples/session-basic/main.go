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
	"github.com/Michaelxwb/ai-api-sdk/provider"
	"github.com/Michaelxwb/ai-api-sdk/session"
)

// 演示基础多轮对话：使用内存存储
func main() {
	cfg, err := config.LoadConfig("examples/config.example.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	authStore := auth.NewFileStore(cfg.Auth.Store.Path)
	mgr, err := auth.NewManager(authStore, &auth.RoundRobinSelector{})
	if err != nil {
		log.Fatalf("Failed to create auth manager: %v", err)
	}
	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	cli := client.NewClient(cfg, mgr)
	cli.SessionStore = sessionstore.NewMemoryStore()
	cli.SessionConfig = client.SessionConfig{
		AutoCreate:     true,
		TruncatePolicy: session.WindowPolicy{MaxMessages: 20, KeepSystemPrompt: true},
	}

	sessionID := "basic-session-001"

	// 第一轮对话
	fmt.Println("=== Turn 1 ===")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel1()

	resp1, err := cli.ChatSession(ctx1, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "你好，介绍一下 Go 语言"}},
	})
	if err != nil {
		log.Fatalf("Turn 1 error: %v", err)
	}
	fmt.Printf("Response: %s\n\n", resp1.Text)

	// 第二轮对话（自动携带第一轮的历史）
	fmt.Println("=== Turn 2 ===")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel2()

	resp2, err := cli.ChatSession(ctx2, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "它的并发模型是什么？"}},
	})
	if err != nil {
		log.Fatalf("Turn 2 error: %v", err)
	}
	fmt.Printf("Response: %s\n", resp2.Text)

	// 第二轮对话（自动携带第一轮的历史）
	fmt.Println("=== Turn 3 ===")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel3()

	resp3, err := cli.ChatSession(ctx3, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "我问的第一个问题是什么，请原封不动的复述一下"}},
	})
	if err != nil {
		log.Fatalf("Turn 3 error: %v", err)
	}
	fmt.Printf("Response: %s\n", resp3.Text)
	//fmt.Println("\n✅ 多轮对话完成！第二轮自动携带了第一轮的上下文。")
}
