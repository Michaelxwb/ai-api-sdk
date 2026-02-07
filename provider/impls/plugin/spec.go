package plugin

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
	"github.com/Michaelxwb/ai-api-sdk/provider/streaming"
)

// PluginSpec implements plugin transport over a local HTTP shim.
type PluginSpec struct{}

func init() {
	base.Register("plugin", &PluginSpec{})
}

func (s *PluginSpec) Name() string { return "plugin" }

func (s *PluginSpec) DefaultBaseURL() string { return "http://plugin.local" }

func (s *PluginSpec) SupportedAuthTypes() []auth.AuthType {
	return []auth.AuthType{auth.AuthTypeNone}
}

func (s *PluginSpec) BuildRequest(ctx context.Context, opts base.BuildOptions, req base.ChatRequest) (*http.Request, error) {
	baseURL := opts.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = s.DefaultBaseURL()
	}
	path := opts.Path
	if strings.TrimSpace(path) == "" {
		path = "/chat"
	}

	payload := map[string]any{
		"messages": req.Messages,
		"stream":   req.Stream,
	}
	if req.SessionID != "" {
		payload["session_id"] = req.SessionID
	}
	if req.StartNewChat {
		payload["startNewChat"] = true
	}
	for k, v := range opts.ExtraBody {
		payload[k] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(baseURL, "/") + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (s *PluginSpec) ParseResponse(resp *http.Response) (base.ChatResponse, error) {
	if resp == nil {
		return base.ChatResponse{}, fmt.Errorf("plugin: response is nil")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return base.ChatResponse{}, err
	}
	var parsed struct {
		Text      string `json:"text"`
		SessionID string `json:"sessionId,omitempty"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return base.ChatResponse{}, fmt.Errorf("plugin: parse response failed: %w", err)
	}
	return base.ChatResponse{Text: parsed.Text, SessionID: parsed.SessionID, Raw: data}, nil
}

func (s *PluginSpec) AuthStrategyOverride(_ *auth.Credential) (auth.AuthStrategy, bool) {
	return auth.NoAuth{}, true
}

// ParseStreamResponse implements ProviderStreamSpec.
func (s *PluginSpec) ParseStreamResponse(resp *http.Response) (<-chan streaming.StreamChunk, error) {
	if resp == nil {
		return nil, fmt.Errorf("plugin: response is nil")
	}

	out := make(chan streaming.StreamChunk, 16)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()

		decoder := json.NewDecoder(resp.Body)
		for {
			var event struct {
				Text      string `json:"text"`
				SessionID string `json:"sessionId,omitempty"`
				Done      bool   `json:"done,omitempty"`
				Error     string `json:"error,omitempty"`
			}
			if err := decoder.Decode(&event); err != nil {
				if err == io.EOF {
					return
				}
				out <- streaming.StreamChunk{Error: err}
				return
			}
			if event.Error != "" {
				out <- streaming.StreamChunk{Error: fmt.Errorf("%s", event.Error), Done: true}
				return
			}
			out <- streaming.StreamChunk{Text: event.Text, SessionID: event.SessionID, Done: event.Done}
			if event.Done {
				return
			}
		}
	}()

	return out, nil
}

var _ base.ProviderSpec = (*PluginSpec)(nil)
var _ streaming.ProviderStreamSpec = (*PluginSpec)(nil)
