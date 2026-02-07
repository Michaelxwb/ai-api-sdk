package plugin

import (
	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

// NewSession creates a unified Session (Chat/ChatStream) backed by the browser plugin transport.
func NewSession(cfg Config, opts ...client.SessionOption) (*client.Session, error) {
	transport, err := NewPluginTransport(cfg)
	if err != nil {
		return nil, err
	}
	cli := client.New()
	cli.HTTP.Transport = transport

	pc := &config.ProviderConfig{
		Name: "plugin",
		Type: "plugin",
	}
	cred := &auth.Credential{
		Provider: "plugin",
		AuthType: auth.AuthTypeNone,
	}
	return cli.NewSessionWith(cred, pc, opts...), nil
}
