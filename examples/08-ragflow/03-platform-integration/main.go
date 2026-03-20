package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

const (
	promptText        = "什么是RAG？"
	streamOutput bool = false
)

func main() {
	cli := client.New()
	cli.HTTP = &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	ctx := context.Background()

	fmt.Println("=== RAGFlow 平台集成示例（NewSessionWith） ===")
	fmt.Println("说明：RAGFlow 的 session_id 由服务端生成，SDK 会自动提取并保存。")
	fmt.Println("注意：chat_id 通过 ProviderConfig.ExtraBody 传入。")

	// 构造凭证（模拟平台从数据库获取）
	cred := &auth.Credential{
		ID:       "user-123-ragflow",
		Provider: "ragflow",
		AuthType: auth.AuthTypeAPIKey,
		APIKey:   "ragflow-TOKEN",
	}

	pc := &config.ProviderConfig{
		Name:    "ragflow",
		Type:    "ragflow",
		BaseURL: "http://ragflow.example.com",
		ExtraBody: map[string]any{
			"chat_id": "your-chat-assistant-id",
		},
	}

	// ========================================
	// 场景1：无SessionStore（最简单）
	// ========================================
	fmt.Println("场景1：无SessionStore")
	example1_NoStore(cli, ctx, cred, pc)

	// ========================================
	// 场景2：Memory SessionStore（内存审计）
	// ========================================
	fmt.Println("\n场景2：Memory SessionStore")
	example2_MemoryStore(cli, ctx, cred, pc)

	// ========================================
	// 场景3：File SessionStore
	// ========================================
	fmt.Println("\n场景3：File SessionStore")
	example3_FileStore(cli, ctx, cred, pc)

	// ========================================
	// 场景4：SQLite SessionStore
	// ========================================
	fmt.Println("\n场景4：SQLite SessionStore")
	example4_SQLiteStore(cli, ctx, cred, pc)
}

func example1_NoStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithHistoryMode(client.HistoryNone),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("session_id: %s (自动提取)\n", sess.ID())
}

func example2_MemoryStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store := sessionstore.NewMemory()

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("session_id: %s (已保存到Memory)\n", sess.ID())
}

func example3_FileStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store := sessionstore.NewFile(sessionstore.FileConfig{
		BaseDir: "/tmp/sessions",
	})

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("session_id: %s (已保存到 /tmp/sessions/)\n", sess.ID())
}

func example4_SQLiteStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store, err := sessionstore.NewSQLite(sessionstore.SQLiteConfig{
		DSN: "file:/tmp/sessions.db",
	})
	if err != nil {
		log.Printf("Error creating SQLite store: %v", err)
		return
	}
	defer func() { _ = store.Close() }()

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("session_id: %s (已保存到SQLite)\n", sess.ID())
}

func chat(ctx context.Context, sess *client.Session, req base.ChatRequest) (string, error) {
	if !streamOutput {
		resp, err := sess.Chat(ctx, req)
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	}

	stream, err := sess.ChatStream(ctx, req)
	if err != nil {
		return "", err
	}

	var fullText string
	for chunk := range stream {
		if chunk.Error != nil {
			return "", chunk.Error
		}
		if chunk.Text != "" {
			fmt.Print(chunk.Text)
			fullText += chunk.Text
		}
	}
	fmt.Println()
	return fullText, nil
}
