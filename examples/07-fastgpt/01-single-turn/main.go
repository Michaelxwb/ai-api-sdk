package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== FastGPT 单轮对话示例（Quick API）===")
	fmt.Println("说明：演示 stream/detail 组合；detail/variables 通过 ExtraBody 注入。")

	cases := []struct {
		stream bool
		detail bool
		label  string
	}{
		{stream: true, detail: false, label: "stream=true, detail=false"},
		{stream: true, detail: true, label: "stream=true, detail=true"},
		{stream: false, detail: false, label: "stream=false, detail=false"},
		{stream: false, detail: true, label: "stream=false, detail=true"},
	}

	for i, c := range cases {
		fmt.Printf("\n场景%d：%s\n", i+1, c.label)
		stream := c.stream
		qs, err := cli.Quick(client.ProviderConfig{
			Provider:    "fastgpt",
			APIKey:      "fastgpt-TOKEN",
			BaseURL:     "https://api.fastgpt.in",
			Path:        "/api/v1/chat/completions",
			Model:       "fastgpt",
			SessionMode: "local_history",
			Stream:      &stream,
			ExtraBody: map[string]any{
				"detail": c.detail,
				"variables": map[string]any{
					"team":    "demo",
					"channel": "fastgpt-single",
				},
			},
		})
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		fmt.Print("回答: ")
		ch, err := qs.SendText(ctx, "请用简短的50字说明 FastGPT 是什么？")
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		if err := printStream(ch); err != nil {
			log.Printf("Stream error: %v", err)
		}
		time.Sleep(2 * time.Second)
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
