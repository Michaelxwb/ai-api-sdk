// Package main 演示最简单的非流式单轮对话示例（Dify）。
//
// 功能特性：
//   - 基于 YAML 的配置和凭证管理
//   - 使用会话管理（SessionStore）
//   - 首次对话通过 ChatStreamSync 获取 conversation_id
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

	// 步骤 4: 发送单轮对话请求（首次不传 SessionID）
	fmt.Println("=== Dify 单轮对话示例 ===")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cli.ChatStreamSync(ctx, "dify", base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "请介绍一下 Dify"}},
	})
	if err != nil {
		log.Fatalf("Chat error: %v", err)
	}

	fmt.Printf("响应: %s\n", resp.Text)
	if resp.SessionID != "" {
		fmt.Printf("会话 ID: %s（由 Dify 生成）\n", resp.SessionID)
	}
	if resp.Usage != nil {
		fmt.Printf("Token 使用: 输入=%d, 输出=%d, 总计=%d\n",
			resp.Usage.PromptTokens,
			resp.Usage.CompletionTokens,
			resp.Usage.TotalTokens)
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
