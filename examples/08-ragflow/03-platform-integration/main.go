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

	fmt.Println("=== RAGFlow 平台集成示例（Quick API）===")
	fmt.Println("说明：平台从数据库获取配置，通过 Quick API 直接构建会话。")

	// 模拟从数据库获取的配置
	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "ragflow",
		APIKey:   "ragflow-TOKEN",
		BaseURL:  "http://ragflow.example.com/api/v1/chats_openai/your-chat-assistant-id/chat/completions",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	ch, err := qs.SendText(ctx, "用20个字简单回答什么是RAG？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	fmt.Printf("session_id: %s\n", qs.Session().ID())
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
