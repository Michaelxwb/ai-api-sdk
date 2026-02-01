// Package main 演示 AI 模型连通性测试。
//
// 功能特性：
//   - 基于 YAML 的配置和凭证管理
//   - 快速验证 Provider 配置和凭证有效性
//   - 测量请求延迟
//
// 前置条件：
//   - 在 examples/config.example.yaml 中配置有效凭证
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

	// 注册所有 Provider（触发 init 副作用）
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	// 步骤 1: 加载 YAML 配置文件
	cfg, err := config.LoadConfig("examples/config.example.yaml")
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 步骤 2: 创建认证管理器
	authStore := auth.NewFileStore(cfg.Auth.Store.Path)
	applyAuthStoreConfig(authStore, cfg)

	mgr, err := auth.NewManager(authStore, &auth.RoundRobinSelector{})
	if err != nil {
		log.Fatalf("认证管理器创建失败: %v", err)
	}
	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	// 步骤 3: 创建客户端
	cli := client.NewClient(cfg, mgr)

	// 步骤 4: 测试连通性
	fmt.Println("=== 测试 AI 模型连通性 ===")

	// 测试选项
	testOpt := &client.TestOptions{
		Model:     "deepseek-r1:7b",
		Timeout:   15 * time.Second,
		MaxTokens: 1,
		Prompt:    "测试",
	}

	ctx := context.Background()

	// 测试 vllm_local Provider
	fmt.Println("\n测试 Provider: vllm_local")
	result, err := cli.Test(ctx, "vllm_local", testOpt)
	if err != nil {
		fmt.Printf("❌ 测试失败: %v\n", err)
	} else {
		fmt.Printf("✅ 测试成功\n")
		fmt.Printf("   延迟: %v\n", result.Latency)
		fmt.Printf("   响应: %s\n", result.Response.Text)
	}

	// 可以测试多个 Provider
	// fmt.Println("\n测试 Provider: openai")
	// testOpt.Model = "gpt-4o-mini"
	// result2, err := cli.Test(ctx, "openai", testOpt)
	// ...
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
