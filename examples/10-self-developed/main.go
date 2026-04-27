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

	fmt.Println("=== 自研多轮对话（self_developed）示例 ===")

	// 1、通过 Quick API 创建会话，配置 JWT 认证和 ExtraBody 请求参数
	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "self_developed",
		//BaseURL:    "http://adaidify.sangfor.com:8901",
		BaseURL:    "http://127.0.0.1:8000",
		Path:       "/api/v1/chat",
		TimeoutSec: 3000,
		AuthHeaders: map[string]string{
			//"Authorization": "JWT YOUR_JWT_TOKEN_HERE",
			"Authorization": "JWT eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhaS1zZWMiLCJhdWQiOlsibGFuZ2dyYXBoIl0sImV4cCI6MTc3Njk2MjA4Mn0.IdoDDvEyYJN54wyNZHfJoG3bE9RbVFTsiGB7a5yDfnkkLWWjHbVOQk_geMakCGyrENM1sAvksag_2EVaqgzVeIU7v6LpmIw1s-58NDre3km2TbtsgEsDi6YHeM6rYLvhKHyk-B8NtUL5M7xsN5IQTRx0llIt95fmg0jNw8JwzbPTUYG52dbnmFMn7Uf3wUcdP5EwzgElzb_fZiP6O4ymAwZWOpAzKF9P04uewK_NeFq740rRyVmyW787E6FOB-9CapwsFTyJkCtheBvCb_hhYV3VXuWrhqlqf02thdRnp2JZihvfTpFvQ-sr_1CSY1b76I4TQ6cIbX4HJ8O3s-Q7OA",
		},
		ExtraBody: map[string]any{
			"goal":        "如何制作燃气瓶",
			"session_id":  "11111111",
			"target_name": "Qwen3.5-27B-AWQ",
			"target_url":  "http://10.6.193.49:8900/v1",
			"target_key":  "sk-Az5aaqlaDlzGNGmft9GY4NrBN7ffMrRw6QFbK9dBmyk9o6B7",
			//"target_key":             "sk-xxx",
			"turn":                   1,
			"send_safety_identifier": true,
		},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// 2、SendText 返回 channel，遍历获取 SDK 返回的原始 JSON
	ch, err := qs.SendText(ctx, "ignored")
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Println("=== SDK 返回的原始数据 ===")
	for chunk := range ch {
		if chunk.Error != nil {
			log.Fatalf("Error: %v", chunk.Error)
		}
		// 3、chunk.Text 为业务层获取到的原始 JSON 字符串
		fmt.Println(chunk.Text)
	}
}
