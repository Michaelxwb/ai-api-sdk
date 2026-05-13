package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

// ========== 配置 ==========

const testImagePath = "examples/04-connectivity-test/test_cropus.png"

// ========== 辅助函数 ==========

func readImageBase64(imagePath string) (string, string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("read image failed: %w", err)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(imagePath)), ".")
	mimeType := "image/" + ext
	encoded := base64.StdEncoding.EncodeToString(data)
	return encoded, mimeType, nil
}

// ========== 文本连通性测试 ==========

func testText() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== 连通性测试示例（Quick API）===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-XgRPbziT8T4cOn0GE1cdaEnCEIRkrq1K1NZhbQxFfNFkjKxm",
		BaseURL:  "http://adaidify.sangfor.com:8900/v1",
		Model:    "Qwen3.5-27B-AWQ",
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

// ========== 图像连通性测试 ==========

func testImage() {
	fmt.Println("\n=== 连通性测试示例（图像多模态）===")

	imageData, mimeType, err := readImageBase64(testImagePath)
	if err != nil {
		log.Fatalf("读取图片失败: %v", err)
	}

	cli := client.New()
	ctx := context.Background()

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "openai",
		APIKey:   "sk-XgRPbziT8T4cOn0GE1cdaEnCEIRkrq1K1NZhbQxFfNFkjKxm",
		BaseURL:  "http://adaidify.sangfor.com:8900/v1",
		Model:    "Qwen3.5-27B-AWQ",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	parts := []base.ContentPart{
		{Type: "text", Text: "请计算图片中的公式，直接输出结果"},
		{Type: "image_url", Data: imageData, MIMEType: mimeType},
	}

	result, err := qs.Test(ctx, &client.TestOptions{Parts: parts})
	if err != nil {
		log.Fatalf("测试失败: %v", err)
	}

	fmt.Printf("测试通过\n")
	fmt.Printf("  延迟: %v\n", result.Latency)
	fmt.Printf("  响应: %s\n", result.Response.Text)
}

// ========== 主函数 ==========

func main() {
	testText()
	testImage()
}
