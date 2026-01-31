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

// 演示高级多轮对话：持久化、恢复、元数据管理
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

	store, err := sessionstore.NewFileStore("examples/sessions.json")
	if err != nil {
		log.Fatalf("Failed to create file store: %v", err)
	}

	cli := client.NewClient(cfg, mgr)
	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate:     true,
		TruncatePolicy: session.WindowPolicy{MaxMessages: 50, KeepSystemPrompt: true},
	}

	sessionID := "advanced-session-001"

	//fmt.Println("=== 演示多轮对话 + 持久化 ===")
	//ctx1, cancel1 := context.WithTimeout(context.Background(), 120*time.Second)
	//defer cancel1()
	//
	//resp, err := cli.ChatSession(ctx1, "vllm_local", sessionID, provider.ChatRequest{
	//	Model:    "minimaxai/minimax-m2.1",
	//	Messages: []provider.Message{{Role: "user", Content: "太阳系中有哪些行星，并详细描述"}},
	//})
	//if err != nil {
	//	log.Fatalf("ChatSession error: %v", err)
	//}
	//fmt.Printf("Response: %s\n\n", resp.Text)
	//
	//// 更新元数据
	//fmt.Println("=== 更新会话元数据 ===")
	//ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel2()
	//
	//if metaStore, ok := cli.SessionStore.(session.SessionStoreWithMeta); ok {
	//	meta := &session.SessionMeta{
	//		Attrs: map[string]any{"user_id": "user-123", "purpose": "demo"},
	//	}
	//	if err := metaStore.UpsertMeta(ctx2, sessionID, meta); err != nil {
	//		log.Fatalf("Failed to update meta: %v", err)
	//	}
	//}

	// 模拟重启：重新加载 store
	fmt.Println("\n=== 模拟进程重启，恢复会话 ===")
	reloaded, err := sessionstore.NewFileStore("examples/sessions.json")
	if err != nil {
		log.Fatalf("Failed to reload store: %v", err)
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()

	msgs, err := reloaded.GetMessages(ctx3, sessionID, session.GetOptions{MaxMessages: 10, KeepSystemPrompt: true})
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}
	fmt.Printf("✓ 从文件恢复了 %d 条消息\n", len(msgs))

	// FileStore implements SessionStoreWithMeta
	meta, err := reloaded.GetMeta(ctx3, sessionID)
	if err != nil {
		log.Fatalf("Failed to get meta: %v", err)
	}
	fmt.Printf("✓ 元数据: provider=%s model=%s attrs=%v\n", meta.Provider, meta.Model, meta.Attrs)

	fmt.Println("=== 演示多轮对话 + 持久化 + 重启后继续对话 ===")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel1()

	resp, err := cli.ChatSession(ctx1, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "总结一下我们前面讨论的内容"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Response: %s\n\n", resp.Text)

	//fmt.Println("\n✅ 高级特性演示完成！会话已持久化到 examples/sessions.json")
}
