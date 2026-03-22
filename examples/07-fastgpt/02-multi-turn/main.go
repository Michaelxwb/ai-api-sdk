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

	fmt.Println("=== FastGPT 多轮对话示例（Quick API）===")
	fmt.Println("说明：同时展示 local_history 与 remote_session 模式。")

	// 场景1：local_history（SDK 自动维护历史）
	fmt.Println("\n场景1：local_history")
	exampleLocalHistory(cli, ctx)

	// 场景2：remote_session（服务端维护历史）
	fmt.Println("\n场景2：remote_session")
	exampleRemoteSession(cli, ctx)
}

func exampleLocalHistory(cli *client.Client, ctx context.Context) {
	qs := cli.Quick(client.ProviderConfig{
		Provider:    "fastgpt",
		APIKey:      "fastgpt-TOKEN",
		BaseURL:     "https://api.fastgpt.in",
		Path:        "/api/v1/chat/completions",
		Model:       "fastgpt",
		SessionMode: "local_history",
		ExtraBody: map[string]any{
			"detail": true,
			"variables": map[string]any{
				"team":    "demo",
				"channel": "fastgpt-local",
			},
		},
	})

	fmt.Print("第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最喜欢的水果是苹果。")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if err := printStream(ch); err != nil {
		log.Printf("Stream error: %v", err)
		return
	}

	fmt.Print("第二次回答: ")
	ch2, err := qs.SendText(ctx, "我最喜欢的水果是什么？")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if err := printStream(ch2); err != nil {
		log.Printf("Stream error: %v", err)
	}
}

func exampleRemoteSession(cli *client.Client, ctx context.Context) {
	qs := cli.Quick(client.ProviderConfig{
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
				"channel": "fastgpt-remote",
			},
		},
	})

	fmt.Print("第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最喜欢的水果是苹果。")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if err := printStream(ch); err != nil {
		log.Printf("Stream error: %v", err)
		return
	}
	fmt.Printf("chatId: %s\n", qs.Session().ID())

	fmt.Print("第二次回答: ")
	ch2, err := qs.SendText(ctx, "我最喜欢的水果是什么？")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if err := printStream(ch2); err != nil {
		log.Printf("Stream error: %v", err)
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
