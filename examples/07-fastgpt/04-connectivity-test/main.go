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

	fmt.Println("=== FastGPT 连通性测试示例（Quick API）===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "fastgpt",
		APIKey:      "fastgpt-TOKEN",
		BaseURL:     "https://api.fastgpt.in",
		Path:        "/api/v1/chat/completions",
		Model:       "fastgpt",
		SessionMode: "local_history",
		ExtraBody: map[string]any{
			"detail": false,
			"variables": map[string]any{
				"team":    "demo",
				"channel": "connectivity",
			},
		},
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
