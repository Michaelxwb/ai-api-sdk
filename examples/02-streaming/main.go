// Package main 演示使用 YAML 认证的流式单轮对话。
//
// 功能特性：
//   - 基于 YAML 的配置和凭证管理
//   - 通过 ChatStream 获取流式响应
//   - 打字机效果输出
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
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"

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

	// 步骤 3: 创建客户端
	cli := client.NewClient(cfg, mgr)

	// 步骤 4: 发送流式请求并打印每个分片
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := cli.ChatStream(ctx, "vllm_local", base.ChatRequest{
		Model:    "deepseek-r1:1.5b",
		Messages: []base.Message{{Role: "user", Content: "用一句话解释Go语言。"}},
	})
	if err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	if err := printStream(stream); err != nil {
		log.Fatalf("Stream failed: %v", err)
	}
}

func printStream(stream <-chan streaming.StreamChunk) error {
	for chunk := range stream {
		if chunk.Error != nil {
			return chunk.Error
		}
		for _, r := range chunk.Text {
			fmt.Printf("%c", r)
			time.Sleep(10 * time.Millisecond)
		}
		if chunk.Done {
			fmt.Println()
		}
	}
	return nil
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
