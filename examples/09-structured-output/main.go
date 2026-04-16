package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

// WeatherResult 用于演示 JSON 结构化输出
type WeatherResult struct {
	City      string  `json:"city"`
	TempC     float64 `json:"temp_celsius"`
	Condition string  `json:"condition"`
	Humidity  int     `json:"humidity_percent"`
}

func main() {
	cli := client.New()
	ctx := context.Background()

	//// 示例 1：使用 ResponseFormat 强制 JSON 输出
	//showResponseFormatExample(cli, ctx)

	//// 示例 2：使用 SystemPrompt 设置系统指令
	//showSystemPromptExample(cli, ctx)

	// 示例 3：结合 SystemPrompt + ResponseFormat
	showCombinedExample(cli, ctx)
}

// showResponseFormatExample 演示如何使用 ResponseFormat 强制模型输出 JSON 格式
func showResponseFormatExample(cli *client.Client, ctx context.Context) {
	fmt.Println("=== 示例 1：ResponseFormat 强制 JSON 输出 ===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-TOKEN",
		Model:    "deepseek-chat",
		// 关键配置：ResponseFormat 强制模型输出 JSON
		ResponseFormat: &base.ResponseFormat{
			Type: "json_object",
		},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	ch, err := qs.Send(ctx, []base.Message{
		{Role: "user", Content: `请以 JSON 格式返回北京今天的天气，包含 city、temp_celsius、condition、humidity_percent 四个字段。`},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	var result WeatherResult
	if err := printAndParseJSON(ch, &result); err != nil {
		log.Fatalf("Parse error: %v", err)
	}
	fmt.Printf("解析结果: %#v\n\n", result)
}

// showSystemPromptExample 演示如何使用 SystemPrompt 设置系统指令
func showSystemPromptExample(cli *client.Client, ctx context.Context) {
	fmt.Println("=== 示例 2：SystemPrompt 设置角色与行为约束 ===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-TOKEN",
		Model:    "deepseek-chat",
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// 通过在 messages 中添加 system 消息实现 SystemPrompt
	ch, err := qs.Send(ctx, []base.Message{
		{Role: "system", Content: "你是一位资深 Go 语言专家，回答时总是先用一句话概括要点，然后给出详细解释。"},
		{Role: "user", Content: "解释一下 Go 中的 goroutine 是什么？"},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	fmt.Print("回答: ")
	if err := printStream(ch); err != nil {
		log.Fatalf("Stream error: %v", err)
	}
	fmt.Println()
}

// showCombinedExample 演示 SystemPrompt + ResponseFormat 组合使用
func showCombinedExample(cli *client.Client, ctx context.Context) {
	fmt.Println("=== 示例 3：SystemPrompt + ResponseFormat 组合 ===")

	qs, err := cli.Quick(client.ProviderConfig{
		Provider: "deepseek",
		APIKey:   "sk-TOKEN",
		Model:    "deepseek-chat",
		// DeepSeek 仅支持 json_object，不支持 json_schema
		ResponseFormat: &base.ResponseFormat{
			Type: "json_object",
		},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// SystemPrompt + json_object 约束，schema 通过 prompt 描述
	ch, err := qs.Send(ctx, []base.Message{
		{Role: "system", Content: `你是一位严格的代码审查员，只输出 JSON，不要有任何额外解释。
输出格式必须严格遵循：{"score": <1-10整数>, "issues": ["问题1", ...], "suggestions": ["建议1", ...]}`},
		{Role: "user", Content: "请审查以下 Go 代码并返回评分和建议：\nfunc getUser(id int) string {\n    return db.Query(\"select * from users where id=\" + string(id))\n}"},
	})
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	var review struct {
		Score       int      `json:"score"`
		Issues      []string `json:"issues"`
		Suggestions []string `json:"suggestions"`
	}
	if err := printAndParseJSON(ch, &review); err != nil {
		log.Fatalf("Parse error: %v", err)
	}
	fmt.Printf("代码评分: %d/10\n", review.Score)
	fmt.Printf("发现问题: %v\n", review.Issues)
	fmt.Printf("改进建议: %v\n", review.Suggestions)
}

// printStream 打印流式输出
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

// printAndParseJSON 打印流式输出并尝试解析 JSON
func printAndParseJSON(ch <-chan streaming.StreamChunk, v any) error {
	var text string
	for chunk := range ch {
		if chunk.Error != nil {
			return chunk.Error
		}
		text += chunk.Text
	}

	fmt.Printf("原始响应: %s\n", text)

	if v != nil {
		// 尝试提取 JSON 对象（去掉可能的 markdown 代码块）
		jsonStr := extractJSON(text)
		if err := json.Unmarshal([]byte(jsonStr), v); err != nil {
			return fmt.Errorf("json parse error: %w", err)
		}
	}
	return nil
}

// extractJSON 从文本中提取 JSON 对象
func extractJSON(s string) string {
	// 尝试解析整个字符串
	if err := json.Unmarshal([]byte(s), new(any)); err == nil {
		return s
	}
	// 尝试提取 markdown 代码块中的 JSON
	const jsonBlockStart = "```json"
	const jsonBlockEnd = "```"
	if start := indexIgnoreCase(s, jsonBlockStart); start >= 0 {
		s = s[start+len(jsonBlockStart):]
	}
	if end := indexIgnoreCase(s, jsonBlockEnd); end >= 0 {
		s = s[:end]
	}
	return trimWhitespace(s)
}

func indexIgnoreCase(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func trimWhitespace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\r' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
