package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"

	_ "github.com/Michaelxwb/ai-api-sdk/provider" // 导入 provider 包以注册所有供应商
)

// ========== 全局变量 ==========

// 图片路径变量（相对于运行目录）
var img1Path = "examples/11-multimodal-image/img1.png"
var img2Path = "examples/11-multimodal-image/ImG2.jpG"

// ========== 辅助函数 ==========

// readImageBase64 读取图片文件并返回 base64 编码字符串和 MIME 类型
func readImageBase64(imagePath string) (string, string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", "", fmt.Errorf("read image failed: %w", err)
	}

	// 动态拼接 MIME type，SDK 层统一拦截非白名单格式
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(imagePath)), ".")
	mimeType := "image/" + ext

	encoded := base64.StdEncoding.EncodeToString(data)
	return encoded, mimeType, nil
}

// printStream 打印流式输出
func printStream(ch <-chan streaming.StreamChunk) error {
	fmt.Print("Response: ")
	for chunk := range ch {
		if chunk.Error != nil {
			return chunk.Error
		}
		fmt.Print(chunk.Text)
	}
	fmt.Println()
	return nil
}

// getBaseURL 获取供应商的 BaseURL
func getBaseURL(provider string) string {
	urls := map[string]string{
		"openai":      "http://adaidify.sangfor.com:8900/v1",
		"fastgpt":     "http://adaidify.sangfor.com:3009",
		"ollama":      "http://127.0.0.1:11434",
		"bailian_app": "https://dashscope.aliyuncs.com/api/v2/apps/agent/2ff3c4c65f7249ac8fcc8a14af56a615/compatible-mode/v1", // /responses
		"dify":        "http://adaidify.sangfor.com:5001/v1",
		"coze":        "https://api.coze.cn/v3",
		"qianfan_app": "https://qianfan.baidubce.com/v2/app/conversation/runs",
		"moonshot":    "https://api.moonshot.cn/v1",
		"ragflow":     "http://adaidify.sangfor.com:8080/api/v1/chats_openai/de3577da2f0e11f18d0f0242c0a8b006/chat/completions",
		"deepseek":    "https://api.deepseek.com/v1",
		"generic":     "http://10.6.193.49:8900/v1/chat/completions",
	}
	return urls[provider]
}

// getModel 获取供应商的默认模型
func getModel(provider string) string {
	models := map[string]string{
		"openai":      "Qwen3.5-27B-AWQ",
		"fastgpt":     "",                                     // 可以不用模型
		"ollama":      "llava",                                // Ollama 视觉模型
		"bailian_app": "",                                     // 需自定义
		"dify":        "",                                     // Dify 使用 bot_id
		"coze":        "7626039521360150579",                  // Coze 使用 bot_id
		"qianfan_app": "a964b230-c9b9-4102-b462-602db59722ef", // Qianfan 使用 app_id
		"moonshot":    "kimi-k2.6",                            // Moonshot kimi-k2.6
		"ragflow":     "",                                     // RAGFlow
		"deepseek":    "deepseek-chat",                        // DeepSeek 纯文本模型
		"generic":     "",                                     // 需自定义
	}
	return models[provider]
}

// getAPIKey 获取供应商的 API Key
func getAPIKey(provider string) string {
	keys := map[string]string{
		"openai":      "sk-XgRPbziT8T4cOn0GE1cdaEnCEIRkrq1K1NZhbQxFfNFkjKxm",
		"fastgpt":     "fastgpt-sTAo8hgTsGqxpdS639i6TLJHgorxiCYZrcGoaaYryUk2BPzdAZFzLA0ZB3bIWGt34",
		"ollama":      "", // Ollama 通常不需要 API Key
		"bailian_app": "sk-3531fe0691e646e292404420619347ae",
		"dify":        "app-il4HjNuPHjzeov8jgXthgBKK",
		"coze":        "pat_KCy2dW0WqljQsgS3O6EHpPKGkXIXwd0sXjUKw4mqIEzq6dEGZR6aQZwOUMo4qMgO",
		"qianfan_app": "bce-v3/ALTAK-zK1VyMatd8iBoAg6LbGE9/d6bcf0c981c34cbaea8e48e4085301569611ae5c",
		"moonshot":    "sk-zpW5M6pLOAOC9gGQaTcyEiXgMNfHondstDW9QiS2R8i4JIUu",
		"ragflow":     "ragflow-IyNDZmYzU4MzE4MzExZjE5NTE3MDI0Mm",
		"deepseek":    "sk-eac084d260b14b39b47e93ae7b02ebcb",
		"generic":     "",
	}
	return keys[provider]
}

// getSessionMode 获取供应商需要的会话模式
func getSessionMode(provider string) string {
	modes := map[string]string{
		"fastgpt": "local_history",
		"generic": "local_history",
		// 其他供应商留空，使用 SDK 默认值
	}
	return modes[provider]
}

// ========== 通用测试函数 ==========

// testMultimodalImages 通用多模态图片测试函数
func testMultimodalImages(provider string, imagePaths []string, query string) {
	fmt.Printf("\n========== Testing %s ==========\n", provider)
	fmt.Printf("Images: %v\n", imagePaths)
	fmt.Printf("Query: %s\n\n", query)

	// 初始化 client
	cli := client.New()

	// 配置供应商
	apiKey := getAPIKey(provider)
	baseURL := getBaseURL(provider)
	model := getModel(provider)
	sessionMode := getSessionMode(provider)

	providerConfig := client.ProviderConfig{
		Provider:    provider,
		BaseURL:     baseURL,
		Model:       model,
		APIKey:      apiKey,
		SessionMode: sessionMode,
	}

	qs, err := cli.Quick(providerConfig)
	if err != nil {
		fmt.Printf("[Error] Failed to create QuickSession: %v\n\n", err)
		return
	}

	// 读取图片并构造 Parts
	parts := []base.ContentPart{
		{Type: "text", Text: query},
	}

	for _, imagePath := range imagePaths {
		imageData, mimeType, err := readImageBase64(imagePath)
		if err != nil {
			fmt.Printf("[Error] Failed to read image %s: %v\n\n", imagePath, err)
			return
		}

		parts = append(parts, base.ContentPart{
			Type:     "image_url",
			Data:     imageData,
			MIMEType: mimeType,
		})
	}

	// 构造消息
	msgs := []base.Message{
		{
			Role:  "user",
			Parts: parts,
		},
	}

	// 发送请求（流式）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch, err := qs.Send(ctx, msgs)
	if err != nil {
		fmt.Printf("[Error] Request failed: %v\n\n", err)
		return
	}

	// 打印流式输出
	if err := printStream(ch); err != nil {
		fmt.Printf("\n[Stream Error] %v\n", err)
	}
	fmt.Println()
}

// ========== 纯文本测试函数 ==========

// testTextOnly 测试纯文本对话（不包含图片）
func testTextOnly(provider string, query string) {
	fmt.Printf("\n========== Testing %s (Text Only) ==========\n", provider)
	fmt.Printf("Query: %s\n\n", query)

	// 初始化 client
	cli := client.New()

	// 配置供应商
	apiKey := getAPIKey(provider)
	baseURL := getBaseURL(provider)
	model := getModel(provider)
	sessionMode := getSessionMode(provider)

	providerConfig := client.ProviderConfig{
		Provider:    provider,
		BaseURL:     baseURL,
		Model:       model,
		APIKey:      apiKey,
		SessionMode: sessionMode,
	}

	qs, err := cli.Quick(providerConfig)
	if err != nil {
		fmt.Printf("[Error] Failed to create QuickSession: %v\n\n", err)
		return
	}

	// 构造纯文本消息
	msgs := []base.Message{
		{
			Role:    "user",
			Content: query, // 使用 Content 字段，不使用 Parts
		},
	}

	// 发送请求（流式）
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ch, err := qs.Send(ctx, msgs)
	if err != nil {
		fmt.Printf("[Error] Request failed: %v\n\n", err)
		return
	}

	// 打印流式输出
	if err := printStream(ch); err != nil {
		fmt.Printf("\n[Stream Error] %v\n", err)
	}
	fmt.Println()
}

// ========== A组供应商测试函数（base64 内联多图） ==========

func testOpenAIMulti() {
	testMultimodalImages(
		"openai",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

func testFastGPTMulti() {
	testMultimodalImages(
		"fastgpt",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

func testOllamaMulti() {
	testMultimodalImages(
		"ollama",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

func testBailianMulti() {
	testMultimodalImages(
		"bailian_app",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

// ========== B组供应商测试函数（文件上传多图） ==========

func testDifyMulti() {
	testMultimodalImages(
		"dify",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

func testCozeMulti() {
	testMultimodalImages(
		"coze",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

func testQianfanMulti() {
	testMultimodalImages(
		"qianfan_app",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

func testMoonshotMulti() {
	testMultimodalImages(
		"moonshot",
		[]string{img1Path, img2Path},
		"请分别描述上述图片的内容",
	)
}

// ========== C组供应商测试函数（预期返回错误） ==========

func testRAGFlow() {
	fmt.Println("\n========== Testing RAGFlow (Expected Error) ==========")
	testMultimodalImages(
		"ragflow",
		[]string{img1Path},
		"请描述这张图片的内容",
	)
	fmt.Println("Expected error: ragflow: image input not supported, provider only accepts text")
}

func testDeepSeek() {
	fmt.Println("\n========== Testing DeepSeek (Expected Error) ==========")
	testMultimodalImages(
		"deepseek",
		[]string{img1Path},
		"请描述这张图片的内容",
	)
	fmt.Println("Expected error: deepseek: vision model not available, only text models supported")
}

func testGeneric() {
	fmt.Println("\n========== Testing Generic (Expected Error) ==========")
	testMultimodalImages(
		"generic",
		[]string{img1Path},
		"请描述这张图片的内容",
	)
	fmt.Println("Expected error: generic: multimodal content not supported in template mode, use text-only messages")
}

// ========== Main 函数 ==========

func main() {
	fmt.Println("==============================================")
	fmt.Println("  Multimodal Image Support Test")
	fmt.Println("==============================================")

	// 默认测试：OpenAI 多图
	// A组：base64 内联（多图）
	//testOpenAIMulti()
	//testFastGPTMulti()
	//testOllamaMulti()
	//testBailianMulti()

	// B组：文件上传（多图）
	//testDifyMulti()
	testCozeMulti()
	//testQianfanMulti()
	//testMoonshotMulti()

	// C组：错误处理（预期返回错误）
	//testRAGFlow()
	//testDeepSeek()
	//testGeneric()

	// #################### 纯文本测试 #################################
	//testTextOnly("bailian_app", "你好,1+1=?")
	//testTextOnly("coze", "你好,1+1=?")
	//testTextOnly("qianfan_app", "你好,1+1=?")
	//testTextOnly("moonshot", "你好,1+1=?")
	//testTextOnly("deepseek", "你好,1+1=?")

	fmt.Println("\n==============================================")
	fmt.Println("  Test Completed")
	fmt.Println("==============================================")
}

/*
=============================================================
多模态图像支持测试表
=============================================================

| 供应商      | 图像上传方式  | 测试环境 | 支持情况 | 测试结果 |
|------------|--------------|--------|--------|--------|
| OpenAI     | Base64 内联   | 内网    | ✓      |  待填 |
| FastGPT    | Base64 内联   | 内网    | ✓      |  待填 |
| Ollama     | Base64 内联   | 外网    | ✓      |  待填 |
| Bailian    | Base64 内联   | 外网    | ✓      |  待填 |
| Dify       | 文件上传       | 内网    | ✓      |  待填|
| Coze       | 文件上传      | 外网    | ✓      |   ️待填 |
| Qianfan    | 文件上传      | 外网    | ✓      | [待填] |
| Moonshot   | 文件上传      | 外网    | ✓       | 待填 |
| RAGFlow    | 不支持        | 内网    | ✗      |  仅文本 |
| DeepSeek   | 不支持        | 外网    | ✗      | 仅文本 |
| Generic    | 模板模式      | 外网    | ✗      | 不支持多模态 |

注意：
- A组供应商使用 Base64 内联方式，将图片编码为 base64 字符串直接嵌入请求
- B组供应商使用文件上传方式，需要先将图片上传到服务器获取 file_id
- C组供应商暂不支持图像输入，测试时会返回预期错误
*/
