package self_developed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

// providerName 是该供应商的唯一标识符，用于 SDK 内部路由和注册。
const providerName = "self_developed"

// init 在包加载时自动注册到全局 ProviderSpec 表。
// 这样 Quick() 和 NewSession() 就能通过 "self_developed" 找到对应的实现。
func init() {
	base.Register(providerName, &MultiroundSpec{})
}

// MultiroundSpec 实现 base.ProviderSpec 接口。
// 适用于"自研多轮对话"场景：请求/响应直接透传，不做标准化转换。
type MultiroundSpec struct{}

// Name 返回供应商名称。
func (s *MultiroundSpec) Name() string { return providerName }

// DefaultBaseURL 返回默认 BaseURL。空字符串表示必须由调用方显式指定。
func (s *MultiroundSpec) DefaultBaseURL() string { return "" }

// SupportedAuthTypes 返回支持的认证类型。
// self_developed 使用 NoAuth，JWT 通过 Headers 外部传入。
func (s *MultiroundSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeNone}
}

// SupportsVision 返回是否支持视觉/图片输入。
func (s *MultiroundSpec) SupportsVision() bool { return false }

// BuildRequest 根据传入的 BuildOptions 和 ChatRequest 构建 HTTP 请求。
//
// 请求构建逻辑：
//   - URL = BaseURL + Path（默认 "/api/v1/chat"）
//   - 请求体直接使用 ExtraBody 序列化，不做消息格式转换
//   - 适用场景：后端 API 格式与 OpenAI 不兼容，需要透传自定义字段
func (s *MultiroundSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	// 1、拼接 URL：baseURL + path
	path := strings.TrimSpace(opts.Path)
	if path == "" {
		path = "/api/v1/chat"
	}
	baseURL := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	url := baseURL + path

	// 2、请求体：使用 ExtraBody（lg.exe 接口参数）
	var body []byte
	var err error
	if len(opts.ExtraBody) > 0 {
		body, err = json.Marshal(opts.ExtraBody)
		if err != nil {
			return nil, fmt.Errorf("self_developed: marshal ExtraBody: %w", err)
		}
	} else {
		body = []byte("{}")
	}

	// 3. 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	// 4. 设置 Headers
	httpReq.Header.Set("Content-Type", "application/json")

	// 5. 附加自定义 Headers（包含 JWT）
	for k, v := range opts.Headers {
		httpReq.Header.Set(k, v)
	}

	return httpReq, nil
}

// ParseResponse 解析 HTTP 响应。
//
// 响应解析逻辑：
//   - 读取响应体（限制 4MB，防止过大响应）
//   - 直接返回原始 JSON 字符串，不做结构化解析
//   - Text = Raw，调用方需自行解析响应内容
func (s *MultiroundSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("self_developed: response is nil")
	}

	// 1. 读取响应体，限制最大 4MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, fmt.Errorf("self_developed: read response: %w", err)
	}

	// 2. 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return base.ChatResponse{}, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	// 3. 直接返回原始 JSON，透传给调用方
	// 不解析 Answer/Message 等字段，保留原始响应结构
	return base.ChatResponse{
		Text: string(body), // 原始 JSON 字符串
		Raw:  body,         // 原始字节数组
	}, nil
}

// AuthStrategyOverride 覆盖默认认证策略。
//
// 认证策略说明：
//   - self_developed 不使用 SDK 内置认证（Bearer/APIKey/JWT Sign）
//   - JWT Token 通过 ProviderConfig.Headers 外部传入
//   - 返回 NoAuth{} 跳过 SDK 认证逻辑，避免重复添加认证头
func (s *MultiroundSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	// JWT token is passed via CustomHeaderStrategy (Headers), not via auth package.
	// Return NoAuth to skip SDK auth logic.
	return auth.NoAuth{}, true
}

// APIError 表示后端返回的 HTTP 错误（非 200 状态码）。
type APIError struct {
	StatusCode int    // HTTP 状态码
	Message    string // 响应体内容（通常是错误信息）
}

// Error 返回格式化错误字符串，便于日志输出。
func (e *APIError) Error() string {
	return fmt.Sprintf("self_developed: status %d: %s", e.StatusCode, e.Message)
}

var _ base.ProviderSpec = (*MultiroundSpec)(nil)
