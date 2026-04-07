package client

import (
	"context"
	"errors"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

const defaultTestTimeout = 10 * time.Second

// TestOptions 连通性测试选项
type TestOptions struct {
	Model   string        // 必填：要测试的模型名称
	Timeout time.Duration // 可选：超时时间，默认 10s
	Prompt  string        // 可选：测试 prompt，默认 "1"
}

// TestResult 连通性测试结果
type TestResult struct {
	Latency  time.Duration     // 请求延迟
	Response base.ChatResponse // 原始响应
}

// TestWith 平台集成模式的连通性测试
// 通过发送最小化流式请求验证全链路：网络可达 → 凭证有效 → 模型可用
// 使用流式模式确保兼容所有 Provider（包括仅支持流式的 Coze 等）
func (c *Client) TestWith(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, opt *TestOptions) (TestResult, error) {
	optVal, err := normalizeTestOptions(opt)
	if err != nil {
		return TestResult{}, err
	}
	ctx, cancel := ensureTestContext(ctx, optVal.Timeout)
	if cancel != nil {
		defer cancel()
	}

	return c.testStream(ctx, optVal, func(req base.ChatRequest) (*Session, error) {
		sess := c.NewSessionWith(cred, pc, WithStore(nil), WithHistoryMode(HistoryNone))
		return sess, nil
	})
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

	return c.testStream(ctx, optVal, func(req base.ChatRequest) (*Session, error) {
		sess := c.NewSession(providerName, WithStore(nil), WithHistoryMode(HistoryNone))
		return sess, nil
	})
}

// testStream 统一的流式连通性测试实现。
// 使用流式模式兼容所有 Provider（部分 Provider 如 Coze 仅支持流式）。
func (c *Client) testStream(ctx context.Context, opt TestOptions, makeSession func(base.ChatRequest) (*Session, error)) (TestResult, error) {
	req := base.ChatRequest{
		Model:    opt.Model,
		Messages: []base.Message{{Role: "user", Content: opt.Prompt}},
		Stream:   true,
	}

	sess, err := makeSession(req)
	if err != nil {
		return TestResult{}, err
	}

	start := time.Now()
	ch, err := sess.ChatStream(ctx, req)
	if err != nil {
		return TestResult{Latency: time.Since(start)}, err
	}

	var text string
	var usage *base.Usage
	for chunk := range ch {
		if chunk.Error != nil {
			return TestResult{Latency: time.Since(start)}, chunk.Error
		}
		text += chunk.Text
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
	}

	return TestResult{
		Latency: time.Since(start),
		Response: base.ChatResponse{
			Text:  text,
			Usage: usage,
		},
	}, nil
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
