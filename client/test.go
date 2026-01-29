package client

import (
	"context"
	"errors"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider"
)

const defaultTestTimeout = 10 * time.Second

// TestOptions 连通性测试选项
type TestOptions struct {
	Model     string        // 必填：要测试的模型名称
	Timeout   time.Duration // 可选：超时时间，默认 10s
	MaxTokens int           // 可选：最大 token 数，默认 1
	Prompt    string        // 可选：测试 prompt，默认 "1"
}

// TestResult 连通性测试结果
type TestResult struct {
	Latency  time.Duration         // 请求延迟
	Response provider.ChatResponse // 原始响应
}

// TestWith 平台集成模式的连通性测试
// 通过发送最小化 chat 请求验证全链路：网络可达 → 凭证有效 → 模型可用
func (c *Client) TestWith(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, opt *TestOptions) (TestResult, error) {
	optVal, err := normalizeTestOptions(opt)
	if err != nil {
		return TestResult{}, err
	}
	ctx, cancel := ensureTestContext(ctx, optVal.Timeout)
	if cancel != nil {
		defer cancel()
	}

	temp := float32(0)
	maxTokens := optVal.MaxTokens
	req := provider.ChatRequest{
		Model:       optVal.Model,
		Messages:    []provider.Message{{Role: "user", Content: optVal.Prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      false,
	}

	start := time.Now()
	resp, err := c.ChatWith(ctx, cred, pc, req)
	return TestResult{Latency: time.Since(start), Response: resp}, err
}

// Test 本地配置模式的连通性测试
func (c *Client) Test(ctx context.Context, providerName string, opt *TestOptions) (TestResult, error) {
	optVal, err := normalizeTestOptions(opt)
	if err != nil {
		return TestResult{}, err
	}
	ctx, cancel := ensureTestContext(ctx, optVal.Timeout)
	if cancel != nil {
		defer cancel()
	}

	temp := float32(0)
	maxTokens := optVal.MaxTokens
	req := provider.ChatRequest{
		Model:       optVal.Model,
		Messages:    []provider.Message{{Role: "user", Content: optVal.Prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		Stream:      false,
	}

	start := time.Now()
	resp, err := c.Chat(ctx, providerName, req)
	return TestResult{Latency: time.Since(start), Response: resp}, err
}

func normalizeTestOptions(opt *TestOptions) (TestOptions, error) {
	if opt == nil {
		return TestOptions{}, errors.New("client: missing test options")
	}
	if opt.Model == "" {
		return TestOptions{}, errors.New("client: missing model in test options")
	}

	out := *opt
	if out.Timeout <= 0 {
		out.Timeout = defaultTestTimeout
	}
	if out.MaxTokens <= 0 {
		out.MaxTokens = 1
	}
	if out.Prompt == "" {
		out.Prompt = "1"
	}
	return out, nil
}

func ensureTestContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, nil
	}
	if timeout <= 0 {
		timeout = defaultTestTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
