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

	fmt.Println("=== 平台集成示例（Quick API）===")
	fmt.Println("说明：平台从数据库读取配置，通过 Quick API 直接构建会话，无需 config.yaml。")

	// 模拟从数据库查询到的配置
	dbProvider := "openai_compat"
	dbAPIKey := "sk-TOKEN"
	dbBaseURL := "http://10.6.193.48:30090/v1"
	dbModel := "Qwen3-32B-FP8"

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: dbProvider,
		APIKey:   dbAPIKey,
		BaseURL:  dbBaseURL,
		Model:    dbModel,
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	ch, err := qs.SendText(ctx, "请用简短的100字，什么是Go语言？")
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
