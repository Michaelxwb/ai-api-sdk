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

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 演示如何使用 SQLite 存储实现多轮对话
func main() {
	// 1. 创建 SQLite 存储
	store, err := sessionstore.NewSQLiteStore("examples/sessions.db")
	if err != nil {
		log.Fatalf("Failed to create SQLite store: %v", err)
	}
	defer store.Close()

	fmt.Println("✓ SQLite store created at examples/sessions.db")

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

	// 4. 创建客户端并注入 SQLite 存储
	cli := client.NewClient(cfg, mgr)
	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate: true, // 如果 session 不存在，自动创建
	}

	fmt.Println("✓ Client configured with SQLite store")

	// === 场景 1：新建会话，多轮对话 ===
	fmt.Println("\n=== Scenario 1: Multi-turn conversation ===")

	sessionID := "demo-session-001"

	// 第一轮对话（增加超时时间到 300 秒 = 5 分钟）
	ctx1, cancel1 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel1()

	resp1, err := cli.ChatSession(ctx1, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "你好，介绍一下 Go 语言"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Turn 1: %s\n", resp1.Text[:min(100, len(resp1.Text))]+"...")

	// 延迟避免触发 API 速率限制
	time.Sleep(2 * time.Second)

	// 第二轮对话（自动携带历史，可能需要更长时间）
	ctx2, cancel2 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel2()

	resp2, err := cli.ChatSession(ctx2, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "它的并发模型是什么？"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Turn 2: %s\n", resp2.Text[:min(100, len(resp2.Text))]+"...")

	// 延迟避免触发 API 速率限制
	time.Sleep(2 * time.Second)

	// 第三轮对话（携带更多历史）
	ctx3, cancel3 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel3()

	resp3, err := cli.ChatSession(ctx3, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "给个代码示例"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Turn 3: %s\n", resp3.Text[:min(100, len(resp3.Text))]+"...")

	// 延迟避免触发 API 速率限制
	time.Sleep(2 * time.Second)

	// === 场景 2：恢复会话 ===
	fmt.Println("\n=== Scenario 2: Resume session from database ===")

	// 模拟程序重启后，从数据库恢复会话
	ctx4, cancel4 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel4()

	messages, err := store.GetMessages(ctx4, sessionID, session.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get messages: %v", err)
	}

	fmt.Printf("✓ Loaded %d messages from session '%s'\n", len(messages), sessionID)
	for i, msg := range messages {
		preview := msg.Content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		fmt.Printf("  [%d] %s: %s\n", i+1, msg.Role, preview)
	}

	// 延迟避免触发 API 速率限制
	time.Sleep(2 * time.Second)

	// 继续对话（恢复会话后）
	ctx5, cancel5 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel5()

	resp4, err := cli.ChatSession(ctx5, "vllm_local", sessionID, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "总结一下我们刚才讨论的内容"}},
	})
	if err != nil {
		log.Fatalf("ChatSession error: %v", err)
	}
	fmt.Printf("Turn 4: %s\n", resp4.Text[:min(100, len(resp4.Text))]+"...")

	// === 场景 3：会话管理 ===
	fmt.Println("\n=== Scenario 3: Session management ===")

	ctx6, cancel6 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel6()

	// 列出所有会话
	sessions, err := store.ListSessions(ctx6, "")
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}
	fmt.Printf("✓ Found %d sessions in database\n", len(sessions))

	// 获取会话元数据
	meta, err := store.GetMeta(ctx6, sessionID)
	if err != nil {
		log.Fatalf("Failed to get meta: %v", err)
	}
	fmt.Printf("✓ Session metadata:\n")
	fmt.Printf("  ID: %s\n", meta.ID)
	fmt.Printf("  Provider: %s\n", meta.Provider)
	fmt.Printf("  Model: %s\n", meta.Model)
	fmt.Printf("  Created: %s\n", meta.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated: %s\n", meta.UpdatedAt.Format(time.RFC3339))

	// === 场景 4：清理旧会话 ===
	fmt.Println("\n=== Scenario 4: Cleanup old sessions ===")

	ctx7, cancel7 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel7()

	// 清理 7 天前的会话
	deleted, err := store.CleanupOldSessions(ctx7, 7*24*time.Hour)
	if err != nil {
		log.Fatalf("Failed to cleanup: %v", err)
	}
	fmt.Printf("✓ Deleted %d old sessions\n", deleted)

	// 延迟避免触发 API 速率限制
	time.Sleep(2 * time.Second)

	// === 场景 5：创建多个独立会话 ===
	fmt.Println("\n=== Scenario 5: Multiple independent sessions ===")

	session1 := "user-alice-001"
	session2 := "user-bob-002"

	ctx8, cancel8 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel8()

	// Alice 的对话
	_, err = cli.ChatSession(ctx8, "vllm_local", session1, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "我想学习 Python"}},
	})
	if err == nil {
		fmt.Printf("✓ Session '%s' created\n", session1)
	}

	// 延迟避免触发 API 速率限制
	time.Sleep(2 * time.Second)

	ctx9, cancel9 := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel9()

	// Bob 的对话
	_, err = cli.ChatSession(ctx9, "vllm_local", session2, provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "我想学习 Rust"}},
	})
	if err == nil {
		fmt.Printf("✓ Session '%s' created\n", session2)
	}

	ctx10, cancel10 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel10()

	// 列出所有会话
	allSessions, _ := store.ListSessions(ctx10, "")
	fmt.Printf("✓ Total sessions: %d\n", len(allSessions))

	fmt.Println("\n=== All scenarios completed ===")
	fmt.Println("Database file: examples/sessions.db")
	fmt.Println("You can inspect it with: sqlite3 sessions.db")
}
