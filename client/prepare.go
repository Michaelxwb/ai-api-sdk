package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
)

type preparedChatRequest struct {
	spec     base.ProviderSpec
	httpReq  *http.Request
	cred     *auth.Credential
	strategy auth.AuthStrategy
}

type resolvedChatInputs struct {
	pc       *config.ProviderConfig
	spec     base.ProviderSpec
	cred     *auth.Credential
	strategy auth.AuthStrategy
}

func (c *Client) prepareChatWithRequest(ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig, req base.ChatRequest) (*preparedChatRequest, error) {
	if pc == nil {
		return nil, errors.New("client: missing provider config")
	}
	specName := pc.Type
	if specName == "" {
		specName = pc.Name
	}
	spec, ok := base.Get(specName)
	if !ok {
		return nil, fmt.Errorf("client: provider spec %s not registered", specName)
	}
	baseURL := pc.BaseURL
	if baseURL == "" {
		baseURL = spec.DefaultBaseURL()
	}

	strategy := auth.NewStrategyFromCredential(cred)
	if cred != nil {
		if override, ok := spec.AuthStrategyOverride(cred); ok {
			strategy = override
		}
	}

	opts := base.BuildOptions{
		BaseURL:    baseURL,
		Path:       pc.Path,
		ExtraBody:  pc.ExtraBody,
		Headers:    pc.Headers,
		Credential: cred,
	}
	httpReq, err := spec.BuildRequest(ctx, opts, req)
	if err != nil {
		return nil, err
	}
	for k, v := range pc.Headers {
		httpReq.Header.Set(k, v)
	}

	return &preparedChatRequest{spec: spec, httpReq: httpReq, cred: cred, strategy: strategy}, nil
}

func (c *Client) resolveChatInputs(providerName string) (*resolvedChatInputs, error) {
	if c == nil || c.Config == nil {
		return nil, errors.New("client: missing config")
	}
	pc := c.Config.FindProvider(providerName)
	if pc == nil {
		return nil, fmt.Errorf("client: provider %s not configured", providerName)
	}
	specName := pc.Type
	if specName == "" {
		specName = pc.Name
	}
	spec, ok := base.Get(specName)
	if !ok {
		return nil, fmt.Errorf("client: provider spec %s not registered", specName)
	}

	var cred *auth.Credential
	var strategy auth.AuthStrategy
	if c.AuthMgr != nil {
		if pc.AuthRef != "" {
			var err error
			cred, err = c.AuthMgr.Get(pc.AuthRef)
			if err != nil {
				return nil, err
			}
			strategy = auth.NewStrategyFromCredential(cred)
		} else {
			var err error
			cred, strategy, err = c.AuthMgr.Resolve(spec.Name())
			if err != nil {
				return nil, err
			}
		}
	}
	if cred != nil {
		if override, ok := spec.AuthStrategyOverride(cred); ok {
			strategy = override
		}
	}

	return &resolvedChatInputs{pc: pc, spec: spec, cred: cred, strategy: strategy}, nil
}
