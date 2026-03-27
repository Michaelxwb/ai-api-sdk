package test

import (
	"strings"
	"testing"

	"github.com/Michaelxwb/ai-api-sdk/client"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

func TestQuickValidation(t *testing.T) {
	cli := client.New()

	t.Run("unregistered provider returns error", func(t *testing.T) {
		_, err := cli.Quick(client.ProviderConfig{
			Provider:    "nonexistent-provider",
			BaseURL:     "https://example.com",
			SessionMode: "local_history",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not registered") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("fastgpt missing BaseURL returns error", func(t *testing.T) {
		_, err := cli.Quick(client.ProviderConfig{
			Provider:    "fastgpt",
			APIKey:      "token",
			SessionMode: "local_history",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "requires BaseURL") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("fastgpt missing SessionMode returns error", func(t *testing.T) {
		_, err := cli.Quick(client.ProviderConfig{
			Provider: "fastgpt",
			APIKey:   "token",
			BaseURL:  "https://api.fastgpt.in",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "requires explicit SessionMode") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("fastgpt with all required fields succeeds", func(t *testing.T) {
		qs, err := cli.Quick(client.ProviderConfig{
			Provider:    "fastgpt",
			APIKey:      "token",
			BaseURL:     "https://api.fastgpt.in",
			SessionMode: "local_history",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qs == nil {
			t.Fatal("expected non-nil QuickSession")
		}
	})

	t.Run("ragflow missing BaseURL returns error", func(t *testing.T) {
		_, err := cli.Quick(client.ProviderConfig{
			Provider: "ragflow",
			APIKey:   "token",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "requires BaseURL") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("openai does not require BaseURL or SessionMode", func(t *testing.T) {
		qs, err := cli.Quick(client.ProviderConfig{
			Provider: "openai",
			APIKey:   "token",
		})
		if err != nil {
			t.Fatalf("unexpected validation error for openai: %v", err)
		}
		if qs == nil {
			t.Fatal("expected non-nil QuickSession")
		}
	})

	t.Run("dify does not require SessionMode", func(t *testing.T) {
		qs, err := cli.Quick(client.ProviderConfig{
			Provider: "dify",
			APIKey:   "token",
			BaseURL:  "https://api.dify.ai/v1",
		})
		if err != nil {
			t.Fatalf("unexpected validation error for dify: %v", err)
		}
		if qs == nil {
			t.Fatal("expected non-nil QuickSession")
		}
	})
}
