// Package main 演示代码构建凭证的连通性测试。
//
// 功能特性：
//   - 代码方式构建凭证和 Provider 配置
//   - 无需配置文件
//   - 适用于动态配置场景
//
// 使用方法：
//
//	go run main.go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	// 步骤 1: 代码构建凭证
	cred := &auth.Credential{
		ID:          "test-key",
		Provider:    "openai",
		AuthType:    auth.AuthTypeBearerToken,
		AccessToken: "sk-your-api-key-here", // 替换为真实 key
	}

	// 步骤 2: 代码构建 Provider 配置
	pc := &config.ProviderConfig{
		Name:    "openai",
		Type:    "openai",
		BaseURL: "https://api.openai.com",
		AuthRef: "test-key",
	}

	// 步骤 3: 创建客户端（无需完整配置）
	cli := client.NewClient(nil, nil)

	// 步骤 4: 测试连通性
	fmt.Println("=== 代码构建凭证 - 连通性测试 ===")

	testOpt := &client.TestOptions{
		Model:     "gpt-4o-mini",
		Timeout:   15 * time.Second,
		MaxTokens: 1,
		Prompt:    "test",
	}

	ctx := context.Background()

	result, err := cli.TestWith(ctx, cred, pc, testOpt)
	if err != nil {
		fmt.Printf("❌ 测试失败: %v\n", err)
		fmt.Println("\n提示：请确保 AccessToken 有效")
	} else {
		fmt.Printf("✅ 测试成功\n")
		fmt.Printf("   延迟: %v\n", result.Latency)
		fmt.Printf("   响应: %s\n", result.Response.Text)
	}
}
