package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== Dify 平台集成示例（Quick API）===")
	fmt.Println("说明：平台从数据库获取配置，通过 Quick API 直接构建会话。")

	// 模拟从数据库获取的配置
	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "dify",
		APIKey:   "app-59zRGqk6BMwGkKz3HWLIezvi",
		BaseURL:  "https://adaidify.sangfor.com/v1",
		ExtraBody: map[string]any{
			"inputs": map[string]any{"var1": "value1"},
			"user":   "platform-user-123",
		},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	ch, err := qs.SendText(ctx, "用20个字简单回答什么是Go语言？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	fmt.Printf("conversation_id: %s\n", qs.Session().ID())
}

func printStream(ch <-chan streaming.StreamChunk) error {
	for chunk := range ch {
		if chunk.Error != nil {
			return chunk.Error
		}
		fmt.Print(chunk.Text)
	}
	fmt.Println()
	return nil
}
