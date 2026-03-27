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

	// 场景1：流式对话（默认）
	fmt.Println("\n场景1：流式对话")
	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "openai_compat",
		APIKey:   "sk-dvqBqqTMuVBYezmYA7sY0YooMbgyS4vzPjlEmC0oXARxTDiA",
		BaseURL:  "https://api.5090523.xyz/v1",
		Model:    "gpt-5.2-codex",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	ch, err := qs.SendText(ctx, "请用简短的100字告诉我什么是Rust语言？")
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
		Provider: "openai_compat",
		APIKey:   "sk-dvqBqqTMuVBYezmYA7sY0YooMbgyS4vzPjlEmC0oXARxTDiA",
		BaseURL:  "https://api.5090523.xyz/v1",
		Model:    "gpt-5.2-codex",
		Stream:   &noStream,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	ch2, err := qs2.SendText(ctx, "请用简短的100字告诉我什么是Go语言？")
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
