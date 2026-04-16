package bailian

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

// BailianAppSpec implements Bailian App Responses API (synchronous text chat).
type BailianAppSpec struct {
	name           string
	defaultBaseURL string
	path           string
}

func init() {
	base.Register("bailian_app", NewBailianAppSpec("bailian_app", ""))
}

func NewBailianAppSpec(name, baseURL string) *BailianAppSpec {
	return &BailianAppSpec{
		name:           name,
		defaultBaseURL: baseURL,
		path:           "/responses",
	}
}

func (s *BailianAppSpec) Name() string { return s.name }

func (s *BailianAppSpec) DefaultBaseURL() string { return s.defaultBaseURL }

func (s *BailianAppSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeBearerToken, auth.AuthTypeAPIKey, auth.AuthTypeNone, auth.AuthTypeOAuth}
}

func (s *BailianAppSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	if err := base.ErrResponseFormatUnsupported("bailian_app", req.ResponseFormat); err != nil {
		return nil, err
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(s.defaultBaseURL)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("bailian_app: missing BaseURL")
	}

	payload := map[string]any{
		"input":  toResponsesInput(req.Messages),
		"stream": req.Stream,
	}

	// For app endpoints, model can be omitted and use app-side defaults.
	// QuickSession.Test uses "test" as placeholder model when unset; ignore it.
	model := strings.TrimSpace(req.Model)
	if model != "" && !strings.EqualFold(model, "test") {
		payload["model"] = model
	}
	if req.Temperature != nil {
		payload["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		payload["max_tokens"] = *req.MaxTokens
	}
	for k, v := range opts.ExtraBody {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	path := s.path
	if strings.TrimSpace(opts.Path) != "" {
		path = opts.Path
	}
	url := joinEndpoint(baseURL, path)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *BailianAppSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("bailian_app: response is nil")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return base.ChatResponse{}, err
	}

	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("bailian_app: parse response failed: %w", err)
	}

	text := strings.TrimSpace(parsed.OutputText)
	if text == "" {
		var sb strings.Builder
		for _, item := range parsed.Output {
			for _, c := range item.Content {
				if strings.TrimSpace(c.Text) == "" {
					continue
				}
				// Keep text-like entries only.
				switch c.Type {
				case "", "output_text", "text":
					sb.WriteString(c.Text)
				}
			}
		}
		text = sb.String()
	}

	var usage *base.Usage
	if parsed.Usage.InputTokens > 0 || parsed.Usage.OutputTokens > 0 || parsed.Usage.TotalTokens > 0 {
		usage = &base.Usage{
			PromptTokens:     parsed.Usage.InputTokens,
			CompletionTokens: parsed.Usage.OutputTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
	}

	return base.ChatResponse{
		Text:  text,
		Usage: usage,
		Raw:   data,
	}, nil
}

func (s *BailianAppSpec) AuthStrategyOverride(cred *auth.Credential) (auth.AuthStrategy, bool) {
	if cred == nil {
		return auth.NoAuth{}, true
	}
	if cred.AuthType == auth.AuthTypeAPIKey && cred.APIKey != "" {
		return auth.BearerTokenStrategy{Token: cred.APIKey}, true
	}
	return nil, false
}

func toResponsesInput(messages []base.Message) []map[string]any {
	input := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "user"
		}
		item := map[string]any{
			"role":    role,
			"content": msg.Content,
		}
		if msg.Name != "" {
			item["name"] = msg.Name
		}
		if msg.ToolCallID != "" {
			item["tool_call_id"] = msg.ToolCallID
		}
		input = append(input, item)
	}
	return input
}

func joinEndpoint(baseURL, path string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path = strings.TrimSpace(path)
	if path == "" {
		return baseURL
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasSuffix(baseURL, path) {
		return baseURL
	}
	return baseURL + path
}
