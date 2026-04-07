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

	fmt.Println("=== 单轮对话示例（Quick API · Coze）===")

	// Coze 流式对话（仅支持流式，remote_session 模式自动推断）
	// Model 填 bot_id，从 Coze 平台获取
	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "coze",
		APIKey:   "pat-TOKEN",              // Coze 个人访问令牌
		Model:    "7607749137564057650",    // Coze Bot ID
		BaseURL:  "https://api.coze.cn/v3", // 国际站；默认国内站 api.coze.cn
		// ExtraBody: map[string]any{
		//     "user_id": "my-user",            // 默认 "sdk-user"
		// },
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	ch, err := qs.SendText(ctx, "请告诉我什么是Go语言？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
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
