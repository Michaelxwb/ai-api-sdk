package test

import (
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/impls/generic"
)

func TestSessionFromInferred_StatusHandling(t *testing.T) {
	c := client.New()

	baseProfile := &generic.GenericProfile{
		Request: generic.RequestProfile{
			Method:       "POST",
			Path:         "/v1/chat/completions",
			BodyTemplate: map[string]any{"prompt": "{{input}}"},
		},
		Response: generic.ResponseProfile{
			TextPath: "content",
		},
		Conversation: generic.ConversationProfile{
			Mode: "remote_session",
		},
	}

	t.Run("pending_confirm creates session", func(t *testing.T) {
		inferred := &generic.InferredIntegration{
			Profile: baseProfile,
			Report:  &generic.InferenceReport{Status: "pending_confirm"},
			BaseURL: "https://mock.example.com",
		}

		sess, got, err := c.ExposeSessionFromInferred(inferred)
		if err != nil {
			t.Fatalf("sessionFromInferred() error = %v, want nil", err)
		}
		if sess == nil {
			t.Fatal("sessionFromInferred() session = nil, want non-nil")
		}
		if got != inferred {
			t.Fatal("sessionFromInferred() inferred pointer changed")
		}
	})

	t.Run("auto_confirmed creates session", func(t *testing.T) {
		inferred := &generic.InferredIntegration{
			Profile: baseProfile,
			Report:  &generic.InferenceReport{Status: "auto_confirmed"},
			BaseURL: "https://mock.example.com",
		}

		sess, _, err := c.ExposeSessionFromInferred(inferred)
		if err != nil {
			t.Fatalf("sessionFromInferred() error = %v, want nil", err)
		}
		if sess == nil {
			t.Fatal("sessionFromInferred() session = nil, want non-nil")
		}
	})

	t.Run("failed returns error", func(t *testing.T) {
		inferred := &generic.InferredIntegration{
			Profile: baseProfile,
			Report:  &generic.InferenceReport{Status: "failed"},
			BaseURL: "https://mock.example.com",
		}

		sess, _, err := c.ExposeSessionFromInferred(inferred)
		if err == nil {
			t.Fatal("sessionFromInferred() error = nil, want non-nil")
		}
		if sess != nil {
			t.Fatal("sessionFromInferred() session != nil, want nil")
		}
	})
}
