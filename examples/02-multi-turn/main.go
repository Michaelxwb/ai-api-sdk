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

	fmt.Println("=== 多轮对话示例（Quick API）===")
	fmt.Println("说明：local_history 模式下，SDK 自动维护会话历史，多轮对话开箱即用。")

	store := sessionstore.NewFile(sessionstore.FileConfig{
		Path: "examples/sessions.json",
	})

	qs, err := cli.Quick(client.ProviderConfig{
		BaseURL:  "https://qianfan.baidubce.com/v2/app/conversation/runs",
		Provider: "qianfan_app",
		APIKey:   "bce-v3/ALTAK-dzGBsXDJ8eGD9FaOc58t9/3d3a92846e9a225c97600d184f2c073be999a830",
		Model:    "ad36f5bd-dff4-4a63-81bb-72a3768b4b32",
		Store:    store,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// 第一轮：记住信息
	fmt.Print("\n第一次回答: ")
	ch, err := qs.SendText(ctx, "请记住：我最喜欢的编程语言是 Python。")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	// 第二轮：验证 SDK 自动携带历史
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
