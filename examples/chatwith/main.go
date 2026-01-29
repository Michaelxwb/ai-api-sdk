// chatwith 示例：演示平台集成模式下如何构建 Credential 和 ProviderConfig
// 适用于平台自行管理凭证（如数据库存储），直接传入 SDK 调用 AI 模型的场景。
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider"

	// 注册所有 provider（init 自动触发）
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	cli := client.New() // 轻量构造，不依赖 config.yaml 和 Manager

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	//// ----------------------------------------------------------------
	//// 示例 1: OpenAI — API Key 认证
	//// ----------------------------------------------------------------
	//openaiCred := &auth.Credential{
	//	ID:       "openai-from-db",
	//	Provider: "openai",
	//	AuthType: auth.AuthTypeAPIKey,
	//	APIKey:   os.Getenv("OPENAI_API_KEY"), // 平台从数据库读取后传入
	//}
	//openaiPC := &config.ProviderConfig{
	//	Name:    "openai",
	//	Type:    "openai", // 对应 provider 注册名
	//	BaseURL: "https://api.openai.com/v1",
	//}
	//
	//resp, err := cli.ChatWith(ctx, openaiCred, openaiPC, provider.ChatRequest{
	//	Model:    "gpt-4o-mini",
	//	Messages: []provider.Message{{Role: "user", Content: "Hello from ChatWith"}},
	//})
	//if err != nil {
	//	fmt.Printf("[OpenAI] error: %v\n", err)
	//} else {
	//	fmt.Printf("[OpenAI] response: %s\n", resp.Text)
	//}
	//
	//// ----------------------------------------------------------------
	//// 示例 2: Claude — API Key 认证（AuthStrategyOverride 自动映射为 x-api-key）
	//// ----------------------------------------------------------------
	//claudeCred := &auth.Credential{
	//	ID:       "claude-from-db",
	//	Provider: "claude",
	//	AuthType: auth.AuthTypeAPIKey,
	//	APIKey:   os.Getenv("ANTHROPIC_API_KEY"),
	//}
	//claudePC := &config.ProviderConfig{
	//	Name: "claude",
	//	Type: "claude",
	//	// BaseURL 留空则使用 provider 默认值 https://api.anthropic.com
	//}
	//
	//resp, err = cli.ChatWith(ctx, claudeCred, claudePC, provider.ChatRequest{
	//	Model:    "claude-sonnet-4-20250514",
	//	Messages: []provider.Message{{Role: "user", Content: "Hello from ChatWith"}},
	//})
	//if err != nil {
	//	fmt.Printf("[Claude] error: %v\n", err)
	//} else {
	//	fmt.Printf("[Claude] response: %s\n", resp.Text)
	//}

	// ----------------------------------------------------------------
	// 示例 3: DeepSeek — 与 OpenAI 兼容，换 BaseURL 即可
	// ----------------------------------------------------------------
	deepseekCred := &auth.Credential{
		ID:          "deepseek-from-db",
		Provider:    "deepseek",
		AuthType:    auth.AuthTypeBearerToken,
		AccessToken: "sk-sdfsdfd",
	}
	deepseekPC := &config.ProviderConfig{
		Name:    "deepseek",
		Type:    "deepseek", // 已注册为 openai_compat 变体
		BaseURL: "https://api.deepseek.com/v1",
	}

	resp, err := cli.ChatWith(ctx, deepseekCred, deepseekPC, provider.ChatRequest{
		Model:    "deepseek-chat",
		Messages: []provider.Message{{Role: "user", Content: "Hello from ChatWith"}},
	})
	if err != nil {
		fmt.Printf("[DeepSeek] error: %v\n", err)
	} else {
		fmt.Printf("[DeepSeek] response: %s\n", resp.Text)
	}

	// ----------------------------------------------------------------
	// 连通性测试示例
	// ----------------------------------------------------------------
	testResult, err := cli.TestWith(ctx, deepseekCred, deepseekPC, &client.TestOptions{
		Model: "deepseek-chat",
	})
	if err != nil {
		fmt.Printf("[DeepSeek Test] failed: %v (latency: %s)\n", err, testResult.Latency)
	} else {
		fmt.Printf("[DeepSeek Test] success, latency: %s\n", testResult.Latency)
	}

	//// ----------------------------------------------------------------
	//// 示例 4: 本地 Ollama — 无认证
	//// ----------------------------------------------------------------
	//ollamaPC := &config.ProviderConfig{
	//	Name:    "ollama",
	//	Type:    "ollama",
	//	BaseURL: "http://127.0.0.1:11434",
	//}
	//// cred 传 nil 表示无认证
	//resp, err = cli.ChatWith(ctx, nil, ollamaPC, provider.ChatRequest{
	//	Model:    "llama3",
	//	Messages: []provider.Message{{Role: "user", Content: "Hello from ChatWith"}},
	//})
	//if err != nil {
	//	fmt.Printf("[Ollama] error: %v\n", err)
	//} else {
	//	fmt.Printf("[Ollama] response: %s\n", resp.Text)
	//}
	//
	//// ----------------------------------------------------------------
	//// 示例 5: 自定义网关 — 自定义 Path + Headers + ExtraBody
	//// 适用于 New API / One API 风格的私有网关
	//// ----------------------------------------------------------------
	//gatewayCred := &auth.Credential{
	//	ID:       "gateway-from-db",
	//	Provider: "openai_compat",
	//	AuthType: auth.AuthTypeBearerToken,
	//	AccessToken: "sk-gateway-token-from-db",
	//}
	//gatewayPC := &config.ProviderConfig{
	//	Name:    "my_gateway",
	//	Type:    "openai_compat",
	//	BaseURL: "https://gateway.example.com",
	//	Path:    "/pg/chat/completions",            // 覆盖默认路径
	//	Headers: map[string]string{
	//		"X-Tenant-ID": "tenant-123",            // 自定义请求头
	//	},
	//	ExtraBody: map[string]any{
	//		"group": "security-eval",               // 额外字段合入请求 body
	//	},
	//}
	//
	//resp, err = cli.ChatWith(ctx, gatewayCred, gatewayPC, provider.ChatRequest{
	//	Model:    "gpt-4o",
	//	Messages: []provider.Message{{Role: "user", Content: "Hello from ChatWith"}},
	//})
	//if err != nil {
	//	fmt.Printf("[Gateway] error: %v\n", err)
	//} else {
	//	fmt.Printf("[Gateway] response: %s\n", resp.Text)
	//}

	// ----------------------------------------------------------------
	// 示例 6: 平台集成的典型用法（模拟从数据库查询凭证）
	// ----------------------------------------------------------------
	// 实际平台中，这段逻辑大致是：
	//
	//   // 1. 从数据库查询用户配置的模型和凭证
	//   model := db.GetModel(taskID)
	//   dbCred := db.GetCredential(model.CredentialID)
	//
	//   // 2. 转换为 SDK 结构体
	//   cred := &auth.Credential{
	//       ID:       dbCred.ID,
	//       Provider: dbCred.Provider,
	//       AuthType: auth.AuthType(dbCred.AuthType),
	//       APIKey:   dbCred.APIKey,
	//   }
	//   pc := &config.ProviderConfig{
	//       Name:    model.Provider,
	//       Type:    model.ProviderType,
	//       BaseURL: model.BaseURL,
	//   }
	//
	//   // 3. 调用 SDK
	//   resp, err := cli.ChatWith(ctx, cred, pc, provider.ChatRequest{
	//       Model:    model.ModelName,
	//       Messages: messages,
	//   })
}
