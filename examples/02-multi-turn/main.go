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

	fmt.Println("=== 多轮对话示例（Quick API）===")
	fmt.Println("说明：local_history 模式下，SDK 自动维护会话历史，多轮对话开箱即用。")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "openai_compat",
		APIKey:   "sk-TOKEN",
		BaseURL:  "http://10.6.193.48:30090/v1",
		Model:    "Qwen3-32B-FP8",
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
