package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Michaelxwb/ai-api-sdk/client"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== 连通性测试示例（Quick API）===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "openai_compat",
		APIKey:   "sk-TOKEN",
		BaseURL:  "https://api.5090523.xyz/v1",
		Model:    "gpt-5.3-codex",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	result, err := qs.Test(ctx)
	if err != nil {
		log.Fatalf("测试失败: %v", err)
	}

	fmt.Printf("测试通过\n")
	fmt.Printf("  延迟: %v\n", result.Latency)
	fmt.Printf("  响应: %s\n", result.Response.Text)
}
