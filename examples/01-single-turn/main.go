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

	fmt.Println("=== 单轮对话示例（Quick API）===")

	// 场景1：流式对话（bailian_app 默认流式）
	fmt.Println("\n场景1：流式对话")
	qs, err := cli.Quick(client.ProviderConfig{
		BaseURL:  "https://qianfan.baidubce.com/v2/app/conversation/runs",
		Provider: "qianfan_app",
		APIKey:   "bce-v3/ALTAK-dzGBsXDJ8eGD9FaOc58t9/3d3a92846e9a225c97600d184f2c073be999a830",
		Model:    "ad36f5bd-dff4-4a63-81bb-72a3768b4b32",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	ch, err := qs.SendText(ctx, "请告诉我什么是Rust语言？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}

	// 场景2：非流式对话
	fmt.Println("\n场景2：非流式对话")
	noStream := false
	qs2, err := cli.Quick(client.ProviderConfig{
		BaseURL:    "https://qianfan.baidubce.com/v2/app/conversation/runs",
		Provider:   "qianfan_app",
		APIKey:     "bce-v3/ALTAK-dzGBsXDJ8eGD9FaOc58t9/3d3a92846e9a225c97600d184f2c073be999a830",
		Model:      "ad36f5bd-dff4-4a63-81bb-72a3768b4b32",
		Stream:     &noStream, // 显式关闭流式，使用同步请求
		TimeoutSec: 120,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	ch2, err := qs2.SendText(ctx, "请告诉我什么是Go语言？")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	for chunk := range ch2 {
		if chunk.Error != nil {
			log.Fatalf("Error: %v", chunk.Error)
		}
		fmt.Printf("回答: %s\n", chunk.Text)
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
