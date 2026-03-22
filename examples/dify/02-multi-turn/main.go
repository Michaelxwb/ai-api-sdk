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

	fmt.Println("=== Dify 多轮对话示例（Quick API）===")
	fmt.Println("说明：Dify 为 remote_session 模式，服务端自动维护对话上下文。")

	qs := cli.Quick(client.ProviderConfig{
		Provider: "dify",
		APIKey:   "app-59zRGqk6BMwGkKz3HWLIezvi",
		BaseURL:  "https://adaidify.sangfor.com/v1",
	})

	fmt.Print("第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最喜欢的编程语言是 Go。")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	fmt.Printf("conversation_id: %s\n", qs.Session().ID())

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
