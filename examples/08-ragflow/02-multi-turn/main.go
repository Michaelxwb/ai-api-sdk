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

	fmt.Println("=== RAGFlow 多轮对话示例（Quick API）===")
	fmt.Println("说明：RAGFlow 为 remote_session 模式，服务端自动维护对话上下文。")

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

	fmt.Print("第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最喜欢的编程语言是 Go。")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	fmt.Printf("session_id: %s\n", qs.Session().ID())

	fmt.Print("第二次回答: ")
	ch2, err := qs.SendText(ctx, "我最喜欢的编程语言是什么？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch2); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
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
