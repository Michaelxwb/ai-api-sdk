// Package main 演示使用内存会话存储的 Dify 多轮对话功能。
//
// 功能特性：
//   - 基于 YAML 的配置和凭证管理
//   - 通过 ChatSessionStreamSync 进行非流式会话对话
//   - 在内存存储中自动管理对话历史
//   - 演示多轮对话的上下文记忆能力
//
// 前置条件：
//   - 在 examples/config.example.yaml 中配置有效的 Dify 凭证
//
// Dify 会话管理说明：
//   - Dify 的 conversation_id 由服务端生成和管理
//   - 首次对话时不传 conversation_id（不设置 SessionID），让 Dify 生成新的 UUID
//   - 从响应的 SessionID 字段获取 Dify 返回的 conversation_id
//   - 后续对话使用这个 conversation_id 来保持上下文
//   - 这是 Dify 的标准流程，与客户端生成 SessionID 的模式不同
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

	// 步骤 3: 创建客户端并配置会话存储
	cli := client.NewClient(cfg, mgr)

	// 使用内存存储来管理会话历史
	store := sessionstore.NewMemoryStore()
	cli.SessionStore = store
	cli.SessionConfig = client.SessionConfig{
		AutoCreate:     true, // 自动创建不存在的会话
		TruncatePolicy: session.WindowPolicy{MaxMessages: 20, KeepSystemPrompt: true},
	}

	// 步骤 4: 定义会话参数
	providerName := "dify"

	fmt.Printf("开始 Dify 多轮对话演示\n")
	fmt.Printf("Provider: %s\n\n", providerName)

	// 第一轮：不指定 SessionID，让 Dify 生成
	fmt.Println("=== 第一轮对话 ===")
	ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel1()

	resp1, err := cli.ChatStreamSync(ctx1, providerName, base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "请介绍一下 Dify 是什么？"}},
	})
	if err != nil {
		log.Fatalf("第一轮对话错误: %v", err)
	}
	fmt.Printf("用户: 请介绍一下 Dify 是什么？\n")
	fmt.Printf("助手: %s\n", resp1.Text)
	printTokenUsage(resp1)

	conversationID := resp1.SessionID
	if conversationID == "" {
		log.Fatal("未获取到 conversation_id")
	}
	fmt.Printf("Dify 会话 ID: %s\n\n", conversationID)

	// 等待一段时间再进行下一轮对话
	time.Sleep(2 * time.Second)

	// 第二轮：询问主要功能（测试上下文记忆）
	fmt.Println("=== 第二轮对话 ===")
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	resp2, err := cli.ChatSessionStreamSync(ctx2, providerName, conversationID, base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "它的主要功能是什么？"}},
	})
	if err != nil {
		log.Fatalf("第二轮对话错误: %v", err)
	}
	fmt.Printf("用户: 它的主要功能是什么？\n")
	fmt.Printf("助手: %s\n", resp2.Text)
	printTokenUsage(resp2)
	fmt.Println()

	// 等待一段时间再进行下一轮对话
	time.Sleep(2 * time.Second)

	// 第三轮：总结对话（测试完整上下文）
	fmt.Println("=== 第三轮对话 ===")
	ctx3, cancel3 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel3()

	resp3, err := cli.ChatSessionStreamSync(ctx3, providerName, conversationID, base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "请总结前面的对话内容。"}},
	})
	if err != nil {
		log.Fatalf("第三轮对话错误: %v", err)
	}
	fmt.Printf("用户: 请总结前面的对话内容。\n")
	fmt.Printf("助手: %s\n", resp3.Text)
	printTokenUsage(resp3)
	fmt.Println()

	fmt.Printf("=== 对话结束 ===\n")
	fmt.Printf("会话 ID: %s\n", conversationID)
	fmt.Println("多轮对话演示完成，上下文记忆功能正常工作。")
}

// printTokenUsage 打印 Token 使用情况（如果有）
func printTokenUsage(resp base.ChatResponse) {
	if resp.Usage.PromptTokens > 0 || resp.Usage.CompletionTokens > 0 {
		fmt.Printf("Token 使用: 输入=%d, 输出=%d, 总计=%d\n",
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			resp.Usage.TotalTokens)
	}
}

// applyAuthStoreConfig 应用认证存储配置
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
