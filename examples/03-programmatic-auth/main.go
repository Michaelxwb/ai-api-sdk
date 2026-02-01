// Package main 演示通过代码方式构建凭证的单轮对话。
//
// 功能特性：
//   - 不使用 YAML 配置
//   - 手动 Credential + ProviderConfig
//   - 通过 ChatWith 发送单轮请求
//
// 前置条件：
//   - 为你的提供者更新 BaseURL 和 AccessToken
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

	// 注册所有 Provider（触发 init 副作用）。
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	// 步骤 1: 在代码中创建凭证
	cred := &auth.Credential{
		ID:          "local-token",
		Provider:    "openai_compat",
		AuthType:    auth.AuthTypeBearerToken,
		AccessToken: "SK-TOKEN",
	}

	// 步骤 2: 在代码中构建 Provider 配置
	pc := &config.ProviderConfig{
		Name:    "vllm_local",
		Type:    "openai_compat",
		BaseURL: "https://integrate.api.nvidia.com/v1",
	}

	// 步骤 3: 创建轻量级客户端
	cli := client.New()

	// 步骤 4: 通过 ChatWith 发送单轮对话请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cli.ChatWith(ctx, cred, pc, base.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []base.Message{{Role: "user", Content: "用一句话概括Go语言。"}},
	})
	if err != nil {
		log.Fatalf("ChatWith error: %v", err)
	}

	fmt.Println("Response:", resp.Text)
}
