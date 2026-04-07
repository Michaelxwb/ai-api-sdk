package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== 多轮对话示例（Quick API · Coze）===")
	fmt.Println("说明：remote_session 模式下，Coze 服务端通过 conversation_id 维护会话历史。")

	store := sessionstore.NewFile(sessionstore.FileConfig{
		Path: "examples/sessions.json",
	})

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "coze",
		APIKey:   "pat-TOKEN",              // Coze 个人访问令牌
		Model:    "7607749137564057650",    // Coze Bot ID
		BaseURL:  "https://api.coze.cn/v3", // 国际站；默认国内站 api.coze.cn
		Store:    store,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// 第一轮：记住信息
	fmt.Print("\n第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最喜欢的编程语言是 GO。")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	// 第二轮：验证 SDK 自动携带历史
	fmt.Print("第二次回答: ")
	ch2, err := qs.SendText(ctx, "请简单直接回复我：我最喜欢的编程语言是什么？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch2); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	// 第二轮：验证 SDK 自动携带历史
	fmt.Print("第三次回答: ")
	ch3, err := qs.SendText(ctx, "请简单直接回复我：我问的第一个问题是什么？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch3); err != nil {
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
