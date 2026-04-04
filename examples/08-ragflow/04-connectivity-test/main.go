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

	fmt.Println("=== RAGFlow 连通性测试示例（Quick API）===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "ragflow",
		APIKey:   "ragflow-TOKEN",
		BaseURL:  "http://ragflow.example.com/api/v1/chats_openai/your-chat-assistant-id/chat/completions",
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
