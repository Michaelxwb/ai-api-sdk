// Package main 演示通过代码方式构建凭证的单轮对话（Dify）。
//
// 功能特性：
//   - 不使用 YAML 配置
//   - 手动 Credential + ProviderConfig
//   - 通过 ChatWith 发送单轮请求
//
// Dify 会话管理说明：
//   - Dify 的 conversation_id 由服务端生成和管理
//   - 首次对话不传 conversation_id（不设置 SessionID），从响应中获取
//   - 后续对话使用返回的 conversation_id 作为 SessionID
//   - 这是 Dify 的标准流程，与客户端生成 SessionID 的模式不同
//
// 前置条件：
//   - 为你的 Dify 应用更新 BaseURL 和 AccessToken
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
		ID:          "dify-token",
		Provider:    "dify",
		AuthType:    auth.AuthTypeBearerToken,
		AccessToken: "app-REPLACE",
	}

	// 步骤 2: 在代码中构建 Provider 配置
	pc := &config.ProviderConfig{
		Name:    "dify",
		Type:    "dify",
		BaseURL: "https://api.dify.ai/v1",
	}

	// 步骤 3: 创建轻量级客户端
	cli := client.New()

	// 步骤 4: 通过 ChatWith 发送单轮对话请求
	// 注意：本示例展示 ChatWith 模式（平台自行管理凭证）。
	// 对于多轮对话场景，平台应配置 SessionStore 并使用会话管理，
	// 参考 examples/dify/session 了解完整流程。
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cli.ChatWith(ctx, cred, pc, base.ChatRequest{
		Messages: []base.Message{{Role: "user", Content: "介绍一下 Dify"}},
	})
	if err != nil {
		log.Fatalf("ChatWith error: %v", err)
	}

	fmt.Println("Response:", resp.Text)
}
