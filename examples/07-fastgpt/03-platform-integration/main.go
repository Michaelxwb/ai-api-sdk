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

	fmt.Println("=== FastGPT 平台集成示例（Quick API）===")
	fmt.Println("说明：平台从数据库获取配置，通过 Quick API 直接构建会话。")

	// 模拟从数据库获取的配置
	qs, err := cli.Quick(client.ProviderConfig{
		Provider:    "fastgpt",
		APIKey:      "fastgpt-TOKEN",
		BaseURL:     "https://api.fastgpt.in",
		Path:        "/api/v1/chat/completions",
		Model:       "fastgpt",
		SessionMode: "remote_session",
		ExtraBody: map[string]any{
			"detail": false,
			"variables": map[string]any{
				"team":    "demo",
				"channel": "platform",
			},
		},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最常用的编辑器是 VS Code。")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	fmt.Printf("chatId: %s\n", qs.Session().ID())

	fmt.Print("第二次回答: ")
	ch2, err := qs.SendText(ctx, "我最常用的编辑器是什么？")
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
