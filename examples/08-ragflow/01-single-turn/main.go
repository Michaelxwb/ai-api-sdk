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

	fmt.Println("=== RAGFlow 单轮对话示例（Quick API）===")
	fmt.Println("说明：RAGFlow 为 remote_session 模式，session_id 由服务端生成，SDK 自动提取。")
	fmt.Println("注意：chat_id 需通过 ExtraBody 传入。")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "ragflow",
		APIKey:   "ragflow-TOKEN",
		BaseURL:  "http://ragflow.example.com",
		ExtraBody: map[string]any{
			"chat_id": "your-chat-assistant-id",
		},
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
