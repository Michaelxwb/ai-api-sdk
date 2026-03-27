package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Michaelxwb/ai-api-sdk/client"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== Dify 连通性测试示例（Quick API）===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "dify",
		APIKey:   "app-59zRGqk6BMwGkKz3HWLIezvi",
		BaseURL:  "https://adaidify.sangfor.com/v1",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	result, err := qs.Test(ctx)
	if err != nil {
		log.Fatalf("测试失败: %v", err)
	}

	fmt.Printf("测试通过\n")
	fmt.Printf("  延迟: %v\n", result.Latency)
	fmt.Printf("  响应: %s\n", result.Response.Text)
}
