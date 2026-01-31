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

// 演示如何使用 PostgreSQL 存储实现多轮对话
func main() {
	// 1. 创建 PostgreSQL 存储
	// 连接字符串格式：postgres://user:password@host:port/dbname?sslmode=disable
	connStr := "postgres://postgres:secret@localhost:5432/sessions?sslmode=disable"
	store, err := sessionstore.NewPostgresStore(connStr)
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL store: %v", err)
	}
	defer store.Close()

	fmt.Println("✓ PostgreSQL store created")

	// 2. 加载配置
	cfg, err := config.LoadConfig("examples/config.example.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 3. 初始化认证管理器
	authStore := auth.NewFileStore(cfg.Auth.Store.Path)
	mgr, err := auth.NewManager(authStore, &auth.RoundRobinSelector{})
	if err != nil {
		log.Fatalf("Failed to create auth manager: %v", err)
	}

	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	// 4. 创建客户端并注入 PostgreSQL 存储
	cli := client.NewClient(cfg, mgr)
	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate: true,
		TruncatePolicy: session.WindowPolicy{
			MaxMessages:      20,
			KeepSystemPrompt: true,
		},
	}

	fmt.Println("✓ Client configured with PostgreSQL store")

	// === 场景 1：多轮对话 ===
	fmt.Println("\n=== Scenario 1: Multi-turn conversation ===")

	sessionID := "postgres-demo-001"

	// 第一轮对话
	ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel1()

	resp1, err := cli.ChatSession(ctx1, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "介绍一下 PostgreSQL"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Turn 1: %s\n", truncate(resp1.Text, 100))

	// 延迟避免速率限制
	time.Sleep(2 * time.Second)

	// 第二轮对话（自动携带历史）
	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel2()

	resp2, err := cli.ChatSession(ctx2, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "它和 MySQL 的主要区别是什么？"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Turn 2: %s\n", truncate(resp2.Text, 100))

	// === 场景 2：会话元数据 ===
	fmt.Println("\n=== Scenario 2: Session metadata ===")

	ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel3()

	meta := &session.SessionMeta{
		ID:       sessionID,
		Provider: "vllm_local",
		Model:    "minimaxai/minimax-m2.1",
		Attrs: map[string]any{
			"user_id":    "user-123",
			"department": "engineering",
			"topic":      "database-comparison",
		},
	}

	if err := store.UpsertMeta(ctx3, sessionID, meta); err != nil {
		log.Fatalf("Failed to upsert meta: %v", err)
	}
	fmt.Println("✓ Metadata saved")

	// 读取元数据
	savedMeta, err := store.GetMeta(ctx3, sessionID)
	if err != nil {
		log.Fatalf("Failed to get meta: %v", err)
	}
	fmt.Printf("✓ User ID: %v, Topic: %v\n", savedMeta.Attrs["user_id"], savedMeta.Attrs["topic"])

	// === 场景 3：会话管理 ===
	fmt.Println("\n=== Scenario 3: Session management ===")

	ctx4, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel4()

	// 列出所有会话
	sessions, err := store.ListSessions(ctx4, "")
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}
	fmt.Printf("✓ Found %d sessions in database\n", len(sessions))

	// 查看历史消息
	messages, err := store.GetMessages(ctx4, sessionID, session.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}
	fmt.Printf("✓ Session has %d messages\n", len(messages))

	// === 场景 4：清理旧会话 ===
	fmt.Println("\n=== Scenario 4: Cleanup old sessions ===")

	ctx5, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel5()

	// 清理 7 天前的会话
	deleted, err := store.CleanupOldSessions(ctx5, 7*24*time.Hour)
	if err != nil {
		log.Fatalf("Failed to cleanup: %v", err)
	}
	fmt.Printf("✓ Deleted %d old sessions\n", deleted)

	fmt.Println("\n=== All scenarios completed ===")
	fmt.Println("PostgreSQL database: sessions")
	fmt.Println("Check with: psql -U postgres -d sessions -c 'SELECT * FROM sessions;'")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
